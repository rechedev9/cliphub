package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

func TestLegacyFullDemoCannotReenterCaptureThroughRetryOrRender(t *testing.T) {
	for _, endpoint := range []string{"/generate", "/record", "/renders/gameplay-pov-60"} {
		t.Run(endpoint, func(t *testing.T) {
			h, j, store, queue, options := fullDemoAPIFixture(t)
			h.capabilities.RecordEnabled = true
			snapshot := fullDemoAPIPlan(t, h, j, options)
			snapshot.Document.PlanID = uuid.NewString()
			snapshot.Document.PlannerVersion = recapplan.LegacyPlannerVersion
			snapshot.Document.PlanHash, _ = snapshot.Document.Hash()
			snapshot.Approval.PlanHash = snapshot.Document.PlanHash
			if err := recapplan.SaveDocument(store, j.ID, snapshot.Document); err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(endpoint, "/renders/") {
				j.Status = job.StatusRecorded
				h.repo.(*fakeRepo).jobs[j.ID] = j
			}
			body := map[string]any{"preset": "gameplay-pov-60", "edit": renderplan.FullDemoEditRequest(snapshot)}
			if strings.HasPrefix(endpoint, "/renders/") {
				delete(body, "preset")
			}
			response := fullDemoAPIRequest(t, h, j, endpoint, body)
			if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), recapplan.ErrPlanStale) || len(queue.enqueued) != 0 {
				t.Fatalf("legacy plan admitted: status=%d tasks=%d body=%s", response.Code, len(queue.enqueued), response.Body.String())
			}
			saved, found, err := recapplan.LoadCurrentDocument(store, j.ID)
			if err != nil || !found || saved.PlanHash != snapshot.Document.PlanHash {
				t.Fatal("retry silently rewrote approved coverage")
			}
		})
	}
}
