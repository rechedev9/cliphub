package renderplan

import (
	"strings"
	"testing"

	"github.com/rechedev9/fragforge/internal/editor"
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

func TestValidateRenderVariantUploadResultAcceptsFailedResult(t *testing.T) {
	err := ValidateRenderVariantUploadResult(editor.Result{Error: "editor failed"})
	if err != nil {
		t.Fatalf("ValidateRenderVariantUploadResult error = %v", err)
	}
}
