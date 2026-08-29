//go:build !capturelab

package main

import (
	"context"
	"strings"
	"testing"
)

func TestProductionBuildRejectsCaptureLabSeedEnvironment(t *testing.T) {
	t.Setenv("ZV_CAPTURE_LAB_SEED", "seed.json")
	err := seedCaptureLabFromEnvironment(context.Background(), config{}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "capturelab build tag") {
		t.Fatalf("seedCaptureLabFromEnvironment error = %v", err)
	}
}
