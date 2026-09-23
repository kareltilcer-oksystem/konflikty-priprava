# Design handoff — Velké konflikty: příprava porady

**For:** Claude Design (mockups / design canvas)
**Source of truth:** [`docs/PRD.md`](PRD.md) v1.0 and [`api/openapi.yaml`](../api/openapi.yaml)
**Date:** 2026-09-16
**Document language:** English. **Everything rendered inside a mockup: Czech.**

This document is self-contained: a designer should not need to read the PRD to produce the
screens. Where it says *PRD-fixed*, the wording or behaviour is already decided and must be
reproduced exactly; where it says *proposal*, the designer may change it.

---

## 1. What this is, in one paragraph

An internal web app for one small dev team that replaces a pen-and-paper ritual. During the
week, three named colleagues drop **problems** into a shared **bucket**. Before the weekly
dev/analysis meeting, the **admin** builds the **agenda**: picks problems out of the bucket,
drags them into a deliberate order, and writes a short preparation note on each. The agenda
lives at a stable URL (`/porada/2026-w38`) that anyone on the internal network can open
**without logging in** — during the meeting it is projected on a wall. Nothing is edited
during the meeting. Afterwards the admin ticks problems off as *vyřešeno*; whatever is left
stays in the bucket and comes back on a later agenda.

**Volume is genuinely small:** 8–15 problems per week, 3 contributors, one meeting per week,
low hundreds of rows over years. No pagination anywhere. Design for a list that comfortably
fits on one screen, not for infinite scroll.

---

## 2. Who uses it

