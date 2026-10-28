// Command tupaqnin serves the tupaqnin API and embedded web page
// from a single binary (specification §6–§7).
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/config"
	"github.com/LaurenteEber/tupaqnin/internal/server"
)

func main() {
	logger := config.NewLogger()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	// No global WriteTimeout: streaming routes (specification §6) write
	// for a long time and set their own pacing and heartbeat.
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.New(logger),
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
