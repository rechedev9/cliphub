package httpapi

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

func TestHookIsPlannedReturnedAndAdmittedForRender(t *testing.T) {
	h, j, store, _, options := fullDemoAPIFixture(t)
	facts, found, err := recapplan.LoadFacts(store, j.ID)
	if err != nil || !found {
		t.Fatalf("facts: %t %v", found, err)
	}
	facts.Rounds[1].Kills = []killplan.Kill{{Tick: 15000}, {Tick: 15100}}
	if err := recapplan.StoreFacts(store, j.ID, facts); err != nil {
		t.Fatal(err)
	}
	plain := fullDemoAPIRequest(t, h, j, "/full-demo/plan", map[string]any{"options": options})
	if plain.Code != 201 || strings.Contains(plain.Body.String(), `"hook"`) {
		t.Fatalf("a plan without the hook: %d %s", plain.Code, plain.Body)
	}
	var before recapplan.Document
	if err := json.Unmarshal(plain.Body.Bytes(), &before); err != nil {
		t.Fatal(err)
	}
	options.Hook = &recapplan.HookOptions{}
	off := fullDemoAPIRequest(t, h, j, "/full-demo/plan", map[string]any{"options": options})
	var same recapplan.Document
	if err := json.Unmarshal(off.Body.Bytes(), &same); err != nil || off.Code != 201 || same.PlanHash != before.PlanHash {
		t.Fatalf("a hook that is off must plan like no hook: %d %v", off.Code, err)
	}
	options.Hook = &recapplan.HookOptions{Enabled: true}
	rw := fullDemoAPIRequest(t, h, j, "/full-demo/plan", map[string]any{"options": options})
	if rw.Code != 201 {
		t.Fatalf("plan: %d %s", rw.Code, rw.Body)
	}
	var d recapplan.Document
	if err := json.Unmarshal(rw.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.Options.Hook == nil || !d.Options.Hook.Enabled || d.Hook == nil || d.Hook.RoundID != "round-002" || d.Hook.Kills != 2 {
		t.Fatalf("planned hook = %+v / %+v, want the double kill of round 2", d.Options.Hook, d.Hook)
	}
	if len(d.Timeline) != len(before.Timeline)+1 || !d.Timeline[0].IsHook() || d.Timeline[1].SourceRef != "round-001" || len(d.Rounds) != len(before.Rounds) {
		t.Fatalf("the hook must open the video before round 1 without adding a round: %+v", d.Timeline)
	}
	if d.PlanHash == before.PlanHash {
		t.Fatal("the hook must change the plan hash")
	}
	a, errA := before.CaptureHash()
	b, errB := d.CaptureHash()
	if errA != nil || errB != nil || a != b {
		t.Fatal("the hook must not change the capture")
	}
	snapshot := recapplan.Snapshot{Document: d, Approval: recapplan.Approval{PlanHash: d.PlanHash, AllowSafeTailTrim: d.Options.Editorial.AllowSafeTailTrim, Timestamp: time.Now().UTC()}}
	if err := renderplan.FullDemoEditRequest(snapshot).Validate(); err != nil {
		t.Fatalf("render request with the hook was refused: %v", err)
	}
}
