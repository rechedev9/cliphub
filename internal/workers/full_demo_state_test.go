package workers

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

func TestFullDemoStaleRenderCannotChangeCurrentState(t *testing.T) {
	b, err := os.ReadFile("../../web/lib/full-demo-plan.fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	var approved recapplan.Snapshot
	if err := json.Unmarshal(b, &approved); err != nil {
		t.Fatal(err)
	}
	newer := approved
	newer.Document.Options.Audio.Game.Gain = 0
	newer.Document.PlanHash, err = newer.Document.Hash()
	if err != nil {
		t.Fatal(err)
	}
	newer.Approval.PlanHash = newer.Document.PlanHash
	for _, status := range []string{renderplan.RenderVariantStatusRendering, renderplan.RenderVariantStatusFailed, renderplan.RenderVariantStatusReady} {
		for _, legacy := range []bool{false, true} {
			t.Run(status+"/legacy="+strconv.FormatBool(legacy), func(t *testing.T) {
				store := newFakeStorage()
				worker := NewRenderWorker(nil, store, RenderWorkerConfig{})
				current := renderplan.NewRenderVariantState(renderplan.NewRenderVariantStateOptions{JobID: uuid.New(), Variant: "gameplay-pov-60", Status: renderplan.RenderVariantStatusQueued, FullDemo: &newer})
				if err := worker.writeRenderVariantState(current); err != nil {
					t.Fatal(err)
				}
				attempt := current
				attempt.Status = status
				request := &approved
				if legacy {
					request = nil
				}
				if err := worker.writeOwnedRenderState(attempt, request); err == nil {
					t.Fatal("stale task published its state")
				}
				got, _, err := worker.readRenderVariantState(current.JobID, current.Variant)
				if err != nil || !reflect.DeepEqual(got, &current) {
					t.Fatalf("stale task changed the pointer: %v", err)
				}
				if err := worker.writeOwnedRenderState(attempt, &newer); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// A Full Demo queued before an upgrade that retired part of its plan is no
// longer admitted by the worker. It must fail durably instead of staying
// queued, or Studio waits on it forever and blocks every new long video.
func TestFullDemoQueuedBeforeAPlannerChangeFailsInsteadOfStayingQueued(t *testing.T) {
	b, err := os.ReadFile("../../web/lib/full-demo-plan.fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	var stale recapplan.Snapshot
	if err := json.Unmarshal(b, &stale); err != nil {
		t.Fatal(err)
	}
	// Its hash was taken by an older planner, so today's Hash differs.
	stale.Document.PlanHash = strings.Repeat("ab", 32)
	stale.Approval.PlanHash = stale.Document.PlanHash
	store := newFakeStorage()
	worker := NewRenderWorker(nil, store, RenderWorkerConfig{})
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, KillPlan: &killplan.Plan{}}
	queued := renderplan.NewRenderVariantState(renderplan.NewRenderVariantStateOptions{JobID: j.ID, Variant: "gameplay-pov-60", Status: renderplan.RenderVariantStatusQueued, FullDemo: &stale})
	if err := worker.writeRenderVariantState(queued); err != nil {
		t.Fatal(err)
	}

	err = worker.render(context.Background(), j, "gameplay-pov-60", "", 0, nil, renderplan.FullDemoEditRequest(stale), nil)
	var planErr *recapplan.Error
	if !errors.As(err, &planErr) || planErr.Code != recapplan.ErrPlanStale {
		t.Fatalf("render error = %v, want %s", err, recapplan.ErrPlanStale)
	}
	got, _, err := worker.readRenderVariantState(j.ID, "gameplay-pov-60")
	if err != nil || got == nil || got.Status != renderplan.RenderVariantStatusFailed || got.Error == "" {
		t.Fatalf("render state = %+v err=%v, want failed with a reason", got, err)
	}
}
