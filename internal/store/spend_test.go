package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

func openSpendDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func linkSpent(t *testing.T, db *store.DB, id string) float64 {
	t.Helper()
	list, err := db.ListInvitations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range list {
		if i.ID == id {
			return i.SpentUSD
		}
	}
	t.Fatalf("no link %s", id)
	return 0
}

// B3-22 row 2: spent_usd grows with each model_call, in its transaction,
// and does not fall when the expired project is deleted with its rows.
func TestB3_22_2_SpentGrowsAndDoesNotFallWithAPurge(t *testing.T) {
	db := openSpendDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	_, linkHash, _ := store.NewSecret()
	link, err := db.CreateInvitation(ctx, store.InvitationInput{Label: "guests", BudgetUSD: 10, MaxAccounts: 3, MaxAnonProjects: 5}, linkHash, now)
	if err != nil {
		t.Fatal(err)
	}
	_, hash, _ := store.NewSecret()
	if err := db.CreateSession(ctx, hash, link, now.Add(time.Hour), now); err != nil {
		t.Fatal(err)
	}
	project, err := db.CreateOwnedProject(ctx, store.Session{IDHash: hash, InvitationID: link}, "p", now)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []float64{0.25, 0.75} {
		if err := db.RecordModelCall(ctx, store.ModelCall{ProjectID: project, Role: "principal", CostUSD: 0.25 + float64(i)*0.25}); err != nil {
			t.Fatal(err)
		}
		if got := linkSpent(t, db, link); got != want {
			t.Fatalf("spent after call %d: %v, want %v", i+1, got, want)
		}
	}
	if projects, _, err := db.PurgeExpired(ctx, now.Add(store.AnonProjectTTL+time.Hour)); err != nil || projects != 1 {
		t.Fatalf("purge: %d, %v", projects, err)
	}
	if got := linkSpent(t, db, link); got != 0.75 {
		t.Fatalf("spent after the purge: %v, want 0.75", got)
	}
}

// A failed insert adds nothing, and a project without a link records as
// ever (an older local database).
func TestB3_22_2_RecordModelCallWithoutALink(t *testing.T) {
	db := openSpendDB(t)
	ctx := context.Background()
	if _, err := db.Writer.ExecContext(ctx, `INSERT INTO project (id, created_at) VALUES ('p1', '2026-10-28T00:00:00.000Z')`); err != nil {
		t.Fatal(err)
	}
	if err := db.RecordModelCall(ctx, store.ModelCall{ProjectID: "p1", Role: "principal", CostUSD: 0.5}); err != nil {
		t.Fatalf("RecordModelCall: %v", err)
	}
	spent, err := db.ProjectSpend(ctx, "p1")
	if err != nil || spent != 0.5 {
		t.Fatalf("spent %v, %v", spent, err)
	}
	if err := db.RecordModelCall(ctx, store.ModelCall{ProjectID: "nope", Role: "principal", CostUSD: 0.5}); err == nil {
		t.Fatal("a call of no project must fail")
	}
}
