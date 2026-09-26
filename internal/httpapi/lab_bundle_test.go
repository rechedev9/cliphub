package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/workers"
)

type recordingLabPreparer struct {
	calls []string
}

func (p *recordingLabPreparer) PrepareLabBundle(_ context.Context, id uuid.UUID, variant, dir string) (workers.LabBundle, error) {
	p.calls = append(p.calls, dir)
	return workers.LabBundle{SchemaVersion: "1.0", JobID: id, Variant: variant, Dir: dir, Args: []string{"--preset", variant}}, nil
}

func TestRenderLabBundleRouteWritesUnderTheLabRootWithoutEnqueueing(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	j := job.Job{ID: uuid.New(), Status: job.StatusDone, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	preparer := &recordingLabPreparer{}
	root := t.TempDir()
	routes := Routes(NewHandlers(repo, newFakeStorage(), queue, WithLabBundles(preparer, root)))

	post := func(variant string) *httptest.ResponseRecorder {
		rw := httptest.NewRecorder()
		routes.ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/renders/"+variant+"/lab-bundle", nil))
		return rw
	}

	rw := post("gameplay-pov-60")
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	var bundle workers.LabBundle
	if err := json.Unmarshal(rw.Body.Bytes(), &bundle); err != nil {
		t.Fatal(err)
	}
	wantDir := filepath.Join(root, j.ID.String()+"-gameplay-pov-60")
	if len(preparer.calls) != 1 || preparer.calls[0] != wantDir || bundle.Dir != wantDir {
		t.Fatalf("bundle dir = %v (response %q), want one bundle at %q", preparer.calls, bundle.Dir, wantDir)
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("lab bundle enqueued %d tasks, want none", len(queue.enqueued))
	}

	if rw := post("not-a-variant"); rw.Code != http.StatusBadRequest || len(preparer.calls) != 1 {
		t.Fatalf("unknown variant = %d after %d bundles, want 400 without a bundle", rw.Code, len(preparer.calls))
	}
}

func TestRenderLabBundleRouteNeedsTheRenderWorker(t *testing.T) {
	repo := newFakeRepo()
	j := job.Job{ID: uuid.New(), Status: job.StatusDone, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	rw := httptest.NewRecorder()
	Routes(NewHandlers(repo, newFakeStorage(), &fakeQueue{})).ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/renders/gameplay-pov-60/lab-bundle", nil))
	if rw.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501 without a render worker; body=%s", rw.Code, rw.Body.String())
	}
}
