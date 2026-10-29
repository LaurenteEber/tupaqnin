-- B-8 (backlog.md; backlog-r1 1.3): an edge is changed or ended, never
-- copied. ChangeEdge writes the next version with the same id, so the
-- edge key becomes (project_id, id, version). The table is rebuilt
-- (SQLite cannot change a primary key) and every row is kept.
CREATE TABLE edge_v2 (
    project_id TEXT NOT NULL,
    id         TEXT NOT NULL,
    type       TEXT NOT NULL,
    from_id    TEXT NOT NULL,
    to_id      TEXT NOT NULL,
    version    INTEGER NOT NULL,
    anchor     TEXT,
    current    INTEGER NOT NULL,
    PRIMARY KEY (project_id, id, version),
    FOREIGN KEY (project_id, from_id) REFERENCES object (project_id, id),
    FOREIGN KEY (project_id, to_id) REFERENCES object (project_id, id)
);

INSERT INTO edge_v2 (project_id, id, type, from_id, to_id, version, anchor, current)
SELECT project_id, id, type, from_id, to_id, version, anchor, current FROM edge;

DROP TABLE edge;

ALTER TABLE edge_v2 RENAME TO edge;
