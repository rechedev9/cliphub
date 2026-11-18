package workers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

// programOf is an approved Full Demo whose timeline ends after d of output.
func programOf(d time.Duration) *recapplan.Snapshot {
	end := int64(d / time.Second * recapplan.OutputFPS)
	return &recapplan.Snapshot{Document: recapplan.Document{Timeline: []recapplan.TimelineItem{
		{Role: "round", EndFrame: end / 2},
		{Role: "round", StartFrame: end / 2, EndFrame: end},
	}}}
}

func TestRenderTimeoutGrowsWithAFullDemo(t *testing.T) {
	cases := []struct {
		name       string
		configured string
		fullDemo   *recapplan.Snapshot
		want       time.Duration
	}{
		{"a Short keeps the configured deadline", "20m", nil, 20 * time.Minute},
		{"a Full Demo without a timeline keeps it", "20m", &recapplan.Snapshot{}, 20 * time.Minute},
		// The slow PC that was cut at 20 minutes needed about 2.5 times its 23 minute video.
		{"a 23 minute Full Demo gets four times its length plus 15 minutes", "20m", programOf(23 * time.Minute), time.Hour + 47*time.Minute},
		{"a short Full Demo keeps the configured deadline", "20m", programOf(time.Minute), 20 * time.Minute},
		{"a configured deadline that already fits is kept", "3h", programOf(23 * time.Minute), 3 * time.Hour},
		{"an unparsable configured deadline is not scaled", "soon", programOf(23 * time.Minute), RenderWorkerConfig{Timeout: "soon"}.timeoutDuration()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (RenderWorkerConfig{Timeout: tc.configured}).renderTimeout(tc.fullDemo, recording.RecordingPlan{}); got != tc.want {
				t.Fatalf("renderTimeout(%q) = %s, want %s", tc.configured, got, tc.want)
			}
		})
	}
}

// playsOf is a capture of plays that each last d at 64 ticks a second.
func playsOf(d ...time.Duration) recording.RecordingPlan {
	plan := recording.RecordingPlan{Tickrate: 64}
	start := 1000
	for _, length := range d {
		ticks := int(length / time.Second * 64)
		plan.Segments = append(plan.Segments, recording.RecordingSegment{ID: "seg", TickStart: start, TickEnd: start + ticks})
		start += ticks + 6400
	}
	return plan
}

// A summary has no length cap (#554), so its deadline grows with its plays like a Full Demo's.
func TestRenderTimeoutGrowsWithALongSummary(t *testing.T) {
	cfg := RenderWorkerConfig{Timeout: "20m"}
	if got := cfg.renderTimeout(nil, playsOf(20*time.Second, 25*time.Second)); got != 20*time.Minute {
		t.Fatalf("a Short of 45 s gets %s, want the configured 20m", got)
	}
	got := cfg.renderTimeout(nil, playsOf(4*time.Minute, 3*time.Minute, 3*time.Minute))
	if got < 4*10*time.Minute+renderTimeoutSlack-time.Minute {
		t.Fatalf("a summary of 10 minutes of plays gets %s, want about four times its length plus %s", got, renderTimeoutSlack)
	}
}

func TestRenderWorkerHandsAFullDemoTheScaledDeadline(t *testing.T) {
	store := newFakeStorage()
	id := uuid.New()
	demo := []byte("demo")
	digest := sha256.Sum256(demo)
	if err := store.Put("demos/test.dem", bytes.NewReader(demo)); err != nil {
		t.Fatal(err)
	}
	// Approval rereads the stored facts strictly and hashes the stored demo.
	result, dir, resultPath, facts := plannedFullDemoFixture(t, "clip", artifacts.FullDemoFactsKey(id), func(f *recapplan.Facts, _ *recapplan.Options) {
		f.Warnings = []recapplan.Notice{}
		f.DemoSHA256 = hex.EncodeToString(digest[:])
	})
	if _, err := uploadFullDemoRecordingOutputs(store, id, dir, resultPath, result, result, false); err != nil {
		t.Fatal(err)
	}
	doc := *result.Plan.FullDemo
	if err := recapplan.StoreFacts(store, id, facts); err != nil {
		t.Fatal(err)
	}
	if err := recapplan.SaveDocument(store, id, doc); err != nil {
		t.Fatal(err)
	}
	snapshot := recapplan.Snapshot{Document: doc, Approval: recapplan.Approval{PlanHash: doc.PlanHash, AllowSafeTailTrim: doc.Options.Editorial.AllowSafeTailTrim, Timestamp: time.Now().UTC()}}
	killPlan := doc.KillPlan(killplan.NewPlan())
	j := job.Job{ID: id, Status: job.StatusRecorded, DemoPath: "demos/test.dem", TargetSteamID: doc.Input.TargetSteamID64, KillPlan: &killPlan}
	repo := newFakeRepo()
	repo.jobs[id] = &j

	// A one-second configured deadline makes any Full Demo scale past it.
	w := NewRenderWorker(repo, store, RenderWorkerConfig{WorkDir: t.TempDir(), EditorPath: "zv-editor", Timeout: "1s"})
	stop := errors.New("stop after the editor launch")
	var remaining time.Duration
	w.runner = &fakeRunner{fn: func(ctx context.Context, _ string, _ ...string) ([]byte, error) {
		if deadline, ok := ctx.Deadline(); ok {
			remaining = time.Until(deadline)
		}
		return nil, stop
	}}
	err := w.render(context.Background(), j, "gameplay-pov-60", "", 0, nil, renderplan.FullDemoEditRequest(snapshot), nil)
	if !errors.Is(err, stop) {
		t.Fatalf("render error = %v, want the editor stop sentinel", err)
	}
	want := w.cfg.renderTimeout(&snapshot, result.Plan)
	if want <= time.Second {
		t.Fatalf("fixture deadline %s did not scale past the configured 1s", want)
	}
	if remaining <= want-time.Minute || remaining > want {
		t.Fatalf("zv-editor deadline in %s, want about %s", remaining, want)
	}
}

func TestRenderWorkerNamesAnEditorCutByItsDeadline(t *testing.T) {
	w := NewRenderWorker(newFakeRepo(), newFakeStorage(), RenderWorkerConfig{EditorPath: "zv-editor"})
	w.runner = &fakeRunner{fn: func(ctx context.Context, _ string, _ ...string) ([]byte, error) {
		<-ctx.Done()
		return nil, errors.New("zv-editor.exe failed: exit status 1")
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := w.runEditor(ctx, nil)
	if err == nil || !strings.HasPrefix(err.Error(), renderTimeoutPrefix) || !strings.Contains(err.Error(), "exit status 1") {
		t.Fatalf("runEditor error = %v, want a %s reason keeping the editor error", err, renderTimeoutPrefix)
	}

	w.runner = &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("zv-editor.exe failed: exit status 1")
	}}
	if err := w.runEditor(context.Background(), nil); err == nil || strings.HasPrefix(err.Error(), renderTimeoutPrefix) {
		t.Fatalf("runEditor error = %v, want the editor error without a timeout reason", err)
	}
}
