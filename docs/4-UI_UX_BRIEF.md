# 4. UI/UX Design Brief — Roshan Rozgar

Part of the six planning documents in `docs/`. Give all six to your AI coding assistant before asking it to build.

## Principles

- **Mobile-first**, max width 480px centred on desktop. Touch targets ≥ 56px.
- **Worker side:** Urdu, right-to-left, font Noto Nastaliq Urdu, almost no text. Icon + audio for every action. Prices in large Western digits.
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
| Audio prompt bar | Top of every worker screen: speaker icon + replay button; auto-plays once on entry |
| Big choice tile | Half-width picture tile, 140px tall, icon + one word; used for role and trade pickers |
| Number pad | 3×4 grid, 64px keys; used for phone, code and price |
| Worker card | Photo 56px, name, trade icon, distance, ★ rating, jobs done; whole card is the tap target |
| Trust card | AI summary sentence, tag chips, review list with play buttons |
| Offer card | Price in 32px digits, "Rs", ✓ (`go`) and ✗ (`stop`) buttons 56px tall; greyed once answered |
| Voice bubble | Play button + waveform bar + transcript underneath (customer side shows transcript; worker side shows play only) |
| Status bar | Four dots: requested → confirmed → in progress → done; current one filled |
| Online toggle | Full-width 88px switch, `go` when on, `stop` when off, icon changes |

## Layout per screen

1. **Role pick:** two big choice tiles stacked, worker on top. Nothing else.
2. **Phone + code:** large digit display on top, number pad below, `go` arrow button. Same frame reused for the code.
3. **Voice onboarding:** audio prompt bar; four small hint icons in a row (person, tools, map pin, calendar) that tick green as each field is heard; mic button centred low.
4. **Profile confirm:** card with photo placeholder, trade icon, name, area, years as icon + value rows; bottom row ✓ (`go`, wide) and mic (redo, narrow).
5. **Worker home:** online toggle on top; below it either an empty "waiting" illustration or the incoming job card (trade icon, distance, play-brief button, open button).
6. **Describe problem:** text box with mic and camera buttons inside it; after analysis a result card: trade icon, urgency chips, price range, "Find workers" button.
7. **Worker list:** trade + area header, then worker cards. Empty state with a "widen area" button.
8. **Worker profile:** photo, name, trade, play-intro button; trust card; price range; sticky "Request this worker" button.
9. **Job chat:** status bar on top; bubbles in the middle; bottom bar = mic button, "Rs" button (opens number pad for an offer), and for the customer a text field.
10. **Complete + review:** worker sees one wide `go` "done" button. Customer sees five large stars and a mic button, then a thank-you state.

## Urdu audio prompts to record (`web/public/audio/`)

Have a native speaker check the wording before recording.

| File | Urdu | Roman Urdu |
|---|---|---|
| `role.mp3` | آپ کاریگر ہیں یا گاہک؟ تصویر دبائیں۔ | Aap kaarigar hain ya gaahak? Tasveer dabayen. |
| `phone.mp3` | اپنا موبائل نمبر لکھیں۔ | Apna mobile number likhein. |
| `code.mp3` | کوڈ لکھیں۔ | Code likhein. |
| `onboard.mp3` | بٹن دبا کر رکھیں اور بتائیں: آپ کا نام، آپ کا کام، آپ کا علاقہ، اور کتنے سال کا تجربہ ہے۔ | Button daba kar rakhein aur batayen: aap ka naam, kaam, ilaqa, aur kitne saal ka tajurba hai. |
| `ask-name.mp3` | آپ کا نام کیا ہے؟ | Aap ka naam kya hai? |
| `ask-trade.mp3` | آپ کیا کام کرتے ہیں؟ | Aap kya kaam karte hain? |
| `ask-area.mp3` | آپ کس علاقے میں کام کرتے ہیں؟ | Aap kis ilaqay mein kaam karte hain? |
| `ask-years.mp3` | آپ کو کتنے سال کا تجربہ ہے؟ | Aap ko kitne saal ka tajurba hai? |
| `confirm.mp3` | کیا یہ معلومات ٹھیک ہیں؟ ٹھیک ہیں تو سبز بٹن دبائیں۔ دوبارہ بولنے کے لیے مائیک دبائیں۔ | Kya yeh maloomat theek hain? Theek hain to sabz button dabayen. Dobara bolne ke liye mic dabayen. |
| `home.mp3` | کام لینے کے لیے بٹن آن کریں۔ | Kaam lene ke liye button on karein. |
| `new-job.mp3` | نیا کام آیا ہے۔ | Naya kaam aaya hai. |
| `chat.mp3` | بات کرنے کے لیے مائیک دبا کر رکھیں۔ | Baat karne ke liye mic daba kar rakhein. |
| `offer.mp3` | قیمت آئی ہے۔ منظور ہے تو سبز بٹن دبائیں۔ | Qeemat aayi hai. Manzoor hai to sabz button dabayen. |
| `done.mp3` | کام مکمل ہو جائے تو یہ بٹن دبائیں۔ | Kaam mukammal ho jaye to yeh button dabayen. |
| `error.mp3` | معاف کیجیے، دوبارہ کوشش کریں۔ | Maaf kijiye, dobara koshish karein. |
