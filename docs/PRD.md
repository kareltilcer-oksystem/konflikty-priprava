# PRD — Velké konflikty: příprava porady

**Version:** 1.0 (ready to build)
**Date:** 2026-09-16
**Status:** All open questions resolved — section 14 is the decision log

---

## 1. Summary

A small internal web application that replaces the pen-and-paper process used today to
prepare the recurring weekly meeting between the **development** and **analysis** teams
about the feature **Velké konflikty**.

Throughout the week, three named users drop problems into a shared **bucket**. Before the
weekly meeting the dev team holds its own short sync, during which the **admin** builds the
**meeting preparation**: they pick problems out of the bucket (new ones and unfinished ones
from previous meetings), put them in a deliberate order, and add a short note to each item.
The prepared agenda lives at a stable, human-readable URL that can be shared with everyone
and opened without logging in.

Nothing is changed during the meeting itself. Problems are worked on later in the week; the
admin ticks them off as **done** whenever that happens.

**UI language:** Czech. **Code, API, database, documentation:** English.

---

## 2. Current state and motivation

| Today | With the app |
|---|---|
| Problems collected on paper during the week | Anyone from the trio can add a problem the moment it appears |
| Agenda handwritten before the meeting | Drag-and-drop ordered agenda, numbered 1..N |
| No shared artifact | One stable URL per week, readable by anyone on the LAN without login |
| Unfinished items are re-copied by hand | Unfinished problems stay in the bucket and can be put on any later meeting |
| No history | Every past week's agenda remains browsable |

Typical volume: **8–15 problems per week**, 3 contributors, one meeting per week.
The dataset stays tiny for years, so nothing in this design optimises for scale.

---

## 3. Goals / Non-goals

### Goals
- G1 — Collect problems continuously during the week with near-zero friction.
- G2 — Let the admin assemble and order a weekly agenda in a few minutes.
- G3 — Give every meeting a stable, shareable, login-free URL.
- G4 — Keep unfinished problems visible until the admin marks them done.
- G5 — Preserve the full history of past meetings and of each problem's appearances.

### Non-goals
- N1 — Real security. The app runs on the internal network only; authentication exists
  merely to separate "can edit" from "can look". Credentials live in plaintext env vars.
- N2 — Minute-taking during the meeting. The app is a **preparation** tool; nothing is
  edited while the meeting runs.
- N3 — Task management (assignees, due dates, states beyond done / not done).
- N4 — Notifications (e-mail, Slack, …).
- N5 — Integration with Jira/Confluence or any other system beyond pasting a URL.
- N6 — User management UI, password changes, self-registration, more than three users.
- N7 — Multi-language UI, mobile-first design, offline support.

---

## 4. Users and roles

Exactly three accounts, defined at start-up from an environment variable. There is no user
table in the database; the username string is stored on records as the author.

| Role | Count | Can |
|---|---|---|
| **Anonymous** (not logged in) | — | Read **everything**: the bucket, every problem, every attachment, every meeting agenda including notes. No writes at all. |
| **Editor** | 2 | Everything anonymous can, plus: create problems, edit problem title / description / link, add and remove attachments. |
| **Admin** | 1 | Everything an editor can, plus: create and delete meetings, put problems on an agenda and remove them, reorder the agenda, write the preparation note and the action note per agenda item, mark a problem done / not done, delete problems. |

The logged-in user's **display name** is shown in the header so it is obvious who is signed
in. Editors are not distinguished from each other by permissions — only by authorship.

---

## 5. Domain model

```
Problem ──< MeetingItem >── Meeting
   │
   └──< Attachment
```

### 5.1 Problem

The unit of work. Lives in the **bucket** from creation until the admin marks it done.

| Field | Type | Written by | Notes |
|---|---|---|---|
| `id` | int | system | |
| `title` | string (1–200) | editor/admin | Required. |
| `description` | text | editor/admin | Optional, free text, line breaks preserved. |
| `labels` | set of `ux` / `analysis` | editor/admin | Optional. A problem is *UX*, *Analýza*, both or neither — a closed vocabulary of two, not free text (Q15, decided). |
| `link` | string (URL) | editor/admin | Optional single external URL. **`http://` or `https://` only** — the empty string means "no link". Any other scheme (`javascript:`, `data:`, …) is rejected on write and refused by the renderer, because this value ends up as an `href` on a page every anonymous visitor can open. |
| `created_at` | timestamp | system | |
| `created_by` | string | system | Username of the author. |
| `updated_at` | timestamp | system | |
| `done` | bool | **admin only** | Default `false`. |
| `done_at`, `done_by` | timestamp / string | system | Set when flipped to done. |
| `attachments` | Attachment[] | editor/admin | Screenshots and short videos. |
| `meetings` | MeetingItem[] | derived | Every meeting this problem has appeared on. |

