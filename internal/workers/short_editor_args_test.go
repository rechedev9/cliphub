package workers

import (
	"slices"
	"testing"

	"github.com/rechedev9/cliphub/internal/renderplan"
)

func TestCompileSegmentsArgsSeparateShortsRendersEachPlayAlone(t *testing.T) {
	ids := []string{"seg-001", "seg-004"}
	if got := compileSegmentsArgs(renderplan.EditRequest{}, ids); !slices.Equal(got, []string{"--compile-segments", "--segments", "seg-001,seg-004"}) {
		t.Fatalf("compiled args = %v", got)
	}
	if got := compileSegmentsArgs(renderplan.EditRequest{SeparateShorts: true}, ids); !slices.Equal(got, []string{"--segments", "seg-001,seg-004"}) {
		t.Fatalf("separate args = %v, want the selection without --compile-segments", got)
	}
	if got := compileSegmentsArgs(renderplan.EditRequest{SeparateShorts: true}, ids[:1]); got != nil {
		t.Fatalf("single segment args = %v, want none", got)
	}
}
