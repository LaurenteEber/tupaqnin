package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite" // SQLite driver (specification §7)
)

//go:embed migrations
var migrationsFS embed.FS

// dbFileName is the SQLite file inside the data directory.
// [EL DISEÑO NO LO DICE]: the design names the data directory
// (TQ_DATA_DIR) but not the file, so this is the smallest choice.
const dbFileName = "tupaqnin.db"

// busyTimeoutMS is applied to every connection at open time.
// [EL DISEÑO NO LO DICE]: T-8 requires a busy_timeout pragma without
// naming a value.
const busyTimeoutMS = 5000

// DB holds the two connection pools over one SQLite file: a single
// writer (SetMaxOpenConns(1)) and a reader pool (T-8, §7).
type DB struct {
	Writer *sql.DB
	Reader *sql.DB
}

// Open creates the data directory if needed, opens the writer and
// reader pools with their pragmas (WAL, foreign keys, busy timeout)
// and applies every pending embedded migration, each in its own
// transaction (T-8).
func Open(ctx context.Context, dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	dsn := "file:" + filepath.Join(dataDir, dbFileName) +
		"?_pragma=busy_timeout(" + strconv.Itoa(busyTimeoutMS) + ")" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=journal_mode(WAL)"

	writer, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open writer: %w", err)
	}
	writer.SetMaxOpenConns(1)
	if err := writer.PingContext(ctx); err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("ping writer: %w", err)
	}
	if err := applyMigrations(ctx, writer); err != nil {
		_ = writer.Close()
		return nil, err
	}

	reader, err := sql.Open("sqlite", dsn)
	if err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("open reader: %w", err)
	}
	if err := reader.PingContext(ctx); err != nil {
		_ = reader.Close()
		_ = writer.Close()
		return nil, fmt.Errorf("ping reader: %w", err)
	}

	return &DB{Writer: writer, Reader: reader}, nil
}

// Close closes both pools.
func (db *DB) Close() error {
	return errors.Join(db.Reader.Close(), db.Writer.Close())
}

func applyMigrations(ctx context.Context, writer *sql.DB) error {
	if _, err := writer.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER PRIMARY KEY)`); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}

	applied, err := appliedVersions(ctx, writer)
	if err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		version, err := strconv.Atoi(strings.SplitN(name, "_", 2)[0])
		if err != nil {
			return fmt.Errorf("migration %s: bad version prefix: %w", name, err)
		}
		if applied[version] {
			continue
		}
		body, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if err := applyMigration(ctx, writer, version, string(body)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}

func appliedVersions(ctx context.Context, writer *sql.DB) (map[int]bool, error) {
	rows, err := writer.QueryContext(ctx, `SELECT version FROM schema_version`)
	if err != nil {
		return nil, fmt.Errorf("read schema_version: %w", err)
	}
	defer func() { _ = rows.Close() }()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("read schema_version: %w", err)
		}
		applied[version] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read schema_version: %w", err)
	}
	return applied, nil
}

func applyMigration(ctx context.Context, writer *sql.DB, version int, body string) error {
	tx, err := writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, body); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_version (version) VALUES (?)`, version); err != nil {
		return err
	}
	return tx.Commit()
}
