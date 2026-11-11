package workers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

func rounds(n, seconds int) *killplan.Plan {
	plan := &killplan.Plan{Demo: killplan.Demo{Tickrate: 64}}
	for i := range n {
		start := i * 200 * 64
		plan.Segments = append(plan.Segments, killplan.Segment{TickStart: start, TickEnd: start + seconds*64})
	}
	return plan
}

func TestCaptureTimeoutGrowsWithAFullDemo(t *testing.T) {
	// Keiko's Inferno 13-16: 29 rounds of about two minutes each was cut at 20 minutes.
	if got := captureTimeout("20m", rounds(29, 120)); got != "2h6m0s" {
		t.Fatalf("29 rounds of 120 s = %s, want 2h6m0s (twice 58 min plus 10)", got)
	}
}

func TestCaptureTimeoutKeepsTheConfiguredOneForAShort(t *testing.T) {
	if got := captureTimeout("20m", rounds(5, 12)); got != "20m" {
		t.Fatalf("five 12 s plays = %s, want the configured 20m", got)
	}
	if got := captureTimeout("3h", rounds(29, 120)); got != "3h" {
		t.Fatalf("a configured 3h = %s, want it kept when it already fits", got)
	}
	if got := captureTimeout("20m", &killplan.Plan{}); got != "20m" {
		t.Fatalf("a plan without tickrate = %s, want the configured 20m", got)
	}
}

func TestRecordWorkerGivesALongFullDemoMoreThanTwentyMinutes(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := minimalKillPlan()
	plan.Segments = nil
	for i := range 29 {
		start := 64 + i*200*64
		plan.Segments = append(plan.Segments, killplan.Segment{ID: fmt.Sprintf("round-%03d", i+1), Round: i + 1, TickStart: start, TickEnd: start + 120*64})
	}
	plan.Demo.DurationTicks = 30 * 200 * 64
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusParsed, DemoPath: "demos/test.dem", Rules: rules.Default(), KillPlan: &plan}
	_ = store.Put("demos/test.dem", bytes.NewReader([]byte("demo")))

	runner := &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("stop after the launch")
	}}
	w := NewRecordWorker(repo, store, RecordWorkerConfig{WorkDir: t.TempDir(), RecorderPath: "zv-recorder", HLAEPath: "HLAE.exe", CS2Path: "cs2.exe"})
	w.runner = runner
	_ = w.HandleRecordDemo(context.Background(), recordTask(t, id))

	if len(runner.calls) == 0 {
		t.Fatal("the recorder was never launched")
	}
	got, err := time.ParseDuration(argValue(runner.calls[0].args, "--timeout"))
	if err != nil {
		t.Fatalf("--timeout: %v", err)
	}
	if got <= 20*time.Minute {
		t.Fatalf("--timeout = %s for 29 rounds of 2 minutes, want more than the 20m default", got)
	}
}
