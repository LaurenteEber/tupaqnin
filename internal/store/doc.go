// Package store is the SQLite store: schema, version-conditioned
// writes and the append-only history (specification §1–§2).
// It uses modernc.org/sqlite in WAL mode with a single writer
// (SetMaxOpenConns(1)) and a separate read pool; the schema is
// created by embedded migrations at startup (T-8). Every object
// belongs to a project (composite key, 128-bit random ids); the
// current state is stored together with its event in one transaction.
package store