The **bucket** is simply `problems where done = false`. A problem that was already discussed
at one or more meetings and is still not done stays in the bucket and is a normal candidate
for the next agenda — this happens regularly and is expected.

### 5.2 Meeting

One weekly dev/analysis session.

| Field | Type | Notes |
|---|---|---|
| `id` | int | |
| `slug` | string | URL identifier, readable, unique. See 6.3. |
| `meeting_date` | date | The date the meeting takes place. |
| `iso_year`, `iso_week` | int | **Derived, not stored:** computed from `meeting_date` on every read, used for the week label. The `slug` keeps a frozen copy of the creation week (6.3), so after a date change the label follows the new date while the URL does not — the single, deliberate divergence. |
| `note` | text | Optional free text about the meeting as a whole, shown above the agenda. Admin only. |
| `archived` | bool | **Derived, not stored:** `meeting_date` is more than `ARCHIVE_AFTER_DAYS` in the past. See FR-M9. |
| `created_at`, `created_by` | timestamp / string | |

A meeting is **visible to everyone from the moment it is created** — there is no draft or
published state and no "close" action. Normally there is one meeting per calendar week; a
second one in the same week is allowed but rare.

### 5.3 MeetingItem (agenda entry)

The link between a problem and a meeting, and the carrier of the admin's per-meeting text.

| Field | Type | Written by | Notes |
|---|---|---|---|
| `id` | int | system | |
| `meeting_id`, `problem_id` | int | admin | Unique together — a problem appears at most once per meeting. |
| `position` | int | admin | 0-based; the UI displays 1..N. Manual drag-and-drop order **is** the priority. |
| `prep_note` | text | **admin only** | Written during the dev sync before the meeting: what the dev team wants to say about this problem this week. |
| `action_note` | text | **admin only** | Outcome / action, added later. |

> **Design decision (confirmed, Q3):** `prep_note` and `action_note` are stored
> **per agenda entry, not per problem**. Because a problem regularly reappears on later
> meetings, storing the notes on the problem itself would mean next week's preparation
> overwrites last week's record. Per-entry storage keeps every week's agenda a faithful
> snapshot, and the problem detail page shows the whole series of notes as a timeline.

### 5.4 Attachment

A file belonging to a problem — usually a screenshot or a screen recording, but any type is
accepted.

| Field | Type | Notes |
|---|---|---|
| `id` | int | |
| `problem_id` | int | |
| `filename` | string | Original file name, used for download. |
| `content_type` | string | **Any type.** Taken from the upload, falling back to sniffing the first bytes, then to `application/octet-stream`. |
| `size_bytes` | int | Max 100 MB per file (`MAX_UPLOAD_BYTES`). |
| `created_at`, `created_by` | timestamp / string | |

