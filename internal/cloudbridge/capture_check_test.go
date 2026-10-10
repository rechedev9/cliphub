package cloudbridge

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/recording"
)

// take is one probed 1080p60 raw take at the given average bitrate.
func take(segment string, seconds, megabitsPerSecond float64) recording.RecordingArtifact {
	return recording.RecordingArtifact{
		SegmentID:       segment,
		Type:            "video",
		Role:            "raw",
		SizeBytes:       int64(megabitsPerSecond * 1e6 / 8 * seconds),
		DurationSeconds: seconds,
		FrameCount:      int64(seconds * 60),
		FrameRate:       "60/1",
		Codec:           "h264",
		Width:           1920,
		Height:          1080,
	}
}

func TestMeasureCaptureTellsTheBlackCaptureFromRealFootage(t *testing.T) {
	audio := recording.RecordingArtifact{Type: "audio", Role: "raw", SizeBytes: 8927276, DurationSeconds: 50.6}
	unprobed := recording.RecordingArtifact{Type: "video", Role: "raw", SizeBytes: 216346452}
	cases := []struct {
		name      string
		artifacts []recording.RecordingArtifact
		measured  bool
		black     bool
	}{
		{
			// docs/incidents.md, 2026-09-23: 2.4 Mb/s at 1080p60.
			name:      "the black capture incident",
			artifacts: []recording.RecordingArtifact{take("round-001", 50.6, 2.4), audio},
			measured:  true,
			black:     true,
		},
		{
			name:      "a normal capture at about 40 Mb/s",
			artifacts: []recording.RecordingArtifact{take("seg-001", 9.3, 41.0), audio},
			measured:  true,
		},
		{
			// The quietest real take measured on a healthy machine: 18.6 Mb/s.
			name:      "low-motion footage",
			artifacts: []recording.RecordingArtifact{take("seg-002", 7.8, 18.6)},
			measured:  true,
		},
		{
			name:      "footage three times quieter than anything measured",
			artifacts: []recording.RecordingArtifact{take("seg-002", 7.8, 6.2)},
			measured:  true,
		},
		{
			name: "one black window among normal ones",
			artifacts: []recording.RecordingArtifact{
				take("seg-001", 8, 2.4), take("seg-002", 8, 38), take("seg-003", 8, 36),
			},
			measured: true,
		},
		{
			name:      "every window black",
			artifacts: []recording.RecordingArtifact{take("seg-001", 8, 2.4), take("seg-002", 6, 3.1)},
			measured:  true,
			black:     true,
		},
		{
			name: "segment clips when no raw take was probed",
			artifacts: []recording.RecordingArtifact{
				unprobed,
				{Type: "video", Role: "segment", SizeBytes: 15_180_000, DurationSeconds: 50.6, FrameRate: "60/1", Width: 1920, Height: 1080},
			},
			measured: true,
			black:    true,
		},
		{name: "nothing probed", artifacts: []recording.RecordingArtifact{unprobed, audio}},
		{name: "no artifacts"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			measure, ok := measureCapture(tc.artifacts)
			if ok != tc.measured {
				t.Fatalf("measured = %v, want %v", ok, tc.measured)
			}
			if !ok {
				return
			}
			if got := measure.looksBlack(); got != tc.black {
				t.Fatalf("looksBlack = %v at %.4f bits per pixel, want %v", got, measure.bitsPerPixel(), tc.black)
			}
		})
	}
}

// writeRecordingResult stores what the recorder leaves next to a capture.
func (h *harness) writeRecordingResult(localJobID string, artifacts ...recording.RecordingArtifact) {
	h.t.Helper()
	body, err := json.Marshal(recording.RecordingResult{Artifacts: artifacts})
	if err != nil {
		h.t.Fatal(err)
	}
	if err := h.files.Put(recording.ResultArtifactKey(uuid.MustParse(localJobID)), bytes.NewReader(body)); err != nil {
		h.t.Fatalf("put recording result: %v", err)
	}
}

func TestWorkerDoesNotDeliverABlackCapture(t *testing.T) {
	h := newHarness(t)
	h.renderOnCapture(map[string][]byte{"seg-003": fakeVideo("black", 40)})
	render := h.local.onAdmit
	h.local.onAdmit = func(job *fakeLocalJob) {
		render(job)
		h.writeRecordingResult(job.id, take("seg-003", 12, 2.4))
	}
	id := h.queueJob(nil)
	h.start()

	eventually(t, "the failure report", func() bool { return h.portal.jobStatus(id) == "failed" })
	h.settled()
	h.portal.with(func() {
		job := h.portal.jobs[id]
		if len(job.fails) != 1 {
			t.Fatalf("fail reports = %+v, want exactly one", job.fails)
		}
		report := job.fails[0]
		// A machine-class code: the portal requeues the job and pauses the worker.
		if report.Code != codeCaptureIncompatible {
			t.Fatalf("code = %q (detail %q), want %s", report.Code, report.Detail, codeCaptureIncompatible)
		}
		for _, want := range []string{"black capture suspected", "2.4 Mb/s", "0.019 bits per pixel", "look at the video before resuming"} {
			if !strings.Contains(report.Detail, want) {
				t.Fatalf("detail = %q, want it to say %q", report.Detail, want)
			}
		}
		// The portal shows the first 300 characters as the pause reason.
		if len(report.Detail) > 300 {
			t.Fatalf("detail is %d characters, want the operator to read all of it in the pause reason", len(report.Detail))
		}
		if len(job.phases) != 0 || len(job.artifacts) != 0 {
			t.Fatalf("a black capture reached uploading: phases=%d artifacts=%d", len(job.phases), len(job.artifacts))
		}
	})
	if !h.local.only().deleted {
		t.Fatal("the black capture's local job was left behind")
	}
}

func TestWorkerDeliversLowMotionFootage(t *testing.T) {
	h := newHarness(t)
	h.renderOnCapture(map[string][]byte{"seg-003": fakeVideo("quiet", 40)})
	render := h.local.onAdmit
	h.local.onAdmit = func(job *fakeLocalJob) {
		render(job)
		h.writeRecordingResult(job.id, take("seg-003", 7.8, 18.6))
	}
	id := h.queueJob(nil)
	h.start()

	eventually(t, "the portal to mark the job done", func() bool { return h.portal.jobStatus(id) == "done" })
	h.portal.with(func() {
		if fails := h.portal.jobs[id].fails; len(fails) != 0 {
			t.Fatalf("fail reports = %+v, want none", fails)
		}
	})
}
