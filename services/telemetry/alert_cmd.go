package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/rechedev9/cliphub/internal/telemetryalert"
)

// runAlert is `cliphub-telemetry alert [--bootstrap] [--dry-run]`, a oneshot
// run by its own systemd timer with egress allowed. It never opens the
// collector databases; it reads the loopback admin API.
func runAlert(args []string) int {
	flags := flag.NewFlagSet("alert", flag.ContinueOnError)
	bootstrap := flags.Bool("bootstrap", false, "ingest the retained 30 days and mark every issue as known without notifying")
	dryRun := flags.Bool("dry-run", false, "print the alerts this run would send and keep no state")
	if err := flags.Parse(args); err != nil || flags.NArg() > 0 {
		return 2
	}
	// A run that cannot start still tells healthchecks why, so check A goes
	// down with a reason instead of only after its grace period.
	failHard := func(reason string) int {
		if url := os.Getenv("CLIPHUB_ALERT_DEADMAN_URL"); url != "" && !*dryRun {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = telemetryalert.PingDeadman(ctx, &http.Client{Timeout: 10 * time.Second}, url, []string{reason})
		}
		return 1
	}
	cfg, err := telemetryalert.LoadConfig(os.Getenv, !*bootstrap && !*dryRun)
	if err != nil {
		log.Printf("telemetry-alert stage=config class=invalid error=%v", err)
		return failHard("config")
	}
	result, err := telemetryalert.Run(context.Background(), cfg, telemetryalert.Options{
		Bootstrap: *bootstrap,
		DryRun:    *dryRun,
		Out:       os.Stdout,
	})
	if err != nil {
		log.Printf("telemetry-alert stage=run class=failed error=%v", err)
		return failHard("run")
	}
	// Partial failures were already reported to the dead-man check; the unit
	// itself stays successful so the journal is not flooded with failed runs.
	log.Printf("telemetry-alert stage=run class=done bootstrap=%t alerts=%d delivered=%d failures=%d",
		result.Bootstrap, len(result.Alerts), result.Delivered, len(result.Failures))
	return 0
}
