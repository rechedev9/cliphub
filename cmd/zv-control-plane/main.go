package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rechedev9/cliphub/internal/controlplane"
)

func main() {
	if err := run(); err != nil {
		log.Printf("control plane: %v", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) == 3 && os.Args[1] == "--healthcheck" {
		return healthcheck(os.Args[2])
	}
	if len(os.Args) != 1 {
		return errors.New("usage: zv-control-plane [--healthcheck URL]")
	}
	address := envOr("CLIPHUB_CONTROL_ADDR", "127.0.0.1:8090")
	databasePath := envOr("CLIPHUB_CONTROL_DB", filepath.Join("data", "control-plane.db"))
	publicOrigin := envOr("CLIPHUB_PUBLIC_ORIGIN", "http://127.0.0.1:3000")
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o700); err != nil {
		return fmt.Errorf("create control database directory: %w", err)
	}
	store, err := controlplane.Open(context.Background(), databasePath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			log.Printf("close control database: %v", closeErr)
		}
	}()
	server, err := controlplane.NewServer(store, publicOrigin, log.Default())
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Printf("control plane listening on %s", address)
	return controlplane.Serve(ctx, address, server.Handler())
}

func healthcheck(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "http" || (parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "localhost") {
		return errors.New("healthcheck URL must use HTTP loopback")
	}
	client := &http.Client{Timeout: 3 * time.Second}
	// #nosec G704 -- the parsed destination is restricted above to an HTTP
	// loopback hostname; this CLI path is used only by the container healthcheck.
	response, err := client.Get(parsed.String())
	if err != nil {
		return fmt.Errorf("healthcheck request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck status: %d", response.StatusCode)
	}
	return nil
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
