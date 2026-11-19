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

func TestStartGenerateRecordsTheSummaryInTheChosenCaptureStyle(t *testing.T) {
	cases := []struct{ edit, wantHUD string }{
		{`"native_hud":true`, "gameplay"},
		{`"capture_hud":"deathnotices"`, "deathnotices"},
		{`"capture_hud":"clean"`, "clean"},
	}
	for _, tc := range cases {
		queue := &fakeQueue{}
		plan := killplan.NewPlan()
		plan.Segments = []killplan.Segment{{ID: "seg-001"}}
		j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
		repo := newFakeRepo()
		repo.jobs[j.ID] = j
		h := NewHandlers(repo, newFakeStorage(), queue, WithCapabilities(Capabilities{RecordEnabled: true}))

		body := fmt.Sprintf(`{"preset":"gameplay-pov-60","segment_ids":["seg-001"],"edit":{"format":"landscape-16x9","voice_comms":true,%s}}`, tc.edit)
		rw := postGenerate(t, h, j.ID, body)
		if rw.Code != http.StatusAccepted {
			t.Fatalf("%s: status = %d, want 202; body=%s", tc.edit, rw.Code, rw.Body.String())
		}
		var payload tasks.RecordDemoPayload
		if err := json.Unmarshal(queue.enqueued[0].Payload(), &payload); err != nil {
			t.Fatalf("unmarshal record payload: %v", err)
		}
		if payload.HUDMode != tc.wantHUD {
			t.Errorf("%s: capture HUD = %q, want %q", tc.edit, payload.HUDMode, tc.wantHUD)
		}
	}
}

func TestStartGenerateRejectsACaptureStyleOutsideTheSummary(t *testing.T) {
	for _, edit := range []string{
		`{"format":"landscape-16x9","capture_hud":"radar"}`,
		`{"format":"short-9x16","capture_hud":"clean"}`,
	} {
		queue := &fakeQueue{}
		plan := killplan.NewPlan()
		plan.Segments = []killplan.Segment{{ID: "seg-001"}}
		j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
		repo := newFakeRepo()
		repo.jobs[j.ID] = j
		h := NewHandlers(repo, newFakeStorage(), queue, WithCapabilities(Capabilities{RecordEnabled: true}))

		rw := postGenerate(t, h, j.ID, `{"preset":"gameplay-pov-60","segment_ids":["seg-001"],"edit":`+edit+`}`)
		if rw.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400; body=%s", edit, rw.Code, rw.Body.String())
		}
		if len(queue.enqueued) != 0 {
			t.Errorf("%s: enqueued = %d, want 0", edit, len(queue.enqueued))
		}
	}
}
