# 3. App Flow — Roshan Rozgar

Part of the six planning documents in `docs/`. Give all six to your AI coding assistant before asking it to build.

Worker screens auto-play a recorded Urdu prompt and have a replay button. One primary action per screen.

| # | Screen | Role | Primary action | Notes / states |
|---|---|---|---|---|
| 1 | Role pick | both | Tap picture: worker (tools) or customer (house) | No text needed |
| 2 | Sign in | both | Worker: one big "Continue with Google" button. Customer: Google button, or email + password | Firebase Auth. Worker never types; error = shake + audio |
| 3 | Voice onboarding | worker | Hold mic, speak name / work / area / years | Asks for location permission; states: idle, recording, thinking, follow-up question (max 2) |
| 4 | Profile confirm | worker | ✓ confirm or 🎤 redo | Shows trade icon, name, area, years; low confidence → 5 trade icons to tap |
| 5 | Worker home | worker | Big online/offline toggle | Incoming job card rings; busy state when on a job |
| 6 | Describe problem | customer | Mic, text box or camera | Result: trade + urgency + price range, editable |
| 7 | Worker list | customer | Tap a worker | Distance, rating, jobs done; empty state: "no one free nearby, widen area" |
| 8 | Worker profile | customer | "Request this worker" | Trust summary, tags, reviews, intro audio |
| 9 | Job chat | both | Hold mic to send voice note | Offer card with price + ✓ / ✗; number pad to make an offer; status bar on top |
| 10 | Complete + review | both | Worker: "job done". Customer: stars + voice | Review only after `completed` |

**Flow:** 1 → 2 → worker: 3 → 4 → 5 → (job arrives) 9 → 10. Customer: 6 → 7 → 8 → 9 → 10.

**Job states:** `requested` → (offer accepted) `confirmed` → (worker starts) `in_progress` → `completed`. Any state before `completed` can go to `cancelled`.
