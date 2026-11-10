# 3. App Flow — Roshan Rozgar

Part of the six planning documents in `docs/`. Give all six to your AI coding assistant before asking it to build.

Worker screens show icon and number first, a few words of Urdu second, and a speaker button that reads the screen or item aloud on tap. Nothing speaks by itself except the onboarding questions and a ringtone for a new job. One primary action per screen.

| # | Screen | Role | Primary action | Notes / states |
|---|---|---|---|---|
| 1 | Role pick | both | Tap picture: worker (tools) or customer (house) | No text needed |
| 2 | Sign in | both | Worker: phone number on the number pad, then the SMS code; or one big "Continue with Google" button. Customer: phone, Google, or email + password | Firebase Auth. Worker enters digits only, never letters; error = shake + audio |
| 3 | Voice onboarding | worker | Answer four spoken questions one at a time: name, work, area, years | One question at a time. The question plays; the worker taps the mic, speaks, and taps again. The screen says it is understanding (about 3 s), then shows and speaks what it understood (tick) and moves to the next question. Unclear answer → it says so, shows what it heard, and asks the same question once more; then the trade falls to the icon picker and the other questions are skipped. A question already answered in an earlier reply is not asked. Asks for location permission |
| 4 | Profile confirm | worker | ✓ confirm or 🎤 redo | Shows trade icon, name, area, years; low confidence → 5 trade icons to tap |
| 5 | Worker home | worker | Big online/offline toggle | New job: ringtone only, then a card with trade icon, distance, short Urdu text and a speaker button; busy state when on a job |
| 6 | Describe problem | customer | Mic, text box or camera | Result: trade + urgency + price range, editable |
| 7 | Worker list | customer | Tap a worker | Distance, rating, jobs done; empty state: "no one free nearby, widen area" |
| 8 | Worker profile | customer | "Request this worker" | Trust summary, tags, reviews, intro audio |
| 9 | Job chat | both | Hold mic to send voice note | Every message the worker receives has a play button, whether the customer spoke or typed. Offer card with price + ✓ / ✗; number pad to make an offer; status bar on top |
| 10 | Complete + review | both | Worker: "job done". Customer: stars + voice | Review only after `completed` |

**Flow:** 1 → 2 → worker: 3 → 4 → 5 → (job arrives) 9 → 10. Customer: 6 → 7 → 8 → 9 → 10.

**Job states:** `requested` → (offer accepted) `confirmed` → (worker starts) `in_progress` → `completed`. Any state before `completed` can go to `cancelled`.
