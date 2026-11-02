package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/backup"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

const (
	secretProject = "SECRET-PROJECT-NAME"
	secretLabel   = "SECRET-LINK-LABEL"
)

var backupNow = time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)

func check(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var out, errs bytes.Buffer
	code := runBackupCheck(args, &out, &errs)
	return out.String(), errs.String(), code
}

// seeded is a database with two projects, each with its own history.
type seeded struct {
	db       *store.DB
	dir      string
	sessions []store.Session
}

func seed(t *testing.T) seeded {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(ctx, dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	link, err := db.CreateInvitation(ctx,
		store.InvitationInput{Label: secretLabel, BudgetUSD: 10, MaxAccounts: 3, MaxAnonProjects: 5},
		store.HashSecret("token"), backupNow)
	if err != nil {
		t.Fatalf("CreateInvitation: %v", err)
	}
	s := seeded{db: db, dir: dir}
	for _, hash := range []string{"session-a", "session-b"} {
		session := store.Session{IDHash: hash, InvitationID: link}
		if err := db.CreateSession(ctx, hash, link, backupNow.Add(time.Hour), backupNow); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		project, err := db.CreateOwnedProject(ctx, session, secretProject, backupNow)
		if err != nil {
			t.Fatalf("CreateOwnedProject: %v", err)
		}
		if err := db.Write(ctx, project, func(tx *store.Tx) error {
			_, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{"statement": "use X"},
				store.EventInput{Type: store.EventStateChanged, Actor: "human"})
			return err
		}); err != nil {
			t.Fatalf("write history: %v", err)
		}
		s.sessions = append(s.sessions, session)
	}
	return s
}

// makeCopy takes the day's copy the way the app does and returns its path.
func (s seeded) makeCopy(t *testing.T) string {
	t.Helper()
	dir := backup.Dir(s.dir)
	if _, err := backup.Once(context.Background(), s.db, dir, backupNow); err != nil {
		t.Fatalf("backup.Once: %v", err)
	}
	return filepath.Join(dir, "tq-20261102.db")
}

// Specification §6, B3-17: with two projects, the copy passes backup-check
// and, opened as a database, lists the same projects with their history.
func TestSpec6_BackupRestores(t *testing.T) {
	ctx := context.Background()
	s := seed(t)
	path := s.makeCopy(t)

	out, errs, code := check(t, path)
	if code != 0 {
		t.Fatalf("backup-check: exit %d, stderr %q", code, errs)
	}
	for _, want := range []string{"integrity_check: ok", "schema_version: 5", "projects: 2", "accounts: 0", "links: 1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("backup-check output %q lacks %q", out, want)
		}
	}

	// The restore: the copy goes in a data directory under the database's name.
	restoredDir := t.TempDir()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(restoredDir, store.DBFileName), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	restored, err := store.Open(ctx, restoredDir)
	if err != nil {
		t.Fatalf("store.Open on the copy: %v", err)
	}
	defer func() { _ = restored.Close() }()

	for _, session := range s.sessions {
		want, err := s.db.ListOwnedProjects(ctx, session)
		if err != nil {
			t.Fatal(err)
		}
		got, err := restored.ListOwnedProjects(ctx, session)
		if err != nil {
			t.Fatalf("list on the copy: %v", err)
		}
		if len(want) != 1 || !reflect.DeepEqual(got, want) {
			t.Fatalf("projects: got %+v, want %+v", got, want)
		}
		wantEvents, err := s.db.EventsSince(ctx, want[0].ID, 0)
		if err != nil {
			t.Fatal(err)
		}
		gotEvents, err := restored.EventsSince(ctx, want[0].ID, 0)
		if err != nil {
			t.Fatalf("history on the copy: %v", err)
		}
		if len(wantEvents) == 0 || !reflect.DeepEqual(gotEvents, wantEvents) {
			t.Fatalf("history: got %+v, want %+v", gotEvents, wantEvents)
		}
	}
}

// B-93: a damaged file makes backup-check fail with a non-zero exit, and
// the failure shows no content.
func TestB93_DamagedFileFailsBackupCheck(t *testing.T) {
	s := seed(t)
	good, err := os.ReadFile(s.makeCopy(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(good) < 3*4096 {
		t.Fatalf("the copy is too small for this test: %d bytes", len(good))
	}
	middle := slices.Clone(good)
	copy(middle[len(good)/2/4096*4096:], bytes.Repeat([]byte{0xA5}, 4096))
	header := slices.Clone(good)
	copy(header, "this is not sqlite")
	cases := map[string][]byte{
		"a page overwritten": middle,
		"truncated":          good[:len(good)/2/4096*4096],
		"header overwritten": header,
		"empty":              {},
		"not a database":     []byte("just some text, long enough to be read as a file header by anything"),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "damaged.db")
			if err := os.WriteFile(path, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			out, errs, code := check(t, path)
			if code == 0 {
				t.Fatalf("backup-check on a damaged file: exit 0, stdout %q", out)
			}
			if errs == "" {
				t.Fatal("backup-check says nothing about the failure")
			}
			if strings.Contains(out+errs, secretProject) || strings.Contains(out+errs, secretLabel) {
				t.Fatalf("backup-check shows content: %q %q", out, errs)
			}
		})
	}
}

// B-93: backup-check shows counts, never content.
func TestB93_BackupCheckShowsNoContent(t *testing.T) {
	s := seed(t)
	out, errs, code := check(t, s.makeCopy(t))
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errs)
	}
	for _, secret := range []string{secretProject, secretLabel, store.HashSecret("token"), "use X", "session-a"} {
		if strings.Contains(out+errs, secret) {
			t.Fatalf("backup-check shows %q: %q %q", secret, out, errs)
		}
	}
}

