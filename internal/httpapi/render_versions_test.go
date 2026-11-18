package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/storage"
)

func TestEditingAVideoKeepsEveryVersion(t *testing.T) {
	repo := newFakeRepo()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	j := job.Job{ID: uuid.New(), Status: job.StatusDone, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, &fakeQueue{})
	variant := editor.DefaultPreset().Name
	first := commitRenderRevision(t, h, j.ID, variant, renderplan.EditRequest{Format: "short-9x16", KillEffect: "clean"}, time.Now().Add(-time.Hour))
	second := commitRenderRevision(t, h, j.ID, variant, renderplan.EditRequest{Format: "short-9x16", KillEffect: "glitch"}, time.Now())
	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/renders/{variant}/versions", h.ListRenderVersions)
	r.Delete("/api/jobs/{id}/renders/{variant}/versions/{revision}", h.DeleteRenderVersion)
	base := "/api/jobs/" + j.ID.String() + "/renders/" + variant + "/versions"

	versions := listRenderVersions(t, r, base)
	if len(versions) != 2 || versions[0].Revision != second || !versions[0].Current || versions[1].Revision != first || versions[1].Current {
		t.Fatalf("versions = %+v, want the edit first and current, then the original", versions)
	}
	if versions[1].Edit.KillEffect != "clean" || len(versions[1].Videos) != 1 || versions[1].Videos[0] != "seg-001" || len(versions[1].SegmentIDs) != 1 {
		t.Fatalf("original version = %+v, want its options, segments and video", versions[1])
	}

	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, httptest.NewRequest(http.MethodDelete, base+"/"+second.String(), nil))
	if rw.Code != http.StatusConflict || !strings.Contains(rw.Body.String(), `"code":"version_current"`) {
		t.Fatalf("delete the current version = %d %s, want 409 version_current", rw.Code, rw.Body.String())
	}
	rw = httptest.NewRecorder()
	r.ServeHTTP(rw, httptest.NewRequest(http.MethodDelete, base+"/"+first.String(), nil))
	if rw.Code != http.StatusOK {
		t.Fatalf("delete the original = %d %s", rw.Code, rw.Body.String())
	}
	if versions := listRenderVersions(t, r, base); len(versions) != 1 || versions[0].Revision != second {
		t.Fatalf("versions after delete = %+v", versions)
	}
}

type renderVersionResponse struct {
	Revision   uuid.UUID              `json:"revision"`
	CreatedAt  time.Time              `json:"created_at"`
	Current    bool                   `json:"current"`
	Videos     []string               `json:"videos"`
	SegmentIDs []string               `json:"segment_ids"`
	Edit       renderplan.EditRequest `json:"edit"`
}

func listRenderVersions(t *testing.T, r http.Handler, url string) []renderVersionResponse {
	t.Helper()
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, url, nil))
	var body struct {
		Versions []renderVersionResponse `json:"versions"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &body); rw.Code != http.StatusOK || err != nil {
		t.Fatalf("list versions = %d %s", rw.Code, rw.Body.String())
	}
	return body.Versions
}

// commitRenderRevision stores a finished render revision the way the render worker commits one.
func commitRenderRevision(t *testing.T, h *Handlers, id uuid.UUID, variant string, edit renderplan.EditRequest, at time.Time) uuid.UUID {
	t.Helper()
	revision := uuid.New()
	loadout, err := renderplan.LoadoutForVariant(variant)
	if err != nil {
		t.Fatal(err)
	}
	state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{JobID: id, Loadout: loadout, Status: renderplan.RenderVariantStatusReady, RevisionID: revision, Now: at})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := renderplan.NewEditDocumentForLoadout(renderplan.NewEditDocumentForLoadoutOptions{JobID: id, Loadout: loadout, SegmentIDs: []string{"seg-001"}, Edit: edit})
	if err != nil {
		t.Fatal(err)
	}
	doc.CreatedAt = at.UTC()
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	for key, body := range map[string][]byte{state.EditDocumentKey: b, path.Join(state.ArtifactPrefix, "videos", "seg-001.mp4"): []byte("video")} {
		if err := h.storage.Put(key, bytes.NewReader(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.writeRenderVariantState(state); err != nil {
		t.Fatal(err)
	}
	return revision
}

// closeCounter is local storage that counts the readers it opened and that were closed.
type closeCounter struct {
	*storage.Local
	opened, closed atomic.Int32
}

type countedReader struct {
	io.ReadCloser
	closed *atomic.Int32
}

func (r countedReader) Close() error {
	r.closed.Add(1)
	return r.ReadCloser.Close()
}

func (s *closeCounter) Open(key string) (io.ReadCloser, error) {
	rc, err := s.Local.Open(key)
	if err != nil {
		return nil, err
	}
	s.opened.Add(1)
	return countedReader{ReadCloser: rc, closed: &s.closed}, nil
}

// A revision whose MP4 is gone is no version, and listing it leaves no handle open that would block deleting it on Windows.
func TestListingVersionsClosesTheEditOfARevisionWithoutVideo(t *testing.T) {
	repo := newFakeRepo()
	local, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store := &closeCounter{Local: local}
	j := job.Job{ID: uuid.New(), Status: job.StatusDone, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, &fakeQueue{})
	variant := editor.DefaultPreset().Name
	gone := commitRenderRevision(t, h, j.ID, variant, renderplan.EditRequest{Format: "short-9x16", KillEffect: "clean"}, time.Now().Add(-time.Hour))
	kept := commitRenderRevision(t, h, j.ID, variant, renderplan.EditRequest{Format: "short-9x16", KillEffect: "glitch"}, time.Now())
	prefix, err := renderplan.RenderVariantRevisionPrefix(j.ID, variant, gone)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(path.Join(prefix, "videos", "seg-001.mp4")); err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/renders/{variant}/versions", h.ListRenderVersions)
	versions := listRenderVersions(t, r, "/api/jobs/"+j.ID.String()+"/renders/"+variant+"/versions")
	if len(versions) != 1 || versions[0].Revision != kept {
		t.Fatalf("versions = %+v, want only the revision that still has its video", versions)
	}
	if opened, closed := store.opened.Load(), store.closed.Load(); opened != closed {
		t.Fatalf("listing opened %d files and closed %d", opened, closed)
	}
}
