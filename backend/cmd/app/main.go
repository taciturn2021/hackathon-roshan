// Roshan Rozgar backend: one binary, two modes — `app api` and `app worker`.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/ai"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/identity"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/jobs"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform/queue"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform/realtime"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/problems"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/reviews"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/tasks"
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
	asynqOpt := asynq.RedisClientOpt{Addr: redisOpt.Addr, Password: redisOpt.Password}

	db, err := pgxpool.New(ctx, env("DATABASE_URL", "postgres://roshan:roshan@localhost:5432/roshan"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	rdb := redis.NewClient(redisOpt)
	q := queue.NewClient(asynqOpt)
	defer q.Close()
	mediaDir := env("MEDIA_DIR", "./media")

	switch mode {
	case "api":
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

		a := buildAPI(newServices(db, rdb, verifier, mediaDir, q), csv(os.Getenv("CORS_ORIGINS")))
		go func() {
			if err := a.hub.Run(ctx, rdb); err != nil {
				log.Printf("realtime hub stopped: %v", err)
			}
		}()

		addr := ":" + env("PORT", "8080")
		log.Printf("api listening on %s", addr)
		log.Fatal(http.ListenAndServe(addr, a.handler))
	case "worker":
		// The worker uses the same services as the api, so task results go through the same
		// validation and push the same WebSocket events (via Redis to the api's hub).
		svc := newServices(db, rdb, nil, mediaDir, q)
		mux := asynq.NewServeMux()
		registerTasks(mux, svc)
		if svc.ai == nil {
			select {} // no handlers: a running server would fail every task and archive it; leave them queued
		}
		srv := asynq.NewServer(asynqOpt, asynq.Config{Concurrency: 5})
		log.Fatal(srv.Run(mux))
	default:
		log.Fatalf("unknown mode %q (use api or worker)", mode)
	}
}

// registerTasks mounts the internal/tasks handlers (owner: Nouman). Contract: internal/platform/queue.
// Each handler reads its payload (queue.MessagePayload / queue.JobPayload), does the AI work and writes
// results back with svc.jbs.SetTranscript, svc.jbs.SetMessageSpeech, svc.jbs.SetBrief, svc.rvs.SetTags,
// svc.wks.SetTrustSummary (each persists and pushes the WebSocket update).
func registerTasks(mux *asynq.ServeMux, svc *services) {
	if svc.ai == nil {
		log.Print("WARNING: AI_GATEWAY_API_KEY is not set; AI tasks stay queued until a worker with a key runs")
		return
	}
	tasks.Register(mux, tasks.Deps{Jobs: svc.jbs, Reviews: svc.rvs, Workers: svc.wks, Media: svc.media, AI: svc.ai})
}

// services are the module services, shared by the api and worker modes.
type services struct {
	db    *pgxpool.Pool
	rdb   *redis.Client
	q     *queue.Client
	media platform.Media
	ids   *identity.Service
	wks   *workers.Service
	jbs   *jobs.Service
	rvs   *reviews.Service
	prb   *problems.Service
	ai    *ai.Service // nil without AI_GATEWAY_API_KEY: inline AI is off and the UI shows pickers
}

// newServices wires the modules together. verifier may be nil in worker mode (no HTTP auth there).
// Modules reference each other only through closures that run after all services exist (resolving the
// cycle workers → reviews/jobs → workers). Enqueue failures are logged, never shown to the user: the
// write they follow has already succeeded. Extract (onboarding AI) is wired when internal/ai lands.
func newServices(db *pgxpool.Pool, rdb *redis.Client, verifier identity.Verifier, mediaDir string, q *queue.Client) *services {
	s := &services{db: db, rdb: rdb, q: q, media: platform.Media{Dir: mediaDir}}
	if os.Getenv("AI_GATEWAY_API_KEY") != "" {
		s.ai = ai.FromEnv(rdb)
	}
	enqueue := func(what string, fn func(context.Context, int64) error) func(context.Context, int64) {
		return func(ctx context.Context, id int64) {
			if err := fn(context.WithoutCancel(ctx), id); err != nil {
				log.Printf("enqueue %s %d: %v", what, id, err)
			}
		}
	}

	s.ids = identity.NewService(db, verifier)
	s.wks = workers.NewService(workers.Deps{
		DB: db, Media: s.media, Users: s.ids,
		Stats: func(ctx context.Context, ids []int64) (map[int64]workers.Stats, error) {
			st, err := s.rvs.Stats(ctx, ids)
			if err != nil {
				return nil, err
			}
			done, err := s.jbs.JobsDone(ctx, ids)
			if err != nil {
				return nil, err
			}
			for id, n := range done {
				w := st[id]
				w.JobsDone = n
				st[id] = w
			}
			return st, nil
		},
		Reviews: func(ctx context.Context, id int64) ([]workers.Review, error) { return s.rvs.ForWorker(ctx, id) },
		PriceRange: func(ctx context.Context, trade string) (*workers.PriceRange, error) {
			return s.jbs.PriceRange(ctx, trade)
		},
		Extract: s.extract(),
	})
	s.jbs = jobs.NewService(jobs.Deps{
		DB: db, Media: s.media, Redis: rdb, Users: s.ids, Workers: s.wks,
		Reviewed:       func(ctx context.Context, ids []int64) (map[int64]bool, error) { return s.rvs.Reviewed(ctx, ids) },
		Publish:        realtime.NewPublisher(rdb).Publish,
		OnJobCreated:   enqueue("job brief", q.JobBrief),
		OnVoiceMessage: enqueue("transcribe", q.Transcribe),
		OnTextMessage:  enqueue("speech", q.Speech),
	})
	s.rvs = reviews.NewService(reviews.Deps{
		DB: db, Media: s.media, Redis: rdb, Jobs: s.jbs,
		OnReviewCreated: enqueue("review analyze", q.ReviewAnalyze),
	})
	pd := problems.Deps{Media: s.media, PriceRange: s.jbs.PriceRange}
	if s.ai != nil {
		pd.Classify = s.ai.Classify
	}
	s.prb = problems.NewService(pd)
	return s
}

// api is the assembled HTTP app.
type api struct {
	*services
	handler http.Handler
	hub     *realtime.Hub
}

// buildAPI mounts every route. The caller runs hub.Run for cross-process events.
func buildAPI(svc *services, origins []string) *api {
	a := &api{services: svc}
	db, rdb := svc.db, svc.rdb
	mux := newMux(db.Ping, func(ctx context.Context) error { return rdb.Ping(ctx).Err() }, svc.media.Dir)

	// Rate limits (Redis token buckets, shared by all api instances; Redis down = allowed).
	limit := func(name string, n int, per time.Duration, key func(*http.Request) string) func(http.Handler) http.Handler {
		return platform.RateLimit(rdb, name, platform.Limit{N: n, Per: per}, key)
	}
	// Plain API calls get generous limits that only stop floods and scripts; the tight limits are on
	// the routes that spend AI gateway money.
	// Per IP on everything. A whole venue can share one IP.
	perIP := limit("ip", 1200, time.Minute, platform.ClientIP)
	// Signing in verifies a token and may create a user: per IP.
	sessions := limit("session", 120, time.Minute, func(r *http.Request) string {
		if r.URL.Path == "/api/auth/session" {
			return platform.ClientIP(r)
		}
		return ""
	})
	// Per user, after auth (r.Pattern is the matched route).
	writes := limit("writes", 300, time.Minute, platform.WritesOnly(platform.UserKey))
	route := func(pattern string) func(*http.Request) string {
		return func(r *http.Request) string {
			if r.Pattern == pattern {
				return platform.UserKey(r)
			}
			return ""
		}
	}
	// AI-backed writes: a chat message queues STT + Jev + LLM (voice) or LLM + TTS (customer text);
	// a new job queues LLM + TTS for the brief.
	chat := limit("chat", 30, time.Minute, route("POST /api/jobs/{id}/messages"))
	newJobs := limit("jobs", 50, 10*time.Minute, route("POST /api/jobs"))
	// Inline AI endpoints (onboarding answer, problem classification).
	aiLimit := limit("ai", 20, time.Minute, platform.UserKey)
	auth := func(h http.Handler) http.Handler { return svc.ids.Middleware(writes(chat(newJobs(h)))) }

	svc.ids.Routes(mux, func(ctx context.Context, userID int64) (any, error) { return svc.wks.Card(ctx, userID, nil) })
	svc.wks.Routes(mux, auth, aiLimit)
	svc.jbs.Routes(mux, auth)
	svc.rvs.Routes(mux, auth)
	svc.prb.Routes(mux, auth, aiLimit)

	// Same-origin in production; extra browser origins come from CORS_ORIGINS.
	a.hub = realtime.NewHub(append([]string{"localhost:*", "127.0.0.1:*"}, hostsOf(origins)...))
	mux.Handle("GET /api/ws", a.hub.Handler(func(r *http.Request) (int64, error) {
		p, err := svc.ids.Authenticate(r.Context(), platform.BearerToken(r))
		return p.UserID, err
	}))

	a.handler = platform.CORS(origins)(perIP(sessions(mux)))
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

// extract is the onboarding AI, or nil (no AI: the UI asks again or shows the picker).
func (s *services) extract() func(ctx context.Context, audioPath, field string) (workers.Extracted, error) {
	if s.ai == nil {
		return nil
	}
	return s.ai.Extract
}
