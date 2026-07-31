package renderplan

import (
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/tickcut/internal/editor"
	"github.com/rechedev9/tickcut/internal/recording"
)

func TestNewQualityReportMarksReadyForUploadReadyArtifact(t *testing.T) {
	report := NewQualityReport(uuid.New(), "viral-60-clean", editor.Result{
		Shorts: []editor.ShortResult{{
			SegmentID: "seg-001",
			PublishArtifact: recording.RecordingArtifact{
				SizeBytes:       10,
				Width:           1080,
				Height:          1920,
				DurationSeconds: 30,
				Codec:           "h264",
			},
		}},
	})

	if report.Status != "ready" {
		t.Fatalf("status = %q, want ready", report.Status)
	}
	if report.Items[0].Status != "ready" {
		t.Fatalf("item status = %q, want ready", report.Items[0].Status)
	}
}

func TestNewQualityReportWarnsForBadArtifactShape(t *testing.T) {
	report := NewQualityReport(uuid.New(), "viral-60-clean", editor.Result{
		Shorts: []editor.ShortResult{{
			SegmentID: "seg-001",
			PublishArtifact: recording.RecordingArtifact{
				SizeBytes:       10,
				Width:           1920,
				Height:          1080,
				DurationSeconds: 75,
			},
		}},
	})

	if report.Status != "warning" {
		t.Fatalf("status = %q, want warning", report.Status)
	}
	for _, want := range []string{"unexpected_output_resolution"} {
		if !containsString(report.Items[0].Warnings, want) {
			t.Fatalf("warnings = %v, missing %q", report.Items[0].Warnings, want)
		}
	}
}

func TestNewQualityReportAcceptsLandscapeLongForm(t *testing.T) {
	result := editor.Result{
		OutputFormat: editor.OutputFormatLandscape16x9,
		Shorts: []editor.ShortResult{{
			SegmentID: "seg-001",
			PublishArtifact: recording.RecordingArtifact{
				SizeBytes:       10,
				Width:           1920,
				Height:          1080,
				DurationSeconds: 900,
				Codec:           "h264",
			},
		}},
	}
	report := NewQualityReport(uuid.New(), "viral-60-clean", result)
	if report.Status != "ready" || report.Items[0].Status != "ready" || len(report.Items[0].Warnings) != 0 {
		t.Fatalf("report = %#v, want result-level landscape fallback without resolution or duration warnings", report)
	}
	if warnings := CompleteRenderWarnings(result); len(warnings) != 0 {
		t.Fatalf("CompleteRenderWarnings = %v, want none for result-level landscape fallback", warnings)
	}
}

func TestQualityReportShortOutputFormatOverridesResultFormat(t *testing.T) {
	result := editor.Result{
		OutputFormat: editor.OutputFormatLandscape16x9,
		Shorts: []editor.ShortResult{{
			SegmentID:    "seg-001",
			OutputFormat: editor.OutputFormatShort9x16,
			PublishArtifact: recording.RecordingArtifact{
				SizeBytes:       10,
				Width:           1080,
				Height:          1920,
				DurationSeconds: 30,
				Codec:           "h264",
			},
		}},
	}
	report := NewQualityReport(uuid.New(), "viral-60-clean", result)
	if report.Status != "ready" || report.Items[0].Status != "ready" || len(report.Items[0].Warnings) != 0 {
		t.Fatalf("report = %#v, want explicit short format to override result landscape format", report)
	}
	if warnings := CompleteRenderWarnings(result); len(warnings) != 0 {
		t.Fatalf("CompleteRenderWarnings = %v, want explicit short override without warnings", warnings)
	}
}

func TestNewQualityReportMarksRendererWarnings(t *testing.T) {
	report := NewQualityReport(uuid.New(), "viral-60-clean", editor.Result{
		Warnings: []string{"quality seg-001 detected frozen frames"},
		Shorts: []editor.ShortResult{{
			SegmentID: "seg-001",
			PublishArtifact: recording.RecordingArtifact{
				SizeBytes:       10,
				Width:           1080,
				Height:          1920,
				DurationSeconds: 30,
				Codec:           "h264",
			},
		}},
	})
	if report.Status != "warning" || report.Items[0].Status != "ready" {
		t.Fatalf("report = %#v, want renderer warnings to block ready status", report)
	}
}

func TestLegacyArtifactWithoutMetadataStaysUnknownWithoutWarningGate(t *testing.T) {
	result := editor.Result{
		Shorts: []editor.ShortResult{{
			SegmentID: "seg-legacy",
		}},
	}
	report := NewQualityReport(uuid.New(), "viral-60-clean", result)

	if report.Status != "unknown" {
		t.Fatalf("report status = %q, want unknown", report.Status)
	}
	if got := report.Items[0].Status; got != "unknown" {
		t.Fatalf("item status = %q, want unknown", got)
	}
	if len(report.Items[0].Warnings) != 0 {
		t.Fatalf("item warnings = %v, want none for unknown legacy evidence", report.Items[0].Warnings)
	}
	if warnings := CompleteRenderWarnings(result); len(warnings) != 0 {
		t.Fatalf("CompleteRenderWarnings = %v, want same no-warning legacy policy as report", warnings)
	}
}

func TestNewQualityReportMarksFailedFromRenderError(t *testing.T) {
	report := NewQualityReport(uuid.New(), "viral-60-clean", editor.Result{
		Error: "render failed",
	})

	if report.Status != "failed" {
		t.Fatalf("status = %q, want failed", report.Status)
	}
}

func TestCompleteRenderWarningsIncludesArtifactQAWithoutDuplicates(t *testing.T) {
	result := editor.Result{
		Warnings: []string{"renderer warning"},
		Shorts: []editor.ShortResult{{
			SegmentID: "seg-001",
			PublishArtifact: recording.RecordingArtifact{
				SizeBytes:  10,
				Width:      1920,
				Height:     1080,
				ProbeError: "ffprobe failed",
			},
		}, {
			SegmentID: "seg-002",
			PublishArtifact: recording.RecordingArtifact{
				Path:   "seg-002.mp4",
				Width:  1080,
				Height: 1920,
			},
		}},
	}
	first := CompleteRenderWarnings(result)
	result.Warnings = first
	second := CompleteRenderWarnings(result)

	for _, want := range []string{
		"renderer warning",
		"quality seg-001: probe_error",
		"quality seg-001: unexpected_output_resolution",
		"quality seg-002: missing_or_empty_video",
	} {
		if !containsString(first, want) {
			t.Fatalf("warnings = %v, missing %q", first, want)
		}
	}
	if len(second) != len(first) {
		t.Fatalf("second merge = %v, want no duplicates from %v", second, first)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
