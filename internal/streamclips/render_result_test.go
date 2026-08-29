package streamclips

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewVideoEntryDerivesClipMetadata(t *testing.T) {
	clip := ClipRange{
		ID:           "clip-001",
		Title:        "Opening pick",
		StartSeconds: 12.25,
		EndSeconds:   16.75,
	}

	got := NewVideoEntry(clip, "stream-jobs/id/renders/variant/videos/clip-001.mp4")

	if got.ClipID != "clip-001" || got.Title != "Opening pick" {
		t.Fatalf("identity fields = %#v", got)
	}
	if got.Key != "stream-jobs/id/renders/variant/videos/clip-001.mp4" {
		t.Fatalf("key = %q", got.Key)
	}
	if got.DurationSeconds != 4.5 {
		t.Fatalf("duration = %v, want 4.5", got.DurationSeconds)
	}
}

func TestNewVideoPerformanceMeasuresRealWork(t *testing.T) {
	t.Parallel()
	got := NewVideoPerformance(1500*time.Millisecond, 6, 42_000)
	if got.RenderMS != 1500 || got.OutputBytes != 42_000 || got.MediaDurationSeconds != 6 {
		t.Fatalf("performance = %#v", got)
	}
	if got.RenderSecondsPerMediaSecond != 0.25 {
		t.Fatalf("seconds per media second = %v, want 0.25", got.RenderSecondsPerMediaSecond)
	}
}

func TestNewRenderResultDerivesMetadataAndCopiesVideos(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	renderedAt := time.Date(2026, 6, 12, 19, 30, 0, 0, time.FixedZone("CEST", 2*60*60))
	videos := []VideoEntry{{ClipID: "clip-001", Key: "video-key", Performance: &VideoPerformance{RenderMS: 1200}}}

	got, err := NewRenderResult(id, VariantStreamerVerticalStack, videos, renderedAt)
	if err != nil {
		t.Fatalf("NewRenderResult error = %v", err)
	}

	if got.SchemaVersion != "1.0" || got.JobID != id || got.Variant != VariantStreamerVerticalStack {
		t.Fatalf("identity fields = %#v", got)
	}
	if !got.RenderedAt.Equal(renderedAt.UTC()) || got.RenderedAt.Location() != time.UTC {
		t.Fatalf("rendered at = %s, want UTC %s", got.RenderedAt, renderedAt.UTC())
	}
	if len(got.Clips) != 1 || got.Clips[0].ClipID != "clip-001" {
		t.Fatalf("clips = %#v", got.Clips)
	}

	videos[0].ClipID = "changed"
	videos[0].Performance.RenderMS = 9999
	if got.Clips[0].ClipID != "clip-001" || got.Clips[0].Performance.RenderMS != 1200 {
		t.Fatalf("NewRenderResult did not deep-copy videos: %#v", got.Clips)
	}
}

func TestNewRenderResultRejectsUnknownVariant(t *testing.T) {
	_, err := NewRenderResult(uuid.New(), "other", nil, time.Now())
	if err == nil {
		t.Fatal("NewRenderResult error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unsupported stream render variant") {
		t.Fatalf("error = %q, want unsupported variant", err.Error())
	}
}
