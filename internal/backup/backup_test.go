package backup_test

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/backup"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

const secretName = "SECRET-PROJECT-NAME"

var day1 = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

func openDB(t *testing.T) (*store.DB, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(context.Background(), dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Writer.ExecContext(context.Background(),
		`INSERT INTO project (id, name, created_at) VALUES ('p1', ?, '2026-11-01T00:00:00.000Z')`, secretName); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return db, filepath.Join(dir, "backup")
}

func names(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	out := []string{}
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func wait(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// B-93: the copy lives under the data directory that TQ_DATA_DIR names.
func TestB93_DirIsUnderTheDataDir(t *testing.T) {
	if got, want := backup.Dir("/some/data"), filepath.Join("/some/data", "backup"); got != want {
		t.Fatalf("Dir: got %q, want %q", got, want)
	}
}

// B-93: one copy per day, named by the date, readable only by the app's
// user; a second run on the same day leaves it as it was.
func TestB93_OneCopyPerDayPrivateFiles(t *testing.T) {
	db, dir := openDB(t)
	ctx := context.Background()
	// A directory that already exists wider than it should is narrowed.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	created, err := backup.Once(ctx, db, dir, day1)
	if err != nil || !created {
		t.Fatalf("Once: created %v, err %v", created, err)
	}
	if got, want := names(t, dir), []string{"tq-20261101.db"}; !slices.Equal(got, want) {
		t.Fatalf("files: got %v, want %v", got, want)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("directory mode: got %o, want 700", perm)
	}
	path := filepath.Join(dir, "tq-20261101.db")
	info, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("file mode: got %o, want 600", perm)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Writer.ExecContext(ctx,
		`INSERT INTO project (id, created_at) VALUES ('p2', '2026-11-01T01:00:00.000Z')`); err != nil {
		t.Fatal(err)
	}
	created, err = backup.Once(ctx, db, dir, day1.Add(time.Hour))
	if err != nil || created {
		t.Fatalf("second Once the same day: created %v, err %v", created, err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("the second run the same day changed today's copy")
	}
}

// B-93 [inicial]: only the last 3 copies stay, by the date in their name;
// nothing else in the directory is touched.
func TestB93_OnlyTheLastThreeCopiesRemain(t *testing.T) {
	db, dir := openDB(t)
	ctx := context.Background()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.db"), []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	for i := range 5 {
		now := day1.AddDate(0, 0, i)
		if _, err := backup.Once(ctx, db, dir, now); err != nil {
			t.Fatalf("Once day %d: %v", i, err)
		}
		// The oldest copy has the newest modification time: the date in
		// the name decides, since a restored file has any time.
		stamp := day1.AddDate(0, 0, 100-i)
		path := filepath.Join(dir, "tq-"+now.Format("20060102")+".db")
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"notes.db", "tq-20261103.db", "tq-20261104.db", "tq-20261105.db"}
	if got := names(t, dir); !slices.Equal(got, want) {
		t.Fatalf("files: got %v, want %v", got, want)
	}
}

// B-93: VACUUM INTO fails if its target exists, and a half-written file is
// never counted as today's copy: what a crash left behind is replaced.
func TestB93_LeftoverTemporaryIsNotACopy(t *testing.T) {
	db, dir := openDB(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"tq-20261101.db.tmp", "tq-20261031.db.tmp"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("half written"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	created, err := backup.Once(context.Background(), db, dir, day1)
	if err != nil || !created {
		t.Fatalf("Once: created %v, err %v", created, err)
	}
	if got, want := names(t, dir), []string{"tq-20261101.db"}; !slices.Equal(got, want) {
		t.Fatalf("files: got %v, want %v", got, want)
	}
}

// B-93: a copy that fails leaves no file for today and deletes no old copy.
func TestB93_FailedCopyLeavesNothingAndKeepsTheOldOnes(t *testing.T) {
	db, dir := openDB(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	old := []string{"tq-20261001.db", "tq-20261002.db", "tq-20261003.db", "tq-20261004.db"}
	for _, name := range old {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	created, err := backup.Once(ctx, db, dir, day1)
	if err == nil || created {
		t.Fatalf("Once with a cancelled context: created %v, err %v", created, err)
	}
	if got := names(t, dir); !slices.Equal(got, old) {
		t.Fatalf("files: got %v, want %v", got, old)
	}
}

// B-93: the copy does not wait for the single writer, so an open write
// does not hold it up and the copy does not hold up the write; and it holds
// only what was committed.
func TestB93_CopyDoesNotNeedTheWriter(t *testing.T) {
	db, dir := openDB(t)
	ctx := context.Background()
	tx, err := db.Writer.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO project (id, created_at) VALUES ('open', '2026-11-01T01:00:00.000Z')`); err != nil {
		t.Fatal(err)
	}
	copyCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := backup.Once(copyCtx, db, dir, day1); err != nil {
		t.Fatalf("Once during an open write: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit after the copy: %v", err)
	}
	report, err := store.CheckCopy(ctx, filepath.Join(dir, "tq-20261101.db"))
	if err != nil {
		t.Fatalf("CheckCopy: %v", err)
	}
	if report.Projects != 1 {
		t.Fatalf("projects in the copy: got %d, want 1 (the open write is not committed)", report.Projects)
	}
}

// B-93: at start and on every tick, with the injected clock deciding the day.
func TestB93_RunCopiesAtStartAndOnEveryTick(t *testing.T) {
	db, dir := openDB(t)
	var day atomic.Int64
	now := func() time.Time { return day1.AddDate(0, 0, int(day.Add(1))-1) }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		backup.Run(ctx, db, dir, slog.New(slog.DiscardHandler), time.Millisecond, now)
	}()
	wait(t, "several days of copies", func() bool { return day.Load() >= 6 })
	cancel()
	<-done
	got := names(t, dir)
	if len(got) != backup.Keep {
		t.Fatalf("files: got %v, want %d", got, backup.Keep)
	}
	for _, name := range got {
		if !strings.HasPrefix(name, "tq-") || !strings.HasSuffix(name, ".db") {
			t.Fatalf("after a clean stop: unexpected file %q in %v", name, got)
		}
	}
}

// B-93: a failed copy logs without content and does not stop the loop: the
// next tick copies. A file where the directory goes makes every copy fail.
func TestB93_FailedCopyLogsWithoutContentAndTheLoopGoesOn(t *testing.T) {
	db, dir := openDB(t)
	if err := os.WriteFile(dir, []byte("in the way"), 0o600); err != nil {
		t.Fatal(err)
	}
	var log syncBuffer
	logger := slog.New(slog.NewJSONHandler(&log, nil))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		backup.Run(ctx, db, dir, logger, time.Millisecond, func() time.Time { return day1 })
	}()
	defer func() { cancel(); <-done }()
	wait(t, "the failure in the log", func() bool { return strings.Contains(log.String(), `"level":"ERROR"`) })
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	wait(t, "the copy after the failure", func() bool {
		_, err := os.Stat(filepath.Join(dir, "tq-20261101.db"))
		return err == nil
	})
	cancel()
	<-done
	if strings.Contains(log.String(), secretName) {
		t.Fatalf("the log carries content: %s", log.String())
	}
	if !strings.Contains(log.String(), `"msg":"backup"`) {
		t.Fatalf("the log does not name the backup: %s", log.String())
	}
}
