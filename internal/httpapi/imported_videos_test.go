package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rechedev9/cliphub/internal/importedvideos"
)

func TestImportedVideoRoutesListServeAndDelete(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	data := t.TempDir()
	file := filepath.Join(data, "cloud", "results", id, "short-01.mp4")
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := importedvideos.Open(data)
	if err != nil {
		t.Fatal(err)
	}
	routes := Routes(NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithImportedVideos(store)))
	serve := func(method, path string, header http.Header) *httptest.ResponseRecorder {
		rw := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, nil)
		for k, v := range header {
			req.Header[k] = v
		}
		routes.ServeHTTP(rw, req)
		return rw
	}

	list := serve(http.MethodGet, "/api/imported-videos", nil)
	var body struct {
		Videos []importedvideos.Video `json:"videos"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &body); err != nil || list.Code != http.StatusOK || len(body.Videos) != 1 || body.Videos[0].CloudJobID != id {
		t.Fatalf("list = %d %s", list.Code, list.Body)
	}

	ranged := serve(http.MethodGet, "/api/imported-videos/"+id+"/short-01.mp4", http.Header{"Range": {"bytes=2-4"}})
	if ranged.Code != http.StatusPartialContent || ranged.Body.String() != "234" || ranged.Header().Get("Content-Type") != "video/mp4" {
		t.Fatalf("ranged get = %d %q %q", ranged.Code, ranged.Body, ranged.Header().Get("Content-Type"))
	}
	if missing := serve(http.MethodGet, "/api/imported-videos/"+id+"/short-02.mp4", nil); missing.Code != http.StatusNotFound {
		t.Fatalf("unknown name = %d, want 404", missing.Code)
	}

	if deleted := serve(http.MethodDelete, "/api/imported-videos/"+id+"/short-01.mp4", nil); deleted.Code != http.StatusNoContent {
		t.Fatalf("delete = %d %s", deleted.Code, deleted.Body)
	}
	if gone := serve(http.MethodGet, "/api/imported-videos/"+id+"/short-01.mp4", nil); gone.Code != http.StatusNotFound {
		t.Fatalf("get after delete = %d, want 404", gone.Code)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("file still on disk: %v", err)
	}
}

func TestImportedVideosListIsEmptyWithoutAStore(t *testing.T) {
	rw := httptest.NewRecorder()
	Routes(NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{})).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/imported-videos", nil))
	if rw.Code != http.StatusOK || rw.Body.String() != "{\"videos\":[]}\n" {
		t.Fatalf("list without store = %d %q", rw.Code, rw.Body)
	}
}
