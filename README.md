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

## Stack

React + Vite + TypeScript + Tailwind · Firebase Auth (Google and email/password) · Go (stdlib `net/http`, `pgx`, `asynq`) · Postgres 16 · Redis 7 · Docker Compose.
Planning documents are in [`docs/`](docs/).

## AI services and tools

All through Vercel AI Gateway: _to be completed with the final models and the speech-to-text comparison table._

AI coding assistants used: OpenAI Codex for the static design package and its validation. The cover illustration was produced with the built-in image generation tool; its prompt and use are documented in `design/HANDOFF.md`.

## Design preview and Figma import

The design package is in [`design/`](design/index.html). It includes 27 role-specific screens, 334 desktop/mobile state frames, an original cover, light/dark theme tokens, component specimens and flows.

```bash
python -m http.server 4173 --bind 127.0.0.1 --directory design
```

Open http://127.0.0.1:4173/gallery.html for the visual gallery or `/index.html` for every state. See [`design/IMPORT-GUIDE.html`](design/IMPORT-GUIDE.html) for html.to.design import and [`design/CHECKLIST.md`](design/CHECKLIST.md) for the full frame checklist. The package is static and has not been uploaded to Figma; the React frontend remains the original skeleton.

Design references and library credits: [`design/RESEARCH.md`](design/RESEARCH.md) and [`design/HANDOFF.md`](design/HANDOFF.md). Icons use Lucide (ISC); typography uses Google Fonts; component conventions follow shadcn/ui, Radix and Tailwind. Thinking Gods/Fernroot and tweakcn Nature informed presentation. The separate Liquid Buttons demo is MIT-licensed; its Three.js animation is not bundled into the application.

## Seeded and mocked

The design package uses fictional worker identities, ratings, job counts, prices and review text. It makes no live bookings and does not authenticate or record audio. These fixtures are not database seeds. Planned backend work remains: seeded workers, jobs and reviews.

## Known limitations

_To be completed._
