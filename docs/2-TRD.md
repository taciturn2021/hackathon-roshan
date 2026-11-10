# 2. Technical Requirements — Roshan Rozgar

Part of the six planning documents in `docs/`. Give all six to your AI coding assistant before asking it to build.

| Layer | Choice |
|---|---|
| Frontend | React + Vite + TypeScript + Tailwind |
| Backend | Go, stdlib `net/http`, one binary with two modes: `app api` and `app worker` |
| Database | Postgres 16, `pgx` + `sqlc` |
| Queue | `asynq` on Redis (dashboard: `asynqmon`) |
| Realtime | WebSocket (`coder/websocket`), fan-out through Redis pub/sub |
| Media | Audio/photo files on a Docker volume, served at `/media/*` |
| Auth | Firebase Auth: phone number + SMS code, Google sign-in, and email + password. The API verifies the Firebase ID token |
| AI | Vercel AI Gateway, key in `AI_GATEWAY_API_KEY` |
| Run | `docker compose up` → `web`, `api`, `worker`, `postgres`, `redis`, `asynqmon` |

**AI models**

| Task | Model | Called from |
|---|---|---|
| Speech-to-text | A `microsoft/mai-transcribe-2` vs B `google/gemini-3.5-transcribe` — Nouman tests both, winner goes in `STT_MODEL` | `POST https://ai-gateway.vercel.sh/v4/ai/transcription-model` |
| Transcript/photo → JSON, review summary | `anthropic/claude-haiku-5.5` | Gateway OpenAI-compatible chat endpoint |
| Every decision/classification | `typesafe-ai/jev` | Gateway decision HTTP API — read https://vercel.com/docs/ai-gateway/modalities/decision first |
| Urdu speech for the worker: job summaries and typed customer messages (text-to-speech) | Test `elevenlabs/eleven-v4` and `google/gemini-3.8-flash-tts` for Urdu; winner goes in `TTS_MODEL` | Gateway speech endpoint, from the queue |

All AI is plain HTTP from Go in `internal/ai`. No Python service: we call hosted models, we do not run any.

**Jev decisions** (if confidence < 0.6, the UI shows an icon picker instead of trusting it)
- Problem → trade (choice of 5 + `other`), urgency (`now` / `today` / `flexible`)
- Voice note contains a price? (yes/no) — only then is the LLM asked to extract the amount
- Review tags (yes/no each): `on_time`, `fair_price`, `clean_work`, `polite`; and `abusive_or_spam`

**Inline vs queue rule**
- **Inline** when the result is needed on the current screen: each onboarding answer (a short clip, sent in the background while the next question plays), problem classification.
- **Queue** when it is not: chat voice-note transcript + price detection, Urdu text and speech for job summaries and typed customer messages, review tags, profile trust summary. Result is pushed over WebSocket.

**AI pipeline: fixed steps, Jev as the router (no agent framework)**
- **Shape:** transcribe → Jev decides → the LLM runs only when Jev says it is needed. The steps are known in advance, so they are code, not an agent choosing tools.
- **Timeouts and retries:** 15s per AI call. Queued steps retry 3 times with backoff (asynq does this); inline steps retry once.
- **Fallback model per step:** the losing speech-to-text model backs up the winner; `openai/gpt-6-luna` backs up `claude-haiku-5.5`.
- **Validation:** LLM JSON is decoded into a Go struct; invalid output gets one retry, then counts as empty fields.
- **Human fallback:** low Jev confidence or an empty field never blocks the user; the UI shows an icon picker.
- **Onboarding is four short questions, not one long recording:** name, work, area, years. The web app plays a recorded question, uploads the answer and immediately plays the next question; the answer is processed meanwhile, so the worker never waits. The client merges each `OnboardAnswer.profile` into its draft and skips a question whose field already arrived. A field still missing is asked once more, then gets an icon picker (trade) or is left empty.
- **Speech for the worker is prepared ahead, played on demand:** when a job is created or a customer types a message, the queue rewrites the text into short Urdu script (LLM) and generates its audio. Both are stored, so tapping the speaker button plays at once. Nothing auto-plays.

