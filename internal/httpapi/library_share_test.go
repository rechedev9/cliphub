package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/sharepkg"
)

// sharedDemoBytes is the .dem the shared match was read from; its hash is the
// match's identity in every library.
var sharedDemoBytes = append(append([]byte{}, demoMagic...), []byte("shared match")...)

func sharedDemoSHA() string {
	sum := sha256.Sum256(sharedDemoBytes)
	return hex.EncodeToString(sum[:])
}

func packageBody(t *testing.T, raw []byte) ([]byte, string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, _ := mw.CreateFormFile("package", "prueba.cliphub")
	_, _ = part.Write(raw)
	_ = mw.Close()
	return body.Bytes(), mw.FormDataContentType()
}

func (e libraryTestEnv) importPackage(raw []byte, wantStatus int) shareImportResponse {
	e.t.Helper()
	body, ct := packageBody(e.t, raw)
	rw := e.do(http.MethodPost, "/api/library/import-package", body, ct)
	if rw.Code != wantStatus {
		e.t.Fatalf("import = %d, want %d; body=%s", rw.Code, wantStatus, rw.Body)
	}
	var resp shareImportResponse
	_ = json.Unmarshal(rw.Body.Bytes(), &resp)
	return resp
}

func TestShareCollectionRoundTrip(t *testing.T) {
	sender := newLibraryTestEnv(t)
	m := sender.seedLibraryMatch(sharedDemoSHA(), "Alpha", "Beta")
	var c library.Collection
	sender.json(http.MethodPost, "/api/library/collections",
		map[string]any{"name": "Prueba Mirage", "items": []map[string]any{{"match_id": m.ID, "round": 2, "tick": 500, "note": "mirad"}}},
		http.StatusCreated, &c)
	sender.json(http.MethodPost, "/api/library/annotations",
		map[string]any{"match_id": m.ID, "round": 2, "tick": 510, "text": "rotad antes"}, http.StatusCreated, nil)
	sender.json(http.MethodPost, "/api/library/annotations",
		map[string]any{"match_id": m.ID, "round": 1, "tick": 110, "text": "fuera de la colección"}, http.StatusCreated, nil)

	rw := sender.do(http.MethodPost, "/api/library/collections/"+c.ID.String()+"/export", []byte(`{"notes":"Para el martes"}`), "application/json")
	if rw.Code != http.StatusOK || rw.Header().Get("Content-Type") != sharepkg.MediaType {
		t.Fatalf("export = %d %s", rw.Code, rw.Header())
	}
	if got := rw.Header().Get("Content-Disposition"); !strings.Contains(got, `filename="prueba-mirage.cliphub"`) {
		t.Fatalf("Content-Disposition = %q", got)
	}
	raw := rw.Body.Bytes()

	receiver := newLibraryTestEnv(t)
	resp := receiver.importPackage(raw, http.StatusOK)
	if resp.Kind != sharepkg.KindCollection || resp.Notes != "Para el martes" || resp.MatchesImported != 1 || resp.Annotations != 1 || resp.CollectionID == nil {
		t.Fatalf("import = %+v", resp)
	}
	var got collectionResponse
	receiver.json(http.MethodGet, "/api/library/collections/"+resp.CollectionID.String(), nil, http.StatusOK, &got)
	if len(got.Items) != 1 || len(got.MissingMatchIDs) != 0 || got.Items[0].Note != "mirad" {
		t.Fatalf("imported collection = %+v", got)
	}
	matchID := got.Items[0].MatchID
	var match library.Match
	receiver.json(http.MethodGet, "/api/library/matches/"+matchID.String(), nil, http.StatusOK, &match)
	if match.Source != library.SourceShared || match.Map != "de_mirage" || len(match.Players) != 2 {
		t.Fatalf("imported match = %+v", match)
	}
	receiver.json(http.MethodGet, fmt.Sprintf("/api/library/matches/%s/tactical/rounds/2", matchID), nil, http.StatusOK, nil)
	if rw := receiver.do(http.MethodGet, fmt.Sprintf("/api/library/matches/%s/tactical/positions", matchID), nil, ""); rw.Code != http.StatusOK {
		t.Fatalf("positions of the shared match = %d", rw.Code)
	}

	for _, path := range []string{"/analyze", "/jobs"} {
		rw := receiver.do(http.MethodPost, "/api/library/demos/"+match.DemoID.String()+path, nil, "")
		if rw.Code != http.StatusConflict || !strings.Contains(rw.Body.String(), codeSharedMatch) {
			t.Fatalf("POST %s on a shared match = %d %s", path, rw.Code, rw.Body)
		}
	}
	if n, err := receiver.h.ResumeLibraryScans(t.Context()); err != nil || n != 0 {
		t.Fatalf("ResumeLibraryScans queued %d shared demos, %v", n, err)
	}

	again := receiver.importPackage(raw, http.StatusOK)
	if !again.Duplicate || again.MatchesExisting != 1 || again.MatchesImported != 0 || again.Annotations != 0 {
		t.Fatalf("second import = %+v", again)
	}
	back := sender.importPackage(raw, http.StatusOK)
	if !back.Duplicate || back.MatchesExisting != 1 {
		t.Fatalf("import into the sender's own library = %+v", back)
	}
}

