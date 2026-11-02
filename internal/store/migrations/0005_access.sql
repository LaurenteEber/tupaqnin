-- Links, accounts and sessions, and the owner of each project
-- (specification §1.7, B3-18, B3-22; backlog-entrega E.1). No data is
-- dropped: projects from before this migration keep their rows, have no
-- owner, and are not reachable from the web (B-87).
-- [EL DISEÑO NO LO DICE] choices made here, kept minimal: SQLite types;
-- NULL for the «vacío» account_id; foreign keys to invitation and account.
CREATE TABLE invitation (
    id                TEXT PRIMARY KEY,
    token_hash        TEXT NOT NULL UNIQUE,
    label             TEXT NOT NULL,
    budget_usd        REAL NOT NULL,
    spent_usd         REAL NOT NULL DEFAULT 0,
    max_accounts      INTEGER NOT NULL,
    max_anon_projects INTEGER NOT NULL,
    revoked_at        TEXT,
    created_at        TEXT NOT NULL
);

CREATE TABLE account (
    id            TEXT PRIMARY KEY,
    invitation_id TEXT NOT NULL REFERENCES invitation (id),
    username      TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TEXT NOT NULL
);

CREATE UNIQUE INDEX account_username ON account (username);

CREATE TABLE session (
    id_hash       TEXT PRIMARY KEY,
    account_id    TEXT REFERENCES account (id),
    invitation_id TEXT NOT NULL REFERENCES invitation (id),
    expires_at    TEXT NOT NULL,
    created_at    TEXT NOT NULL
);

ALTER TABLE project ADD COLUMN account_id TEXT REFERENCES account (id);
ALTER TABLE project ADD COLUMN anon_session_hash TEXT;

CREATE INDEX project_account_id ON project (account_id);
