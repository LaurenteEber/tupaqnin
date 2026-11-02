// Package backup is the daily copy of the database (B-93, specification
// §6, B3-22 row 5): a VACUUM INTO at start, if today has no copy, and every
// 24 h. The copies live on the data volume; the volume's snapshot takes them
// off the host.
package backup

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

const (
	// Keep is how many copies stay [inicial] (B-93).
	Keep = 3
	// Every is how often the copy runs (B-93).
	Every = 24 * time.Hour

	dirName = "backup"
	dirMode = 0o700
	// fileMode: a copy holds everything the database does.
	fileMode = 0o600
	// tmpSuffix never matches copyName, so a half-written file is never
	// taken for a copy nor counted among the ones kept.
	tmpSuffix = ".tmp"
)

var (
	copyName = regexp.MustCompile(`^tq-\d{8}\.db$`)
	tmpName  = regexp.MustCompile(`^tq-\d{8}\.db\.tmp$`)
)

// Dir is where the copies of a data directory live.
func Dir(dataDir string) string { return filepath.Join(dataDir, dirName) }

// Once makes today's copy if there is none and drops the ones past Keep.
// It reports whether it made one. A failure leaves no file for today and
// deletes no old copy.
//
// [EL DISEÑO NO LO DICE]: which day names the copy. Smallest choice: UTC,
// the app's stored dates; the hosting machine's zone would make it move.
func Once(ctx context.Context, db *store.DB, dir string, now time.Time) (bool, error) {
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return false, fmt.Errorf("create directory: %w", err)
	}
	// MkdirAll keeps the mode of a directory that already exists.
	if err := os.Chmod(dir, dirMode); err != nil {
		return false, fmt.Errorf("directory mode: %w", err)
	}
	if err := removeLeftovers(dir); err != nil {
		return false, err
	}
	final := filepath.Join(dir, "tq-"+now.UTC().Format("20060102")+".db")
	created := false
	switch _, err := os.Lstat(final); {
	case errors.Is(err, fs.ErrNotExist):
		if err := write(ctx, db, final); err != nil {
			return false, err
		}
		created = true
	case err != nil:
		return false, fmt.Errorf("look for today's copy: %w", err)
	}
	return created, prune(dir)
}

// write makes the copy under a temporary name and only then gives it its
// final one, so a copy under that name is always complete. VACUUM INTO
// accepts an empty target, which lets the file be private from its first byte.
func write(ctx context.Context, db *store.DB, final string) (err error) {
	tmp := final + tmpSuffix
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, fileMode)
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("create temporary file: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmp)
		}
	}()
	if err := db.VacuumInto(ctx, tmp); err != nil {
		return err
	}
	// VACUUM INTO does not sync its output: after a crash the rename could
	// survive without the bytes.
	if err := syncPath(tmp); err != nil {
		return fmt.Errorf("sync copy: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		return fmt.Errorf("name copy: %w", err)
	}
	if err := syncPath(filepath.Dir(final)); err != nil {
		return fmt.Errorf("sync directory: %w", err)
	}
	return nil
}

func syncPath(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return f.Sync()
}

// removeLeftovers clears what a crash left mid-copy: VACUUM INTO could not
// write over it, and it is no copy.
func removeLeftovers(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() && tmpName.MatchString(entry.Name()) {
			if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil {
				return fmt.Errorf("remove leftover: %w", err)
			}
		}
	}
	return nil
}

// prune keeps the newest Keep copies, by the date in the name: a file's
// modification time changes when it is restored or moved. It touches nothing
// but files named as copies.
func prune(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}
	copies := []string{}
	for _, entry := range entries {
		if entry.Type().IsRegular() && copyName.MatchString(entry.Name()) {
			copies = append(copies, entry.Name())
		}
	}
	slices.Sort(copies)
	for len(copies) > Keep {
		if err := os.Remove(filepath.Join(dir, copies[0])); err != nil {
			return fmt.Errorf("remove old copy: %w", err)
		}
		copies = copies[1:]
	}
	return nil
}

// Run copies at start and every `every`, until ctx ends. A failed copy is
// logged, without content, and does not stop the app: the next one tries again.
func Run(ctx context.Context, db *store.DB, dir string, logger *slog.Logger, every time.Duration, now func() time.Time) {
	copyOnce := func() {
		created, err := Once(ctx, db, dir, now())
		if err != nil {
			if ctx.Err() == nil {
				logger.Error("backup", "error", err.Error())
			}
			return
		}
		logger.Info("backup", "created", created)
	}
	copyOnce()
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			copyOnce()
		}
	}
}
