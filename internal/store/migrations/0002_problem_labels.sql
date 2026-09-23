-- Problem labels (PRD 5.1).

-- A row per label a problem carries, rather than columns on `problems` or one
-- comma-separated string: a third label is then data rather than a migration,
-- and a label filter, should one be added, is a plain `EXISTS (... AND label = ?)`
-- like the `scheduled` filter over meeting_items. No index on `label` until such
-- a filter exists — the primary key already serves every read there is.
--
-- The vocabulary itself is closed and lives in Go (store.Label): a CHECK
-- constraint here would put the same two names in a second place, where
-- changing them means a migration rather than a constant.
CREATE TABLE problem_labels (
  problem_id INTEGER NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
  label      TEXT    NOT NULL,
  PRIMARY KEY (problem_id, label)
);
