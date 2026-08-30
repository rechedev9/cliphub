package renderplan

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/recording"
)

func TestValidateRenderVariantRunResultAcceptsSuccessfulResult(t *testing.T) {
	err := ValidateRenderVariantRunResult(editor.Result{
		Shorts: []editor.ShortResult{{SegmentID: "seg-001"}},
	})
	if err != nil {
		t.Fatalf("ValidateRenderVariantRunResult error = %v", err)
	}
}

func TestValidateRenderVariantRunResultRejectsFailedResult(t *testing.T) {
	err := ValidateRenderVariantRunResult(editor.Result{Error: "editor failed"})
	if err == nil {
		t.Fatal("ValidateRenderVariantRunResult error = nil, want render result error")
	}
	if !strings.Contains(err.Error(), "render result error: editor failed") {
		t.Fatalf("error = %q, want render result error", err.Error())
	}
}

func TestValidateRenderVariantRunResultRejectsInvalidShortIdentities(t *testing.T) {
	tests := []struct {
		name   string
		shorts []editor.ShortResult
		want   string
	}{
		{
			name:   "empty",
			shorts: []editor.ShortResult{{}},
			want:   "invalid render segment id",
		},
		{
			name: "duplicate",
			shorts: []editor.ShortResult{
				{SegmentID: "seg-001"},
				{SegmentID: "seg-001"},
			},
			want: `duplicate segment id "seg-001"`,
		},
		{
			name:   "unsafe",
			shorts: []editor.ShortResult{{SegmentID: "../seg-001"}},
			want:   "render segment id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRenderVariantRunResult(editor.Result{Shorts: tt.shorts})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateRenderVariantRunResult error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestValidateRenderVariantUploadResultAcceptsSuccessfulShorts(t *testing.T) {
	err := ValidateRenderVariantUploadResult(editor.Result{
		Shorts: []editor.ShortResult{{SegmentID: "seg-001"}},
	})
	if err != nil {
		t.Fatalf("ValidateRenderVariantUploadResult error = %v", err)
	}
}

func TestValidateRenderVariantUploadResultRejectsSuccessfulEmptyResult(t *testing.T) {
	err := ValidateRenderVariantUploadResult(editor.Result{})
	if err == nil {
		t.Fatal("ValidateRenderVariantUploadResult error = nil, want empty-result error")
	}
	if !strings.Contains(err.Error(), "render result has no shorts") {
		t.Fatalf("error = %q, want no shorts", err.Error())
	}
}

func TestValidateFullDemoPublishRejectsNineBySixteen(t *testing.T) {
	t.Parallel()
	edit := RecapEditRequest()
	err := ValidateFullDemoPublish(edit, editor.Result{
		Shorts: []editor.ShortResult{{
			SegmentID:       "demo-compilation",
			OutputFormat:    editor.OutputFormatShort9x16,
			PublishArtifact: recording.RecordingArtifact{Width: 1080, Height: 1920},
		}},
	})
	if err == nil {
		t.Fatal("ValidateFullDemoPublish error = nil, want 9:16 rejection")
	}
	if !strings.Contains(err.Error(), "short-9x16") && !strings.Contains(err.Error(), "1080x1920") {
		t.Fatalf("ValidateFullDemoPublish error = %v, want 9:16 rejection", err)
	}
}

func TestValidateFullDemoPublishAcceptsLandscapeRecap(t *testing.T) {
	t.Parallel()
	err := ValidateFullDemoPublish(RecapEditRequest(), editor.Result{
		Shorts: []editor.ShortResult{{
			SegmentID:       "demo-compilation",
			OutputFormat:    editor.OutputFormatLandscape16x9,
			PublishArtifact: recording.RecordingArtifact{Width: 1920, Height: 1080},
		}},
	})
	if err != nil {
		t.Fatalf("ValidateFullDemoPublish error = %v", err)
	}
}

func TestValidateFullDemoPublishIgnoresShorts(t *testing.T) {
	t.Parallel()
	err := ValidateFullDemoPublish(DefaultEditRequest(), editor.Result{
		Shorts: []editor.ShortResult{{
			SegmentID:       "seg-001",
			OutputFormat:    editor.OutputFormatShort9x16,
			PublishArtifact: recording.RecordingArtifact{Width: 1080, Height: 1920},
		}},
	})
	if err != nil {
		t.Fatalf("ValidateFullDemoPublish error = %v, want nil for Shorts", err)
	}
}

func TestValidateRenderVariantUploadResultAcceptsFailedResult(t *testing.T) {
	err := ValidateRenderVariantUploadResult(editor.Result{Error: "editor failed"})
	if err != nil {
		t.Fatalf("ValidateRenderVariantUploadResult error = %v", err)
	}
}
