// zv:server-binary

// Command clipmind-server is the ClipMind web server: accounts, workspaces and
// one analysis library per workspace. It runs on a VPS, not on the desktop.
//
// Configuration comes from the environment:
//
//	CLIPMIND_ADDR          listen address (default 127.0.0.1:8090)
//	CLIPMIND_DATA_DIR      where catalog.db, signing.key and workspaces/ live (default ./clipmind-data)
//	CLIPMIND_PUBLIC_URL    address users open, used in email links (default http://localhost:3000)
//	CLIPMIND_MAIL          "log" (default, prints emails) or "smtp"
//	CLIPMIND_SMTP_ADDR     host:587 for smtp
//	CLIPMIND_SMTP_USER     smtp user
//	CLIPMIND_SMTP_PASS     smtp password
//	CLIPMIND_SMTP_FROM     sender address
//	CLIPMIND_TRUST_PROXY   "1" to read client addresses from X-Forwarded-For
//	CLIPMIND_CS2_PATH      cs2.exe or the linux cs2 binary of a CS2 install (SteamCMD app 730); maps and icons come from it
//	CLIPMIND_COMMUNITY_URL pro data server: empty for the default, "off" for none
//	CLIPMIND_FACEIT_API_KEY FACEIT Data API key; empty disables linking players to FACEIT
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rechedev9/cliphub/internal/clipmind"
)

func main() {
	if err := run(); err != nil {
		log.Printf("fatal: %v", err)
		os.Exit(1)
	}
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func mailerFromEnv() (clipmind.Mailer, error) {
	switch mode := env("CLIPMIND_MAIL", "log"); mode {
	case "log":
		log.Printf("mail: logging emails to this log; set CLIPMIND_MAIL=smtp to deliver them")
		return clipmind.LogMailer{}, nil
	case "smtp":
		m := clipmind.SMTPMailer{
			Addr:     os.Getenv("CLIPMIND_SMTP_ADDR"),
			Username: os.Getenv("CLIPMIND_SMTP_USER"),
			Password: os.Getenv("CLIPMIND_SMTP_PASS"),
			From:     os.Getenv("CLIPMIND_SMTP_FROM"),
		}
		if m.Addr == "" || m.From == "" {
			return nil, errors.New("CLIPMIND_MAIL=smtp needs CLIPMIND_SMTP_ADDR and CLIPMIND_SMTP_FROM")
		}
		return m, nil
	default:
		return nil, fmt.Errorf("unknown CLIPMIND_MAIL %q", mode)
	}
}

func run() error {
	mailer, err := mailerFromEnv()
	if err != nil {
		return err
	}
	srv, err := clipmind.New(clipmind.Config{
		DataDir:      env("CLIPMIND_DATA_DIR", "clipmind-data"),
		PublicURL:    env("CLIPMIND_PUBLIC_URL", "http://localhost:3000"),
		Mailer:       mailer,
		TrustProxy:   os.Getenv("CLIPMIND_TRUST_PROXY") == "1",
		CS2Path:      os.Getenv("CLIPMIND_CS2_PATH"),
		CommunityURL: os.Getenv("CLIPMIND_COMMUNITY_URL"),
		FaceitAPIKey: os.Getenv("CLIPMIND_FACEIT_API_KEY"),
	})
	if err != nil {
		return err
	}
	defer func() { _ = srv.Close() }()

	httpServer := &http.Server{
		Addr:              env("CLIPMIND_ADDR", "127.0.0.1:8090"),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       5 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() { errc <- httpServer.ListenAndServe() }()
	log.Printf("clipmind-server listening on %s", httpServer.Addr)
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdown)
}
