//go:build !capturelab

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/store"
)

// Production orchestrator builds do not contain the fixture loader. Rejecting
// its environment variable makes a stale developer shell fail closed instead
// of appearing to have enabled a laboratory seam that is not present.
func seedCaptureLabFromEnvironment(_ context.Context, _ config, _ store.JobRepository, _ storage.Storage) error {
	if os.Getenv("ZV_CAPTURE_LAB_SEED") != "" || os.Getenv("ZV_CAPTURE_LAB_EVIDENCE_ROOT") != "" {
		return fmt.Errorf("capture lab seeding requires an orchestrator built with the capturelab build tag")
	}
	return nil
}
