# 4. UI/UX Design Brief — Roshan Rozgar

Part of the six planning documents in `docs/`. Give all six to your AI coding assistant before asking it to build.

## Principles

- **Mobile-first**, max width 480px centred on desktop. Touch targets ≥ 56px.
- **Worker side:** Urdu, right-to-left, font Noto Nastaliq Urdu. Three layers for every piece of information: icon and number first, a few words of Urdu text second, a speaker button third. Prices in large Western digits.
- **Audio on demand:** nothing speaks by itself except the onboarding questions and a ringtone for a new job. Everything else plays only when its speaker button is tapped.
- **Customer side:** English labels; accepts Urdu, Roman Urdu or English by voice or text.
- **Colour:** green = go/accept/online, red = cancel/reject/offline, amber = waiting. White background, near-black text, high contrast.
- **Feedback:** every tap gives a visible state change within 100ms; AI waits show an animated mic/thinking state, never a blank screen.
- **Trade icons:** five fixed pictures reused everywhere (onboarding, list, profile, chat header).

## Design values

| Token | Value | Use |
|---|---|---|
| `go` | `#15803D` | Accept, online, mic idle, primary buttons (white text) |
| `stop` | `#B91C1C` | Reject, cancel, offline, recording |
| `wait` | `#F59E0B` fill, `#111827` text | Waiting, "thinking", pending offer |
| `ink` | `#111827` | Text |
| `muted` | `#4B5563` | Secondary text |
| `surface` | `#FFFFFF` | Page background |
| `surface-alt` | `#F3F4F6` | Cards, chat bubbles (other person) |
| `border` | `#D1D5DB` | Card and input borders |

- **Type, customer side:** Inter. 14 caption, 16 body, 20 section title, 24 screen title, 32 prices.
- **Type, worker side:** Noto Nastaliq Urdu, 20 body and 28 titles, line-height 1.9 (Nastaliq clips below that). Digits in Inter, 32+.
- **Spacing:** 8 / 12 / 16 / 24. Page padding 16.
- **Radius:** 16 on cards and buttons; mic button is a circle.
- **Icons:** Lucide (`lucide-react`; there is a Lucide Figma plugin). Trades: `Zap` electrician, `Droplets` plumber, `Snowflake` AC, `Hammer` carpenter, `Wrench` mechanic. Use them at 48px inside a 72px tinted circle.

## Components

| Component | Spec |
|---|---|
| Mic button | 96px circle, `go` idle → `stop` while held, pulsing ring while recording, spinner while thinking |
| Speaker button | 48px round button with a speaker icon. One at the top of every worker screen (reads the instruction) and one on every job card and received message. Plays on tap only; shows a small spinner if the audio is not ready yet |
| Big choice tile | Half-width picture tile, 140px tall, icon + one word; used for role and trade pickers |
| Number pad | 3×4 grid, 64px keys; used for price offers |
| Worker card | Photo 56px, name, trade icon, distance, ★ rating, jobs done; whole card is the tap target |
| Trust card | AI summary sentence, tag chips, review list with play buttons |
| Offer card | Price in 32px digits, "Rs", ✓ (`go`) and ✗ (`stop`) buttons 56px tall; greyed once answered |
| Message bubble | Customer side: text or transcript, with a play button on voice notes. Worker side: every received bubble has a play button (the customer's voice, or Urdu speech of what they typed) with the Urdu text underneath |
| Status bar | Four dots: requested → confirmed → in progress → done; current one filled |
| Online toggle | Full-width 88px switch, `go` when on, `stop` when off, icon changes |

## Layout per screen

1. **Role pick:** two big choice tiles stacked, worker on top. Nothing else.
2. **Sign in:** worker frame = audio prompt bar and one full-width "Continue with Google" button (Google logo, 72px tall), nothing else. Customer frame = the same Google button, an "or" divider, then email and password fields with a sign in / create account toggle.
3. **Voice onboarding:** one question at a time. Large icon for the current question in the centre (person, tools, map pin, calendar), a speaker button to hear it again, mic button low. Four progress dots on top that turn into green ticks as answers are understood. No spinner between questions.
4. **Profile confirm:** card with photo placeholder, trade icon, name, area, years as icon + value rows; bottom row ✓ (`go`, wide) and mic (redo, narrow).
5. **Worker home:** online toggle on top; below it either an empty "waiting" illustration or the incoming job card (trade icon, distance, two lines of Urdu text, speaker button, open button).
6. **Describe problem:** text box with mic and camera buttons inside it; after analysis a result card: trade icon, urgency chips, price range, "Find workers" button.
7. **Worker list:** trade + area header, then worker cards. Empty state with a "widen area" button.
8. **Worker profile:** photo, name, trade, play-intro button; trust card; price range; sticky "Request this worker" button.
9. **Job chat:** status bar on top; bubbles in the middle; bottom bar = mic button, "Rs" button (opens number pad for an offer), and for the customer a text field. Draw two versions: customer view (text and transcripts) and worker view (play button on every received bubble).
10. **Complete + review:** worker sees one wide `go` "done" button. Customer sees five large stars and a mic button, then a thank-you state.

## Urdu audio prompts to record (`web/public/audio/`)

Have a native speaker check the wording before recording.

| File | Urdu | Roman Urdu |
|---|---|---|
| `role.mp3` | آپ کاریگر ہیں یا گاہک؟ تصویر دبائیں۔ | Aap kaarigar hain ya gaahak? Tasveer dabayen. |
| `signin.mp3` | آگے بڑھنے کے لیے گوگل والا بٹن دبائیں۔ | Aagay barhne ke liye Google wala button dabayen. |
| `ask-name.mp3` | آپ کا نام کیا ہے؟ | Aap ka naam kya hai? |
| `ask-trade.mp3` | آپ کیا کام کرتے ہیں؟ | Aap kya kaam karte hain? |
| `ask-area.mp3` | آپ کس علاقے میں کام کرتے ہیں؟ | Aap kis ilaqay mein kaam karte hain? |
| `ask-years.mp3` | آپ کو کتنے سال کا تجربہ ہے؟ | Aap ko kitne saal ka tajurba hai? |
| `confirm.mp3` | کیا یہ معلومات ٹھیک ہیں؟ ٹھیک ہیں تو سبز بٹن دبائیں۔ دوبارہ بولنے کے لیے مائیک دبائیں۔ | Kya yeh maloomat theek hain? Theek hain to sabz button dabayen. Dobara bolne ke liye mic dabayen. |
| `home.mp3` | کام لینے کے لیے بٹن آن کریں۔ | Kaam lene ke liye button on karein. |
| `ring.mp3` | (short ringtone, no speech) | Plays when a new job arrives. |
| `chat.mp3` | بات کرنے کے لیے مائیک دبا کر رکھیں۔ | Baat karne ke liye mic daba kar rakhein. |
| `offer.mp3` | قیمت آئی ہے۔ منظور ہے تو سبز بٹن دبائیں۔ | Qeemat aayi hai. Manzoor hai to sabz button dabayen. |
| `done.mp3` | کام مکمل ہو جائے تو یہ بٹن دبائیں۔ | Kaam mukammal ho jaye to yeh button dabayen. |
| `error.mp3` | معاف کیجیے، دوبارہ کوشش کریں۔ | Maaf kijiye, dobara koshish karein. |
