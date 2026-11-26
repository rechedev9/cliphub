package workers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

func TestRecordWorkerForgetsAnEarlierGenerateFailureAfterACapture(t *testing.T) {
	store := newFakeStorage()
	repo, id := parsedRecordJob(store)
	w := newRecordWorkerForTest(repo, store, t)
	putJSON(t, store, artifacts.GenerateIntentKey(id), renderplan.GenerateIntent{
		Variant:       editor.PresetViral60Clean,
		Edit:          renderplan.DefaultEditRequest(),
		FailureReason: "recorder crashed before producing a result",
		FailedAt:      time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC),
	})

	if err := w.HandleRecordDemo(context.Background(), recordTask(t, id)); err != nil {
		t.Fatalf("HandleRecordDemo error = %v", err)
	}
	if repo.jobs[id].Status != job.StatusRecorded {
		t.Fatalf("Status = %s, want recorded", repo.jobs[id].Status)
	}
	var intent renderplan.GenerateIntent
	if err := json.Unmarshal(store.files[artifacts.GenerateIntentKey(id)], &intent); err != nil {
		t.Fatal(err)
	}
	if intent.FailureReason != "" || !intent.FailedAt.IsZero() || intent.Variant != editor.PresetViral60Clean {
		t.Fatalf("generate intent after a later capture = %+v, want the same choice without the earlier failure", intent)
	}
}
