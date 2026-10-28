// Command tupaqnin serves the tupaqnin API and embedded web page
// from a single binary (specification §6–§7).
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/server"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           server.New(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("tupaqnin listening on :%s", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}
}
