package main

import (
	"slices"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const captureQueuedStage = "capture queued"

func followedRun(run *tuiclient.GenerateRun, variant string) *tuiclient.GenerateRun {
	if run == nil || run.Active || variant == "" || run.Variant == variant {
		return run
	}
	return nil
}

func captureFailed(run *tuiclient.GenerateRun, renders []tuiclient.RenderVariantState) bool {
	if run == nil || run.Active || run.FailureReason == "" {
		return false
	}
	return !slices.ContainsFunc(renders, func(r tuiclient.RenderVariantState) bool { return r.UpdatedAt.After(run.FailedAt) })
}
