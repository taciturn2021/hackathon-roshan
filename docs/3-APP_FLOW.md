# 3. App Flow — Roshan Rozgar

Part of the six planning documents in `docs/`. Give all six to your AI coding assistant before asking it to build.

Worker screens show icon and number first, a few words of Urdu second, and a speaker button that reads the screen or item aloud on tap. Nothing speaks by itself except the onboarding questions and a ringtone for a new job. One primary action per screen.

| # | Screen | Role | Primary action | Notes / states |
|---|---|---|---|---|
| 1 | Role pick | both | Tap picture: worker (tools) or customer (house) | No text needed |
| 2 | Sign in | both | Worker: one big "Continue with Google" button. Customer: Google button, or email + password | Firebase Auth. Worker never types; error = shake + audio |
| 3 | Voice onboarding | worker | Answer four spoken questions one at a time: name, work, area, years | A question plays; worker holds the mic to answer; a tick appears and the next question plays at once (the answer is processed in the background, no spinner). Unclear answer → asked once more, then icon picker. Asks for location permission |
| 4 | Profile confirm | worker | ✓ confirm or 🎤 redo | Shows trade icon, name, area, years; low confidence → 5 trade icons to tap |
| 5 | Worker home | worker | Big online/offline toggle | New job: ringtone only, then a card with trade icon, distance, short Urdu text and a speaker button; busy state when on a job |
| 6 | Describe problem | customer | Mic, text box or camera | Result: trade + urgency + price range, editable |
| 7 | Worker list | customer | Tap a worker | Distance, rating, jobs done; empty state: "no one free nearby, widen area" |
| 8 | Worker profile | customer | "Request this worker" | Trust summary, tags, reviews, intro audio |
| 9 | Job chat | both | Hold mic to send voice note | Every message the worker receives has a play button, whether the customer spoke or typed. Offer card with price + ✓ / ✗; number pad to make an offer; status bar on top |
| 10 | Complete + review | both | Worker: "job done". Customer: stars + voice | Review only after `completed` |

**Flow:** 1 → 2 → worker: 3 → 4 → 5 → (job arrives) 9 → 10. Customer: 6 → 7 → 8 → 9 → 10.

**Job states:** `requested` → (offer accepted) `confirmed` → (worker starts) `in_progress` → `completed`. Any state before `completed` can go to `cancelled`.
