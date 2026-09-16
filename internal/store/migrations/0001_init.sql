-- Initial schema (PRD 9.4).

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
CREATE INDEX idx_problems_created ON problems(created_at DESC);

CREATE TABLE meetings (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  slug         TEXT    NOT NULL UNIQUE,
  meeting_date TEXT    NOT NULL,            -- YYYY-MM-DD
  -- no iso_year / iso_week columns: they are derived from meeting_date on read.
  note         TEXT    NOT NULL DEFAULT '',
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
-- Deliberately no UNIQUE(meeting_id, position): it would force every reorder
-- into a two-pass offset dance for no benefit.
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
-- ON DELETE CASCADE removes attachment rows, but SQLite cannot remove the bytes.
-- Deleting a problem collects its storage_names inside the transaction and the
-- handler unlinks the files once the commit succeeds.

CREATE TABLE sessions (
  token      TEXT PRIMARY KEY,              -- opaque, 32 random bytes hex
  username   TEXT NOT NULL,
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL
);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);
