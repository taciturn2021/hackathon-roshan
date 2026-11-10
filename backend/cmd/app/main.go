// Roshan Rozgar backend: one binary, two modes — `app api` and `app worker`.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	mode := "api"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	redisAddr := env("REDIS_ADDR", "localhost:6379")

	switch mode {
	case "api":
		db, err := pgxpool.New(context.Background(), env("DATABASE_URL", "postgres://roshan:roshan@localhost:5432/roshan"))
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()
		rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
		pingRedis := func(ctx context.Context) error { return rdb.Ping(ctx).Err() }

		addr := ":" + env("PORT", "8080")
		log.Printf("api listening on %s", addr)
		log.Fatal(http.ListenAndServe(addr, newMux(db.Ping, pingRedis, env("MEDIA_DIR", "./media"))))
	case "worker":
		srv := asynq.NewServer(asynq.RedisClientOpt{Addr: redisAddr}, asynq.Config{Concurrency: 5})
		mux := asynq.NewServeMux()
		// Queue handlers from internal/tasks get registered on mux here.
		log.Fatal(srv.Run(mux))
	default:
		log.Fatalf("unknown mode %q (use api or worker)", mode)
	}
}

// newMux builds the HTTP routes. Module routes get mounted here as they are written.
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
