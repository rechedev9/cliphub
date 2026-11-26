package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

func TestJobStatusViewReportsTheGenerateRunTheJobRecordHides(t *testing.T) {
	repo := newFakeRepo()
	id := uuid.New()
	repo.jobs[id] = job.Job{ID: id, Status: job.StatusRecorded}
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})
	router := chi.NewRouter()
	router.Get("/api/jobs/batch-status", h.BatchStatus)
	router.Get("/api/jobs/{id}", h.GetJob)

	generate := func() map[string]any {
		t.Helper()
		single := httptest.NewRecorder()
		router.ServeHTTP(single, httptest.NewRequest(http.MethodGet, "/api/jobs/"+id.String()+"?view=status", nil))
		batch := httptest.NewRecorder()
		router.ServeHTTP(batch, httptest.NewRequest(http.MethodGet, "/api/jobs/batch-status?items="+id.String()+":"+editor.PresetViral60Clean, nil))
		if single.Code != http.StatusOK || batch.Code != http.StatusOK {
			t.Fatalf("status = %d, batch = %d: %s %s", single.Code, batch.Code, single.Body.String(), batch.Body.String())
		}
		var one struct {
			Generate map[string]any `json:"generate"`
		}
		var many struct {
			Items []struct {
				Job struct {
					Generate map[string]any `json:"generate"`
				} `json:"job"`
			} `json:"items"`
		}
		if err := json.Unmarshal(single.Body.Bytes(), &one); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(batch.Body.Bytes(), &many); err != nil || len(many.Items) != 1 {
			t.Fatalf("batch = %s (%v)", batch.Body.String(), err)
		}
		if !reflect.DeepEqual(one.Generate, many.Items[0].Job.Generate) {
			t.Fatalf("status view generate = %v, batch generate = %v; want the same document", one.Generate, many.Items[0].Job.Generate)
		}
		return one.Generate
	}

	if idle := generate(); idle != nil {
		t.Fatalf("a job with no generate run = %v, want none", idle)
	}

	run := renderplan.GenerateIntent{Variant: editor.PresetViral60Clean, Edit: renderplan.DefaultEditRequest(), ActiveRunID: uuid.New()}
	if err := h.generateIntents.Begin(id, run, nil); err != nil {
		t.Fatal(err)
	}
	if active := generate(); active["active"] != true || active["variant"] != editor.PresetViral60Clean || active["failure_reason"] != nil {
		t.Fatalf("an accepted generate run = %v, want it active", active)
	}

	if _, err := h.generateIntents.Fail(id, run.ActiveRunID, errors.New("recorder failed: observer target 76561198000000000 drifted from 76561198000000001 during seg-001"), nil); err != nil {
		t.Fatal(err)
	}
	failed := generate()
	reason, _ := failed["failure_reason"].(string)
	if failed["active"] != false || !strings.Contains(reason, "drifted") || failed["failure_code"] != obs.ClassCaptureFlake || failed["failed_at"] == nil {
		t.Fatalf("a failed run that kept the earlier recording = %v, want its failure", failed)
	}

	next := run
	next.ActiveRunID = uuid.New()
	if err := h.generateIntents.Begin(id, next, nil); err != nil {
		t.Fatal(err)
	}
	if again := generate(); again["active"] != true || again["failure_reason"] != nil {
		t.Fatalf("the next accepted run = %v, want it active without the earlier failure", again)
	}
	if _, err := h.generateIntents.Finish(id, next.ActiveRunID, nil); err != nil {
		t.Fatal(err)
	}
	if done := generate(); done != nil {
		t.Fatalf("a run that handed its render off = %v, want none", done)
	}
}
