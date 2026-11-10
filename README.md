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

AI coding assistants used: OpenAI Codex for the static design package and its validation. The cover illustration was produced with the built-in image generation tool; its prompt and use are documented in `_archive/design-reference/HANDOFF.md`.

## Design preview and Figma import

The design package is in [`_archive/design-reference/`](_archive/design-reference/index.html). It includes 27 role-specific screens, 334 desktop/mobile state frames, an original cover, light/dark theme tokens, component specimens and flows.

```bash
python -m http.server 4173 --bind 127.0.0.1 --directory _archive/design-reference
```

Open http://127.0.0.1:4173/gallery.html for the visual gallery or `/index.html` for every state. See [`_archive/design-reference/IMPORT-GUIDE.html`](_archive/design-reference/IMPORT-GUIDE.html) for html.to.design import and [`_archive/design-reference/CHECKLIST.md`](_archive/design-reference/CHECKLIST.md) for the full frame checklist. The package is static and has not been uploaded to Figma; the React frontend remains the original skeleton.

Design references and library credits: [`_archive/design-reference/RESEARCH.md`](_archive/design-reference/RESEARCH.md) and [`_archive/design-reference/HANDOFF.md`](_archive/design-reference/HANDOFF.md). Icons use Lucide (ISC); typography uses Google Fonts; component conventions follow shadcn/ui, Radix and Tailwind. Thinking Gods/Fernroot and tweakcn Nature informed presentation. The separate Liquid Buttons demo is MIT-licensed; its Three.js animation is not bundled into the application.

## Seeded and mocked

The design package uses fictional worker identities, ratings, job counts, prices and review text. It makes no live bookings and does not authenticate or record audio. These fixtures are not database seeds. Planned backend work remains: seeded workers, jobs and reviews.

## Known limitations

_To be completed._
