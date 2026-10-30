-- B-45 (backlog-r2 2.2): what a using edge used (used_in, covers,
-- uses), so a change in a decision, a figure or a source finds the
-- sections that used another version (A-1). Existing rows keep NULL.
ALTER TABLE edge ADD COLUMN data TEXT;
