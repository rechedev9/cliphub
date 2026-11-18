package httpapi

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

func TestHUDIntroDesignIsPlannedAndAdmittedForRender(t *testing.T) {
	h, j, _, _, options := fullDemoAPIFixture(t)
	options.Overlays.Theme = recapplan.OverlayThemeHUD
	rw := fullDemoAPIRequest(t, h, j, "/full-demo/plan", map[string]any{"options": options})
	if rw.Code != 201 {
		t.Fatalf("plan: %d %s", rw.Code, rw.Body)
	}
	var d recapplan.Document
	if err := json.Unmarshal(rw.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.Options.Overlays.Theme != recapplan.OverlayThemeHUD {
		t.Fatalf("planned design = %q, want hud", d.Options.Overlays.Theme)
	}
	snapshot := recapplan.Snapshot{Document: d, Approval: recapplan.Approval{PlanHash: d.PlanHash, AllowSafeTailTrim: d.Options.Editorial.AllowSafeTailTrim, Timestamp: time.Now().UTC()}}
	edit := renderplan.FullDemoEditRequest(snapshot)
	if err := edit.Validate(); err != nil || edit.OverlayTheme != recapplan.OverlayThemeHUD {
		t.Fatalf("render request = %q %v, want the HUD design admitted", edit.OverlayTheme, err)
	}
}
