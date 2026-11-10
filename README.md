# Roshan Rozgar (روشن روزگار)

A hyper-local marketplace connecting households with nearby skilled trade workers. Built for the CUI Tech Fest AI Web Forge hackathon.

## Run it

```bash
cp .env.example .env   # then put the Vercel AI Gateway key in .env
docker compose up --build
```

| What | Where |
|---|---|
| Web app | http://localhost:5173 |
| API | http://localhost:8080/api/healthz |
| Queue dashboard | http://localhost:8081 |

After changing `backend/db/schema.sql`, reset the database with `docker compose down -v`.

The API exits at startup unless `.env` has `FIREBASE_PROJECT_ID` (the team's Firebase project). For local testing without Firebase, set `AUTH_DEV=1` instead and send `Authorization: Bearer dev:<any-name>`; never set that on a deployed server.

## Tests

```bash
backend/scripts/test-local.sh
```

Runs `go vet` and the full Go suite with the race detector against throwaway Postgres and Redis containers. It needs only Docker. It includes the double-booking race test (two simultaneous accepts for one worker, exactly one confirms) and an end-to-end test of the whole flow.

## Stack

React + Vite + TypeScript (plain CSS) · Firebase Auth (phone, Google, email/password) · Go (stdlib `net/http`, `pgx`, `asynq`) · Postgres 16 · Redis 7 · Docker Compose.
Planning documents are in [`docs/`](docs/). Frontend integration notes are in [`HUMZA-INTEGRATE.md`](HUMZA-INTEGRATE.md).

## AI services and tools

Every model is called through Vercel AI Gateway, from one Go module (`backend/internal/ai`).

| Step | Model | Fallback |
|---|---|---|
| Urdu speech to text | `microsoft/mai-transcribe-2` | `google/gemini-3.5-transcribe` |
| Decisions: trade, price mentioned, review tags, abuse | `typesafe-ai/jev` | `openai/gpt-6-luna-decisions` |
| Extraction and Urdu writing: profile fields, problem summary, job brief, message in Urdu, trust summary | `anthropic/claude-haiku-5.5` | `openai/gpt-6-luna` |
| Urdu text to speech | `elevenlabs/eleven-v4` | `google/gemini-3.8-flash-tts` |

Speech-to-text comparison on the same Urdu clip (one run each): the Microsoft model returned Urdu script in 1.9 s; the Gemini model took 3.6 s and returned Hindi script, which then needs one more model call to convert. The browser therefore records WAV, which the Microsoft model accepts.

AI coding assistants used: OpenAI Codex for the frontend design package. The landing illustration was produced with an image generation tool.

Icons are Lucide (ISC). Fonts are Inter and Noto Nastaliq Urdu from Google Fonts.

## Seeded and mocked

- **Seed data** (`backend/seed/seed.sql`): 40 workers, 8 customers, 95 past jobs and 90 reviews, all fictional. Review tags and trust summaries of seeded workers are written by the seed, not by the AI. Seeded accounts cannot sign in.
- **Demo sign-in**: Firebase test phone numbers, so no SMS is sent. Worker `+92 300 0000001`, customer `+92 300 0000002`, code `123456` for both.
- **Demo location**: the location screens offer "Use G-10, Islamabad", where the seeded workers are.
- **Spoken prompts** (`web/public/audio/`): generated once with the text-to-speech model, not recorded by a person.
- **Mock mode**: `VITE_MOCK_API=true` runs the screens on an in-browser mock backend with fictional data. It is off by default.

## Known limitations

- The web app polls every 4 to 5 seconds for new messages and job changes; it does not use the API's WebSocket yet.
- Urdu speech quality was checked with generated speech, not with a wide range of real speakers and accents.
- Media files are stored on the server's disk.
