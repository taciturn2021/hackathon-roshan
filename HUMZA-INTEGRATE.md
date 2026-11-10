# Integration notes (Humza)

What you need to wire the frontend to the backend that is **not** already in `docs/2-TRD.md` (API contract) and `web/src/contract.ts`.
The contract there is complete and current: every route, shape and WebSocket event is implemented exactly as written. This file covers behaviour, errors, limits, AI timing, seed data and deploy.

## 1. Where things run

| What | URL |
|---|---|
| Web (nginx, serves the SPA and proxies `/api`, `/api/ws`, `/media`) | https://hackathon.taciturn2021.dpdns.org |
| API directly | https://hackathon-api.taciturn2021.dpdns.org/api/healthz |
| Local | web http://localhost:5173 · api http://localhost:8080 · asynqmon http://localhost:8081 |

- In production the SPA calls the API **same-origin** (`/api/...`), so no CORS setup is needed. Extra origins go in `CORS_ORIGINS`.
- Media URLs the API returns (`/media/audio/...`, `/media/photo/...`) are relative: use them as-is in `<audio src>` / `<img src>`.
- Seed audio (if any) lives under `web/public/audio/seed/` and is referenced as `/audio/seed/...`.

## 2. Auth flow (frontend side)

1. Sign in with Firebase: **phone OTP**, Google, or email/password.
2. Call `POST /api/auth/session {role}` once after every sign-in, with `Authorization: Bearer <Firebase ID token>`.
3. Every other call sends the same header. The WebSocket takes the token as `?token=`.
4. `401 no_session` from any route means step 2 was skipped for this user. Call it, then retry.
5. Firebase ID tokens expire after an hour. Use `getIdToken()` before each call (it refreshes), not a cached string. Reconnect the WebSocket with a fresh token.

**Demo login (Firebase test number, no SMS sent):** phone `+92 300 0000001`, code `123456`.
Use it instead of real numbers: Firebase allows only 10 real SMS a day.

Local without Firebase: `AUTH_DEV=1` in the repo-root `.env`, then `Authorization: Bearer dev:<any-name>`. Never set it on the server.

## 3. Errors

The body is always `{"error": "<code>", "message": "..."}`. Show `message` if you have nothing better.

| Status | `error` | When / what the UI should do |
|---|---|---|
| 400 | `bad_request` | Bad input; `message` says what. An upload over 20 MB or a wrong file type also comes back as 400 (`invalid upload`) |
| 401 | `unauthorized` | Missing or expired token: refresh the token, else sign in again |
| 401 | `no_session` | Call `POST /api/auth/session` first |
| 403 | `forbidden`, `worker_only`, `own_offer` | Wrong role, or the sender tried to accept or reject their own offer |
| 404 | `not_found` | Also returned for other people's jobs |
| 409 | `worker_busy` | Accept failed: the worker took another job. Tell the customer and offer other workers |
| 409 | `offer_not_pending` | Offer was already answered: refetch the job |
| 409 | `invalid_status`, `job_closed`, `job_not_completed`, `already_reviewed` | State moved on: refetch |
| 429 | `rate_limited` | Too many requests. A `Retry-After` header (seconds) is set: wait, then retry once |

## 4. Rate limits

Redis token buckets: the full number is available as a burst, then it refills evenly. Normal use never hits them.

| Scope | Limit |
|---|---|
| Per IP, everything | 1200 / min |
| `POST /api/auth/session`, per IP | 120 / min |
| Writes per user (offers, accept/reject, status, profile, online, review) | 300 / min |
| `POST /api/jobs/{id}/messages`, per user (AI-backed) | 30 / min |
| `POST /api/jobs`, per user (AI-backed) | 50 / 10 min |
| `POST /api/workers/onboard` and `POST /api/problems`, per user (inline AI) | 20 / min |
| All AI gateway calls, whole app | 3000 / day (`AI_DAILY_CAP`); after that the AI fields just stay empty |

## 5. Uploads

- Multipart field names: `audio`, `photo`, `text`, `field`, `rating`, `body` (see the TRD table per route).
- **Record audio as WAV.** The main speech-to-text model (MAI) handles WAV; webm only works through the fallback model. Accepted: `.wav .webm .ogg .mp3 .m4a .mp4`.
- Photos: `.jpg .jpeg .png .webp`. Max 20 MB per request.
- The type is taken from the file extension, else from the part's `Content-Type`. Give the Blob a filename, e.g. `form.append('audio', blob, 'answer.wav')`.

