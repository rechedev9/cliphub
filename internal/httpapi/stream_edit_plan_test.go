package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/streamclips"
)

func TestPutStreamEditPlanExpectedUpdatedAt(t *testing.T) {
	savedAt := time.Date(2026, 11, 6, 10, 0, 0, 123456789, time.UTC)
	plan := streamclips.DefaultEditPlan()
	plan.UpdatedAt = savedAt
	plan.Clips = []streamclips.ClipRange{{ID: "clip-1", StartSeconds: 0, EndSeconds: 2, Title: "first"}}
	savedJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	next := plan
	next.Clips = []streamclips.ClipRange{{ID: "clip-1", StartSeconds: 0, EndSeconds: 2, Title: "second"}}
	body, err := json.Marshal(next)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		saved    []byte
		expected string
		want     int
	}{
		{name: "matching revision saves", saved: savedJSON, expected: savedAt.Format(time.RFC3339Nano), want: http.StatusOK},
		{name: "older revision conflicts", saved: savedJSON, expected: savedAt.Add(-time.Second).Format(time.RFC3339Nano), want: http.StatusConflict},
		{name: "never saved plan has nothing to race", expected: savedAt.Format(time.RFC3339Nano), want: http.StatusOK},
		{name: "no expectation saves", saved: savedJSON, want: http.StatusOK},
		{name: "bad timestamp", saved: savedJSON, expected: "yesterday", want: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := uuid.New()
			repo := newFakeStreamRepo()
			repo.jobs[id] = streamclips.Job{ID: id, Status: streamclips.StatusReady, Probe: streamclips.SourceProbe{DurationSeconds: 10}, EditPlan: tt.saved}
			r := Routes(NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithStreamRepository(repo)))
			target := "/api/stream-jobs/" + id.String() + "/edit-plan"
			if tt.expected != "" {
				target += "?expected_updated_at=" + url.QueryEscape(tt.expected)
			}
			req := httptest.NewRequest(http.MethodPut, target, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rw := httptest.NewRecorder()
			r.ServeHTTP(rw, req)
			if rw.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", rw.Code, tt.want, rw.Body.String())
			}
			var stored streamclips.EditPlan
			if len(repo.jobs[id].EditPlan) > 0 {
				if err := json.Unmarshal(repo.jobs[id].EditPlan, &stored); err != nil {
					t.Fatal(err)
				}
			}
			wantTitle := "second"
			if tt.want != http.StatusOK {
				wantTitle = "first"
			}
			if len(stored.Clips) != 1 || stored.Clips[0].Title != wantTitle {
				t.Fatalf("stored clips = %#v, want title %q", stored.Clips, wantTitle)
			}
		})
	}
}
