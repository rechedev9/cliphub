package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestJobStatusViewReportsTheGenerateRunTheJobRecordHides(t *testing.T) {
	repo := newFakeRepo()
	id := uuid.New()
	repo.jobs[id] = job.Job{ID: id, Status: job.StatusRecorded}
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})
	srv := httptest.NewServer(Routes(h))
	t.Cleanup(srv.Close)
	client := tuiclient.New(tuiclient.Config{BaseURL: srv.URL})

	generate := func() *tuiclient.GenerateRun {
		t.Helper()
		single, err := client.GetJobStatus(context.Background(), id.String())
		if err != nil {
			t.Fatal(err)
		}
		res, err := http.Get(srv.URL + "/api/jobs/batch-status?items=" + id.String() + ":" + editor.PresetViral60Clean)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var batch struct {
			Items []struct {
				Job tuiclient.JobStatus `json:"job"`
			} `json:"items"`
		}
		if err := json.NewDecoder(res.Body).Decode(&batch); err != nil || len(batch.Items) != 1 {
			t.Fatalf("batch status = %d, %d items (%v)", res.StatusCode, len(batch.Items), err)
		}
		if !reflect.DeepEqual(single.Generate, batch.Items[0].Job.Generate) {
			t.Fatalf("status view generate = %+v, batch generate = %+v; want the same document", single.Generate, batch.Items[0].Job.Generate)
		}
		return single.Generate
	}

	if idle := generate(); idle != nil {
		t.Fatalf("a job with no generate run = %+v, want none", idle)
	}

	run := renderplan.GenerateIntent{Variant: editor.PresetViral60Clean, Edit: renderplan.DefaultEditRequest(), ActiveRunID: uuid.New()}
	if err := h.generateIntents.Begin(id, run, nil); err != nil {
		t.Fatal(err)
	}
	if active := generate(); active == nil || !active.Active || active.Variant != editor.PresetViral60Clean || active.FailureReason != "" {
		t.Fatalf("an accepted generate run = %+v, want it active", active)
	}

	if _, err := h.generateIntents.Fail(id, run.ActiveRunID, errors.New("recorder failed: observer target 76561198000000000 drifted from 76561198000000001 during seg-001"), nil); err != nil {
		t.Fatal(err)
	}
	failed := generate()
	if failed == nil || failed.Active || !strings.Contains(failed.FailureReason, "drifted") || failed.FailureCode != obs.ClassCaptureFlake || failed.FailedAt.IsZero() {
		t.Fatalf("a failed run that kept the earlier recording = %+v, want its failure", failed)
	}

	repo.jobs[id] = job.Job{ID: id, Status: job.StatusFailed, FailureReason: "recorder failed"}
	if hidden := generate(); hidden != nil {
		t.Fatalf("a failed job = %+v, want the failure on the job record only", hidden)
	}
	repo.jobs[id] = job.Job{ID: id, Status: job.StatusRecorded}

	next := run
	next.ActiveRunID = uuid.New()
	if err := h.generateIntents.Begin(id, next, nil); err != nil {
		t.Fatal(err)
	}
	if again := generate(); again == nil || !again.Active || again.FailureReason != "" {
		t.Fatalf("the next accepted run = %+v, want it active without the earlier failure", again)
	}
	if _, err := h.generateIntents.Finish(id, next.ActiveRunID, nil); err != nil {
		t.Fatal(err)
	}
	if done := generate(); done != nil {
		t.Fatalf("a run that handed its render off = %+v, want none", done)
	}
}