func TestShareReportRoundTripAssignsTheTeam(t *testing.T) {
	sender := newLibraryTestEnv(t)
	m := sender.seedLibraryMatch(sharedDemoSHA(), "Alpha", "Beta")
	teamID := m.Teams[0].TeamID
	var report library.Report
	sender.json(http.MethodPost, "/api/library/reports",
		map[string]any{"title": "Alpha en Mirage", "team_id": teamID, "match_ids": []any{m.ID}}, http.StatusCreated, &report)
	rw := sender.do(http.MethodPost, "/api/library/reports/"+report.ID.String()+"/export", nil, "")
	if rw.Code != http.StatusOK {
		t.Fatalf("export = %d %s", rw.Code, rw.Body)
	}

	receiver := newLibraryTestEnv(t)
	resp := receiver.importPackage(rw.Body.Bytes(), http.StatusOK)
	if resp.ReportID == nil {
		t.Fatalf("import = %+v", resp)
	}
	var got library.Report
	receiver.json(http.MethodGet, "/api/library/reports/"+resp.ReportID.String(), nil, http.StatusOK, &got)
	if got.TeamName != "Alpha" || len(got.MatchIDs) != 1 {
		t.Fatalf("imported report = %+v", got)
	}
	var scout scoutResponse
	receiver.json(http.MethodGet, "/api/library/reports/"+got.ID.String()+"/scout", nil, http.StatusOK, &scout)
	if len(scout.Skipped) != 0 {
		t.Fatalf("the report skips its shared match: %+v", scout.Skipped)
	}
}

func TestImportPackageRefusesBrokenFiles(t *testing.T) {
	env := newLibraryTestEnv(t)
	for name, raw := range map[string][]byte{
		"not a zip": []byte("PK\x03\x04 not really"),
		"empty":     {},
	} {
		body, ct := packageBody(t, raw)
		rw := env.do(http.MethodPost, "/api/library/import-package", body, ct)
		if rw.Code != http.StatusBadRequest || !strings.Contains(rw.Body.String(), codeSharePackage) {
			t.Errorf("%s: import = %d %s", name, rw.Code, rw.Body)
		}
	}
	if rw := env.do(http.MethodPost, "/api/library/import-package", []byte("x"), "text/plain"); rw.Code != http.StatusBadRequest {
		t.Errorf("non-multipart import = %d", rw.Code)
	}
	if demos, _ := env.repo.ListDemos(t.Context(), library.DemoFilter{}); len(demos) != 0 {
		t.Fatalf("a refused package left %d demos", len(demos))
	}
}

// Importing the real .dem of a shared match turns it into an ordinary match
// that can be analyzed and recorded.
func TestImportingTheDemoOfASharedMatchAttachesIt(t *testing.T) {
	sender := newLibraryTestEnv(t)
	m := sender.seedLibraryMatch(sharedDemoSHA(), "Alpha", "Beta")
	var c library.Collection
	sender.json(http.MethodPost, "/api/library/collections",
		map[string]any{"name": "x", "items": []map[string]any{{"match_id": m.ID, "round": 1, "tick": 100}}}, http.StatusCreated, &c)
	raw := sender.do(http.MethodPost, "/api/library/collections/"+c.ID.String()+"/export", nil, "").Body.Bytes()

	receiver := newLibraryTestEnv(t)
	receiver.importPackage(raw, http.StatusOK)

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, _ := mw.CreateFormFile("demo", "real.dem")
	_, _ = part.Write(sharedDemoBytes)
	_ = mw.Close()
	rw := receiver.do(http.MethodPost, "/api/library/demos", body.Bytes(), mw.FormDataContentType())
	var resp struct {
		Demos []libraryImportResult `json:"demos"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &resp); err != nil || len(resp.Demos) != 1 {
		t.Fatalf("import demo = %d %s", rw.Code, rw.Body)
	}
	d := resp.Demos[0].Demo
	if resp.Demos[0].Status != libraryImportImported || d == nil || d.Source == library.SourceShared || d.State != library.ParseQueued || d.MatchID == nil {
		t.Fatalf("attached demo = %+v %+v", resp.Demos[0], d)
	}
	if exists, _ := receiver.storage.Exists(d.Path); !exists {
		t.Fatal("the attached demo has no stored file")
	}
}
