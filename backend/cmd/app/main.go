// Roshan Rozgar backend: one binary, two modes — `app api` and `app worker`.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/identity"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform/realtime"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/workers"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// csv splits a comma-separated env value, dropping blanks.
func csv(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func main() {
	mode := "api"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	ctx := context.Background()
	redisOpt := &redis.Options{Addr: env("REDIS_ADDR", "localhost:6379"), Password: os.Getenv("REDIS_PASSWORD")}

	switch mode {
	case "api":
		db, err := pgxpool.New(ctx, env("DATABASE_URL", "postgres://roshan:roshan@localhost:5432/roshan"))
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()
		rdb := redis.NewClient(redisOpt)
		pingRedis := func(ctx context.Context) error { return rdb.Ping(ctx).Err() }

		var verifier identity.Verifier
		if pid := os.Getenv("FIREBASE_PROJECT_ID"); pid != "" {
			if verifier, err = identity.NewFirebaseVerifier(ctx, pid, os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")); err != nil {
				log.Fatal(err)
			}
		}
		if os.Getenv("AUTH_DEV") == "1" {
			verifier = identity.DevVerifier(verifier)
		}
		if verifier == nil {
			log.Fatal("set FIREBASE_PROJECT_ID (or AUTH_DEV=1 for local testing)")
		}
		ids := identity.NewService(db, verifier)

		// Same-origin in production (nginx serves web and API together); extra origins come from CORS_ORIGINS.
		origins := csv(os.Getenv("CORS_ORIGINS"))
		hub := realtime.NewHub(append([]string{"localhost:*", "127.0.0.1:*"}, hostsOf(origins)...))
		go func() {
			if err := hub.Run(ctx, rdb); err != nil {
				log.Printf("realtime hub stopped: %v", err)
			}
		}()

		media := platform.Media{Dir: env("MEDIA_DIR", "./media")}
		mux := newMux(db.Ping, pingRedis, media.Dir)

		// AI endpoints: 20 calls a minute per user, to protect the gateway key.
		aiLimit := platform.RateLimit(rdb, 20, time.Minute, func(r *http.Request) string {
			if p, ok := platform.PrincipalFrom(r.Context()); ok {
				return fmt.Sprintf("ai:%d", p.UserID)
			}
			return ""
		})

		// Stats, Reviews, PriceRange and Extract are wired as the reviews, jobs and ai modules land.
		wks := workers.NewService(workers.Deps{DB: db, Media: media, Users: ids})
		wks.Routes(mux, ids.Middleware, aiLimit)

		ids.Routes(mux, func(ctx context.Context, userID int64) (any, error) {
			return wks.Card(ctx, userID, nil)
		})
		mux.Handle("GET /api/ws", hub.Handler(func(r *http.Request) (int64, error) {
			p, err := ids.Authenticate(r.Context(), platform.BearerToken(r))
			return p.UserID, err
		}))

		addr := ":" + env("PORT", "8080")
		log.Printf("api listening on %s", addr)
		log.Fatal(http.ListenAndServe(addr, platform.CORS(origins)(mux)))
	case "worker":
		srv := asynq.NewServer(asynq.RedisClientOpt{Addr: redisOpt.Addr, Password: redisOpt.Password}, asynq.Config{Concurrency: 5})
		mux := asynq.NewServeMux()
		// Queue handlers from internal/tasks get registered on mux here.
		log.Fatal(srv.Run(mux))
	default:
		log.Fatalf("unknown mode %q (use api or worker)", mode)
	}
}

// hostsOf turns origins like "https://a.example.com" into WebSocket origin patterns ("a.example.com").
func hostsOf(origins []string) []string {
	out := make([]string, 0, len(origins))
	for _, o := range origins {
		_, host, ok := strings.Cut(o, "://")
		if !ok {
			host = o
		}
		out = append(out, host)
	}
	return out
}

// newMux builds the base HTTP routes; module routes are mounted on it in main.
func newMux(pingDB, pingRedis func(context.Context) error, mediaDir string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		status := map[string]bool{"postgres": pingDB(ctx) == nil, "redis": pingRedis(ctx) == nil}
		w.Header().Set("Content-Type", "application/json")
		if !status["postgres"] || !status["redis"] {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(status)
	})
	// ponytail: media on a local volume; S3-compatible storage when the API runs on more than one host.
	mux.Handle("GET /media/", http.StripPrefix("/media/", http.FileServer(http.Dir(mediaDir))))
	return mux
}
