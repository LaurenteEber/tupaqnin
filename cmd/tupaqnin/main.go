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

	"github.com/LaurenteEber/tupaqnin/internal/config"
	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/memory"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

func main() {
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
		fake := provider.NewFake(fixtures...)
		fake.Recorder = db
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

	// No global WriteTimeout: streaming routes (specification §6) write
	// for a long time and set their own pacing and heartbeat.
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.NewWithAPI(logger, hub, db, runner),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
}
