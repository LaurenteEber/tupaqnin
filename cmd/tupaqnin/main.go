// Command tupaqnin serves the tupaqnin API and embedded web page
// from a single binary (specification §6–§7).
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/server"
)

// resolvePort validates the PORT value: empty means the default 8080,
// anything else must be a plain TCP port number.
func resolvePort(raw string) (string, error) {
	if raw == "" {
		return "8080", nil
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("invalid PORT %q: want a number between 1 and 65535", raw)
	}
	return raw, nil
}

func main() {
	port, err := resolvePort(os.Getenv("PORT"))
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// No global WriteTimeout: streaming routes (specification §6) write
	// for a long time and set their own pacing and heartbeat.
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           server.New(),
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
			log.Printf("shutdown: %v", err)
		}
	}()

	log.Printf("tupaqnin listening on :%s", port)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server: %v", err)
	}
}
