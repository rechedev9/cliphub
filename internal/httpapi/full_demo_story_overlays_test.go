package httpapi

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

func TestStoryOverlaysArePlannedReturnedAndAdmittedForRender(t *testing.T) {
	h, j, _, _, options := fullDemoAPIFixture(t)
	plain := fullDemoAPIRequest(t, h, j, "/full-demo/plan", map[string]any{"options": options})
	if plain.Code != 201 || strings.Contains(plain.Body.String(), `"halftime"`) || strings.Contains(plain.Body.String(), `"subscribe"`) {
		t.Fatalf("a plan without story overlays: %d %s", plain.Code, plain.Body)
	}
	var before recapplan.Document
	if err := json.Unmarshal(plain.Body.Bytes(), &before); err != nil {
		t.Fatal(err)
	}
	options.Overlays.Halftime, options.Overlays.Subscribe = true, true
	rw := fullDemoAPIRequest(t, h, j, "/full-demo/plan", map[string]any{"options": options})
	if rw.Code != 201 {
		t.Fatalf("plan: %d %s", rw.Code, rw.Body)
	}
	var d recapplan.Document
	if err := json.Unmarshal(rw.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if !d.Options.Overlays.Halftime || !d.Options.Overlays.Subscribe {
		t.Fatalf("planned overlays = %+v, want the halftime card and the subscribe button", d.Options.Overlays)
	}
	if d.PlanHash == before.PlanHash {
		t.Fatal("the story overlays must change the plan hash")
	}
	a, errA := before.CaptureHash()
	b, errB := d.CaptureHash()
	if errA != nil || errB != nil || a != b {
		t.Fatal("the story overlays must not change the capture")
	}
	snapshot := recapplan.Snapshot{Document: d, Approval: recapplan.Approval{PlanHash: d.PlanHash, AllowSafeTailTrim: d.Options.Editorial.AllowSafeTailTrim, Timestamp: time.Now().UTC()}}
	if err := renderplan.FullDemoEditRequest(snapshot).Validate(); err != nil {
		t.Fatalf("render request with story overlays was refused: %v", err)
	}
}
