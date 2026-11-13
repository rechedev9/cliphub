package httpapi

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/radarmap"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func uploadFixture(t *testing.T, sha string) (document, positions []byte) {
	t.Helper()
	blob, err := tacticalplan.EncodePositions([]tacticalplan.RoundFrames{{
		Round:  1,
		Frames: []tacticalplan.Frame{{Tick: 100, Samples: []tacticalplan.Sample{{Slot: 0, X: 10, Y: 20, Z: 0, Health: 100, Flags: tacticalplan.FlagAlive}}}},
	}}, 8, 64)
	if err != nil {
		t.Fatalf("EncodePositions: %v", err)
	}
	doc := tacticalplan.Document{
		SchemaVersion: "1",
		Demo:          tacticalplan.Demo{Path: `C:\secret\path\match.dem`, SHA256: sha, Map: "de_dust2", Tickrate: 64, DurationTicks: 1000},
		Teams: []tacticalplan.Team{
			{Key: "a", Name: "Team A", StartSide: tacticalplan.SideCT},
			{Key: "b", Name: "Team B", StartSide: tacticalplan.SideT},
		},
		Rounds:    []tacticalplan.Round{{Number: 1, TickStart: 0, TickEnd: 900, Winner: tacticalplan.SideCT}},
		Geometry:  tacticalplan.MapGeometry{Calibration: radarmap.Calibration{Map: "de_dust2", PosX: -2476, PosY: 3239, Scale: 4.4, Size: 1024}},
		Positions: blob.Descriptor,
	}
	document, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return document, blob.Data
}

func uploadBody(t *testing.T, fields map[string]string, parts map[string][]byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for name, value := range fields {
		if err := mw.WriteField(name, value); err != nil {
			t.Fatal(err)
		}
	}
	for name, data := range parts {
		part, err := mw.CreateFormFile(name, name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write(data)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, mw.FormDataContentType()
}

func newUploadEnv(t *testing.T) libraryTestEnv {
	t.Helper()
	repo := store.NewMemoryLibraryRepository()
	files := newFakeStorage()
	h := NewHandlers(newFakeRepo(), files, &fakeQueue{}, WithLibrary(repo), WithLibraryUploads())
	return libraryTestEnv{t: t, repo: repo, storage: files, h: h, router: Routes(h)}
}

func TestUploadLibraryAnalysisStoresTheMatchAndNoPath(t *testing.T) {
	env := newUploadEnv(t)
	sha := strings.Repeat("ab", 32)
	document, positions := uploadFixture(t, sha)
	body, contentType := uploadBody(t,
		map[string]string{"file_name": `C:\Users\luis\demos\match.dem`, "size_bytes": "123456"},
		map[string][]byte{"document": document, "positions": positions})
	rw := env.do(http.MethodPost, "/api/library/uploads", body.Bytes(), contentType)
	if rw.Code != http.StatusCreated {
		t.Fatalf("upload = %d, body=%s", rw.Code, rw.Body.String())
	}
	var res uploadResult
	if err := json.Unmarshal(rw.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if !res.Created || res.Demo.State != library.ParseReady || res.Match.Map != "de_dust2" || res.Demo.FileName != "match.dem" {
		t.Fatalf("result = %+v", res)
	}
	if res.Demo.Path != "" {
		t.Fatalf("the demo path = %q, want none", res.Demo.Path)
	}
	stored := env.storage.puts[artifacts.LibraryTacticalIndexKey(res.Demo.ID)]
	if len(stored) == 0 || bytes.Contains(stored, []byte("secret")) {
		t.Fatalf("stored document keeps the client path or is missing: %q", stored)
	}
	if !bytes.Equal(env.storage.puts[artifacts.LibraryTacticalPositionsKey(res.Demo.ID)], positions) {
		t.Fatal("stored positions differ from the upload")
	}

	again, contentType := uploadBody(t,
		map[string]string{"file_name": "match.dem"},
		map[string][]byte{"document": document, "positions": positions})
	rw = env.do(http.MethodPost, "/api/library/uploads", again.Bytes(), contentType)
	if rw.Code != http.StatusOK {
		t.Fatalf("second upload = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	var second uploadResult
	_ = json.Unmarshal(rw.Body.Bytes(), &second)
	if second.Created || second.Demo.ID != res.Demo.ID || second.Match.ID != res.Match.ID {
		t.Fatalf("second upload = %+v, want the same match", second)
	}
}

func TestUploadLibraryAnalysisRejectsBadInput(t *testing.T) {
	env := newUploadEnv(t)
	sha := strings.Repeat("cd", 32)
	document, positions := uploadFixture(t, sha)
	cases := map[string]struct {
		parts map[string][]byte
		want  int
	}{
		"no positions":        {map[string][]byte{"document": document}, http.StatusBadRequest},
		"no document":         {map[string][]byte{"positions": positions}, http.StatusBadRequest},
		"not json":            {map[string][]byte{"document": []byte("nope"), "positions": positions}, http.StatusUnprocessableEntity},
		"blob does not match": {map[string][]byte{"document": document, "positions": append([]byte("X"), positions[1:]...)}, http.StatusUnprocessableEntity},
		"blob is cut":         {map[string][]byte{"document": document, "positions": positions[:len(positions)-1]}, http.StatusUnprocessableEntity},
	}
	for name, tc := range cases {
		body, contentType := uploadBody(t, map[string]string{"file_name": "x.dem"}, tc.parts)
		if rw := env.do(http.MethodPost, "/api/library/uploads", body.Bytes(), contentType); rw.Code != tc.want {
			t.Errorf("%s: status %d, want %d; body=%s", name, rw.Code, tc.want, rw.Body.String())
		}
	}
	if demos, _ := env.repo.ListDemos(t.Context(), library.DemoFilter{}); len(demos) != 0 {
		t.Fatalf("%d demos were created by rejected uploads", len(demos))
	}
}

func TestUploadRouteIsOffUnlessEnabled(t *testing.T) {
	env := newLibraryTestEnv(t)
	body, contentType := uploadBody(t, nil, nil)
	if rw := env.do(http.MethodPost, "/api/library/uploads", body.Bytes(), contentType); rw.Code != http.StatusNotFound && rw.Code != http.StatusMethodNotAllowed {
		t.Fatalf("upload without the option = %d, want 404 or 405", rw.Code)
	}
}
