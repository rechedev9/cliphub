package editor

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/recording"
)

func TestValidateShortArtifactWarnsWhenTooLongForYouTubeShorts(t *testing.T) {
	warnings := validateShortArtifact(recording.RecordingArtifact{
		SegmentID:       "seg-long",
		Path:            "short.mp4",
		SizeBytes:       1,
		DurationSeconds: 181,
		Codec:           "h264",
		Width:           1080,
		Height:          1920,
		FrameRate:       "60/1",
	}, DefaultPreset().FPS, OutputFormatShort9x16)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "want <= 180s for YouTube Shorts") {
		t.Fatalf("warnings = %#v", warnings)
	}
}

func TestValidateShortArtifactAcceptsValidOutputs(t *testing.T) {
	tests := []struct {
		name     string
		artifact recording.RecordingArtifact
		fps      int
		format   string
	}{
		{
			name: "upload-ready short",
			artifact: recording.RecordingArtifact{
				SegmentID: "seg-ok", Path: "short.mp4", SizeBytes: 1, DurationSeconds: 60,
				Codec: "h264", Width: 1080, Height: 1920, FrameRate: "60/1",
			},
			fps:    DefaultPreset().FPS,
			format: OutputFormatShort9x16,
		},
		{
			name: "configured fps",
			artifact: recording.RecordingArtifact{
				SegmentID: "seg-ok", Path: "short.mp4", SizeBytes: 1, DurationSeconds: 60,
				Codec: "h264", Width: 1080, Height: 1920, FrameRate: "24/1",
			},
			fps:    24,
			format: OutputFormatShort9x16,
		},
		{
			name: "landscape long-form",
			artifact: recording.RecordingArtifact{
				SegmentID: "seg-landscape", Path: "long-form.mp4", SizeBytes: 1, DurationSeconds: 900,
				Codec: "h264", Width: 1920, Height: 1080, FrameRate: "60/1",
			},
			fps:    DefaultPreset().FPS,
			format: OutputFormatLandscape16x9,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if warnings := validateShortArtifact(tt.artifact, tt.fps, tt.format); len(warnings) != 0 {
				t.Fatalf("warnings = %#v, want none", warnings)
			}
		})
	}
}

func TestValidateCoverArtifactAcceptsLandscapeGeometry(t *testing.T) {
	warnings := ValidateCoverArtifact(recording.RecordingArtifact{
		SegmentID: "seg-landscape",
		Path:      "cover.jpg",
		SizeBytes: 1,
		Width:     1920,
		Height:    1080,
	}, OutputFormatLandscape16x9)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none for valid landscape cover", warnings)
	}
}

func TestQualityWarningsFromFFmpegLog(t *testing.T) {
	log := `
[blackdetect @ 000] black_start:0 black_end:0.5 black_duration:0.5
[freezedetect @ 000] freeze_start:1 freeze_duration:1.2 freeze_end:2.2
[Parsed_cropdetect_2 @ 000] crop=960:1728:60:96
`
	warnings := QualityWarningsFromFFmpegLog("seg-001", log)
	if len(warnings) != 3 {
		t.Fatalf("warnings len = %d, want 3: %#v", len(warnings), warnings)
	}
}

func TestQualityWarningsIgnoresFullFrameCrop(t *testing.T) {
	warnings := QualityWarningsFromFFmpegLog("seg-001", "[Parsed_cropdetect_2 @ 000] crop=1072:1904:4:8")
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
}
