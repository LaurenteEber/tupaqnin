package main

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

func invite(t *testing.T, dir string, args ...string) (string, string, int) {
	t.Helper()
	var out, errs bytes.Buffer
	code := runInvite(args, dir, &out, &errs)
	return out.String(), errs.String(), code
}

// B-92: `invite create` prints the token once; the database keeps its
// SHA-256 and `invite list` shows what the link has, never the token.
func TestB92_InviteCreatePrintsTheTokenOnceAndListHidesIt(t *testing.T) {
	dir := t.TempDir()
	out, errs, code := invite(t, dir, "create", "--label", "jury", "--budget", "30", "--accounts", "10", "--anon", "10")
	if code != 0 {
		t.Fatalf("create: exit %d, stderr %q", code, errs)
	}
	m := regexp.MustCompile(`^/i/([A-Za-z0-9_-]{43})\n$`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("create output: %q, want one line /i/<token>", out)
	}
	token := m[1]
	if strings.Contains(errs, token) {
		t.Fatalf("the token is in stderr: %q", errs)
	}

	listed, errs, code := invite(t, dir, "list")
	if code != 0 {
		t.Fatalf("list: exit %d, stderr %q", code, errs)
	}
	if strings.Contains(listed, token) || strings.Contains(listed, store.HashSecret(token)) {
		t.Fatalf("list shows the token or its hash: %q", listed)
	}
	for _, want := range []string{"jury", "30.00", "0.00", "0/10", "0/10"} {
		if !strings.Contains(listed, want) {
			t.Fatalf("list %q lacks %q", listed, want)
		}
	}

	ctx := context.Background()
	db, err := store.Open(ctx, dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	id, ok, err := db.ActiveInvitationByTokenHash(ctx, store.HashSecret(token))
	if err != nil || !ok {
		t.Fatalf("the link is not stored by its hash: %v, %v", ok, err)
	}
	links, err := db.ListInvitations(ctx)
	if err != nil || len(links) != 1 || links[0].BudgetUSD != 30 || links[0].MaxAccounts != 10 || links[0].MaxAnonProjects != 10 {
		t.Fatalf("stored link: %+v, %v", links, err)
	}
	if !strings.Contains(listed, id) {
		t.Fatalf("list %q does not name the id revoke needs", listed)
	}
}

// B-92, B3-22 row 2: `invite revoke` ends the link and list says so.
func TestB92_InviteRevoke(t *testing.T) {
	dir := t.TempDir()
	out, _, code := invite(t, dir, "create", "--label", "guests")
	if code != 0 {
		t.Fatalf("create: exit %d", code)
	}
	token := strings.TrimSpace(strings.TrimPrefix(out, "/i/"))
	ctx := context.Background()
	db, err := store.Open(ctx, dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	id, _, err := db.ActiveInvitationByTokenHash(ctx, store.HashSecret(token))
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if _, errs, code := invite(t, dir, "revoke", id); code != 0 {
		t.Fatalf("revoke: exit %d, stderr %q", code, errs)
	}
	if _, ok, err := db.ActiveInvitationByTokenHash(ctx, store.HashSecret(token)); err != nil || ok {
		t.Fatalf("a revoked link is active: %v, %v", ok, err)
	}
	if listed, _, _ := invite(t, dir, "list"); !strings.Contains(listed, "revoked") {
		t.Fatalf("list %q does not say revoked", listed)
	}
	if _, errs, code := invite(t, dir, "revoke", id); code == 0 || errs == "" {
		t.Fatalf("second revoke: exit %d, stderr %q", code, errs)
	}
}

// §1.7: the defaults of a link, by label.
func TestB3_22_2_InviteDefaultsByLabel(t *testing.T) {
	dir := t.TempDir()
	for _, label := range []string{"jury", "guests"} {
		if _, errs, code := invite(t, dir, "create", "--label", label); code != 0 {
			t.Fatalf("create %s: exit %d, stderr %q", label, code, errs)
		}
	}
	ctx := context.Background()
	db, err := store.Open(ctx, dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	links, err := db.ListInvitations(ctx)
	if err != nil || len(links) != 2 {
		t.Fatalf("links: %+v, %v", links, err)
	}
	for _, l := range links {
		want := store.Invitation{BudgetUSD: 10, MaxAccounts: 3, MaxAnonProjects: 5}
		if l.Label == "jury" {
			want = store.Invitation{BudgetUSD: 30, MaxAccounts: 10, MaxAnonProjects: 10}
		}
		if l.BudgetUSD != want.BudgetUSD || l.MaxAccounts != want.MaxAccounts || l.MaxAnonProjects != want.MaxAnonProjects {
			t.Fatalf("%s: %+v", l.Label, l)
		}
	}
}

// A wrong command says so and creates nothing.
func TestB92_InviteRefusesBadArguments(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{
		nil,
		{"nope"},
		{"create"},
		{"create", "--label", "x", "--budget", "-1"},
		{"create", "--label", "x", "--accounts", "-1"},
		{"create", "--label", "x", "--anon", "-1"},
		{"create", "--label", "x", "--nope", "1"},
		{"revoke"},
		{"list", "extra"},
		{"list", "--json"},
	} {
		out, errs, code := invite(t, dir, args...)
		if code == 0 || errs == "" || strings.Contains(out, "/i/") {
			t.Fatalf("%v: exit %d, stdout %q, stderr %q", args, code, out, errs)
		}
	}
	if listed, _, _ := invite(t, dir, "list"); strings.Contains(listed, "/i/") || strings.Count(listed, "\n") > 1 {
		t.Fatalf("list after refused commands: %q", listed)
	}
}