**Caching (Redis, in `internal/platform`)**
- **Cached:** price range per trade (TTL 5 min); worker rating/jobs-done aggregates (deleted when a review is saved); AI results keyed by a hash of the input, so a retry never pays twice.
- **Never cached:** worker availability and search results. A stale "free" worker breaks the booking guarantee, so those always read Postgres.
- **Rate limit:** per-user counter on the AI endpoints (`INCR` + `EXPIRE`), 20 calls a minute, to protect the gateway key.

**Queue and streaming**
- `asynq` is the Go equivalent of Celery: Redis-backed, retries, dashboard. Celery itself is Python-only.
- No Kafka. Nothing here needs replay or multi-consumer event logs; Redis pub/sub + refetch-on-reconnect covers delivery. Upgrade path: publish job events to a Kafka-compatible log (Redpanda) when a second consumer such as analytics appears.

**Auth (Firebase)**
- **Web:** Firebase JS SDK. `signInWithPhoneNumber(auth, '+92…', new RecaptchaVerifier(auth, el, {size: 'invisible'}))` then `confirmationResult.confirm(code)` for phone; `signInWithPopup(GoogleAuthProvider)` for Google; `signInWithEmailAndPassword` / `createUserWithEmailAndPassword` for password. Call `getIdToken()` before every request (the SDK refreshes it; tokens last one hour).
- **API:** `identity` verifies the ID token with the Firebase Admin Go SDK (`firebase.google.com/go/v4`, `VerifyIDToken`) using `FIREBASE_PROJECT_ID`, then maps `uid` → `users.firebase_uid`. Name and picture (Google) or the phone number (phone sign-in, stored in `users.phone`) come from the token on first sign-in. All three methods give the same ID token, so the API does not care which was used.
- **Workers: phone number + code on the number pad, or one-tap Google.** Digits only, no reading, so outcome 1 still holds. Customers: phone, Google, or email + password.
- **SMS limit:** the Firebase project sends at most 10 SMS a day. The demo worker and customer use **Firebase test phone numbers** (console → Authentication → Sign-in method → Phone → test numbers, fixed code, no SMS sent); Google and email cover everyone else when the quota is used up. Say this in the README.
- **Setup (once, in the Firebase console):** create the project, enable the Phone, Google and Email/Password providers, add the test phone numbers, copy the web config into `.env`. For the phone demo, add the tunnel's domain under Authentication → Settings → Authorized domains, or Google sign-in will be refused.

**Module rule (modular monolith)**
- `internal/{identity,workers,jobs,reviews,ai}`; each exposes one `Service`. Modules talk only through `Service` methods.
- Each module owns its tables. One documented exception: the view `available_workers` reads `jobs`.
- `internal/platform` holds db, redis, WebSocket hub, auth middleware, media upload. `internal/tasks` holds asynq handlers.

**Deliberate shortcuts (say these in Q&A; mark each in code with a `ponytail:` comment)**
- Media on a local volume. S3-compatible storage when the API runs on more than one host.
- Haversine in SQL. PostGIS index when workers exceed ~100k.
- Price range is per trade only. Per job-type when there is enough data.
- Seeded workers have no Firebase account (`firebase_uid` = `seed-<n>`), so nobody can log in as them. The demo worker and customer are real Firebase accounts on test phone numbers.
- Phone sign-in is capped at 10 SMS a day on our Firebase plan; demo accounts use Firebase test numbers. A paid plan or another SMS provider when there are real users.

---

## API contract

Base `/api`. JSON unless marked multipart. Auth: `Authorization: Bearer <Firebase ID token>` on every endpoint.
Errors: `{ "error": "code", "message": "..." }` with 400 / 401 / 403 / 404 / 409.

