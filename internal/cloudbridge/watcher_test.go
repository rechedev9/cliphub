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
			name: "ready with reviewed warnings on this revision",
			state: &renderplan.RenderVariantState{
				Status:         renderplan.RenderVariantStatusReady,
				ArtifactPrefix: "jobs/x/renders/viral-60-clean",
				Warnings:       []string{"loud audio"},
				ReviewResolution: &renderplan.RenderReviewResolution{
					ArtifactPrefix: "jobs/x/renders/viral-60-clean",
					Warnings:       []string{"loud audio"},
				},
			},
			want: true,
		},
		{
			name: "ready with unreviewed warnings",
			state: &renderplan.RenderVariantState{
				Status:   renderplan.RenderVariantStatusReady,
				Warnings: []string{"loud audio"},
			},
			want: false,
		},
		{
			name: "ready with a review that belongs to an older revision",
			state: &renderplan.RenderVariantState{
				Status:         renderplan.RenderVariantStatusReady,
				ArtifactPrefix: "jobs/x/renders/viral-60-clean-v2",
				Warnings:       []string{"loud audio"},
				ReviewResolution: &renderplan.RenderReviewResolution{
					ArtifactPrefix: "jobs/x/renders/viral-60-clean",
					Warnings:       []string{"loud audio"},
				},
			},
			want: false,
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
			name:  "awaiting review",
			state: &renderplan.RenderVariantState{Status: renderplan.RenderVariantStatusReview},
			want:  false,
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