| Role | How many | What they see |
|---|---|---|
| **Anonymous** (not logged in) | anyone on the LAN | **Everything** — bucket, problem details, all attachments, every agenda including all notes and author names. Zero write controls. |
| **Editor** | 2 | All of the above + create/edit any problem (including the other editor's), add/remove attachments. |
| **Admin** | 1 | All of the above + create/delete meetings, build and reorder agendas, write the notes, toggle *done*, delete problems. |

**The single most important visual rule:** the anonymous view is not a stripped-down or
"locked" variant with disabled buttons and padlocks. Mutating controls are simply **absent**.
The read-only page should look complete and intentional, because it is the page that gets
projected in the meeting and shared as a link. Never show a login wall, never redirect an
anonymous visitor to a login prompt.

Sample accounts — use these names in every mockup:

- `Petr Admin` — admin
- `Jan Novák` — editor
- `Eva Dvořáková` — editor

---

## 3. Deliverables

### Core artboards (must have)

| # | Artboard | Route | Role shown |
|---|---|---|---|
| A1 | Bucket — default | `/` | admin, logged in |
| A2 | Bucket — anonymous | `/` | not logged in |
| A3 | Bucket — empty / no results | `/` | logged in |
| A4 | *Přidat problém* form | modal or inline panel on `/` — open question 1 | editor |
| A5 | Problem detail | `/problem/42` | admin |
| A6 | Meetings list | `/porady` | admin |
| A7 | Meeting agenda — admin, editing | `/porada/2026-w38` | admin |
| A8 | Meeting agenda — anonymous / projected | `/porada/2026-w38` | not logged in |
| A9 | Meeting agenda — print (A4) | `/porada/2026-w38` | — |
| A10 | Add-problems-to-agenda picker | overlay on `/porada/2026-w38` | admin |

### Secondary artboards (nice to have, in this order)

| # | Artboard | Notes |
|---|---|---|
| B1 | Login dialog | Small modal opened from the header. |
| B2 | *Nová porada* dialog | Date picker + optional meeting note. |
| B3 | Destructive confirmation — delete problem | Must list the meetings the problem appears on. |
| B4 | Attachment lightbox | Image zoom + video player. |
| B5 | Error / validation states | Inline field error, 404, and all **three** upload rejections (per-file, file count, whole submit) — each loses the entire submit. |
| B6 | Agenda mid-drag | Drag shadow, drop indicator, renumbering. |

### Canvas sizes

- Desktop artboards: **1440 × 1024** — the primary target, current Chrome/Edge/Firefox.
- A8 (projected) reuses 1440 × 1024; it is the same page without the write controls.
- A9 print: **A4 portrait, 794 × 1123** (96 dpi).
- Tablet (1024 × 768) is **optional**: the meeting page must stay *usable* on a tablet, but
  the app is explicitly not mobile-first. Do not produce phone artboards.

---

## 4. Global chrome

**Header**, present on every screen:

- App name on the left: **Velké konflikty — příprava**
- Navigation: **Problémy** (`/`) · **Porady** (`/porady`)
- Right side: either a **Přihlásit se** button, or the logged-in user's display name plus
  **Odhlásit**. The display name must be visibly present — it is how a user knows which of
  the three accounts is signed in, and it is the only account UI in the product.

There is no sidebar, no avatar system, no settings screen, no notification bell, no global
search. Four routes, one header. Resist the urge to add more.

**Formatting conventions:**

| Thing | Format | Example | Status |
|---|---|---|---|
| Date | Czech, spaces after the dots | `16. 9. 2026` | PRD-fixed (§8) |
| Week label | | `Týden 38` | PRD-fixed (§8) |
| Meeting page title | | `Porada — týden 38 (2026)` | PRD-fixed (§8) |
| Agenda numbering | 1..N, never 0 | `1.`, `2.`, `3.` | PRD-fixed (5.3, FR-M5) |
| Age of a problem | Czech relative | `před 3 dny`, `před 2 týdny` | proposal |
| File size | Czech decimal comma | `180 kB`, `24,3 MB` | proposal |

The last two rows are **not** in the PRD — they are this document's suggestion, and the team
has not decided them. Change them if something reads better.

---

## 5. Screen specs

### A1–A3 · `/` — *Problémy* (the bucket)

The landing screen and the busiest one: a list of problems that are **not done**.

**Controls above the list:**

- State filter (PRD-fixed labels, three options): **nevyřešené** (default) · **vyřešené** · **vše**
- Text filter over title *and* description, diacritics- and case-insensitive — `reseni` finds
  `řešení`. Placeholder proposal: *Hledat v problémech…*
- Sort (PRD-fixed labels, three options): **nejnovější** (default) · **nejstarší** ·
  **nejčastěji na poradě**
- **Přidat problém** — primary action, **logged in only**

The third sort option is not filler: it surfaces problems that keep getting deferred, which is
exactly what the admin is hunting for while building an agenda. Give it room.

**Each row shows:** title · its labels, as chips beside the title · author (`Jan Novák`) ·
age (`před 3 dny`) · attachment count · a badge with how many past meetings the problem has
already been on. A *done* problem
(visible under *vyřešené* / *vše*) is struck through and marked **Vyřešeno**.

Design notes:

- The meeting-count badge is the signal that matters most on this screen. A problem on its
  fourth meeting is a problem in trouble — make `4×` legible at a glance, and keep `0` quiet.
- Row density: ~10–15 rows should fit without scrolling. This is an internal tool used weekly
  by three people; favour compact, scannable rows over generous cards.
- The whole row links to the problem detail.
- **Per-row write controls (A1, logged in):** the admin's *mark done / not-done* toggle —
  PRD 6.2 puts it on the bucket, the problem detail *and* every agenda, because ticking
  problems off is the admin's weekly routine and must not require opening each detail page.
  That is the only per-row control; edit and delete live on the detail (A5).
- **A2 (anonymous)** drops *Přidat problém* and every per-row write control. Nothing else
  changes.
- **A3** covers two empty states that read differently and deserve different copy: an empty
  bucket (nothing to prepare — a good state) and a filter that matched nothing. Add a third
  state to the same artboard: the *vše* filter with done and not-done rows interleaved —
  that is the case open question 2 is about, and the done row in section 8 is there for it.

### A4 · *Přidat problém* — the create form

Fields: **Název** (required, max 200 characters) · **Popis** (optional, multi-line, plain
text, line breaks preserved) · **Odkaz** (optional, single `http(s)` URL) · **Štítky**
(optional; two independent toggles, *UX* and *Analýza* — both, one or neither) · **Přílohy**.
Modal or inline panel — designer's call.

Three things this form must get right:

1. **Clipboard paste is the dominant way screenshots arrive.** Ctrl+V into the form attaches
   the image. This has to be *visible* — a paste/drop zone that says so, not an affordance
   hidden behind a file picker. PRD-fixed behaviour; the wording is a proposal.
2. **The problem and its files are saved in one shot.** There is no "save first, then attach"
   step, and a cancelled submit leaves nothing behind. Do not design a two-phase flow.
3. **Any file type is accepted**, bounded on **three** axes: 100 MB per file, 20 files per
   submit, and 512 MB for the whole submit. Breaching any one of them rejects the request
   whole and — because of point 2 — loses the entire problem, so all three need an error
   state in B5, not just the per-file one. Eight 75 MB screen recordings breach only the
   third. Screenshots and screen recordings dominate; logs, PDFs and exports also happen.

Attachment rendering rules — the same everywhere attachments appear:

| Type | Rendering |
|---|---|
| `image/*` except SVG | Thumbnail, opens in a lightbox |
| `video/*` | Inline player, scrubbable |
| everything else, **SVG included** | Download tile: file name, type icon, size |

The SVG carve-out is a deliberate security decision, not an oversight: an SVG is a download
tile like a `.zip`, never a preview.

The edit form is this same form with values filled in, opened from the problem detail.

### A5 · `/problem/42` — *Detail problému*

Shows everything: title, description, link, author, created/updated, done state, the full
attachment gallery — and below it **the timeline of every meeting this problem has appeared
on**, oldest first, each with that meeting's *poznámka k přípravě* and *akce / výsledek*.

The timeline is the payoff of the whole data model and deserves to be designed as a real
feature, not a footnote list. A problem on its fourth meeting shows four entries; reading down
the column tells the story of what was said each week and what came of it. Each entry: week
label and date (linking to `/porada/{slug}`), the position it held on that agenda, the prep
note, and the action note — often empty, so design that case so it does not look broken.

Write controls by role: edit, add attachments and **remove an attachment from the gallery**
(editor+ — this is the only screen where an already-saved attachment can be deleted, so a
wrongly pasted screenshot has nowhere else to go), mark done / not-done (admin), delete
problem (admin — see B3).

### A6 · `/porady` — *Porady*

All meetings, newest first: date, week label, item count. Meetings older than 365 days sit
behind a **Zobrazit archiv** toggle — nothing is deleted or moved, and archived meetings' URLs
keep working forever.

The list is **not** a date window: the meeting being prepared for the coming week is always
**future-dated** and must appear at the top. Show a future meeting as the natural first row —
it is the one the admin clicks most.

**Nová porada** is admin-only (B2: pick a date, optionally write the meeting note). The URL
slug is derived automatically and is never an input field.

### A7–A9 · `/porada/2026-w38` — the agenda

The product's centrepiece and its only shared artifact. Three renderings of one page.

**Content (PRD-fixed):** the meeting's general note above the agenda, then the numbered items
1..N. Each item shows: order number · problem title · description · link · attachments ·
*poznámka k přípravě* · *akce / výsledek* · a done indicator.

**A7 — admin, editing.** Drag handles on each item, drag-and-drop reordering with the numbers
updating immediately, inline editing of both notes, *remove from agenda* per item, an *add
problems* action (A10), a **mark done / not-done** toggle per item (PRD 6.2 — the admin ticks
problems off from the agenda too, including an agenda from weeks ago), edit of the meeting
note, **edit of the meeting date**, delete meeting. This is a working screen: the admin sits
in a 20-minute dev sync and builds the whole agenda here.

The date is editable because a sync slips or a meeting is created on the wrong day, and the
alternative — delete and recreate — destroys every note on the agenda and breaks the shared
link. Changing the date moves the week label but **never** the URL: the slug is frozen at
creation, so `/porada/2026-w38` can legitimately show *Týden 39*. That divergence is
deliberate (PRD 5.2); do not design an interface that promises to fix the URL.

**A8 — anonymous, projected.** Same content, no controls, no drag handles, no empty inline
editors. This is projected on a wall and read from three metres away: **generous font size**,
strong hierarchy between the item number, the title, and the notes. An empty note is simply
absent. Treat this artboard as the app's cover image — it is what everyone except the admin
actually experiences.

**A9 — print (`Ctrl+P`).** A `@media print` stylesheet on the same page, not a separate route:
navigation and all controls hidden, **every note expanded** (nothing collapsed or truncated),
links rendered as readable text rather than blue underlines. Show the **same 6-item agenda as
A7 and A8** — the section 8 sample, not a different meeting — with every note expanded; at
that length with full descriptions it runs onto a second page, so show the page break and how
an item behaves across it.

Attachments need a print form too, since they are agenda-item content (FR-M8) and nothing on
this page is allowed to be truncated: an image prints as the image, a video prints as a still
or a labelled tile — never a dead player control — and a download tile prints as file name,
type and size. Reserve colour for nothing here; assume a black-and-white office printer.

### A10 · Add problems to the agenda

A picker over the bucket: multi-select, several problems added in one action, problems already
on this agenda shown **disabled** rather than hidden, so the admin can see they are handled.
The same meeting-count badge and sort options as the bucket apply — this is where
*nejčastěji na poradě* earns its keep.

### B3 · Destructive confirmations

Two destructive actions, both admin-only, both confirmed:

- **Delete a problem** — the dialog must **name the meetings the problem appears on**, because
  deleting it strips it from those past agendas too. This is the one confirmation in the app
  that carries real information; design it as such, not as a generic *Opravdu?*.
- **Delete a meeting** — removes the agenda and its notes, never the problems themselves. Say
  so explicitly, or the admin will hesitate.

---

## 6. Component inventory

| Component | Where | Notes |
|---|---|---|
| Header + nav + auth slot | everywhere | Two states: anonymous / signed in. |
| Filter bar | A1, A10 | Text input + sort select in both. The segmented state filter is **A1 only** — the picker shows the bucket, and a done problem must not be addable to an agenda. |
| Problem row | A1, A10 | Title, labels, author, age, attachment count, meeting badge, done state. |
| Label chip | A1, A5, A7, A8, A9, A10 | *UX* / *Analýza*, beside the title wherever a problem is shown. Colourless on purpose — colour here means the primary action, done, or deferral. |
| Meeting-count badge | A1, A10 | The deferral signal. Needs a zero, a one, and a many state. Not on A5: the timeline there already tells that story in full. |
| Done toggle | A1, A5, A7 | Admin-only control. Anywhere the problem is shown (PRD 6.2). |
| Done marker | A1, A5, A7, A8 | *Vyřešeno* + strike-through. Also readable in print (A9). |
| Attachment thumbnail / video / download tile | A4, A5, A7, A8, A9 | Three variants, one grid; plus the print forms in A9. |
| Lightbox | B4 | Image and video. |
| Problem form | A4 | Create and edit; paste zone; per-file progress and remove. |
| Agenda item | A7, A8, A9 | Three densities of one component: editing, projected, printed. |
| Drag handle + drop indicator | A7, B6 | Numbers renumber live during the drag. |
| Inline note editor | A7 | Two per item, plus one for the whole meeting. Empty-but-editable state matters. |
| Meeting row | A6 | Date, week label, item count; future meeting at the top; archived variant. |
| Modal shell | A10, B1, B2, B3 | One shell, four contents — plus A4 if open question 1 lands on a modal. |
| Confirmation dialog | B3 | Carries a list, not just a warning. |
| Inline error / empty state | A3, B5 | Server messages arrive already in Czech. |

---

## 7. Czech copy sheet

Strings marked **fixed** appear in the PRD and must be used verbatim. Those marked
**fixed (API)** are emitted by the server and arrive in the response already in Czech
(`error.message`, PRD §9.3) — the frontend prints them as they come, so rewording one in a
mockup changes nothing in the running app. The rest are **proposals** — sensible Czech to
mock with, to be confirmed by the team before implementation. All user-visible text that is
*not* server-emitted lives in one place in the frontend, so a late copy change is cheap.

| Context | Czech | Status |
|---|---|---|
| App name | Velké konflikty — příprava | fixed |
| Nav / page title | Problémy | fixed |
| Nav / page title | Porady | fixed |
| Page title | Detail problému | fixed |
| Page title | Porada — týden 38 (2026) | fixed |
| Header action | Přihlásit se · Odhlásit | fixed |
| Primary action | Přidat problém | fixed |
| Primary action | Nová porada | fixed |
| Toggle | Zobrazit archiv | fixed |
| Done marker | Vyřešeno | fixed |
| Filter options | nevyřešené · vyřešené · vše | fixed |
| Sort options | nejnovější · nejstarší · nejčastěji na poradě | fixed |
| Field label | Název · Popis · Odkaz · Štítky · Přílohy | proposal |
| Label names | UX · Analýza | fixed |
| Field label | Poznámka k přípravě | proposal |
| Field label | Akce / výsledek | proposal |
| Field label | Poznámka k celé poradě | proposal |
| Field label | Datum porady | proposal |
| Action | Uložit · Zrušit · Upravit · Smazat | proposal |
| Action | Přidat na program · Odebrat z programu | proposal |
| Action | Označit jako vyřešené · Vrátit mezi nevyřešené | proposal |
| Search placeholder | Hledat v problémech… | proposal |
| Paste hint | Vložte snímek obrazovky (Ctrl+V) nebo přetáhněte soubory | proposal |
| Empty bucket | Žádné nevyřešené problémy. | proposal |
| Empty search | Hledání nic nenašlo. | proposal |
| Empty agenda | Program zatím nemá žádné body. | proposal |
| Timeline empty | Tento problém zatím nebyl na žádné poradě. | proposal |
| Error (server) | Záznam nebyl nalezen. | fixed (API) |
| Error (upload, per file) | Soubor je větší než povolených 100 MB. | fixed (API) |
| Error (upload, too many files) | *needs a string — 20-file cap* | proposal |
| Error (upload, whole submit) | *needs a string — 512 MB cap* | proposal |
| Error (validation) | Vyplňte název problému. | fixed (API) |
| Login dialog | Přihlášení · Uživatelské jméno · Heslo | proposal |
| Login error | Nesprávné jméno nebo heslo. | fixed (API) |

Glossary for anyone writing more Czech copy — Czech UI term ↔ English code term:

problém ↔ problem · zásobník / seznam problémů ↔ bucket · porada ↔ meeting · bod programu ↔
agenda item · pořadí ↔ position · poznámka k přípravě ↔ prep note · poznámka k celé poradě ↔
meeting note · akce / výsledek ↔ action note · archiv ↔ archive · vyřešeno ↔ done · příloha ↔
attachment · odkaz ↔ link

**Never label a field just *poznámka k poradě*.** PRD §15 glossary maps that exact phrase to
the per-item **prep note**, while it reads naturally as the note on the whole meeting — two
different fields, one string. This document avoids it entirely: the per-item note is
*poznámka k přípravě* (`prep_note`), the meeting-wide one is *poznámka k celé poradě*
(`meetings.note`). Worth correcting in the PRD glossary too.

---

## 8. Sample data — use this, not lorem ipsum

The app is about a feature called *Velké konflikty* (large data conflicts). Realistic Czech
content, with realistic length variance, is what makes these mockups useful. Suggested set:

**Bucket (A1):**

| Title | Author | Age | Attachments | Meetings |
|---|---|---|---|---|
| Konflikt se neuloží při současné editaci dvou uživatelů | Jan Novák | před 2 dny | 2 | 0 |
| Chybné číslování verzí po sloučení velkého konfliktu | Eva Dvořáková | před 4 dny | 1 | 3 |
| Import nad 5 000 řádků spadne na timeout | Jan Novák | před týdnem | 0 | 2 |
| Uživatel nevidí, kdo konflikt vyřešil | Petr Admin | před 9 dny | 3 | 1 |
| Náhled rozdílů nezobrazuje diakritiku správně | Eva Dvořáková | před 11 dny | 1 | 0 |
| Tlačítko „Vyřešit vše“ nejde zrušit | Jan Novák | před 2 týdny | 0 | 4 |
| Export protokolu konfliktů chybí ve starších verzích | Eva Dvořáková | před 3 týdny | 2 | 1 |
| ~~Duplicitní záznamy po opakovaném importu~~ *Vyřešeno* | Petr Admin | před měsícem | 1 | 2 |

**The last row is done** and therefore **must not appear in A1's default view** — the default
filter is *nevyřešené*. Use the first seven rows for A1, A2 and A10; bring the eighth in only
for the *vše* state on A3, which is where open question 2 gets answered. The seven rows are
already in *nejnovější* order, the default sort.

**Meeting (A7–A9):** `/porada/2026-w38`, meeting date `18. 9. 2026`, *Týden 38*, 6 items,
created by `Petr Admin`. The same six items in A7, A8 and A9 — they are one page rendered
three ways, so the item count must not drift between them.

Meeting note proposal: *Analytici dorazí až v 10:00, začínáme body 1–3 bez nich.*

Prep note examples (short, written under time pressure — that is the register):

- *Reprodukovatelné jen na testu 2, potřebujeme od analytiků potvrdit očekávané chování.*
- *Návrh: číslovat verze až po potvrzení sloučení. Chce to rozhodnutí, ne diskuzi.*
- *Vrací se potřetí. Buď to tento týden zavřeme, nebo to chce vlastníka.*

Action note examples (added later, often empty):

- *Dohodnuto — verze se přečíslují až po potvrzení. Jan implementuje do pátku.*
- *Odloženo na příští týden, chybí data z produkce.*

**Meetings list (A6):** `25. 9. 2026` (future, *Týden 39*, 4 items) · `18. 9. 2026`
(*Týden 38*, 6 items) · `11. 9. 2026` (*Týden 37*, 9 items) · `4. 9. 2026` (*Týden 36*,
7 items) — plus one archived row, e.g. `12. 9. 2025` (*Týden 37*, 5 items), visible only with
*Zobrazit archiv* on.

---

## 9. Visual direction

- **Tone:** a quiet internal tool, not a SaaS product. No hero sections, no gradients, no
  marketing polish, no illustrated empty states. Three colleagues open this every week; it
  should be fast to read and unremarkable to use.
- **Density:** compact on the bucket and the meetings list; airy on the projected agenda.
  Those two needs pull in opposite directions and that is the main design problem here.
- **Colour:** mostly neutral. Reserve colour for exactly three jobs — the primary action, the
  done state, and the deferral badge. Destructive actions get red only inside confirmations.
- **Typography:** one family. On A8 the agenda item title should be readable across a meeting
  room; treat ~24–28 px as the floor for titles there, and keep notes clearly subordinate but
  still legible from a distance.
- **Light mode is the deliverable.** Dark mode is not required — a projector and a print
  stylesheet are the two output devices that matter.
- **Implementation context:** React 18 + Tailwind, `dnd-kit` for drag-and-drop. Staying close
  to standard Tailwind spacing and type scales will make the build cheap; a design that needs
  custom tokens everywhere will not survive contact with the implementation.

---

## 10. Do not design these

The PRD rules them out explicitly. Adding them to a mockup creates expectations the build will
not meet:

- Assignees, due dates, estimates, priorities, statuses beyond done / not done
- Tags, labels, categories, comment threads, @mentions, reactions
- Notifications of any kind — no bell, no e-mail, no Slack
- User management, avatars, profile pages, password change, registration
- Dashboards, charts, burndowns, activity feeds, analytics
- Jira/Confluence integration (a pasted URL is the entire integration story)
- Multi-language switcher, mobile/phone layouts, offline states
- Exports beyond the print view; no PDF or Excel buttons
- A draft/published/closed lifecycle for meetings — a meeting is public the moment it exists
- Pagination, infinite scroll, virtualised lists

One clarification on the ordering model: **manual drag-and-drop order is the priority.** There
is no priority field, no P1/P2/P3, no severity. Position 1 means "we talk about this first",
and that is the whole mechanism.

---

## 11. Open questions for the designer

Decide these while designing; none of them blocks starting.

1. **Is the problem form a modal or an inline panel?** Paste-to-attach plus up to 20 files and
   a multi-line description is a lot for a small modal, but the bucket behind it is useful
   context.
2. **How do the bucket rows show the done state under *vše*?** Strike-through alone may be too
   subtle when done and not-done rows are interleaved.
3. **Where do the two notes sit inside an agenda item?** The prep note is written before and
   read aloud during the meeting; the action note is added days later and is usually empty.
   They are not symmetric and probably should not look symmetric.
4. **How does A7 handle a long agenda while dragging?** Twelve items with expanded notes is
   taller than the viewport; the admin needs to drag item 11 to position 2.
5. **What does the projected view do with long descriptions and many attachments?** Truncation
   is dangerous — A9 must expand everything anyway.

---

## 12. Suggested prompt for Claude Design

> Design a Czech-language internal web app for preparing a weekly dev/analysis meeting, per
> `docs/DESIGN-HANDOFF.md`. Produce artboards A1–A8 and A10 at 1440 × 1024, and A9 — the
> print view — at 794 × 1123 (A4 portrait). Use the sample data in section 8 verbatim,
> respecting the note under the bucket table about the done row — the mockups must read as
> real Czech content. The anonymous agenda view (A8) is the hero screen: it gets projected in
> a meeting room and shared as a link, so it must look complete with no controls on it at
> all. Follow section 10 strictly — do not add fields, statuses, or features that are not in
> this document.
