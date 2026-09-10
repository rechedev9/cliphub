package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

// Regression: a FACEIT demo planned with a custom HUD rendered demo-facts-only
// intro/outro overlays because the plan default overlays.source="demo" was the
// only input to the overlay layout. The demo origin must own the format.
func TestFACEITOriginKeepsOverlayFormatWithCustomHUD(t *testing.T) {
	h, j, _, _, options := fullDemoAPIFixture(t)
	options.SourceKind = "faceit"
	options.Overlays.Source = "demo"
	options.Overlays.HUDTheme = "circuit"
	options.Capture.HUDProfile = customhud.CaptureProfile
	rw := fullDemoAPIRequest(t, h, j, "/full-demo/plan", map[string]any{"options": options})
	if rw.Code != 201 {
		t.Fatalf("plan: %d %s", rw.Code, rw.Body)
	}
	var d recapplan.Document
	if err := json.Unmarshal(rw.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	snapshot := recapplan.Snapshot{Document: d, Approval: recapplan.Approval{PlanHash: d.PlanHash, AllowSafeTailTrim: d.Options.Editorial.AllowSafeTailTrim, Timestamp: time.Now().UTC()}}
	edit := renderplan.FullDemoEditRequest(snapshot)
	if edit.DemoSource != renderplan.DemoSourceFACEIT || !edit.UsesFACEITOverlay() {
		t.Fatalf("custom HUD lost the FACEIT overlay format: %+v", edit)
	}
	if err := edit.Validate(); err != nil {
		t.Fatal(err)
	}
	// A hand-built edit that drops the origin must be rejected as contradictory.
	edit.DemoSource = ""
	if err := edit.Validate(); err == nil {
		t.Fatal("edit without the document's overlay origin was accepted")
	}
}

func TestFullDemoDefaultsInheritPersistedOverlayOrigin(t *testing.T) {
	h, j, store, _, _ := fullDemoAPIFixture(t)
	get := func() recapplan.Options {
		t.Helper()
		router := chi.NewRouter()
		router.Get("/api/jobs/{id}/full-demo/plan", h.GetFullDemoPlan)
		rw := httptest.NewRecorder()
		router.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/full-demo/plan", nil))
		if rw.Code != 200 {
			t.Fatalf("get plan: %d %s", rw.Code, rw.Body)
		}
		var envelope struct {
			Defaults recapplan.Options `json:"defaults"`
		}
		if err := json.Unmarshal(rw.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		return envelope.Defaults
	}
	if got := get().SourceKind; got != "demo" {
		t.Fatalf("fresh job default source_kind = %q, want demo", got)
	}
	store.puts[artifacts.FullDemoFaceitKey(j.ID)] = []byte(`{}`)
	if got := get().SourceKind; got != "faceit" {
		t.Fatalf("stored FACEIT roster default source_kind = %q, want faceit", got)
	}
	h.persistFullDemoSource(j.ID, "premier")
	if got := get().SourceKind; got != "premier" {
		t.Fatalf("persisted capture origin default source_kind = %q, want premier", got)
	}
}
