package main

import (
	"context"
	"fmt"
	"io"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// runBackupCheck is `tupaqnin backup-check <file>` (B-93): it says whether a
// copy can be restored, in counts and never in content, and exits non-zero if
// it cannot. The file is opened read-only and stays as it was.
func runBackupCheck(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || args[0] == "" {
		_, _ = fmt.Fprintln(stderr, "usage: tupaqnin backup-check <file>")
		return 2
	}
	report, err := store.CheckCopy(context.Background(), args[0])
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "backup-check: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintln(stdout, "integrity_check: ok")
	if report.SchemaVersion < report.LatestVersion {
		_, _ = fmt.Fprintf(stdout, "schema_version: %d (older than this binary's %d: opening it migrates it)\n",
			report.SchemaVersion, report.LatestVersion)
	} else {
		_, _ = fmt.Fprintf(stdout, "schema_version: %d\n", report.SchemaVersion)
	}
	_, _ = fmt.Fprintf(stdout, "projects: %d\naccounts: %d\nlinks: %d\n", report.Projects, report.Accounts, report.Links)
	return 0
}