## 6. AI behaviour the UI must handle

**Inline:** `POST /workers/onboard` and `POST /problems`. Expect about 2–6 s.

- **Onboarding: no spinner** (`docs/4-UI_UX_BRIEF.md`). When the worker releases the mic, show the tick and play the next question at once; send the answer in the background and merge the result when it returns. Wait for outstanding answers only before the profile confirm screen, and handle unclear ones there (ask once more, or the trade picker).
- **Describe problem: the customer waits**, because the result card is the next thing on screen. Show progress on the send button and keep the five trade icons tappable underneath, so the customer can pick a trade and move on without waiting.
- Onboarding returns `OnboardAnswer` for one spoken answer.
  - `transcript === ""` means nothing was heard: ask again.
  - For the `trade` question, `confidence < 0.6` means show the trade icon picker, with `profile.trade` preselected if it is set.
  - For other questions, `confidence` is 0.9 when that field was understood and 0 when not.
  - The answer can include other fields too ("Main Akram hoon, plumber"): merge everything you get.
  - Name and area come back in Latin letters; sectors are normalised (`G-10`).
- `POST /problems` returns a `Problem`.
  - `trade === null` or `trade_confidence < 0.6` means show the trade picker.
  - `description` is a short English sentence the AI wrote, or the customer's own text when AI is off.
  - `price_range` can be `null` when there is no history for that trade.
- If the AI fails or is off, these routes still return 200 with empty fields. Never block on them; fall back to the pickers.

**Queued (results arrive later over the WebSocket; render the nulls first):**

| Trigger | Field that fills in | Event | Typical delay |
|---|---|---|---|
| Voice message | `Message.body` (transcript), and `suggested_price` when a price was spoken (show it as a one-tap "make an offer") | `message.updated` | 3–8 s |
| Customer's text message | `Message.body_ur` + `speech_url`: show the speaker button **to the worker only** | `message.updated` | 3–8 s |
| New job | `Job.brief_ur` + `brief_audio_url` on the worker's job alert | `job.updated` | 3–8 s |
| Review saved | Review `tags` and the worker's `trust_summary` | **no event**: they show on the next `GET /workers/{id}` | 5–10 s |

- An audio URL can stay empty while the text is set (text-to-speech failed): hide the speaker button when the URL is empty.
- A review tagged `abusive_or_spam` is hidden from lists and ratings automatically.
- After a WebSocket reconnect, refetch `GET /jobs/{id}` (TRD), because events sent while disconnected are lost.

## 7. Seed data (loaded on prod)

- 40 workers, 8 per trade, across Islamabad and Rawalpindi; 34 are online. At least 4 of every trade are within 4 km of **G-10 (33.6844, 73.0145)**, so use G-10 as the demo location.
- 90 completed jobs with reviews, so cards have ratings, `jobs_done`, tags and trust summaries, and every trade has a price range.
- Seed accounts are `seed-*` and nobody can sign in as them. To demo the worker side, sign up a new worker with the test number.
- Local stack: the seed is not loaded automatically. Load it once:
  ```bash
  docker compose exec -T postgres psql -U roshan -d roshan < backend/seed/seed.sql
  ```
  It is safe to run twice.

## 8. Deploy (server)

- The server clone is at `~/hackathon` on `oracle`. Secrets are in `~/hackathon/deploy/.env` (Firebase, AI gateway key and models); they are not in git.
- Redeploy after a merge:
  ```bash
  ssh oracle 'cd ~/hackathon && git pull && docker compose -f deploy/docker-compose.yml up -d --build'
  ```
- The server currently tracks branch `abdullah`. Switch it to `main` once this PR is merged (`git switch main`).
- The `VITE_FIREBASE_*` values are build args of the `web` image, taken from `deploy/.env`; they are already set.

## 9. Backend checks before the demo

- `backend/scripts/test-local.sh` runs the full suite, including the double-booking race and an end-to-end flow test.
- Smoke-test on prod with the test number: onboard a worker by voice, then as a customer describe a problem, book, chat (text and voice), make and accept an offer, complete the job and review it.