type fileState struct {
	sum     [32]byte
	modTime time.Time
	entries []string
}

func stateOf(t *testing.T, path string) fileState {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	st := fileState{sum: sha256.Sum256(raw), modTime: info.ModTime()}
	for _, e := range entries {
		st.entries = append(st.entries, e.Name())
	}
	return st
}

// B-93: checking never alters the file and no WAL or journal appears beside
// it, even for a file whose header says WAL (a raw copy of the live file)
// and for a damaged one. The path may hold characters a URI treats specially.
func TestB93_BackupCheckLeavesTheFileAsItWas(t *testing.T) {
	s := seed(t)
	madeCopy, err := os.ReadFile(s.makeCopy(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.db.Close(); err != nil {
		t.Fatal(err)
	}
	live, err := os.ReadFile(filepath.Join(s.dir, store.DBFileName))
	if err != nil {
		t.Fatal(err)
	}
	if live[18] != 2 {
		t.Fatalf("the live file's header is not WAL: %d", live[18])
	}
	damaged := slices.Clone(madeCopy)
	copy(damaged[len(damaged)/2/4096*4096:], bytes.Repeat([]byte{0xA5}, 4096))
	cases := []struct {
		name string
		raw  []byte
		code int
	}{
		{"the copy", madeCopy, 0},
		{"wal header", live, 0},
		{"damaged", damaged, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "a b#c?d%20")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "copy.db")
			if err := os.WriteFile(path, c.raw, 0o600); err != nil {
				t.Fatal(err)
			}
			before := stateOf(t, path)
			if _, errs, code := check(t, path); code != c.code {
				t.Fatalf("exit %d, want %d, stderr %q", code, c.code, errs)
			}
			if after := stateOf(t, path); !reflect.DeepEqual(after, before) {
				t.Fatalf("the check changed the file or its directory: %+v, was %+v", after, before)
			}
		})
	}
}

// B-93: the schema version is checked. A copy from an older binary still
// passes, since opening it migrates it; one without a version, or newer than
// this binary, does not.
func TestB93_BackupCheckChecksTheSchemaVersion(t *testing.T) {
	ctx := context.Background()
	s := seed(t)
	raw, err := os.ReadFile(s.makeCopy(t))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		sql  string
		code int
		out  string
	}{
		{"older", `DELETE FROM schema_version WHERE version = 5`, 0, "schema_version: 4"},
		{"newer", `INSERT INTO schema_version (version) VALUES (9999)`, 1, ""},
		{"no versions", `DELETE FROM schema_version`, 1, ""},
		{"no table", `DROP TABLE schema_version`, 1, ""},
		{"not ours", `DROP TABLE account`, 1, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, store.DBFileName)
			if err := os.WriteFile(path, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := changeRaw(ctx, path, c.sql); err != nil {
				t.Fatalf("change the copy: %v", err)
			}
			out, errs, code := check(t, path)
			if code != c.code {
				t.Fatalf("exit %d, want %d, stdout %q, stderr %q", code, c.code, out, errs)
			}
			if !strings.Contains(out, c.out) {
				t.Fatalf("stdout %q lacks %q", out, c.out)
			}
		})
	}
}

func TestB93_BackupCheckUsageAndMissingFile(t *testing.T) {
	if _, errs, code := check(t); code != 2 || !strings.Contains(errs, "usage") {
		t.Fatalf("no file: exit %d, stderr %q", code, errs)
	}
	if _, errs, code := check(t, "a", "b"); code != 2 || !strings.Contains(errs, "usage") {
		t.Fatalf("two files: exit %d, stderr %q", code, errs)
	}
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.db")
	if _, _, code := check(t, missing); code != 1 {
		t.Fatalf("missing file: exit %d, want 1", code)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("the check created the missing file: %v", err)
	}
	if _, _, code := check(t, dir); code != 1 {
		t.Fatalf("a directory: exit %d, want 1", code)
	}
}

// Previous behavior holds: the database the app serves keeps working after a
// copy, and `invite` sees the same links before and after it.
func TestB92_InviteAndTheDatabaseHoldAfterACopy(t *testing.T) {
	ctx := context.Background()
	dir := inviteDir(t)
	if _, errs, code := invite(t, dir, "create", "--label", "jury"); code != 0 {
		t.Fatalf("create: exit %d, stderr %q", code, errs)
	}
	before, errs, code := invite(t, dir, "list")
	if code != 0 {
		t.Fatalf("list: exit %d, stderr %q", code, errs)
	}
	db, err := store.Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := backup.Once(ctx, db, backup.Dir(dir), backupNow); err != nil {
		t.Fatalf("backup.Once: %v", err)
	}
	after, errs, code := invite(t, dir, "list")
	if code != 0 || after != before {
		t.Fatalf("list after the copy: exit %d, stderr %q, got %q, want %q", code, errs, after, before)
	}
	if _, err := db.Writer.ExecContext(ctx,
		`INSERT INTO project (id, created_at) VALUES ('after', '2026-11-02T13:00:00.000Z')`); err != nil {
		t.Fatalf("write after the copy: %v", err)
	}
	if _, err := db.Reader.ExecContext(ctx,
		`INSERT INTO project (id, created_at) VALUES ('reader', '2026-11-02T13:00:00.000Z')`); err == nil {
		t.Fatal("the reader pool can write")
	}
}

// changeRaw edits a test's copy of a database behind the store's back, to
// build the cases the store itself would never write.
func changeRaw(ctx context.Context, path, stmt string) error {
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	_, err = db.ExecContext(ctx, stmt)
	return err
}
