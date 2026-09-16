# Velké konflikty — příprava porady

An internal web app for preparing the weekly dev/analysis meeting about the feature
*Velké konflikty*. Problems are collected in a shared bucket during the week; before the
meeting the admin builds an ordered agenda and writes a short note on each item. The agenda
lives at a stable, login-free URL (`/porada/2026-w38`) that anyone on the internal network can
open and that gets projected in the room.

**UI is Czech; code, API, database and documentation are English.**

Source of truth: [`docs/PRD.md`](docs/PRD.md), [`api/openapi.yaml`](api/openapi.yaml) and the
design canvas in [`design/v1/`](design/v1).

---

## Running it

One executable, two environment variables, no runtime dependencies:

```bash
AUTH_USERS='admin:tajne:Petr Admin:admin;jan:heslo1:Jan Novák:editor;eva:heslo2:Eva Dvořáková:editor' DATA_DIR=./data ./konflikty-priprava.exe
```

The app is then on <http://localhost:9999> and the API alone on <http://localhost:9998>.
Backup is a copy of `DATA_DIR`, which holds `app.db` and `attachments/`.

Configuration is environment variables only — every option is listed in
[`.env.example`](.env.example) and explained in PRD §10. Two worth knowing:

- **`AUTH_USERS`** defines the accounts as `username:password:Display Name:role`, separated by
  `;`. No field may contain `:` or `;`, and start-up refuses an account that does not split
  into exactly four fields rather than guessing. Exactly one `admin` is expected.
- **`WEB_PORT=0`** suppresses the SPA listener. That is what development sets, because Vite
  already owns 9999.

---

## Development

Two terminals:

```bash
pwsh scripts/dev-api.ps1
```

```bash
npm --prefix web run dev
```

The first reads `.env` (falling back to `.env.example`), forces `WEB_PORT=0` and runs the Go
API on 9998. The second runs Vite on 9999, proxying `/api` to it.

The browser always talks to `/api` on its own origin — proxied by Vite in development, routed
by the SPA listener to the same in-process handler in production. Root-relative attachment
URLs therefore resolve identically in both, and the app needs no CORS.

### Building

```bash
pwsh scripts/build.ps1
```

The order is load-bearing: Vite writes into `internal/spa/dist`, which the Go binary embeds,
so building Go first produces an executable whose frontend is a placeholder page. The script
fails loudly if the frontend step produced no `index.html`.

A fresh clone builds before anyone has run npm: `internal/spa/dist/.gitkeep` is tracked so the
`go:embed` pattern matches something. Such a binary serves a short Czech page explaining how
to build the frontend, and its API works normally.

### Tests

```bash
go test ./...
```

Run `-race` on Linux or in CI — on Windows the race detector needs a C toolchain.

---

## Layout

```
cmd/server/          config → store → API handler → one or two listeners
internal/config/     every environment variable, with cross-field validation
internal/auth/       the three accounts, constant-time verify, session tokens
internal/store/      SQLite: schema, migrations, and all queries
internal/httpapi/    the REST API under /api
internal/uploads/    streaming multipart staging, commit and rollback
internal/search/     diacritics-insensitive filtering
internal/slug/       ISO-week meeting slugs
internal/spa/        the embedded frontend
web/                 React + TypeScript + Tailwind; builds into internal/spa/dist
```

---

## A few decisions worth knowing

- **The agenda URL is frozen at creation.** Changing a meeting's date moves the week label but
  never the slug, so `/porada/2026-w38` can legitimately show *Týden 39*. Links shared weeks
  ago keep working; that divergence is deliberate (PRD 5.2).
- **Notes belong to the agenda entry, not the problem.** A problem that comes back for a third
  week gets its own pair of notes each time, so every past agenda stays a faithful snapshot and
  the problem detail can show the whole series as a timeline.
- **A problem and its files are saved in one request.** Uploads are staged on disk, committed,
  and rolled back if the insert fails, so breaching any of the three caps (100 MB per file,
  20 files, 512 MB per submit) rejects the request whole and leaves no orphan row or file.
- **Attachments are files on disk, not BLOBs**, which is what lets videos be scrubbed: the API
  answers Range requests with `http.ServeContent`. `image/*` and `video/*` are served inline;
  everything else, **SVG included**, is a download with `nosniff`, so an uploaded document can
  never execute inside the app's own origin.
- **The text filter runs in Go, not SQL.** SQLite folds ASCII only, so it cannot match
  `reseni` against `řešení`; the candidate set is a few hundred rows by design.
- **Anonymous visitors read everything.** Mutating controls are absent rather than disabled,
  and no page ever redirects to a login prompt.

### Divergences from the specification

- **`GET /api/users` is not in `api/openapi.yaml`.** Records store the author's *username*
  (PRD 5.1) while every screen in the design shows the *display name*, and the contract
  offered no way to resolve one to the other. This public, read-only endpoint returns the
  configured accounts. Author names are already public (PRD decision log, Q12), and a name
  lookup is not the user management PRD non-goal N6 rules out. Worth folding into the spec.
- **`ServeMux` answers a method mismatch under `/api` with plain text**, so a wrong method
  escapes the JSON error envelope. Unknown *paths* under `/api` do return the envelope.
- **Files are unlinked after the transaction commits**, not inside it as PRD §9.4's comment
  suggests — SQLite and the filesystem share no transaction. The choice is between harmless
  orphan bytes and rows pointing at missing files; this takes the former.
