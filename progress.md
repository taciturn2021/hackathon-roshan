# Roshan Rozgar — progress

Updated: 10 November 2026 (Asia/Karachi).

## Current result

A connected, responsive **website design prototype** is available locally. This is not yet the production React application or a submitted Figma file.

- Website: http://127.0.0.1:4173/website/
- Every flow: http://127.0.0.1:4173/website/journeys.html
- Files: `design/website/`
- Per-file checklist: `design/website/CHECKLIST.md`
- To restart the preview from the repository root: `python -m http.server 4173 --bind 127.0.0.1 --directory design`

## Completed

- Read the project PDF, six planning documents, repository Markdown and starter application/contracts/schema. Reviewed the supplied hero reference, Thinking Gods fonts, shadcn/ui and tweakcn Nature references; research is in `design/RESEARCH.md`.
- Pulled `origin/main` successfully, fast-forwarding `48c6fd1` to `13532b9`. Read all eight changed files' diffs. Preserved local design work; no changes committed or pushed.
- Incorporated the updated guidance for four separate onboarding questions and on-demand Urdu playback for job summaries and received typed messages into `CLAUDE.md` and design patterns. The new happy-path onboarding previews four questions, then location and profile confirmation.
- Wrote `CLAUDE.md`, design direction, website patterns and a responsive screen inventory. The latest website request authorises this connected preview; older submission exports remain preserved.
- Created the original static design package: 27 screen variants, 334 mobile/desktop state frames, theme tokens, component boards, flows, cover, inventory and import guidance. These original frames use the previous narrow desktop layout and are now an archive, not the current website preview.
- Built a standalone dependency-free WebGL mesh gradient, with Mint default, Noir/Plum/Neon alternatives, double domain warp, five-octave noise, circular 24-second loop, Bayer dithering, eased parallax, reduced motion and matching CSS fallback.
- Built the current public website landing page with the Mint gradient, tool imagery, five trade categories, how-it-works section, worker invitation, trust explanation, full website header and footer.
- Created **172 responsive preview pages**: the 167 original mobile state variants adapted to the website shell, three additional onboarding question pages, landing page and journey index. Each adapts to the viewport rather than being a fixed phone frame.
- Added mobile navigation drawer, desktop navigation, state selector, selectable review stars, editable price keypad, urgency selection and session-local input draft retention. Audio buttons explain their prototype status instead of pretending to play recordings.
- Connected customer discovery, profile, chat, price acceptance, worker start/completion and customer review. Walkthrough controls explicitly identify role switches.
- Installed and ran the separate liquid-buttons reference project on the Desktop earlier. The website uses a CSS glossy CTA reference; the Three.js liquid effect is not integrated.

## Flow coverage

**Customer:** sign-in/create account → describe problem → trade/urgency → location → nearby workers → worker profile → request/chat → price offer/acceptance → confirmed job → worker starts/completes → customer review → job history.

**Worker:** Google sign-in preview → four short questions (name, trade, area, experience) → location → profile confirmation → online availability → incoming request → voice/message chat → price offer/acceptance → start job → complete job → history.

**Additional pages/states:** role selection, trade picker, missing-answer retry, cancellation, empty results, loading/error/success, microphone permission, offline/busy, reconnection, rejected offer and worker-busy conflict. These are visual demonstrations, not a live state machine.

## Verification completed

- `python design/source/verify_website.py`: all 172 pages' local links and referenced assets resolve; four onboarding question pages exist.
- Browser checks: all 27 default screens at 390px and 1440px (54 checks), no horizontal overflow or JavaScript page errors.
- Clicked through customer sign-in/discovery/booking, switched to worker start and completion, then reached the review and selected four stars.
- Clicked through all four worker onboarding questions, location, profile confirmation and worker home.
- Mobile drawer opens and closes with Escape. Chat price button checked at 56px wide after fixing flex shrink.
- Gradient compiled and rendered without WebGL errors; loop endpoints differed by at most one 8-bit channel level, motion changed the field, forced unavailable-WebGL used the matching fallback, and mobile sizing respected the pixel-ratio cap.
- Visually inspected the landing page and desktop chat. Screenshots: `design/website-desktop-preview.png`, `design/website-mobile-preview.png`, `design/website-chat-preview.png`. The chat screenshot precedes the small price-button sizing fix.

## Remaining / limitations

- **Figma is not confirmed imported, uploaded or submitted. No native `.fig` file has been produced.** Existing `output/` import ZIP and submission board predate the website redesign. They must not be presented as the new website export.
- The new HTML is the reviewable prototype. Figma Auto Layout, component instances, Variables and imported layout/font fidelity still need creation/verification. Shader animation will not survive a static Figma import.
- Authentication, microphone recording/playback, AI processing, real location, messaging, offers, booking constraints and reviews are not connected to services. Prototype navigation shows predetermined fictional AC-job outcomes. Selecting another trade or entering a price does not create a matching live job.
- Most secondary state content was carried forward from the earlier package. Audit all onboarding recovery/loading wording against the new four-question contract before final submission; the new happy path is updated, but this is not a claim that every legacy state has been rewritten.
- React/Vite frontend remains separate from this design prototype; backend contracts were pulled, not implemented by this design work.
- Tablet CSS exists but has not received the same complete screen-by-screen browser audit. Full accessibility/contrast audit of the new composition, native Urdu review, physical phone GPU performance and cross-browser testing remain.
- Contact/social/legal destinations have not been invented; add real information when supplied.

## Source files and rebuild

- `design/source/build_website.py`: generates the current responsive website from existing frame content plus the new landing, shared shell and updated onboarding.
- `design/source/website.css`: shared responsive website styles.
- `design/source/website.js`: prototype interactions.
- `design/source/verify_website.py`: local link/asset checks.
- `design/mesh-gradient.html`: self-contained gradient source.
- Rebuild with `python design/source/build_website.py`, then `python design/source/verify_website.py`.
- The original `design/source/build.py` regenerates the older package; do not run it expecting the latest website layout or onboarding rules.
