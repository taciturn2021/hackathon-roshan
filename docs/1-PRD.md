# 1. Product Requirements — Roshan Rozgar

Part of the six planning documents in `docs/`. Give all six to your AI coding assistant before asking it to build.

**What:** a mobile-first web app that connects households with nearby skilled trade workers (electrician, plumber, AC technician, carpenter, mechanic) in Islamabad/Rawalpindi.

**Users**
- **Worker:** limited literacy, speaks Urdu/Punjabi-accented Urdu, has a smartphone. Never has to read or type. Information is shown as icon and number first, a few words of Urdu text second, and a speaker button reads it aloud on demand.
- **Customer:** literate, in a hurry, wants someone nearby, free now, competent and safe.

**The five outcomes (all must work, both roles, one flow)**

| # | Outcome | How we deliver it |
|---|---|---|
| 1 | Onboarding | Worker answers four short spoken questions; AI builds the profile as they go; worker confirms with one tap |
| 2 | Discovery | Customer says/types/photographs the problem; AI picks the trade; list of workers who are online, not on a job, sorted by distance |
| 3 | Communication | Voice-note chat with transcripts; price is a structured offer accepted with one tap |
| 4 | Booking | Accepting an offer confirms the job; DB allows one active job per worker |
| 5 | Trust | Star + voice review, only on a completed job; AI tags and a summary on the profile |

**Customer-depth features (in this order; cut from the bottom)**
1. Price range for the trade before hiring (from completed jobs)
2. Trust card: verified reviews, AI summary, jobs done
3. Live job status over WebSocket
4. Photo of the problem → trade *(stretch)*
5. Map of nearby workers, Leaflet *(stretch)*

**Not building:** payments, phone/SMS login, CNIC verification, scheduling for later, worker live GPS tracking, admin panel, native app.

**Never cut:** voice onboarding, the booking constraint, the review step.

**What sets us apart (build after the core flow is green; all three are cheap)**
1. **Everything the worker receives can be heard.** Job summaries and the customer's typed messages get an Urdu speaker button, so the worker can *receive* information without reading, not only send it. Nothing speaks by itself except the onboarding questions and a ringtone. If Urdu text-to-speech is poor, fall back to icon + number + text only and list it as a limitation.
2. **Live double-booking proof.** In the demo, fire two accepts at the same worker at once; one confirms, one gets 409. It is the race test, run in front of the judges.
3. **Measured model choice.** The A vs B speech-to-text results on our five Urdu clips go in the README as a table.
