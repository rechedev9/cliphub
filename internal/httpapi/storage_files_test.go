package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/rechedev9/cliphub/internal/diskfiles"
	"github.com/rechedev9/cliphub/internal/storage"
)

type storageFilesBody struct {
	Scanning    bool             `json:"scanning"`
	Demos       []diskfiles.File `json:"demos"`
	StudioDemos []diskfiles.File `json:"studio_demos"`
	Videos      []diskfiles.File `json:"videos"`
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestStorageFilesListThenDeleteOnlyListedFiles(t *testing.T) {
	dataDir := t.TempDir()
	pcRoot := t.TempDir()
	store, err := storage.NewLocal(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	userDemo := filepath.Join(pcRoot, "Downloads", "match.dem")
	render := filepath.Join(dataDir, "jobs", "j1", "renders", "short-pov", "videos", "reel.mp4")
	cover := filepath.Join(dataDir, "jobs", "j1", "renders", "short-pov", "covers", "reel.jpg")
	studioDemo := filepath.Join(dataDir, "demos", "j1.dem")
	unlisted := filepath.Join(pcRoot, "keep.txt")
	for _, path := range []string{userDemo, render, cover, studioDemo, unlisted} {
		writeTestFile(t, path)
	}

	h := NewHandlers(newFakeRepo(), store, &fakeQueue{})
	h.storageFiles.scanning = true
	h.storageFiles.runScan([]string{pcRoot}, dataDir)
	router := chi.NewRouter()
	router.Get("/api/storage/files", h.ListStorageFiles)
	router.Post("/api/storage/files/delete", h.DeleteStorageFile)
	send := func(method, url, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(method, url, strings.NewReader(body)))
		return rec
	}
	deleteBody := func(path string) string {
		raw, _ := json.Marshal(map[string]string{"path": path})
		return string(raw)
	}

	if rec := send(http.MethodPost, "/api/storage/files/delete", deleteBody(userDemo)); rec.Code != http.StatusNotFound {
		t.Fatalf("delete before any list = %d, want 404", rec.Code)
	}
	rec := send(http.MethodGet, "/api/storage/files", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list = %d: %s", rec.Code, rec.Body)
	}
	var body storageFilesBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Scanning || len(body.Demos) != 1 || body.Demos[0].Path != userDemo {
		t.Fatalf("demos = %+v", body.Demos)
	}
	if len(body.StudioDemos) != 1 || body.StudioDemos[0].JobID != "j1" {
		t.Fatalf("studio demos = %+v", body.StudioDemos)
	}
	if len(body.Videos) != 1 || body.Videos[0].Kind != diskfiles.KindShort {
		t.Fatalf("videos = %+v", body.Videos)
	}

	if rec := send(http.MethodPost, "/api/storage/files/delete", deleteBody(unlisted)); rec.Code != http.StatusNotFound {
		t.Fatalf("delete unlisted = %d, want 404", rec.Code)
	}
	if rec := send(http.MethodPost, "/api/storage/files/delete", deleteBody(studioDemo)); rec.Code != http.StatusConflict {
		t.Fatalf("delete studio demo = %d, want 409", rec.Code)
	}
	for _, path := range []string{userDemo, render} {
		if rec := send(http.MethodPost, "/api/storage/files/delete", deleteBody(path)); rec.Code != http.StatusOK {
			t.Fatalf("delete %s = %d: %s", path, rec.Code, rec.Body)
		}
	}
	for _, path := range []string{userDemo, render, cover} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s still exists", path)
		}
	}
	rec = send(http.MethodGet, "/api/storage/files", "")
	body = storageFilesBody{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Demos) != 0 || len(body.Videos) != 0 {
		t.Fatalf("deleted files still listed: %+v", body)
	}
	for _, path := range []string{unlisted, studioDemo} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s was removed: %v", path, err)
		}
	}
}
