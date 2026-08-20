package httpapi

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/storage"
)

// segmentPlan builds a kill plan with n segments (s1..sn) for progress tests.
func segmentPlan(n int) *killplan.Plan {
	plan := &killplan.Plan{}
	for i := 1; i <= n; i++ {
		plan.Segments = append(plan.Segments, killplan.Segment{ID: "s" + string(rune('0'+i))})
	}
	return plan
}

func TestCaptureProgressUsesNonCommittingAttemptDocument(t *testing.T) {
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	progress, err := recording.NewCaptureProgress(
		uuid.New(),
		[]string{"s1", "s2", "s3"},
		[]string{"s1"},
		time.Now(),
	)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(progress)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(artifacts.CaptureProgressKey(id), bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}

	got, ok := captureProgressWithTotal(store, id, job.StatusRecording, 99)
	if !ok || got.Done != 1 || got.Total != 3 || got.Percent != 33 {
		t.Fatalf("captureProgressWithTotal = (%+v, %v), want 1/3 33%%", got, ok)
	}
	clipKey, err := artifacts.SegmentClipKey(id, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if exists, err := store.Exists(clipKey); err != nil || exists {
		t.Fatalf("progress document committed a segment clip: exists=%v err=%v", exists, err)
	}
}

// writeSegmentClips writes size-1 MP4 blobs for the given segment ids so the
// dir listing sees completed clips, mirroring what the recorder uploads.
func writeSegmentClips(t *testing.T, store storage.Storage, id uuid.UUID, segmentIDs ...string) {
	t.Helper()
	for _, sid := range segmentIDs {
		key, err := artifacts.SegmentClipKey(id, sid)
		if err != nil {
			t.Fatalf("SegmentClipKey(%q): %v", sid, err)
		}
		if err := store.Put(key, bytes.NewReader([]byte("x"))); err != nil {
			t.Fatalf("put segment clip %q: %v", sid, err)
		}
	}
}

// writeCaptureSelection persists the reel's segment-id selection, mirroring what
// the record worker writes at the start of a capture.
func writeCaptureSelection(t *testing.T, store storage.Storage, id uuid.UUID, segmentIDs []string) {
	t.Helper()
	b, err := json.Marshal(segmentIDs)
	if err != nil {
		t.Fatalf("marshal selection: %v", err)
	}
	if err := store.Put(artifacts.CaptureSelectionKey(id), bytes.NewReader(b)); err != nil {
		t.Fatalf("put capture selection: %v", err)
	}
}

func TestCaptureProgressDocumentUsesStoredLivePercent(t *testing.T) {
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	progress, err := recording.NewCaptureProgress(uuid.New(), []string{"s1", "s2", "s3", "s4"}, []string{"s1", "s2", "s3"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	progress.Percent = 82
	body, err := json.Marshal(progress)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(artifacts.CaptureProgressKey(id), bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}

	got, ok := captureProgressWithTotal(store, id, job.StatusRecording, 99)
	if !ok || got.Done != 3 || got.Total != 4 || got.Percent != 82 {
		t.Fatalf("captureProgressWithTotal = (%+v, %v), want 3/4 82%%", got, ok)
	}
}

func TestCaptureProgress(t *testing.T) {
	tests := []struct {
		name        string
		status      job.Status
		plan        *killplan.Plan
		clips       []string
		selection   []string // nil = no selection artifact (fall back to full plan)
		wantOK      bool
		wantDone    int
		wantTotal   int
		wantPercent int
	}{
		{
			name:        "two of four recorded",
			status:      job.StatusRecording,
			plan:        segmentPlan(4),
			clips:       []string{"s1", "s2"},
			wantOK:      true,
			wantDone:    2,
			wantTotal:   4,
			wantPercent: 50,
		},
		{
			name:   "no segments dir yet omits progress",
			status: job.StatusRecording,
			plan:   segmentPlan(4),
			clips:  nil,
			wantOK: false,
		},
		{
			name:   "not recording omits progress",
			status: job.StatusRecorded,
			plan:   segmentPlan(4),
			clips:  []string{"s1", "s2"},
			wantOK: false,
		},
		{
			name:   "parsed idle omits progress",
			status: job.StatusParsed,
			plan:   segmentPlan(4),
			clips:  []string{"s1"},
			wantOK: false,
		},
		{
			name:   "failed omits progress",
			status: job.StatusFailed,
			plan:   segmentPlan(4),
			clips:  []string{"s1", "s2"},
			wantOK: false,
		},
		{
			name:   "no kill plan omits progress",
			status: job.StatusRecording,
			plan:   nil,
			clips:  nil,
			wantOK: false,
		},
		{
			name:        "selection reports progress without loading kill plan",
			status:      job.StatusRecording,
			plan:        nil,
			clips:       []string{"s2"},
			selection:   []string{"s2", "s3"},
			wantOK:      true,
			wantDone:    1,
			wantTotal:   2,
			wantPercent: 50,
		},
		{
			name:        "extra clips clamp done to total",
			status:      job.StatusRecording,
			plan:        segmentPlan(2),
			clips:       []string{"s1", "s2", "s3"},
			wantOK:      true,
			wantDone:    2,
			wantTotal:   2,
			wantPercent: 100,
		},
		{
			name:        "all recorded reports full",
			status:      job.StatusRecording,
			plan:        segmentPlan(3),
			clips:       []string{"s1", "s2", "s3"},
			wantOK:      true,
			wantDone:    3,
			wantTotal:   3,
			wantPercent: 100,
		},
		{
			// The reel selects s2,s3 out of a 4-segment plan; s1 is a stale clip
			// from a previous reel and must not be counted, and total is the
			// selection size (2), not the plan size (4).
			name:        "selection scopes total and ignores stale clips",
			status:      job.StatusRecording,
			plan:        segmentPlan(4),
			clips:       []string{"s1", "s2"},
			selection:   []string{"s2", "s3"},
			wantOK:      true,
			wantDone:    1,
			wantTotal:   2,
			wantPercent: 50,
		},
		{
			name:        "selection fully recorded reports full",
			status:      job.StatusRecording,
			plan:        segmentPlan(4),
			clips:       []string{"s2", "s3"},
			selection:   []string{"s2", "s3"},
			wantOK:      true,
			wantDone:    2,
			wantTotal:   2,
			wantPercent: 100,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := storage.NewLocal(t.TempDir())
			if err != nil {
				t.Fatalf("NewLocal: %v", err)
			}
			jobID := uuid.New()
			if tt.clips != nil {
				writeSegmentClips(t, store, jobID, tt.clips...)
			}
			if tt.selection != nil {
				writeCaptureSelection(t, store, jobID, tt.selection)
			}
			j := job.Job{ID: jobID, Status: tt.status, KillPlan: tt.plan}

			got, ok := captureProgress(store, j)
			if ok != tt.wantOK {
				t.Fatalf("captureProgress ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if got.Done != tt.wantDone {
				t.Errorf("done = %d, want %d", got.Done, tt.wantDone)
			}
			if got.Total != tt.wantTotal {
				t.Errorf("total = %d, want %d", got.Total, tt.wantTotal)
			}
			if got.Percent != tt.wantPercent {
				t.Errorf("percent = %d, want %d", got.Percent, tt.wantPercent)
			}
		})
	}
}
