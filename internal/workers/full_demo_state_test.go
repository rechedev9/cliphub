package workers

import (
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/google/uuid"
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
