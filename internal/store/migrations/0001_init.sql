-- Initial schema: specification §1 (project graph, history, delivery,
-- model usage, label counters). Criticality and the computed "ready"
-- state are never stored (§1.1, §1.5). Dates are fixed-width UTC text
-- (T-7): 2006-01-02T15:04:05.000Z.

CREATE TABLE project (
    id            TEXT PRIMARY KEY,
    invitation_id TEXT,
    expires_at    TEXT,
    created_at    TEXT NOT NULL
);

CREATE TABLE object (
    project_id      TEXT NOT NULL REFERENCES project (id),
    id              TEXT NOT NULL,
    label           TEXT NOT NULL,
    type            TEXT NOT NULL,
    state           TEXT,
    current_version INTEGER NOT NULL,
    PRIMARY KEY (project_id, id),
    UNIQUE (project_id, label)
);

CREATE TABLE version (
    project_id  TEXT NOT NULL,
    object_id   TEXT NOT NULL,
    version     INTEGER NOT NULL,
    body        TEXT NOT NULL,
    fingerprint TEXT,
    created_at  TEXT NOT NULL,
    PRIMARY KEY (project_id, object_id, version),
    FOREIGN KEY (project_id, object_id) REFERENCES object (project_id, id)
);

CREATE TABLE edge (
    project_id TEXT NOT NULL,
    id         TEXT NOT NULL,
    type       TEXT NOT NULL,
    from_id    TEXT NOT NULL,
    to_id      TEXT NOT NULL,
    version    INTEGER NOT NULL,
    anchor     TEXT,
    current    INTEGER NOT NULL,
    PRIMARY KEY (project_id, id),
    FOREIGN KEY (project_id, from_id) REFERENCES object (project_id, id),
    FOREIGN KEY (project_id, to_id) REFERENCES object (project_id, id)
);

CREATE TABLE event (
    project_id       TEXT NOT NULL REFERENCES project (id),
    seq              INTEGER NOT NULL,
    type             TEXT NOT NULL,
    actor            TEXT,
    agent_role       TEXT,
    model_and_effort TEXT,
    reason           TEXT,
    object_id        TEXT,
    base_version     INTEGER,
    new_version      INTEGER,
    turn_id          TEXT,
    data             TEXT,
    created_at       TEXT NOT NULL,
    PRIMARY KEY (project_id, seq)
);

CREATE TABLE delivery (
    project_id  TEXT NOT NULL REFERENCES project (id),
    instance_id TEXT,
    turn        INTEGER,
    via         TEXT,
    object_id   TEXT,
    version     INTEGER,
    created_at  TEXT NOT NULL
);

CREATE TABLE model_call (
    project_id       TEXT NOT NULL REFERENCES project (id),
    instance_id      TEXT,
    turn_id          TEXT,
    role             TEXT,
    model_and_effort TEXT,
    input_tokens     INTEGER,
    output_tokens    INTEGER,
    context_window   INTEGER,
    input_items      TEXT,
    raw_items        TEXT,
    cost             REAL,
    created_at       TEXT NOT NULL
);

CREATE TABLE label_counter (
    project_id TEXT NOT NULL REFERENCES project (id),
    prefix     TEXT NOT NULL,
    next       INTEGER NOT NULL,
    PRIMARY KEY (project_id, prefix)
);
