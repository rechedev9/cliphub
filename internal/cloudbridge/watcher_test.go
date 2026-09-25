package cloudbridge

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/renderplan"
)

func TestVariantPublishable(t *testing.T) {
	cases := []struct {
		name  string
		state *renderplan.RenderVariantState
		want  bool
	}{
		{
			name: "ready with no warnings",
			state: &renderplan.RenderVariantState{
				Status: renderplan.RenderVariantStatusReady,
			},
			want: true,
		},
		{
			name: "ready with informational warnings",
			state: &renderplan.RenderVariantState{
				Status:   renderplan.RenderVariantStatusReady,
				Warnings: []string{"loud audio"},
			},
			want: true,
		},
		{
			name:  "still rendering",
			state: &renderplan.RenderVariantState{Status: renderplan.RenderVariantStatusRendering},
			want:  false,
		},
		{
			name:  "queued",
			state: &renderplan.RenderVariantState{Status: renderplan.RenderVariantStatusQueued},
			want:  false,
		},
		{
			name:  "legacy review_required counts as rendered",
			state: &renderplan.RenderVariantState{Status: renderplan.RenderVariantStatusReview, Warnings: []string{"loud audio"}},
			want:  true,
		},
		{
			name:  "failed",
			state: &renderplan.RenderVariantState{Status: renderplan.RenderVariantStatusFailed},
			want:  false,
		},
		{
			name:  "nil state",
			state: nil,
			want:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := variantPublishable(tc.state); got != tc.want {
				t.Fatalf("variantPublishable = %v, want %v", got, tc.want)
			}
		})
	}
}
