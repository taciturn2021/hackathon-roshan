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
	"github.com/taciturn2021/hackathon-roshan/backend/internal/jobs"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform/realtime"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/reviews"
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

		a := buildAPI(db, rdb, verifier, env("MEDIA_DIR", "./media"), csv(os.Getenv("CORS_ORIGINS")))
		go func() {
			if err := a.hub.Run(ctx, rdb); err != nil {
				log.Printf("realtime hub stopped: %v", err)
			}
		}()

		addr := ":" + env("PORT", "8080")
		log.Printf("api listening on %s", addr)
		log.Fatal(http.ListenAndServe(addr, a.handler))
	case "worker":
		srv := asynq.NewServer(asynq.RedisClientOpt{Addr: redisOpt.Addr, Password: redisOpt.Password}, asynq.Config{Concurrency: 5})
		mux := asynq.NewServeMux()
		// Queue handlers from internal/tasks get registered on mux here.
		log.Fatal(srv.Run(mux))
	default:
		log.Fatalf("unknown mode %q (use api or worker)", mode)
	}
}

// api is the assembled HTTP app. The services are exposed for tests and the queue wiring.
type api struct {
	handler http.Handler
	hub     *realtime.Hub
	ids     *identity.Service
	wks     *workers.Service
	jbs     *jobs.Service
	rvs     *reviews.Service
}

// buildAPI wires every module and mounts all routes. The caller runs hub.Run for cross-process events.
func buildAPI(db *pgxpool.Pool, rdb *redis.Client, verifier identity.Verifier, mediaDir string, origins []string) *api {
	a := &api{}
	media := platform.Media{Dir: mediaDir}
	mux := newMux(db.Ping, func(ctx context.Context) error { return rdb.Ping(ctx).Err() }, media.Dir)

	a.ids = identity.NewService(db, verifier)
	auth := a.ids.Middleware

	// AI endpoints: 20 calls a minute per user, to protect the gateway key.
	aiLimit := platform.RateLimit(rdb, 20, time.Minute, func(r *http.Request) string {
		if p, ok := platform.PrincipalFrom(r.Context()); ok {
			return fmt.Sprintf("ai:%d", p.UserID)
		}
		return ""
	})

	// Modules reference each other only through these closures, which run after all three services
	// exist (resolving the cycle workers → reviews/jobs → workers). Extract and the queue hooks
	// (OnJobCreated, OnVoiceMessage, OnReviewCreated) are wired when the ai and tasks packages land.
	a.wks = workers.NewService(workers.Deps{
		DB: db, Media: media, Users: a.ids,
		Stats: func(ctx context.Context, ids []int64) (map[int64]workers.Stats, error) {
			st, err := a.rvs.Stats(ctx, ids)
			if err != nil {
				return nil, err
			}
			done, err := a.jbs.JobsDone(ctx, ids)
			if err != nil {
				return nil, err
			}
			for id, n := range done {
				s := st[id]
				s.JobsDone = n
				st[id] = s
			}
			return st, nil
		},
		Reviews: func(ctx context.Context, id int64) ([]workers.Review, error) { return a.rvs.ForWorker(ctx, id) },
		PriceRange: func(ctx context.Context, trade string) (*workers.PriceRange, error) {
			return a.jbs.PriceRange(ctx, trade)
		},
	})
	a.jbs = jobs.NewService(jobs.Deps{
		DB: db, Media: media, Redis: rdb, Users: a.ids, Workers: a.wks,
		Reviewed: func(ctx context.Context, ids []int64) (map[int64]bool, error) { return a.rvs.Reviewed(ctx, ids) },
		Publish:  realtime.NewPublisher(rdb).Publish,
	})
	a.rvs = reviews.NewService(reviews.Deps{DB: db, Media: media, Redis: rdb, Jobs: a.jbs})

	a.ids.Routes(mux, func(ctx context.Context, userID int64) (any, error) { return a.wks.Card(ctx, userID, nil) })
	a.wks.Routes(mux, auth, aiLimit)
	a.jbs.Routes(mux, auth)
	a.rvs.Routes(mux, auth)

	// Same-origin in production; extra browser origins come from CORS_ORIGINS.
	a.hub = realtime.NewHub(append([]string{"localhost:*", "127.0.0.1:*"}, hostsOf(origins)...))
	mux.Handle("GET /api/ws", a.hub.Handler(func(r *http.Request) (int64, error) {
		p, err := a.ids.Authenticate(r.Context(), platform.BearerToken(r))
		return p.UserID, err
	}))

	a.handler = platform.CORS(origins)(mux)
	return a
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

// newMux builds the base HTTP routes; module routes are mounted on it in buildAPI.
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