```ts
type Trade = 'electrician' | 'plumber' | 'ac_technician' | 'carpenter' | 'mechanic';
type Urgency = 'now' | 'today' | 'flexible';
type JobStatus = 'requested' | 'confirmed' | 'in_progress' | 'completed' | 'cancelled';

type User = { id: number; email: string | null; phone: string | null; role: 'worker' | 'customer'; name: string | null; photo_url: string | null };

type OnboardField = 'name' | 'trade' | 'area' | 'years_exp';
type OnboardAnswer = {                // AI reading of one spoken answer; nothing is saved yet
  field: OnboardField;                // the question that was asked
  profile: { name?: string; trade?: Trade; skills?: string[]; area?: string; years_exp?: number };
                                      // whatever was understood; may include other fields, empty if nothing
  confidence: number; transcript: string; audio_url: string;
};

type WorkerCard = {
  id: number; name: string; trade: Trade; area: string | null; years_exp: number | null;
  photo_url: string | null; intro_audio_url: string | null;
  is_online: boolean; available: boolean; distance_km: number | null;
  rating_avg: number | null; jobs_done: number; trust_summary: string | null; tags: string[];
};
type PriceRange = { low: number; high: number; sample: number } | null;
type Review = { job_id: number; rating: number; body: string | null; audio_url: string | null; tags: string[]; created_at: string };
type WorkerDetail = WorkerCard & { reviews: Review[]; price_range: PriceRange };

type Problem = { trade: Trade | null; trade_confidence: number; urgency: Urgency;
                 description: string; photo_url: string | null; price_range: PriceRange };

type Job = { id: number; customer: User; worker: WorkerCard; trade: Trade; description: string;
             urgency: Urgency; photo_url: string | null; status: JobStatus;
             agreed_price: number | null;
             brief_ur: string | null; brief_audio_url: string | null;   // short Urdu summary + its audio; arrive via job.updated
             created_at: string; completed_at: string | null; reviewed: boolean };
type Message = { id: number; job_id: number; sender_id: number; kind: 'text' | 'voice' | 'offer';
                 body: string | null; audio_url: string | null;
                 body_ur: string | null; speech_url: string | null;   // typed text in Urdu script + its audio, for the worker
                 suggested_price: number | null;
                 offer_price: number | null; offer_status: 'pending' | 'accepted' | 'rejected' | null;
                 created_at: string };
```

| Method | Path | Body | Returns |
|---|---|---|---|
| POST | `/auth/session` | `{role}` | `{user: User, worker: WorkerCard \| null}` — call once after every Firebase sign-in. Creates the user on first call; `role` is ignored for an existing user |
| GET | `/me` | — | `{user: User, worker: WorkerCard \| null}` |
| POST | `/workers/onboard` | multipart `audio`, `field` (`name` \| `trade` \| `area` \| `years_exp`) | `OnboardAnswer` for that one spoken answer (inline AI). Saves nothing; the client merges answers and then calls `PUT /workers/me`, using the `trade` answer's `audio_url` as `intro_audio_url` |
| PUT | `/workers/me` | `{name, trade, skills, years_exp, area, lat, lng, intro_audio_url}` | `WorkerCard` |
| PUT | `/workers/me/online` | `{is_online}` | `WorkerCard` |
| GET | `/workers?trade=&lat=&lng=&radius_km=5` | — | `WorkerCard[]`, available only, nearest first |
| GET | `/workers/{id}` | — | `WorkerDetail` |
| POST | `/problems` | multipart: any of `audio`, `photo`, `text` | `Problem` (inline AI) |
| POST | `/jobs` | `{worker_id, trade, description, urgency, lat, lng, photo_url?}` | `Job` |
| GET | `/jobs` | — | `Job[]` (mine, newest first) |
| GET | `/jobs/{id}` | — | `{job: Job, messages: Message[]}` |
| POST | `/jobs/{id}/messages` | JSON `{kind:'text', body}` or `{kind:'offer', offer_price}`; multipart `audio` for voice | `Message` (voice returns at once with `body: null`) |
| POST | `/jobs/{id}/offers/{message_id}/accept` | — | `Job` (now `confirmed`), or **409 `worker_busy`** |
| POST | `/jobs/{id}/offers/{message_id}/reject` | — | `Message` |
| POST | `/jobs/{id}/status` | `{status: 'in_progress' \| 'completed' \| 'cancelled'}` | `Job` |
| POST | `/jobs/{id}/review` | multipart: `rating`, optional `audio`, optional `body` | `Review` |

**WebSocket** `GET /api/ws?token=<Firebase ID token>` — server to client only; all writes go through REST.
After any reconnect the client refetches `GET /jobs/{id}`.

```ts
type Event =
  | { type: 'job.created';     data: Job }       // to the worker
  | { type: 'job.updated';     data: Job }       // to both parties
  | { type: 'message.created'; data: Message }   // to both parties
  | { type: 'message.updated'; data: Message };  // transcript, suggested_price, body_ur or speech_url filled in
```

**Contract changes:** edit this section and `web/src/contract.ts` in the same PR, tag Humza. Nobody changes a shape silently.
