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

	"github.com/rechedev9/cliphub/internal/overlayassets"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

func TestFullDemoPortraitUploadReadPlanAndChangedBytes(t *testing.T) {
	h, j, store, _, options := fullDemoAPIFixture(t)
	var pngBody bytes.Buffer
	if err := png.Encode(&pngBody, image.NewNRGBA(image.Rect(0, 0, 20, 40))); err != nil {
		t.Fatal(err)
	}
	var uploaded overlayassets.Asset
	for _, valid := range []bool{false, true} {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		part, err := form.CreateFormFile("image", "portrait.png")
		if err != nil {
			t.Fatal(err)
		}
		data := []byte("not an image")
		if valid {
			data = pngBody.Bytes()
		}
		_, _ = part.Write(data)
		_ = form.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/full-demo/overlay-images", &body)
		req.Header.Set("Content-Type", form.FormDataContentType())
		if !isMultipartUpload(req) {
			t.Fatal("portrait bypasses upload limiter")
		}
		rw := httptest.NewRecorder()
		Routes(h).ServeHTTP(rw, req)
		if !valid {
			if rw.Code != http.StatusBadRequest {
				t.Fatalf("invalid image: %d %s", rw.Code, rw.Body)
			}
			continue
		}
		if rw.Code != http.StatusCreated {
			t.Fatalf("upload: %d %s", rw.Code, rw.Body)
		}
		if err := json.Unmarshal(rw.Body.Bytes(), &uploaded); err != nil {
			t.Fatal(err)
		}
	}
	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/full-demo/overlay-images/"+uploaded.ID.String(), nil))
	if rw.Code != http.StatusOK || rw.Header().Get("Content-Type") != "image/png" || !bytes.Equal(rw.Body.Bytes(), pngBody.Bytes()) {
		t.Fatal("preview did not return uploaded PNG")
	}
	options.Overlays.HUDTheme = "focus"
	options.Overlays.HUDPortrait = &recapplan.AssetRef{ID: uploaded.ID.String(), SHA256: uploaded.SHA256}
	rw = fullDemoAPIRequest(t, h, j, "/full-demo/plan", map[string]any{"options": options})
	if rw.Code != http.StatusCreated {
		t.Fatalf("plan: %d %s", rw.Code, rw.Body)
	}
	var plan recapplan.Document
	if err := json.Unmarshal(rw.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.Blockers) != 0 || len(plan.Assets) != 1 || !plan.Assets[0].HasImage || plan.Options.Overlays.HUDPortrait == nil {
		t.Fatalf("portrait not resolved: %+v", plan.Blockers)
	}
	store.puts[overlayassets.MediaKey(uploaded.ID)] = []byte("changed")
	rw = fullDemoAPIRequest(t, h, j, "/full-demo/plan", map[string]any{"options": options})
	if rw.Code < 400 {
		t.Fatal("replaced image bytes accepted")
	}
}
