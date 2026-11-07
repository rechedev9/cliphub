// ClipHub community server: the pro reference measured from public pro demos
// and the opt-in anonymous rankings of Studio installs. It runs on the VPS,
// never inside the desktop installer.
//
//	cliphub-community serve             answer the public API on a loopback port
//	cliphub-community index [-max N]    index new pro demos and rebuild the reference
//	cliphub-community prepare-hltv      scan harvested HLTV archives into analyses (operator PC)
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	defaultAddress  = "127.0.0.1:8160"
	defaultDatasets = "cs2povarchive/cs2-demos,aldigobbler/cs2demos@tier1"
	defaultMaxDemos = 40
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("community stage=service class=fatal error=%v", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: cliphub-community serve | index [-max N] | prepare-hltv -downloads DIR -out DIR")
	}
	database := os.Getenv("CLIPHUB_COMMUNITY_DATABASE")
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	switch args[0] {
	case "serve":
		address := envDefault("CLIPHUB_COMMUNITY_ADDR", defaultAddress)
		if err := requireLoopback(address); err != nil {
			return err
		}
		s, err := openStore(database)
		if err != nil {
			return err
		}
		defer s.Close()
		return serve(ctx, s, address)
	case "index":
		fs := flag.NewFlagSet("index", flag.ContinueOnError)
		maxDemos := fs.Int("max", defaultMaxDemos, "most new demos to index in this run")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		s, err := openStore(database)
		if err != nil {
			return err
		}
		defer s.Close()
		inbox := os.Getenv("CLIPHUB_COMMUNITY_INBOX")
		hltv := os.Getenv("CLIPHUB_COMMUNITY_HLTV_DIR")
		ix := &indexer{store: s, http: &http.Client{Timeout: 30 * time.Minute}, workDir: envDefault("CLIPHUB_COMMUNITY_WORK_DIR", os.TempDir()), inboxDir: inbox,
			hltvDir: hltv, archiveDir: os.Getenv("CLIPHUB_COMMUNITY_ANALYSES_DIR")}
		datasets := parseDatasetSpecs(envDefault("CLIPHUB_COMMUNITY_DATASETS", defaultDatasets))
		return ix.run(ctx, indexOptions{datasets: datasets, inboxDir: inbox, hltvDir: hltv, maxDemos: *maxDemos})
	case "prepare-hltv":
		fs := flag.NewFlagSet("prepare-hltv", flag.ContinueOnError)
		downloads := fs.String("downloads", "", "folder of harvested <match id>.rar archives and their .json sidecars")
		out := fs.String("out", "", "folder for the prepared analyses")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *downloads == "" || *out == "" {
			return errors.New("prepare-hltv needs -downloads and -out")
		}
		return prepareHLTV(ctx, *downloads, *out, envDefault("CLIPHUB_COMMUNITY_WORK_DIR", os.TempDir()))
	}
	return fmt.Errorf("unknown command %q", args[0])
}

func serve(ctx context.Context, s *store, address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	server := &http.Server{
		Handler:           newAPI(s).routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	go purgeDaily(ctx, s)
	failed := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			failed <- err
		}
	}()
	log.Printf("community stage=service class=started address=%s", listener.Addr())
	select {
	case <-ctx.Done():
	case err := <-failed:
		return err
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdown)
}

func purgeDaily(ctx context.Context, s *store) {
	for {
		if n, err := s.purgeRankings(ctx, time.Now().UTC()); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("community stage=retention class=failed error=%v", err)
		} else if n > 0 {
			log.Printf("community stage=retention class=deleted lines=%d", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(24 * time.Hour):
		}
	}
}

func requireLoopback(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("address: %w", err)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() || port == "" {
		return errors.New("address must use a numeric loopback host")
	}
	return nil
}

func envDefault(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
