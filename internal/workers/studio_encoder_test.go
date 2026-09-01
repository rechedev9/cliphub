package workers

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/rules"
)

// setStudioCaptureEncoderForTest pins the studio capture-encoder decision so a
// test exercises the NVENC argv/plan path without probing the host's ffmpeg.
func setStudioCaptureEncoderForTest(t *testing.T, encoder string) {
	t.Helper()
	studioEncoders.mu.Lock()
	prevProbed, prev := studioEncoders.captureProbed, studioEncoders.capture
	studioEncoders.captureProbed = true
	studioEncoders.capture = encoder
	studioEncoders.mu.Unlock()
	t.Cleanup(func() {
		studioEncoders.mu.Lock()
		studioEncoders.captureProbed, studioEncoders.capture = prevProbed, prev
		studioEncoders.mu.Unlock()
	})
}

// Regression for the Studio NVENC rollout: the worker sent --encoder to the
// recorder but built its expected plan without it, so ValidateRecordingAttempt
// rejected every successful NVENC capture with "recording result plan does not
// match the launched attempt".
func TestRecordDemoStudioNVENCEncoderRoundTripsThroughAttemptValidation(t *testing.T) {
	setStudioCaptureEncoderForTest(t, recording.EncoderNVENC)
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := multiSegmentKillPlan("seg-001", "seg-002")
	repo.jobs[id] = &job.Job{
		ID:       id,
		Status:   job.StatusParsed,
		DemoPath: "demos/test.dem",
		Rules:    rules.Default(),
		KillPlan: &plan,
	}
	if err := store.Put("demos/test.dem", bytes.NewReader([]byte("demo"))); err != nil {
		t.Fatal(err)
	}

	w := publicationRecordWorker(t, repo, store, "nvenc-seg-001")
	if err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, []string{"seg-001"})); err != nil {
		t.Fatalf("HandleRecordDemo with studio NVENC = %v, want success", err)
	}
	committed := storedRecordingResult(t, store, id)
	if committed.Plan.Stream.Encoder != recording.EncoderNVENC {
		t.Fatalf("committed plan encoder = %q, want %q", committed.Plan.Stream.Encoder, recording.EncoderNVENC)
	}
	if got := repo.jobs[id].Status; got != job.StatusRecorded {
		t.Fatalf("job status = %s, want recorded", got)
	}

	// The encoder is part of the durable recording identity: a retry of the
	// same selection must reuse the committed NVENC clip, not recapture it.
	w = publicationRecordWorker(t, repo, store, "must-not-recapture")
	w.runner = &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("retry with a committed compatible NVENC capture launched the recorder")
		return nil, nil
	}}
	if err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, []string{"seg-001"})); err != nil {
		t.Fatalf("idempotent retry = %v, want skip", err)
	}
}
