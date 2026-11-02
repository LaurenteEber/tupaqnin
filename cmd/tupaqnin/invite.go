package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// labelJury names the jury's link (§1.7; B-89 reads this label too).
const labelJury = store.LabelJury

// runInvite is `tupaqnin invite create|list|revoke` (B-92, B3-22 row 2):
// the links are made here, on the server; there is no admin screen.
func runInvite(args []string, dataDir string, stdout, stderr io.Writer) int {
	const usage = "usage: tupaqnin invite create --label L [--budget USD] [--accounts N] [--anon N] | list | revoke <id>"
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, usage)
		return 2
	}
	switch args[0] {
	case "create":
		return inviteCreate(args[1:], dataDir, stdout, stderr)
	case "list":
		if len(args) != 1 {
			_, _ = fmt.Fprintln(stderr, usage)
			return 2
		}
		return inviteList(dataDir, stdout, stderr)
	case "revoke":
		if len(args) != 2 {
			_, _ = fmt.Fprintln(stderr, usage)
			return 2
		}
		return inviteRevoke(args[1], dataDir, stderr)
	}
	_, _ = fmt.Fprintln(stderr, usage)
	return 2
}

// openForInvite opens the database that exists, and refuses otherwise
// (P-113): a shell that did not inherit TQ_DATA_DIR must not create a second
// one, whose links the app would never see.
func openForInvite(dataDir string, stderr io.Writer) (*store.DB, bool) {
	path := store.DatabasePath(dataDir)
	if _, err := os.Stat(path); err != nil {
		_, _ = fmt.Fprintf(stderr, "invite: no database at %s: start the app first, or set TQ_DATA_DIR to its data directory\n", path)
		return nil, false
	}
	db, err := store.Open(context.Background(), dataDir)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "store: %v\n", err)
		return nil, false
	}
	return db, true
}

// inviteCreate prints the link once: the database only keeps its hash.
func inviteCreate(args []string, dataDir string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("invite create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	label := fs.String("label", "", "who the link is for: jury or another name")
	budget := fs.Float64("budget", -1, "the link's balance in USD (default by label, §1.7)")
	accounts := fs.Int("accounts", -1, "most accounts (default by label, §1.7)")
	anon := fs.Int("anon", -1, "most projects without an account (default by label, §1.7)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *label == "" || fs.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "invite create: --label is required")
		return 2
	}
	// §1.7's defaults: the jury's, or the guests' for any other label.
	// [EL DISEÑO NO LO DICE]: --label is required; only "jury" is a label
	// the design fixes (B-89).
	in := store.InvitationInput{Label: *label, BudgetUSD: 10, MaxAccounts: 3, MaxAnonProjects: 5}
	if *label == labelJury {
		in = store.InvitationInput{Label: *label, BudgetUSD: 30, MaxAccounts: 10, MaxAnonProjects: 10}
	}
	var bad bool
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "budget":
			in.BudgetUSD, bad = *budget, bad || *budget < 0
		case "accounts":
			in.MaxAccounts, bad = *accounts, bad || *accounts < 0
		case "anon":
			in.MaxAnonProjects, bad = *anon, bad || *anon < 0
		}
	})
	if bad {
		_, _ = fmt.Fprintln(stderr, "invite create: --budget, --accounts and --anon cannot be negative")
		return 2
	}
	db, ok := openForInvite(dataDir, stderr)
	if !ok {
		return 1
	}
	defer func() { _ = db.Close() }()
	token, hash, err := store.NewSecret()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "invite create: %v\n", err)
		return 1
	}
	if _, err := db.CreateInvitation(context.Background(), in, hash, time.Now()); err != nil {
		_, _ = fmt.Fprintf(stderr, "invite create: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "/i/%s\n", token)
	return 0
}

func inviteList(dataDir string, stdout, stderr io.Writer) int {
	db, ok := openForInvite(dataDir, stderr)
	if !ok {
		return 1
	}
	defer func() { _ = db.Close() }()
	links, err := db.ListInvitations(context.Background())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "invite list: %v\n", err)
		return 1
	}
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tLABEL\tBUDGET\tSPENT\tACCOUNTS\tPROJECTS\tSTATE")
	for _, l := range links {
		state := "active"
		if l.RevokedAt != "" {
			state = "revoked"
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%.2f\t%.2f\t%d/%d\t%d/%d\t%s\n", l.ID, l.Label, l.BudgetUSD, l.SpentUSD,
			l.Accounts, l.MaxAccounts, l.Projects, l.MaxAnonProjects, state)
	}
	if err := w.Flush(); err != nil {
		return 1
	}
	return 0
}

func inviteRevoke(id, dataDir string, stderr io.Writer) int {
	db, ok := openForInvite(dataDir, stderr)
	if !ok {
		return 1
	}
	defer func() { _ = db.Close() }()
	err := db.RevokeInvitation(context.Background(), id, time.Now())
	if errors.Is(err, store.ErrInvitationNotFound) {
		_, _ = fmt.Fprintln(stderr, "invite revoke: no active link with that id")
		return 1
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "invite revoke: %v\n", err)
		return 1
	}
	return 0
}
