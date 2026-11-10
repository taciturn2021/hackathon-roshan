# 6. Implementation Plan — Roshan Rozgar

Deadlines: **Figma 12:00**, **full flow working 14:30**, **freeze 15:00**, **submit 15:30** (hard limit 15:45).

Part of the six planning documents in `docs/`. Give all six to your AI coding assistant before asking it to build.

```
docker-compose.yml  .github/        e2e/            → Humza
backend/cmd  internal/{platform,identity,workers,jobs,reviews}  db/   → Abdullah
backend/internal/{ai,tasks}  backend/seed/  README.md  web/public/audio/  → Nouman
web/                                                 → Abdul Muizz
```

| Time | Abdullah | Abdul Muizz | Nouman | Humza |
|---|---|---|---|---|
| now → 12:00 | Firebase project setup, schema, sqlc, `platform`, `identity` (verify Firebase token) | **Figma: 10 frames** | STT test A vs B (5 Urdu clips), confirm gateway access, then help on Figma | Public repo, compose skeleton, CI, branch protection |
| 12:00 → 13:15 | `workers` (onboard, search, profile) | Screens 1–5 against mocked API | `ai` package, onboarding extraction, seed ~40 workers + jobs + reviews | First green `docker compose up`; double-booking race test |
| 13:15 → 14:30 | `jobs`, messages, offers, WebSocket hub, `reviews` | Screens 6–10, wire real API + WebSocket | Queue handlers, Jev decisions, record Urdu prompts, then job read-aloud | Merge continuously; end-to-end happy-path test |
| 14:30 → 15:00 | Bug fixes only | Bug fixes only | README | Clean-clone run; HTTPS tunnel for phone demo |
| 15:00 → 15:30 | Freeze. Two full demo rehearsals, both roles. Submit. | | | |

**Merge rules**
- Short branches, small PRs, only Humza merges to `main`. Stay inside your folders.
- CI is manual only (`gh workflow run CI --ref <branch>`); Humza runs it before merging when needed. It checks: `go vet`, `go test ./...`, `tsc --noEmit`, `eslint`.
- Two tests are mandatory: the race test (two concurrent accepts → exactly one succeeds, the other gets 409) and one browser test of the full flow.

**README must list** (rules require it): setup, stack, every AI service and model, AI coding assistants used, what is seeded (workers, jobs, reviews), what is mocked, Firebase Auth as a third-party service, known limitations.

**Phone demo:** the browser microphone needs HTTPS. Use a tunnel (e.g. `cloudflared`) or demo in a laptop browser at phone width.

**Q&A prep — "what stops working without the AI?"**
- A worker who cannot type cannot create a profile, negotiate or be reviewed in their own words.
- A customer has to know which trade their problem belongs to.
- Voice notes stay unsearchable audio; prices in them never become a one-tap offer.
- Reviews stay raw audio with no tags or summary for the next customer.

**Q&A prep — architecture**
- *Why no Kafka?* No consumer needs replay yet; we named the point at which we would add it.
- *Why is availability not cached?* Consistency of who is free matters more than a few milliseconds.
- *Why no agents?* Every step is known in advance, so a fixed pipeline with a decision model as router is faster, cheaper and testable. Agents fit when the next step is unknown.
- *Why Go, not Python, for AI?* The models are hosted; the work is three HTTP calls, so a second service would only add a network hop.
