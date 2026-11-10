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

React + Vite + TypeScript + Tailwind · Firebase Auth (Google and email/password) · Go (stdlib `net/http`, `pgx`, `asynq`) · Postgres 16 · Redis 7 · Docker Compose.
Planning documents are in [`docs/`](docs/).

## AI services and tools

All through Vercel AI Gateway: _to be completed with the final models and the speech-to-text comparison table._

AI coding assistants used: _to be completed._

## Seeded and mocked

_To be completed._ Planned: seeded workers, jobs and reviews.

## Known limitations

_To be completed._