**Storage:** the file's bytes go to the local filesystem under `DATA_DIR/attachments/`,
named by a generated UUID plus an extension **derived from the resolved content type against
a fixed table** (falling back to `bin`) — never from the submitted file name, which is
user-controlled and would otherwise let `x.a/../../../app.db` write outside the attachments
directory, on Windows via `\` as well. The stored `filename` is metadata only: its directory
part and control characters are stripped, it is truncated to 255 characters, and it is
emitted on download through RFC 5987 `filename*` encoding so quotes or CRLF cannot break the
`Content-Disposition` header. Only metadata goes into SQLite. No object storage is involved. Files are served back by the API with HTTP Range support so that
videos can be scrubbed in the browser. Keeping multi-megabyte videos out of the SQLite file
keeps the database small and cheap to copy, and range-serving a real file is one call to
`http.ServeContent`.

**Serving rules:** `image/*` — **except `image/svg+xml` and `image/svg`** — and `video/*` are
served inline so the browser previews them. Every other type, SVG included, is served with
`Content-Disposition: attachment` and `X-Content-Type-Options: nosniff`, so an uploaded
`.html` or `.svg` can never execute inside the app's own origin — a one-line precaution that
costs nothing even on a trusted network. SVG has to be carved out of the `image/*` branch
explicitly: it is a scriptable document rather than a picture, and `nosniff` is no help when
the declared type really is `image/svg+xml`.

---

## 6. Workflows

### 6.1 The weekly cycle

1. **Monday–Friday, any of the three:** logs in, clicks *Přidat problém*, fills in a title,
   optionally a description, a link and screenshots. The problem lands in the bucket.
2. **Before the meeting, dev sync, admin:** creates the meeting for the week
   (*Nová porada*, pick a date → slug is generated). Opens the bucket, ticks the problems
   that belong on this agenda, drags them into the intended order, and writes a `prep_note`
   for each one.
3. **Meeting:** the shareable URL is opened by everyone / projected. The page is read-only in
   practice — **nothing is edited during the meeting**.
4. **During the following week:** problems get solved. The admin marks them **done**, and
   may add an `action_note` to the relevant agenda item. Anything still not done stays in the
   bucket and is a candidate for the next agenda.

### 6.2 Marking done

Only the admin can toggle `done`. It can be done from anywhere the problem is shown: the
bucket, the problem detail, or an agenda (including an agenda from weeks ago). A done problem
disappears from the default bucket view but stays on every agenda it ever appeared on, struck
through and marked *Vyřešeno*. Un-ticking is allowed and returns it to the bucket.

### 6.3 Meeting URL

Readable slug derived from the ISO week of the meeting date:

```
/porada/2026-w38            → first meeting of ISO week 38 of 2026
/porada/2026-w38-2          → an additional meeting in the same week
```

The week is **always zero-padded to two digits** — `2026-w05`, never `2026-w5` — so a week has
exactly one spelling and a retyped link cannot miss; only ISO weeks `01`–`53` are valid and the
suffix for a second meeting starts at `-2` and continues `-3`, `-10`, `-11`, … without a gap.
The slug is generated on creation and never changes afterwards, so shared links keep working.
The URL is public: no login, no token, no redirect to a login prompt.

**Both halves of the slug come from one `time.ISOWeek()` call.** The year is the ISO
week-numbering year, *not* `meeting_date.Year()` — the two disagree at every year boundary. A
meeting on `2027-01-01` is ISO `2026-W53`, so its slug is `2026-w53`; a meeting on `2025-12-29`
is ISO `2026-W01`, so its slug is `2026-w01`. Combining the calendar year with the ISO week
number instead yields `2027-w53` for a year with no 53rd week, and makes `2025-12-29` collide
with the meeting of `2025-01-02` — both would claim `2025-w01`, tripping the `UNIQUE` constraint
on `meetings.slug` or handing a `-2` suffix to two meetings a year apart. The derived
`iso_year` (5.2) is read from the same pair.

---

## 7. Functional requirements

### 7.1 Authentication (FR-A)

- **FR-A1** — All read endpoints and all read-only screens work without authentication.
- **FR-A2** — Login is an **inline dialog** (a small modal opened from a *Přihlásit se*
  button in the header), not a separate page, because the content behind it is already
  visible.
- **FR-A3** — On success the server sets an opaque session token in an `HttpOnly` cookie;
  the client never handles the token itself.
- **FR-A4** — The header shows the display name and an *Odhlásit* action when logged in.
- **FR-A5** — Credentials come from a single environment variable (see 10). Sessions are
  stored in SQLite, so restarting or updating the binary does not log anyone out. Expired
  sessions are purged at start-up and once a day. The session is **sliding on both sides**:
  every authenticated request extends `expires_at` *and* re-sends `Set-Cookie` with a fresh
  `Max-Age = SESSION_TTL_HOURS × 3600`. Setting the cookie's lifetime only at login would log
  an actively used session out on day 30 however long the server-side row was extended.
- **FR-A6** — A failed login returns 401 with a generic Czech message; the response is
  delayed ~1 s so scripted guessing is boring. No lockout.

### 7.2 Problems (FR-P)

- **FR-P1** — Any authenticated user can create a problem: title (required), description,
  labels, link, attachments. Attachments are sent **in the same request** as the rest of the form
  (`multipart/form-data` on `POST /problems`), so the problem and its files are created
  atomically: a cancelled or failed submit leaves no half-made problem in the bucket.
- **FR-P2** — Any authenticated user can edit the title, description, labels, link and
  attachments of any problem, including another editor's; deleting is admin-only (Q2, decided).
  Labels are replaced as a set: sending an empty one clears them, omitting them leaves them be.
- **FR-P3** — Only the admin can delete a problem. The UI must warn which meetings the
  problem appears on; deleting it removes those agenda entries too, and every meeting that
  loses an entry has its remaining items renumbered to stay contiguous in the same
  transaction — otherwise that agenda would render as 1, 3, 4.
- **FR-P4** — Only the admin can toggle `done`.
- **FR-P5** — The bucket view lists not-done problems, newest first by default, showing
  title, author, age, attachment count, and a badge with the number of past meetings the
  problem has already been on.
- **FR-P6** — The bucket view has filters: *nevyřešené* (default) / *vyřešené* / *vše*, plus
  a text filter over title and description (diacritics-insensitive). SQLite's `LIKE`/`NOCASE`
  folds ASCII only and `modernc.org/sqlite` has no ICU, so this filter is **not** done in SQL:
  the handler loads the candidate rows — low hundreds, by design — and matches in Go after
  Unicode NFD normalisation with combining marks dropped and case folded, so `reseni` finds
  `řešení`.
- **FR-P7** — The problem detail shows all fields, the attachments, and the timeline of
  meetings the problem appeared on, with each meeting's `prep_note` and `action_note`.
- **FR-P8** — The bucket view has a sort control: *nejnovější* (default), *nejstarší*, and
  *nejčastěji na poradě*. The last one surfaces problems that keep getting deferred, which is
  exactly what the admin needs when assembling an agenda.
- **FR-P9** — Labels are shown as chips beside the problem's title everywhere it appears: the
  bucket, the detail, the agenda picker and both agenda renderings, printed one included. They
  carry no colour of their own — colour in this UI means the primary action, the done state or
  the deferral badge, and spending two more hues on two short words would drain those.

### 7.3 Attachments (FR-T)

- **FR-T1** — Authenticated users can attach files of **any type** to a problem — screenshots
  and screen recordings above all, but logs, PDFs, exports and documents as well.
- **FR-T2** — Screenshots can be pasted straight from the clipboard (Ctrl+V) into the problem
  form — this is the dominant way screenshots will arrive. The form works the same whether the
  problem already exists or is being created, because **both** create and edit accept the files
  with the form: `POST /problems` and `PATCH /problems/{id}` each take `multipart/form-data`
  alongside their JSON shape (FR-P1). Pasting never requires saving a problem first, and an
  edit that adds three screenshots is one atomic request — not a `PATCH` followed by three
  uploads, which would strand some of them if the third failed.
- **FR-T3** — Images are shown as thumbnails and open in a lightbox; videos play inline;
  every other type — SVG included, see 5.4 — appears as a download tile with file name, type
  icon and size.
- **FR-T4** — Uploads exceeding `MAX_UPLOAD_BYTES` (100 MB) are rejected with a clear Czech
  message. There is no content-type allowlist. A submit is bounded on two further axes, since
  FR-P1's atomicity makes the server stage the whole request before it commits anything:
  at most `MAX_ATTACHMENTS_PER_REQUEST` (20) files in one request and at most
  `MAX_REQUEST_BYTES` (512 MB) in total. Breaching any of the three rejects the request whole.
- **FR-T5** — Authenticated users can delete an attachment; the file on disk is removed too.

### 7.4 Meetings (FR-M)

- **FR-M1** — Only the admin can create a meeting; the input is the meeting date and the slug
  is derived. Creating a meeting for a week that already has one produces a `-2` suffix.
  The admin may also write an optional general `note` about the meeting as a whole, shown
  above the agenda — context that belongs to no single problem.
- **FR-M2** — Only the admin can delete a meeting (with a confirmation). Deleting a meeting
  removes its agenda entries and their notes, but never the problems themselves.
- **FR-M3** — The meeting list shows all meetings, newest first, with date, week label and
  item count.
- **FR-M4** — The admin adds problems to the agenda from a picker showing the bucket, with the
  problems already on this agenda disabled. Multi-select in one action; a problem id repeated
  within one request is a `400` (a problem already on the agenda is the separate `409`).
- **FR-M5** — The admin reorders agenda items by drag-and-drop; the numbering 1..N updates
  immediately and is persisted.
- **FR-M6** — The admin edits `prep_note` and `action_note` inline on each agenda item.
- **FR-M7** — The admin can remove an item from the agenda; this does not affect the problem.
- **FR-M8** — The meeting page shows the meeting `note` above the agenda, and per item: order
  number, title, description, link, attachments, `prep_note`, `action_note`, done indicator.
- **FR-M9** — Meetings whose date is more than `ARCHIVE_AFTER_DAYS` (365) in the past drop
  out of the default meeting list, behind a *Zobrazit archiv* toggle. Archiving is purely a
  view filter derived from the date: nothing is moved, nothing is deleted, and **direct URLs
  keep working forever** — a link shared three years ago still opens its agenda.

### 7.5 Public / read-only view (FR-R)

- **FR-R1** — Every screen renders fully for anonymous visitors; only the controls that
  mutate data are hidden.
- **FR-R2** — Opening a meeting URL never redirects to a login prompt.
- **FR-R3** — The meeting page carries a `@media print` stylesheet: Ctrl+P hides the
  navigation and all controls, expands every note, and renders links as readable text. There
  is no separate print route.

---

## 8. Screens (Czech UI)

| Route | Czech title | Content |
|---|---|---|
| `/` | *Problémy* | The bucket: filters, list, *Přidat problém* (when logged in). |
| `/problem/{id}` | *Detail problému* | All fields, attachments, meeting timeline. |
| `/porady` | *Porady* | Meetings from the last year, *Zobrazit archiv* toggle, *Nová porada* (admin). |
| `/porada/{slug}` | *Porada — týden 38 (2026)* | The shareable ordered agenda: meeting note, numbered items, print stylesheet. |

Header: the app name *Velké konflikty — příprava*, navigation, and either *Přihlásit se* or
the display name + *Odhlásit*.

Dates and weeks are formatted the Czech way: `16. 9. 2026`, *Týden 38*.

---

## 9. Technical design

### 9.1 Stack

- **Frontend:** React 18 + TypeScript, Vite, React Router, TanStack Query for server state,
  `dnd-kit` for drag-and-drop. Styling: Tailwind (a proposal — any small CSS approach is
  fine).
- **Backend:** Go 1.22+, standard-library routing (`http.ServeMux` with method patterns) —
  no framework needed for ~23 endpoints.
- **Database:** SQLite through `modernc.org/sqlite` (pure Go, no cgo — builds cleanly on
  Windows). The schema is created and migrated by the binary at start-up.
- **Distribution:** a single Go binary. The built frontend is compiled into it with
  `go:embed`, so deployment is: copy one executable, set env vars, run.

### 9.2 Ports

| | Frontend | Backend / API |
|---|---|---|
| Development | Vite dev server on **9999**, proxying `/api` → 9998 | Go on **9998** |
| Production | Go serves the embedded SPA **and `/api`** on **9999** | same process, API also on **9998** |

In production one process opens two listeners, so the ports mean the same thing in both
environments and bookmarks do not change (Q7). In development the Go process opens only the
API listener: development sets `WEB_PORT=0`, which suppresses the SPA listener entirely (10).
That opt-out has to exist — the `WEB_PORT` default is 9999, the same port Vite binds, so
without it the two would fight over it on every `go run`.

**The browser always talks to `/api` on its own origin.** In development the Vite proxy
forwards it to 9998; in production the 9999 listener routes `/api` to the same in-process
handler that 9998 serves. Without that, root-relative URLs — `Attachment.url`, every `<img>`,
every `<video>`, every download tile — would resolve to the SPA listener in production and
fall through to the SPA's index.html, so attachments would break after deployment while
working perfectly in dev. It also means the app needs no CORS at all in either environment.

The 9998 listener stays for direct, non-browser calls (curl, scripts, a future integration).
Cross-origin browser access to it is off by default and enabled per deployment with
`CORS_ORIGINS` (see 10); it must name origins explicitly, because credentialed requests
cannot use `*`.

### 9.3 API shape

REST over JSON under `/api`, described fully in [`api/openapi.yaml`](../api/openapi.yaml).
All `GET` endpoints are public; every mutating endpoint requires the session cookie, and the
admin-only ones additionally check the role. Errors use a single JSON shape
`{"error": {"code": "...", "message": "..."}}` with a Czech `message` fit for display.

### 9.4 Database schema

```sql
CREATE TABLE problems (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  title       TEXT    NOT NULL,
  description TEXT    NOT NULL DEFAULT '',
  link        TEXT    NOT NULL DEFAULT '',
  created_at  TEXT    NOT NULL,             -- RFC3339, UTC
  created_by  TEXT    NOT NULL,             -- username
  updated_at  TEXT    NOT NULL,
  done        INTEGER NOT NULL DEFAULT 0,
  done_at     TEXT,
  done_by     TEXT
);

CREATE TABLE problem_labels (
  problem_id INTEGER NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
  label      TEXT    NOT NULL,             -- 'ux' | 'analysis', validated in Go
  PRIMARY KEY (problem_id, label)
);
-- A row per label rather than two columns on `problems` or one comma-separated string: a third
-- label is then data rather than a migration, and a label filter, should one be added, is a
-- plain EXISTS (... AND label = ?) like `scheduled` over meeting_items. No CHECK constraint —
-- the vocabulary lives in Go (store.Label), and a second copy here could only drift.

CREATE TABLE meetings (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  slug         TEXT    NOT NULL UNIQUE,
  meeting_date TEXT    NOT NULL,            -- YYYY-MM-DD
  -- no iso_year / iso_week columns: they are derived from meeting_date on read (5.2).
  -- Storing them would leave a third copy of the same fact to drift when the date changes.
  note         TEXT    NOT NULL DEFAULT '', -- optional, about the whole meeting
  created_at   TEXT    NOT NULL,
  created_by   TEXT    NOT NULL
);
CREATE INDEX idx_meetings_date ON meetings(meeting_date DESC);

CREATE TABLE meeting_items (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  meeting_id  INTEGER NOT NULL REFERENCES meetings(id) ON DELETE CASCADE,
  problem_id  INTEGER NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
  position    INTEGER NOT NULL,
  prep_note   TEXT    NOT NULL DEFAULT '',
  action_note TEXT    NOT NULL DEFAULT '',
  created_at  TEXT    NOT NULL,
  UNIQUE (meeting_id, problem_id)
);
CREATE INDEX idx_items_meeting ON meeting_items(meeting_id, position);
CREATE INDEX idx_items_problem ON meeting_items(problem_id);

CREATE TABLE attachments (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  problem_id   INTEGER NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
  filename     TEXT    NOT NULL,
  content_type TEXT    NOT NULL,
  size_bytes   INTEGER NOT NULL,
  storage_name TEXT    NOT NULL,            -- <uuid>.<ext>, ext from content type, never from filename
  created_at   TEXT    NOT NULL,
  created_by   TEXT    NOT NULL
);
CREATE INDEX idx_attachments_problem ON attachments(problem_id);
-- ON DELETE CASCADE removes attachment rows, but SQLite cannot remove the bytes. Deleting a
-- problem must SELECT its storage_names first and unlink the files in the same transaction,
-- or the cascade silently orphans multi-megabyte videos in DATA_DIR/attachments forever.
-- The same applies to meeting_items: the cascade deletes agenda entries without renumbering,
-- so the handler renumbers every affected meeting itself (FR-P3).

CREATE TABLE sessions (
  token      TEXT PRIMARY KEY,              -- opaque, 32 random bytes hex
  username   TEXT NOT NULL,
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL
);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);
```

`PRAGMA foreign_keys = ON` and `PRAGMA journal_mode = WAL` are set on every connection.
Reordering rewrites the `position` values of one meeting in a single transaction — with ~15
rows that is trivially cheap.

---

## 10. Configuration

All configuration is environment variables; there is no config file.

| Variable | Default | Meaning |
|---|---|---|
| `AUTH_USERS` | — (required) | Accounts, `;`-separated; each is `username:password:Display Name:role` with role `admin` or `editor`. **No field may contain `:` or `;`** — not the password and not the display name, which is the one most likely to be written as `Petr Admin, CTO:` or `Novák; Petr`. A stray `:` shifts every later field by one and the role ends up as `CTO` or `CTO:admin`; a stray `;` splits one account into two malformed ones. Start-up rejects an account whose line does not split into exactly four fields, rather than guessing. Exactly one `admin` is expected. |
| `DATA_DIR` | `./data` | Holds `app.db` and `attachments/`. Created if missing. |
| `API_PORT` | `9998` | API listener. |
| `WEB_PORT` | `9999` | Static SPA listener. Also serves `/api` from the same process, so the frontend is always same-origin (9.2). **`WEB_PORT=0` does not open the listener at all — that is what development sets**, because Vite already owns 9999 there (9.2) and two processes cannot bind it. Without the opt-out the default would collide on every `go run` and one of the two would die with *address already in use*. |
| `CORS_ORIGINS` | — (empty) | Comma-separated origins allowed to call the API listener cross-origin *with credentials*, e.g. `http://vyvoj-srv:9999,http://10.0.0.7:9999`. Empty — the normal case — means no CORS headers at all, because the SPA is same-origin. `*` is not accepted: browsers reject it together with credentials. |
| `MAX_UPLOAD_BYTES` | `104857600` | 100 MB per attachment. Any file type is accepted. |
| `MAX_ATTACHMENTS_PER_REQUEST` | `20` | Cap on the number of `file` parts in one create or edit submit. There is still no limit on the total number of attachments a problem may accumulate over time (FR-T1) — this bounds a single request. |
| `MAX_REQUEST_BYTES` | `536870912` | 512 MB, the cap on a whole multipart body. Needed because FR-P1's atomicity forces the server to stage the entire request before committing: the per-file cap alone lets 20 × 99 MB through as one ~2 GB body, which NFR3's ~30 MB RAM budget cannot absorb. |
| `SESSION_TTL_HOURS` | `720` | 30 days, refreshed on use — the value drives both `sessions.expires_at` and the cookie's `Max-Age`, which is re-sent on every refresh (FR-A5). Sessions are stored in SQLite and survive restarts. |
| `ARCHIVE_AFTER_DAYS` | `365` | Meetings older than this drop out of the default list (FR-M9). |

Example:

```
AUTH_USERS=admin:tajne123:Petr Admin:admin;jan:heslo1:Jan Novák:editor;eva:heslo2:Eva Dvořáková:editor
```

---

## 11. Non-functional requirements

- **NFR1** — Runs on the internal network only; no TLS, no hardening. Stated explicitly as a
  non-goal so nobody mistakes this for a hardened service.
- **NFR2** — Passwords are compared with a constant-time compare, purely so they are not
  trivially timing-leaked; they are still plaintext in the environment by design.
- **NFR3** — Single binary, no runtime dependencies, starts in under a second, ~30 MB RAM.
- **NFR4** — Target browsers: current Chrome/Edge/Firefox on desktop. The meeting page must
  stay readable when projected (generous font size) and usable on a tablet.
- **NFR5** — Backup = copy `DATA_DIR`. The app must tolerate being stopped and copied.
- **NFR6** — Every write records who did it and when; that is the extent of auditing.
- **NFR7** — All user-visible text is Czech and lives in one place in the frontend so it can
  be corrected without hunting through components.

---

## 12. Out of scope (section 3 in concrete terms)

E-mail/Slack notifications · full-text search
beyond a simple title/description filter · assignees, due dates, estimates · comment threads ·
tags/labels/categories · per-user permissions beyond the three fixed accounts · password
changes and user management · an audit-log UI · exports to Excel/PDF (beyond the print view) ·
Jira/Confluence integration · a mobile app · Docker packaging.

---

## 13. Delivery outline (suggested)

1. Go skeleton: config, SQLite schema + migrations, health endpoint, session auth.
2. Problems CRUD + done toggle + bucket filters (API + React screens).
3. Meetings: create, list with the archive filter, agenda add/remove, drag-and-drop ordering,
   per-item notes and the per-meeting note.
4. Attachments: upload of any type (including clipboard paste), serving with Range and the
   download disposition, delete.
5. Public read-only polish, print stylesheet, Czech copy pass, embedding the SPA into the
   binary.

---

## 14. Decision log

All questions raised during drafting were resolved on 2026-09-16. Nothing is blocking.

| # | Question | Decision | Where |
|---|---|---|---|
| **Q1** | Markdown or plain text in the text fields? | **Plain text**, line breaks preserved, `http(s)://` auto-linked. No Markdown renderer, no sanitizer. | 5.1, 5.3 |
| **Q2** | May an editor change the other editor's problem? | **Yes** — any authenticated user can edit any problem. **Deleting is admin-only.** | FR-P2, FR-P3 |
| **Q3** | Do the admin's notes belong to the problem or to the agenda entry? | **Agenda entry** — each week a recurring problem is discussed it gets its own pair of notes, so past agendas stay a faithful snapshot. | 5.3 |
| **Q4** | Is `action_note` needed at all? | **Kept**, admin-only and optional, written after the meeting. Usually empty; costs nothing unused. | 5.3, FR-M6 |
| **Q5** | Print-friendly meeting view? | **Yes, as a `@media print` stylesheet** on the normal page. The separate `/tisk` route was dropped. | FR-R3 |
| **Q6** | Attachments on disk or as SQLite BLOBs? | **Files on disk** under `DATA_DIR/attachments/`, metadata in SQLite. Backup = copy `DATA_DIR`. | 5.4 |
| **Q7** | One port or two in production? | **Two listeners** — SPA on 9999, API on 9998, same as development. The 9999 listener also serves `/api` from the same process, so the browser is always same-origin and no CORS is needed; `CORS_ORIGINS` exists only for direct cross-origin calls to 9998. | 9.2, 10 |
| **Q8** | Sessions in memory or persisted? | **Persisted in SQLite** — restarting or updating the binary does not log anyone out. | FR-A5, 9.4 |
| **Q9** | Retention of old meetings? | **Archived automatically after 365 days**: they leave the default list, stay behind a *Zobrazit archiv* toggle, and their **URLs keep working forever**. Nothing is ever deleted. | FR-M9 |
| **Q10** | Bucket sorting? | **Newest first by default**, plus *nejstarší* and *nejčastěji na poradě* — the latter surfaces repeatedly deferred problems. | FR-P8 |
| **Q11** | Attachment limits and types? | **Any file type**, **100 MB** per file. Images and video preview inline; everything else is a download tile. | 5.4, FR-T1, FR-T4 |
| **Q12** | Do anonymous visitors see author names? | **Yes** — everyone sees everything, names included. | FR-R1 |
| **Q13** | Meeting fields of its own? | **Date + an optional general note** for the whole meeting, shown above the agenda. No title, no participants. | 5.2, FR-M1 |
| **Q14** | Slug format? | **`2026-w38`**, with `-2` for a second meeting in the same week. | 6.3 |
| **Q15** | Labels on a problem: free text or a fixed set? | **A fixed set of two** — *UX* and *Analýza*, any combination including none. Free text would hold "UX", "ux " and "uix" within a month: three tags for one thing and a filter that finds none of them. | 5.1, FR-P1, FR-P9 |

### Small calls made while applying the above

These follow from the decisions rather than replacing them; say the word if any is wrong.

- The archive cutoff compares `meeting_date` against `today − ARCHIVE_AFTER_DAYS`; the
  toggle switches between "last year" and "everything", with no separate archive screen.
- Since any file type is now allowed, non-media attachments are served as downloads with
  `nosniff` rather than rendered in the app's origin (5.4) — SVG counts as non-media here and
  is carved out of the `image/*` inline rule, since `nosniff` cannot stop a document whose
  declared type really is `image/svg+xml`. No limit on the *number* of attachments per
  problem.
- The meeting `note` is admin-only and plain text, consistent with Q1 and with who prepares
  the meeting.

---

## 15. Glossary (CZ ↔ EN)

| Czech (UI) | English (code/API) |
|---|---|
| problém | problem |
| zásobník / seznam problémů | bucket |
| porada | meeting |
| bod programu | meeting item / agenda item |
| pořadí | position |
| poznámka k poradě | prep note |
| poznámka k celé poradě | meeting note |
| akce / výsledek | action note |
| archiv / zobrazit archiv | archive / show archive |
| vyřešeno | done |
| příloha | attachment |
| odkaz | link |
| přihlásit se / odhlásit | log in / log out |
