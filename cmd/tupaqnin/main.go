// Command tupaqnin serves the tupaqnin API and embedded web page
// from a single binary (specification §6–§7).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/backup"
	"github.com/LaurenteEber/tupaqnin/internal/config"
	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/evidence"
	"github.com/LaurenteEber/tupaqnin/internal/memory"
	"github.com/LaurenteEber/tupaqnin/internal/pdfextract"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

func main() {
	// Hidden child for PDF extraction (B-9): a parser stuck in a
	// loop cannot be killed in-process, so it runs here, on stdin,
	// where the parent can kill it at the deadline.
	if len(os.Args) > 1 && os.Args[1] == pdfextract.Subcommand {
		os.Exit(pdfextract.RunChild(os.Stdin, os.Stdout, os.Stderr))
	}

	// Links by command on the same binary (B-92): no admin screen.
	if len(os.Args) > 1 && os.Args[1] == "invite" {
		os.Exit(runInvite(os.Args[2:], config.DataDir(), os.Stdout, os.Stderr))
	}

	// The check of a copy (B-93): it reads the named file only, so it needs
	// neither the configuration nor the data directory.
	if len(os.Args) > 1 && os.Args[1] == "backup-check" {
		os.Exit(runBackupCheck(os.Args[2:], os.Stdout, os.Stderr))
	}

	logger := config.NewLogger()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	// The database creates itself (schema and migrations) at startup.
	db, err := store.Open(context.Background(), cfg.DataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "store: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	// The model provider (B-6) and the token counter bound to the
	// principal's model (B-5). With the fake, the app fixtures are
	// replayed in filename order and no API key is needed.
	var prov provider.Provider
	var countTokens memory.TokenCounter
	switch cfg.Provider {
	case config.ProviderFake:
		fixtures, err := provider.AppFixtures()
		if err != nil {
			fmt.Fprintf(os.Stderr, "provider: %v\n", err)
			os.Exit(1)
		}
		// The readers of a round replay the 3.0 reader calls (backlog-r3 3.2).
		readers, err := provider.ReaderFixtures()
		if err != nil {
			fmt.Fprintf(os.Stderr, "provider: %v\n", err)
			os.Exit(1)
		}
		fake := provider.NewFake(append(fixtures, readers...)...)
		fake.Recorder = db
		fake.FitSignoff = true
		prov = fake
		countTokens = func(ctx context.Context, input []json.RawMessage) (int, error) {
			return fake.CountTokens(ctx, provider.PrincipalModel, input)
		}
	default:
		meta := provider.NewMeta(cfg.MetaAPIKey, db, logger)
		prov = meta
		countTokens = func(ctx context.Context, input []json.RawMessage) (int, error) {
			return meta.CountTokens(ctx, provider.PrincipalModel, input)
		}
	}

	hub := server.NewHub()
	runner := cycle.NewRunner(db, prov, countTokens, hub)
	runner.Logger = logger
	runner.ProjectCapUSD = cfg.SpendCapProjectUSD
	runner.SpendCapDayUSD = cfg.SpendCapDayUSD
	runner.SpendCapTotalUSD = cfg.SpendCapTotalUSD
	// P-114: a mode a restart left on ends here, before anything is served.
	if err := runner.EndModesLeftRunning(context.Background()); err != nil {
		logger.Error("ending the modes left running failed", "error", err.Error())
	}
	// The readers' evidence (B-60): one shared client; with the fake
	// provider, the embedded recordings, never the network (3.2).
	if cfg.Provider == config.ProviderFake {
		runner.Evidence = evidence.NewReplay()
	} else {
		runner.Evidence = evidence.New(evidence.Options{ContactEmail: cfg.ContactEmail, BaseURLs: cfg.EvidenceBaseURLs})
	}

	// No global WriteTimeout: streaming routes (specification §6) write
	// for a long time and set their own pacing and heartbeat.
	srv := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: server.NewWithAPI(logger, hub, db, runner, server.WithProjectCap(cfg.SpendCapProjectUSD),
			server.WithProvider(cfg.Provider), server.WithDiskFull(server.DiskFull(cfg.DataDir))),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The hourly cleanup of expired anonymous projects and sessions (B-88).
	go server.RunCleanup(ctx, db, logger, server.CleanupEvery, time.Now)
	// The daily copy of the database (B-93, specification §6).
	go backup.Run(ctx, db, backup.Dir(cfg.DataDir), logger, backup.Every, time.Now)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown", "error", err.Error())
		}
	}()

	logger.Info("tupaqnin listening", "port", cfg.Port, "provider", cfg.Provider)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(os.Stderr, "server: %v\n", err)
		os.Exit(1)
	}
	// The pending waits before the batch of your edits stop with the
	// server (B-36); a batch in flight is cancelled.
	runner.Close()
}
