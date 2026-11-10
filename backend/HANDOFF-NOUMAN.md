# Hand-off: AI, queue and seed (Nouman)

Everything you need to plug `internal/ai`, `internal/tasks` and the seed data into the backend Abdullah built.
The **what** (models, Jev decisions, inline vs queue, Urdu speech) is in `docs/2-TRD.md`; this file is the **how**.

## 0. Start here

- **Branch from `main`** (the backend was merged in PR #1):
  ```bash
  git fetch origin && git switch -c nouman origin/main
  ```
- Your folders: `backend/internal/ai/`, `backend/internal/tasks/`, `backend/seed/`, `web/public/audio/`, `README.md`.
- In `backend/cmd/app/main.go` you may edit **only the three marked spots** in section 4. Everything else in `cmd/`, `internal/{platform,identity,workers,jobs,reviews,problems}` and `db/` is Abdullah's: ask him for changes.
- Never commit keys. `AI_GATEWAY_API_KEY` goes in the repo-root `.env` (gitignored).

## 1. What the backend already does for you

- Uploads are saved before your code runs. You get an **on-disk file path** (audio/photo), never a request.
- Your inline functions may fail or return nothing: the backend logs the error and the UI falls back to an icon picker. **Never block the user, never panic.**
- The API already **enqueues** every queue task (section 3). You only write handlers.
- Writing a result back through the services listed below saves it **and** pushes the WebSocket update to the right users. You never touch SQL or WebSockets.
- Rate limit (20 AI calls/min per user) is already on the inline endpoints.

## 2. Inline AI: two functions (the user is waiting on the screen)

### 2a. Onboarding answer: `workers.Deps.Extract`

`POST /api/workers/onboard` sends ONE spoken answer to ONE question. You read it.

```go
// backend/internal/workers/workers.go (already exists, do not change)
type OnboardProfile struct {
	Name     *string  // only set what you understood; leave the rest nil
	Trade    *string  // one of: electrician plumber ac_technician carpenter mechanic
	Skills   []string
	Area     *string  // e.g. "G-10", "Saddar"
	YearsExp *int
}
type Extracted struct {
	Profile    OnboardProfile
	Confidence float64 // 0..1; for field "trade" the UI shows the trade picker below 0.6
	Transcript string
}

// You implement:
func Extract(ctx context.Context, audioPath, field string) (workers.Extracted, error)
// field is the question that was asked: "name" | "trade" | "area" | "years_exp".
// The answer may contain other fields too ("Main Akram hoon, plumber") — fill whatever you understood.
```

The backend cleans your output before it reaches the user (invalid trade → dropped, years outside 0..80 → dropped, confidence clamped). Flow per TRD: STT → Jev/LLM to JSON → struct.

### 2b. Problem → trade: `problems.Deps.Classify`

`POST /api/problems`: the customer describes the problem by text, voice and/or photo (any combination).

```go
// backend/internal/problems/problems.go (already exists, do not change)
type Input struct { Text, AudioPath, PhotoPath string } // "" when not sent
type Classification struct {
	Trade           *string // nil if unsure
	TradeConfidence float64 // < 0.6 → the UI shows the trade picker
	Urgency         string  // "now" | "today" | "flexible" (anything else becomes "now")
	Description     string  // short clean description; empty → the customer's text is used
}

// You implement:
func Classify(ctx context.Context, in problems.Input) (problems.Classification, error)
```

The price range is attached by the backend; you don't compute it.

## 3. Queue tasks: four handlers (the user is not waiting)

Contract: `backend/internal/platform/queue/queue.go`. The API enqueues these; asynq retries a failed handler **3 times** with backoff, timeout 60 s. Return `nil` on success, an error to retry, or `fmt.Errorf("...: %w", asynq.SkipRetry)` for a permanent failure.

| Task type | Payload | When | Your work | Write back with |
|---|---|---|---|---|
| `message:transcribe` | `queue.MessagePayload{MessageID}` | voice note sent in chat | STT → Jev "contains a price?" → if yes LLM extracts the amount | `Jobs.SetTranscript(ctx, messageID, transcript, suggestedPrice *int)` |
| `message:speech` | `queue.MessagePayload{MessageID}` | **customer** typed a text message | LLM rewrites it as short Urdu script → TTS | `Jobs.SetMessageSpeech(ctx, messageID, bodyUr, speechURL)` |
| `job:brief` | `queue.JobPayload{JobID}` | new job requested | short Urdu summary (trade, area/description, urgency) → TTS | `Jobs.SetBrief(ctx, jobID, briefUr, audioURL)` |
| `review:analyze` | `queue.JobPayload{JobID}` | customer saved a review | transcribe review audio if any → Jev tags (`on_time fair_price clean_work polite abusive_or_spam`) → then a new 1–2 sentence trust summary from the worker's reviews | `Reviews.SetTags(ctx, jobID, tags)` then `Workers.SetTrustSummary(ctx, workerID, summary)` |

Reading the inputs (no auth needed in the worker process):

```go
msg, _ := Jobs.MessageByID(ctx, id)      // jobs.Message: Kind, Body *string, AudioURL *string, JobID, SenderID
job, _ := Jobs.JobRow(ctx, jobID)        // jobs.Row: Trade, Description, Urgency, Lat, Lng, WorkerID, CustomerID
rv,  _ := Reviews.ByJob(ctx, jobID)      // workers.Review: Rating, Body *string, AudioURL *string, Tags
all, _ := Reviews.ForWorker(ctx, workerID) // newest 20 reviews, for the trust summary
card, _ := Workers.Card(ctx, workerID, nil) // *workers.WorkerCard: Area, Trade, ... (nil if no profile)
path, _ := Media.Path(*msg.AudioURL)     // "/media/audio/x.webm" → file on disk
url,  _ := Media.SaveBytes("audio", ".mp3", ttsBytes) // store TTS output → "/media/audio/<random>.mp3"
```

Allowed audio extensions for `SaveBytes`: `.mp3 .wav .ogg .webm .m4a .mp4`.
If Urdu TTS is poor (TRD), write the text with an empty audio URL: `SetBrief(ctx, id, briefUr, "")` — text-only is fine.
`SetTags` drops unknown tags; an `abusive_or_spam` review is hidden from ratings and lists automatically.

## 4. Wiring in `backend/cmd/app/main.go` (the only three spots you edit)

```go
// (1) in newServices, workers.Deps:
Extract: ai.Extract,            // or a method on your ai client
// (2) in newServices, problems.Deps:
s.prb = problems.NewService(problems.Deps{Media: s.media, PriceRange: s.jbs.PriceRange, Classify: ai.Classify})
// (3) registerTasks:
func registerTasks(mux *asynq.ServeMux, svc *services) {
	tasks.Register(mux, tasks.Deps{Jobs: svc.jbs, Reviews: svc.rvs, Workers: svc.wks, Media: svc.media})
}
```

Define `tasks.Deps` / `tasks.Register` however you like in `internal/tasks`; `svc.jbs`, `svc.rvs`, `svc.wks` are `*jobs.Service`, `*reviews.Service`, `*workers.Service`, and `svc.rdb` is the Redis client (for the AI-result cache keyed by input hash, TRD caching section). Env vars are already passed to both containers: `AI_GATEWAY_API_KEY STT_MODEL STT_FALLBACK_MODEL LLM_MODEL LLM_FALLBACK_MODEL JEV_MODEL TTS_MODEL`.

## 5. Seed data: `backend/seed/seed.sql`

Plain SQL against `backend/db/schema.sql`. Make it **idempotent** (safe to run twice). Ids are generated, so look rows up by `firebase_uid`:

```sql
INSERT INTO users (firebase_uid, role, name, photo_url) VALUES
  ('seed-1', 'worker', 'Muhammad Akram', NULL),
  ('seed-c1', 'customer', 'Ayesha', NULL)
ON CONFLICT (firebase_uid) DO NOTHING;

INSERT INTO worker_profiles (user_id, trade, skills, years_exp, area, lat, lng, is_online, trust_summary)
SELECT id, 'plumber', '{leaks,geyser}', 12, 'G-10', 33.6844, 73.0145, true, 'Punctual; customers mention fair prices.'
FROM users WHERE firebase_uid = 'seed-1'
ON CONFLICT (user_id) DO NOTHING;
```

Rules so the demo works:
- `firebase_uid` = `seed-<n>` (nobody can log in as them). ~40 workers across all 5 trades, real Islamabad/Rawalpindi lat/lng (F-6 33.7294,73.0758 · G-10 33.6844,73.0145 · Saddar 33.5973,73.0479), most `is_online = true`.
- Seed jobs only as **`completed`** (with `agreed_price` and `completed_at`) or `cancelled`. A `confirmed`/`in_progress` job makes that worker unavailable.
- Price ranges need **≥ 3 completed jobs with `agreed_price` per trade**.
- Reviews: one per completed job (`reviews.job_id`), rating 1–5, `tags` from the five allowed tags. Ratings, jobs done and tags on worker cards are computed from these automatically.
- Seed intro/review audio: put files in `web/public/audio/seed/` and use URLs like `/audio/seed/akram.mp3`, or leave them `NULL`.
- Abdullah loads it on the server: `docker exec -i hackathon-postgres-1 psql -U roshan -d roshan < backend/seed/seed.sql`.

## 6. Running and testing

- **Unit tests** (no database needed): `cd backend && go test ./...`. Test your AI code against a fake gateway with `httptest.NewServer`; don't call the real gateway in tests.
- **Full stack locally** (needs Docker): put `AUTH_DEV=1` and your gateway key in the repo-root `.env`, then `docker compose up --build`. Act as any user without Firebase:
  ```bash
  curl -X POST localhost:8080/api/auth/session -H 'Authorization: Bearer dev:nouman-w' -d '{"role":"worker"}'
  curl -X POST localhost:8080/api/workers/onboard -H 'Authorization: Bearer dev:nouman-w' -F field=name -F audio=@clip.webm
  ```
  Queue tasks show up in asynqmon at http://localhost:8081.
- **Full suite including DB tests**: `backend/scripts/test-local.sh` (needs only Docker; throwaway Postgres + Redis). Without Docker, ask Abdullah to run `backend/scripts/test-on-server.sh` on your branch.
- **Live**: https://hackathon.taciturn2021.dpdns.org — Abdullah redeploys after merges.

## 7. Done means

- [ ] `Extract` and `Classify` wired; onboarding and problem screen work with real clips.
- [ ] All four task handlers registered; a voice note gets a transcript (and a suggested price when one is spoken), a customer's text and a new job get Urdu text + audio, a review gets tags and the worker's trust summary updates.
- [ ] Every AI call: 15 s timeout, fallback model, one retry on invalid JSON (TRD).
- [ ] Seed loads cleanly twice in a row; search near G-10 shows workers with ratings and price ranges.
- [ ] README: every AI service/model, STT A-vs-B table, AI coding assistants, what is seeded/mocked (incl. Firebase test phone numbers), known limitations.
