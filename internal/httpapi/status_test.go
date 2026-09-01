package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/streamclips"
)

func TestGetJobExposesStructuredFailureCodeWithoutParsingMessage(t *testing.T) {
	cases := []struct {
		name   string
		reason string
		want   string
	}{
		{
			name:   "missing_plate",
			reason: `composite keydrop banner code "HUASO": keydrop banner style "jcorko" plate is missing`,
			want:   obs.ClassMissingPlate,
		},
		{
			name:   "capture_flake",
			reason: "recorder failed: observer target 76561198000000000 drifted from 76561198000000001 during seg-001",
			want:   obs.ClassCaptureFlake,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			id := uuid.New()
			repo.jobs[id] = job.Job{ID: id, Status: job.StatusFailed, FailureReason: tc.reason}

			h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})
			router := chi.NewRouter()
			router.Get("/api/jobs/{id}", h.GetJob)

			full := httptest.NewRecorder()
			router.ServeHTTP(full, httptest.NewRequest(http.MethodGet, "/api/jobs/"+id.String(), nil))
			if full.Code != http.StatusOK {
				t.Fatalf("full GET status = %d: %s", full.Code, full.Body.String())
			}
			var got job.Job
			if err := json.Unmarshal(full.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode full job: %v", err)
			}
			if got.FailureCode != tc.want {
				t.Fatalf("full failure_code = %q, want %q", got.FailureCode, tc.want)
			}
			if got.FailureReason != tc.reason {
				t.Fatalf("full failure_reason = %q, want original prose", got.FailureReason)
			}

			status := httptest.NewRecorder()
			router.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/api/jobs/"+id.String()+"?view=status", nil))
			if status.Code != http.StatusOK {
				t.Fatalf("status GET = %d: %s", status.Code, status.Body.String())
			}
			var view jobStatusResponse
			if err := json.Unmarshal(status.Body.Bytes(), &view); err != nil {
				t.Fatalf("decode status: %v", err)
			}
			if view.FailureCode != tc.want || view.FailureReason != tc.reason {
				t.Fatalf("status view = %+v, want code %q", view, tc.want)
			}
		})
	}
}

func TestGetStreamJobExposesAcquireFailureCodeFromSpanishReason(t *testing.T) {
	repo := newFakeStreamRepo()
	id := uuid.New()
	repo.jobs[id] = streamclips.Job{
		ID:            id,
		Status:        streamclips.StatusFailed,
		FailureReason: streamclips.AcquireReasonNotFound,
	}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithStreamRepository(repo))
	router := chi.NewRouter()
	router.Get("/api/stream-jobs/{id}", h.GetStreamJob)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/stream-jobs/"+id.String(), nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var got streamclips.Job
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.FailureCode != streamclips.AcquireCodeNotFound {
		t.Fatalf("failure_code = %q, want %q", got.FailureCode, streamclips.AcquireCodeNotFound)
	}
	if got.FailureReason != streamclips.AcquireReasonNotFound {
		t.Fatalf("failure_reason = %q, want the Spanish display text", got.FailureReason)
	}
}

func TestGetJobStatusOmitsKillPlanAndPreservesLifecycleFields(t *testing.T) {
	plan := killplan.NewPlan()
	plan.Segments = []killplan.Segment{{ID: "seg-001"}}
	cases := []struct {
		name       string
		status     job.Status
		reason     string
		wantReason string
	}{
		{name: "failed", status: job.StatusFailed, reason: "capture failed", wantReason: "capture failed"},
		{name: "parsed_idle", status: job.StatusParsed},
		{name: "recording_without_clips", status: job.StatusRecording},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			id := uuid.New()
			repo.jobs[id] = job.Job{
				ID:            id,
				Status:        tc.status,
				FailureReason: tc.reason,
				KillPlan:      &plan,
			}

			h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})
			router := chi.NewRouter()
			router.Get("/api/jobs/{id}", h.GetJob)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/jobs/"+id.String()+"?view=status", nil))

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "kill_plan") || strings.Contains(response.Body.String(), "seg-001") {
				t.Fatalf("status response contains kill plan: %s", response.Body.String())
			}
			var got jobStatusResponse
			if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode status response: %v", err)
			}
			if got.Status != tc.status || got.FailureReason != tc.wantReason {
				t.Fatalf("status response = %+v, want %s/%q", got, tc.status, tc.wantReason)
			}
			if got.Progress != nil {
				t.Fatalf("status response included progress %+v", got.Progress)
			}
		})
	}
}

func TestGetJobStatusReportsCaptureSelectionProgressWithoutKillPlan(t *testing.T) {
	repo := newFakeRepo()
	id := uuid.New()
	repo.jobs[id] = job.Job{ID: id, Status: job.StatusRecording, KillPlan: segmentPlan(4)}
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	writeCaptureSelection(t, store, id, []string{"s2", "s3"})
	writeSegmentClips(t, store, id, "s1", "s2")

	h := NewHandlers(repo, store, &fakeQueue{})
	router := chi.NewRouter()
	router.Get("/api/jobs/{id}", h.GetJob)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/jobs/"+id.String()+"?view=status", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var got jobStatusResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if got.Progress == nil || got.Progress.Done != 1 || got.Progress.Total != 2 || got.Progress.Percent != 50 {
		t.Fatalf("progress = %+v, want 1/2 50%%", got.Progress)
	}
}

func TestGetJobFullPayloadLargerThanStatusView(t *testing.T) {
	repo := newFakeRepo()
	j := benchmarkStatusJob()
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})
	router := chi.NewRouter()
	router.Get("/api/jobs/{id}", h.GetJob)

	full := httptest.NewRecorder()
	router.ServeHTTP(full, httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String(), nil))
	status := httptest.NewRecorder()
	router.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"?view=status", nil))

	if full.Code != http.StatusOK || status.Code != http.StatusOK {
		t.Fatalf("full=%d status=%d, want 200/200", full.Code, status.Code)
	}
	fullLen := full.Body.Len()
	statusLen := status.Body.Len()
	if fullLen <= statusLen {
		t.Fatalf("full GET body %d bytes is not larger than ?view=status %d bytes", fullLen, statusLen)
	}
	if strings.Contains(status.Body.String(), "kill_plan") {
		t.Fatalf("status view still embeds kill_plan: %s", status.Body.String())
	}
	if !strings.Contains(full.Body.String(), "kill_plan") {
		t.Fatal("full GET omitted kill_plan")
	}
}
