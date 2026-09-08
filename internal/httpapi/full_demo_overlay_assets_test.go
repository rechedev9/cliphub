package httpapi

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rechedev9/cliphub/internal/overlayassets"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

func TestScreenshotUploadPlanAndAdmissionDoNotRequireFACEIT(t *testing.T) {
	h, j, store, _, options := fullDemoAPIFixture(t)
	router := chi.NewRouter()
	router.Post("/api/full-demo/overlay-images", h.CreateFullDemoOverlayAsset)
	router.Get("/api/full-demo/overlay-images/{assetID}", h.GetFullDemoOverlayAsset)
	var imageBytes bytes.Buffer
	if err := png.Encode(&imageBytes, image.NewNRGBA(image.Rect(0, 0, 436, 513))); err != nil {
		t.Fatal(err)
	}
	refs := []recapplan.AssetRef{}
	for i := 0; i < 3; i++ {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		part, err := form.CreateFormFile("image", "captura.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(imageBytes.Bytes()); err != nil {
			t.Fatal(err)
		}
		if err := form.Close(); err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("POST", "/api/full-demo/overlay-images", &body)
		r.Header.Set("Content-Type", form.FormDataContentType())
		rw := httptest.NewRecorder()
		router.ServeHTTP(rw, r)
		if rw.Code != http.StatusCreated {
			t.Fatalf("upload: %d %s", rw.Code, rw.Body)
		}
		var a overlayassets.Asset
		if err := json.Unmarshal(rw.Body.Bytes(), &a); err != nil {
			t.Fatal(err)
		}
		refs = append(refs, recapplan.AssetRef{ID: a.ID.String(), SHA256: a.SHA256})
		preview := httptest.NewRecorder()
		router.ServeHTTP(preview, httptest.NewRequest("GET", "/api/full-demo/overlay-images/"+a.ID.String(), nil))
		if preview.Code != 200 || preview.Header().Get("Content-Type") != "image/png" || !bytes.Equal(preview.Body.Bytes(), imageBytes.Bytes()) {
			t.Fatalf("preview changed the image: %d %s", preview.Code, preview.Header())
		}
	}
	options.Overlays = recapplan.OverlayOptions{Mode: "screenshots", Source: "faceit", Theme: "neon-violet", Roster: true, Scoreboard: true, Team1Image: &refs[0], Team2Image: &refs[1], ScoreboardImage: &refs[2]}
	rw := fullDemoAPIRequest(t, h, j, "/full-demo/plan", map[string]any{"options": options})
	if rw.Code != 201 {
		t.Fatalf("plan: %d %s", rw.Code, rw.Body)
	}
	var d recapplan.Document
	if err := json.Unmarshal(rw.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if len(d.Assets) != 3 || len(d.Blockers) != 0 {
		t.Fatalf("assets not resolved: %+v", d)
	}
	for _, a := range d.Assets {
		if !a.HasImage {
			t.Fatal("screenshot lost image type")
		}
	}
	snapshot := recapplan.Snapshot{Document: d, Approval: recapplan.Approval{PlanHash: d.PlanHash, AllowSafeTailTrim: d.Options.Editorial.AllowSafeTailTrim, Timestamp: time.Now().UTC()}}
	edit := renderplan.FullDemoEditRequest(snapshot)
	if edit.UsesFACEITOverlay() {
		t.Fatal("screenshots require FACEIT lookup")
	}
	resolved, err := recapplan.ResolveApproval(httptest.NewRequest("GET", "/", nil).Context(), store, j.ID, j.DemoPath, j.TargetSteamID, "", snapshot)
	if err != nil || resolved.Document.PlanHash != d.PlanHash {
		t.Fatalf("approval: %+v %v", resolved, err)
	}
	h.capabilities.RecordEnabled = true
	admission := fullDemoAPIRequest(t, h, j, "/generate", map[string]any{"preset": "gameplay-pov-60", "edit": edit})
	if admission.Code != http.StatusAccepted {
		t.Fatalf("screenshot admission unexpectedly needs FACEIT: %d %s", admission.Code, admission.Body)
	}
	key := d.Assets[0].MediaKey()
	store.puts[key] = []byte("changed")
	if _, err := recapplan.ResolveApproval(httptest.NewRequest("GET", "/", nil).Context(), store, j.ID, j.DemoPath, j.TargetSteamID, "", snapshot); err == nil {
		t.Fatal("approved changed image")
	}
}
