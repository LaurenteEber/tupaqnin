-- Project name (specification §1, B-18; backlog-r1 1.1). Migration
-- 0002 is reserved for the edge key change of B-8 (backlog-r1 1.3).
ALTER TABLE project ADD COLUMN name TEXT;
