package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DBFileName is the database file inside the data directory: restoring a
// copy means putting it there under this name (B-93).
const DBFileName = dbFileName

// ErrCopyDamaged says a file failed PRAGMA integrity_check.
var ErrCopyDamaged = errors.New("integrity_check failed")

// CopyReport is what backup-check says about a copy: counts, never content.
type CopyReport struct {
	SchemaVersion int // the copy's
	LatestVersion int // this binary's
	Projects      int
	Accounts      int
	Links         int
}

// VacuumInto writes a consistent copy of the database to path (B-93,
// specification §6). VACUUM INTO refuses a target with content, so path must
// not exist or must be empty. The copy runs on a connection of its own: in
// WAL mode it only holds a read snapshot, so writers carry on meanwhile, and
// the single writer connection (T-8) is never taken for it.
func (db *DB) VacuumInto(ctx context.Context, path string) error {
	conn, err := sql.Open("sqlite", "file:"+db.path+"?_pragma=busy_timeout("+strconv.Itoa(busyTimeoutMS)+")")
	if err != nil {
		return fmt.Errorf("open for copy: %w", err)
	}
	defer func() { _ = conn.Close() }()
	conn.SetMaxOpenConns(1)
	if _, err := conn.ExecContext(ctx, `VACUUM INTO ?`, path); err != nil {
		return fmt.Errorf("vacuum into: %w", err)
	}
	return nil
}

// CheckCopy checks a copy of the database (B-93): PRAGMA integrity_check,
// the schema version, and how many projects, accounts and links it holds.
// immutable=1 makes SQLite read the file as it is: no lock, no WAL and no
// journal, so checking never alters it, whatever its header says. The errors
// name no row: integrity_check's own messages are only counted.
func CheckCopy(ctx context.Context, path string) (CopyReport, error) {
	info, err := os.Stat(path)
	if err != nil {
		return CopyReport{}, fmt.Errorf("open copy: %w", err)
	}
	if !info.Mode().IsRegular() {
		return CopyReport{}, errors.New("open copy: not a regular file")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return CopyReport{}, fmt.Errorf("open copy: %w", err)
	}
	// The path is escaped because SQLite reads this as a URI.
	dsn := "file:" + (&url.URL{Path: abs}).EscapedPath() + "?mode=ro&immutable=1&_pragma=query_only(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return CopyReport{}, fmt.Errorf("open copy: %w", err)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)

	if err := integrityCheck(ctx, db); err != nil {
		return CopyReport{}, err
	}
	report := CopyReport{}
	if report.LatestVersion, err = latestMigration(); err != nil {
		return CopyReport{}, err
	}
	var version sql.NullInt64
	if err := db.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_version`).Scan(&version); err != nil {
		return CopyReport{}, fmt.Errorf("schema version: %w", err)
	}
	if !version.Valid {
		return CopyReport{}, errors.New("schema version: none recorded")
	}
	report.SchemaVersion = int(version.Int64)
	// [EL DISEÑO NO LO DICE]: B-93 asks to check the schema version without
	// saying against what. Smallest choice: an older copy passes, since
	// opening it applies the missing migrations; one this binary cannot read,
	// being newer, fails.
	if report.SchemaVersion > report.LatestVersion {
		return CopyReport{}, fmt.Errorf("schema version: %d is newer than this binary's %d",
			report.SchemaVersion, report.LatestVersion)
	}
	for _, count := range []struct {
		table string
		into  *int
	}{{"project", &report.Projects}, {"account", &report.Accounts}, {"invitation", &report.Links}} {
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+count.table).Scan(count.into); err != nil {
			return CopyReport{}, fmt.Errorf("count %s: %w", count.table, err)
		}
	}
	return report, nil
}

// integrityCheck runs the pragma, whose single row is "ok" when the file is
// sound; otherwise each row describes one problem.
func integrityCheck(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `PRAGMA integrity_check`)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCopyDamaged, err)
	}
	defer func() { _ = rows.Close() }()
	problems := 0
	sound := false
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return fmt.Errorf("%w: %w", ErrCopyDamaged, err)
		}
		if line == "ok" {
			sound = true
		} else {
			problems++
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("%w: %w", ErrCopyDamaged, err)
	}
	if problems > 0 || !sound {
		return fmt.Errorf("%w: %d problems", ErrCopyDamaged, problems)
	}
	return nil
}

// latestMigration is the highest version this binary carries.
func latestMigration() (int, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return 0, fmt.Errorf("read migrations: %w", err)
	}
	latest := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version, err := strconv.Atoi(strings.SplitN(entry.Name(), "_", 2)[0])
		if err != nil {
			return 0, fmt.Errorf("migration %s: bad version prefix: %w", entry.Name(), err)
		}
		if version > latest {
			latest = version
		}
	}
	return latest, nil
}
