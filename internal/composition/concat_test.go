package composition

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/recording"
)

func TestConcatListEscapesPaths(t *testing.T) {
	path := `C:\tmp\clip's\seg-001.mp4`
	want := "file 'C:/tmp/clip'\\''s/seg-001.mp4'\n"
	if got := ConcatFileLine(path); got != want {
		t.Fatalf("ConcatFileLine = %q, want %q", got, want)
	}
	got := ConcatList([]recording.SegmentClip{{Path: path}})
	if got != want {
		t.Fatalf("ConcatList = %q, want %q", got, want)
	}
}

func eligibleArtifact() recording.RecordingArtifact {
	return recording.RecordingArtifact{
		Codec:           "h264",
		Width:           1920,
		Height:          1080,
		FrameRate:       "60/1",
		FrameCount:      300,
		DurationSeconds: 5,
	}
}

func TestCopyConcatEligible(t *testing.T) {
	good := eligibleArtifact()
	tests := []struct {
		name  string
		clips []recording.SegmentClip
		want  bool
	}{
		{
			name:  "empty set falls back",
			clips: nil,
			want:  false,
		},
		{
			name:  "single eligible clip",
			clips: []recording.SegmentClip{{SegmentID: "s1", Path: "s1.mp4", Artifact: good}},
			want:  true,
		},
		{
			name: "multiple eligible clips",
			clips: []recording.SegmentClip{
				{SegmentID: "s1", Artifact: good},
				{SegmentID: "s2", Artifact: eligibleArtifact()},
			},
			want: true,
		},
		{
			name:  "empty artifact metadata falls back",
			clips: []recording.SegmentClip{{SegmentID: "s1", Artifact: recording.RecordingArtifact{}}},
			want:  false,
		},
		{
			name:  "wrong codec falls back",
			clips: []recording.SegmentClip{{SegmentID: "s1", Artifact: func() recording.RecordingArtifact { a := good; a.Codec = "mpeg4"; return a }()}},
			want:  false,
		},
		{
			name:  "wrong resolution falls back",
			clips: []recording.SegmentClip{{SegmentID: "s1", Artifact: func() recording.RecordingArtifact { a := good; a.Width = 1280; return a }()}},
			want:  false,
		},
		{
			name:  "non-60 frame rate falls back",
			clips: []recording.SegmentClip{{SegmentID: "s1", Artifact: func() recording.RecordingArtifact { a := good; a.FrameRate = "30000/1001"; return a }()}},
			want:  false,
		},
		{
			name:  "zero frame count falls back",
			clips: []recording.SegmentClip{{SegmentID: "s1", Artifact: func() recording.RecordingArtifact { a := good; a.FrameCount = 0; return a }()}},
			want:  false,
		},
		{
			name:  "frame count disagrees with duration falls back",
			clips: []recording.SegmentClip{{SegmentID: "s1", Artifact: func() recording.RecordingArtifact { a := good; a.FrameCount = 200; return a }()}},
			want:  false,
		},
		{
			name:  "frame count within tolerance stays eligible",
			clips: []recording.SegmentClip{{SegmentID: "s1", Artifact: func() recording.RecordingArtifact { a := good; a.FrameCount = 302; return a }()}},
			want:  true,
		},
		{
			name: "one ineligible clip disqualifies the group",
			clips: []recording.SegmentClip{
				{SegmentID: "s1", Artifact: good},
				{SegmentID: "s2", Artifact: recording.RecordingArtifact{}},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CopyConcatEligible(tt.clips); got != tt.want {
				t.Errorf("CopyConcatEligible = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateFinalArtifactAcceptsExpectedShape(t *testing.T) {
	warnings := ValidateFinalArtifact(recording.RecordingArtifact{
		Path:            "final.mp4",
		SizeBytes:       10,
		Codec:           "h264",
		Width:           1920,
		Height:          1080,
		FrameRate:       "60/1",
		DurationSeconds: 10,
	}, 1920, 1080, 60, 10.1)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
}

func TestValidateFinalArtifactReportsBadShape(t *testing.T) {
	warnings := ValidateFinalArtifact(recording.RecordingArtifact{
		Path:            "final.mp4",
		Codec:           "mpeg4",
		Width:           1280,
		Height:          720,
		FrameRate:       "30000/1001",
		DurationSeconds: 4,
	}, 1920, 1080, 60, 10)
	joined := strings.Join(warnings, "\n")
	for _, want := range []string{"missing or empty", "codec", "resolution", "frame_rate", "duration"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("warnings missing %q:\n%s", want, joined)
		}
	}
}

func TestConcatArgBuilders(t *testing.T) {
	list := "concat-list.txt"
	out := "final.mp4"
	copyJoin := strings.Join(copyConcatArgs(list, out), " ")
	if !strings.Contains(copyJoin, "-c copy") {
		t.Errorf("copyConcatArgs missing -c copy: %s", copyJoin)
	}
	if strings.Contains(copyJoin, "libx264") || strings.Contains(copyJoin, "-vf") {
		t.Errorf("copyConcatArgs must not re-encode: %s", copyJoin)
	}
	reJoin := strings.Join(reencodeConcatArgs(list, out), " ")
	for _, want := range []string{"-c:v libx264", "fps=60,format=yuv420p", "-vf"} {
		if !strings.Contains(reJoin, want) {
			t.Errorf("reencodeConcatArgs missing %q: %s", want, reJoin)
		}
	}
	for _, join := range []string{copyJoin, reJoin} {
		for _, want := range []string{"-f concat", "-safe 0", "-i " + list, "-movflags +faststart", out} {
			if !strings.Contains(join, want) {
				t.Errorf("concat args missing %q:\n%s", want, join)
			}
		}
	}
}

func TestArtifactKeys(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	if got, want := ResultArtifactKey(id), "jobs/11111111-1111-1111-1111-111111111111/composition/composition-result.json"; got != want {
		t.Fatalf("result artifact key = %q, want %q", got, want)
	}
	if got, want := FinalArtifactKey(id), "jobs/11111111-1111-1111-1111-111111111111/composition/final.mp4"; got != want {
		t.Fatalf("final artifact key = %q, want %q", got, want)
	}
}
