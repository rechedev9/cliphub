package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/tasks"
)

func preRollGenerateBody(seconds int) string {
	return fmt.Sprintf(`{"preset":"gameplay-pov-60","segment_ids":["seg-001"],"edit":{"format":"landscape-16x9","native_hud":true,"voice_comms":true,"pre_roll_seconds":%d}}`, seconds)
}

func TestStartGenerateCarriesTheChosenPreRollToTheCapture(t *testing.T) {
	// 3 s is the analysis default, so it travels as unset and the plan keeps its windows.
	for seconds, want := range map[int]int{3: 0, 5: 5, 8: 8} {
		queue := &fakeQueue{}
		plan := killplan.NewPlan()
		plan.Segments = []killplan.Segment{{ID: "seg-001"}}
		j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
		repo := newFakeRepo()
		repo.jobs[j.ID] = j
		h := NewHandlers(repo, newFakeStorage(), queue, WithCapabilities(Capabilities{RecordEnabled: true}))

		rw := postGenerate(t, h, j.ID, preRollGenerateBody(seconds))
		if rw.Code != http.StatusAccepted {
			t.Fatalf("%d s: status = %d, want 202; body=%s", seconds, rw.Code, rw.Body.String())
		}
		var payload tasks.RecordDemoPayload
		if err := json.Unmarshal(queue.enqueued[0].Payload(), &payload); err != nil {
			t.Fatalf("unmarshal record payload: %v", err)
		}
		if payload.PreRollSeconds != want {
			t.Errorf("%d s: capture pre-roll = %d, want %d", seconds, payload.PreRollSeconds, want)
		}
		intent, ok, err := tasks.GenerateIntentFromTask(queue.enqueued[0])
		if err != nil || !ok {
			t.Fatalf("GenerateIntentFromTask = (%v, %v)", ok, err)
		}
		if intent.Edit.PreRollSeconds != want {
			t.Errorf("%d s: render pre-roll = %d, want %d", seconds, intent.Edit.PreRollSeconds, want)
		}
	}
}

func TestStartGenerateRejectsAPreRollOutsideThreeFiveOrEight(t *testing.T) {
	for _, seconds := range []int{-1, 1, 4, 10} {
		queue := &fakeQueue{}
		plan := killplan.NewPlan()
		plan.Segments = []killplan.Segment{{ID: "seg-001"}}
		j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
		repo := newFakeRepo()
		repo.jobs[j.ID] = j
		h := NewHandlers(repo, newFakeStorage(), queue, WithCapabilities(Capabilities{RecordEnabled: true}))

		rw := postGenerate(t, h, j.ID, preRollGenerateBody(seconds))
		if rw.Code != http.StatusBadRequest {
			t.Fatalf("%d s: status = %d, want 400; body=%s", seconds, rw.Code, rw.Body.String())
		}
		var body struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(rw.Body.Bytes(), &body); err != nil || body.Error != "pre_roll_seconds must be 3, 5 or 8" {
			t.Errorf("%d s: error = %q (%v), want the allowed values", seconds, body.Error, err)
		}
		if len(queue.enqueued) != 0 {
			t.Errorf("%d s: enqueued = %d, want 0", seconds, len(queue.enqueued))
		}
	}
}

func TestStartGenerateRefusesAPreRollForSeveralPlayers(t *testing.T) {
	queue := &fakeQueue{}
	j := multiPOVJob()
	repo := newFakeRepo()
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	rw := postGenerate(t, h, j.ID, preRollGenerateBody(5))
	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("enqueued = %d, want 0", len(queue.enqueued))
	}
	if rw := postGenerate(t, h, j.ID, preRollGenerateBody(3)); rw.Code != http.StatusAccepted {
		t.Fatalf("3 s: status = %d, want 202 as today; body=%s", rw.Code, rw.Body.String())
	}
}
