// ClipHub telemetry collector. This binary is deployed separately from the
// Windows-local product and is never included in the desktop installer.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/rechedev9/cliphub/internal/telemetry"
)

const (
	defaultPublicAddress = "127.0.0.1:8120"
	defaultAdminAddress  = "127.0.0.1:8121"
	defaultRetentionDays = 30
)

type config struct {
	publicAddress string
	adminAddress  string
	databasePath  string
	ingestKey     string
	adminToken    string
	retention     time.Duration
	proxyProtocol bool
}

func main() {
	if err := run(); err != nil {
		log.Printf("telemetry stage=service class=fatal error=%v", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	store, err := telemetry.OpenStore(cfg.databasePath)
	if err != nil {
		return err
	}
	defer store.Close()
	api, err := telemetry.NewAPI(store, cfg.ingestKey, cfg.adminToken)
	if err != nil {
		return err
	}
	publicListener, err := net.Listen("tcp", cfg.publicAddress)
	if err != nil {
		return fmt.Errorf("listen public: %w", err)
	}
	if cfg.proxyProtocol {
		publicListener = proxyV2Listener{Listener: publicListener}
	}
	defer publicListener.Close()
	adminListener, err := net.Listen("tcp", cfg.adminAddress)
	if err != nil {
		return fmt.Errorf("listen admin: %w", err)
	}
	defer adminListener.Close()

	publicServer := newServer(api.PublicHandler())
	adminServer := newServer(api.AdminHandler())
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	retentionDone := make(chan struct{})
	go runRetention(ctx, store, cfg.retention, retentionDone)

	serveErrors := make(chan error, 2)
	go serve("public", publicServer, publicListener, serveErrors)
	go serve("admin", adminServer, adminListener, serveErrors)
	log.Printf(
		"telemetry stage=service class=started public=%s admin=%s retention_days=%d proxy_protocol=%t",
		cfg.publicAddress,
		cfg.adminAddress,
		int(cfg.retention/(24*time.Hour)),
		cfg.proxyProtocol,
	)

	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-serveErrors:
		stop()
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	shutdownErr := errors.Join(publicServer.Shutdown(shutdownCtx), adminServer.Shutdown(shutdownCtx))
	<-retentionDone
	if serveErr != nil {
		return errors.Join(serveErr, shutdownErr)
	}
	return shutdownErr
}

func newServer(handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
}

func serve(name string, server *http.Server, listener net.Listener, failures chan<- error) {
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		failures <- fmt.Errorf("serve %s: %w", name, err)
	}
}

func runRetention(ctx context.Context, store *telemetry.Store, retention time.Duration, done chan<- struct{}) {
	defer close(done)
	deleteExpired := func() {
		deleted, err := store.DeleteBefore(ctx, time.Now().UTC().Add(-retention))
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("telemetry stage=retention class=delete_failed error=%v", err)
			return
		}
		if deleted > 0 {
			log.Printf("telemetry stage=retention class=deleted count=%d", deleted)
		}
	}
	deleteExpired()
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			deleteExpired()
		}
	}
}

func loadConfig() (config, error) {
	retentionDays := defaultRetentionDays
	if raw := os.Getenv("CLIPHUB_TELEMETRY_RETENTION_DAYS"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 365 {
			return config{}, errors.New("CLIPHUB_TELEMETRY_RETENTION_DAYS must be between 1 and 365")
		}
		retentionDays = value
	}
	cfg := config{
		publicAddress: envDefault("CLIPHUB_TELEMETRY_PUBLIC_ADDR", defaultPublicAddress),
		adminAddress:  envDefault("CLIPHUB_TELEMETRY_ADMIN_ADDR", defaultAdminAddress),
		databasePath:  os.Getenv("CLIPHUB_TELEMETRY_DATABASE"),
		ingestKey:     os.Getenv("CLIPHUB_TELEMETRY_INGEST_KEY"),
		adminToken:    os.Getenv("CLIPHUB_TELEMETRY_ADMIN_TOKEN"),
		retention:     time.Duration(retentionDays) * 24 * time.Hour,
		proxyProtocol: os.Getenv("CLIPHUB_TELEMETRY_PROXY_PROTOCOL") == "v2",
	}
	proxyProtocol := os.Getenv("CLIPHUB_TELEMETRY_PROXY_PROTOCOL")
	if proxyProtocol != "" && proxyProtocol != "v2" {
		return config{}, errors.New("CLIPHUB_TELEMETRY_PROXY_PROTOCOL must be empty or v2")
	}
	if cfg.databasePath == "" {
		return config{}, errors.New("CLIPHUB_TELEMETRY_DATABASE is required")
	}
	if len(cfg.ingestKey) < 24 {
		return config{}, errors.New("CLIPHUB_TELEMETRY_INGEST_KEY must contain at least 24 characters")
	}
	if len(cfg.adminToken) < 32 {
		return config{}, errors.New("CLIPHUB_TELEMETRY_ADMIN_TOKEN must contain at least 32 characters")
	}
	if retentionDays != defaultRetentionDays {
		return config{}, errors.New("CLIPHUB_TELEMETRY_RETENTION_DAYS must remain 30")
	}
	if cfg.publicAddress == cfg.adminAddress {
		return config{}, errors.New("public and admin addresses must differ")
	}
	if err := requireLoopbackAddress("public", cfg.publicAddress); err != nil {
		return config{}, err
	}
	if err := requireLoopbackAddress("admin", cfg.adminAddress); err != nil {
		return config{}, err
	}
	return cfg, nil
}

func requireLoopbackAddress(name, address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%s address: %w", name, err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() || port == "" {
		return fmt.Errorf("%s address must use a numeric loopback host", name)
	}
	return nil
}

func envDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
