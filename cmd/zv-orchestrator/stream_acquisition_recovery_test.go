package main

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/fragforge/internal/streamclips"
)

func TestRecoverStreamAcquisitionsFailsJobsWhenWorkerDisabled(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryStreamJobRepository()
	streamJob := streamclips.Job{Status: streamclips.StatusAcquiring}
	if err := repo.Create(ctx, &streamJob); err != nil {
		t.Fatal(err)
	}

	err := recoverStreamAcquisitions(
		ctx,
		[]uuid.UUID{streamJob.ID},
		false,
		repo,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("recoverStreamAcquisitions error = %v", err)
	}

	got, err := repo.Get(ctx, streamJob.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != streamclips.StatusFailed || got.FailureReason != streamAcquireRecoveryDisabledReason {
		t.Fatalf(
			"recovered stream = status %q reason %q, want failed/%q",
			got.Status,
			got.FailureReason,
			streamAcquireRecoveryDisabledReason,
		)
	}
}
