package renderplan

import (
	"errors"
	"testing"

	"github.com/rechedev9/cliphub/internal/editor"
)

func TestRenderVariantFailureMessage(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result editor.Result
		err    error
		want   string
	}{
		{name: "prefers result error", result: editor.Result{Error: "encoder failed"}, err: errors.New("process failed"), want: "encoder failed"},
		{name: "falls back to process error", err: errors.New("process failed"), want: "process failed"},
		{name: "allows empty input", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := RenderVariantFailureMessage(tc.result, tc.err); got != tc.want {
				t.Fatalf("RenderVariantFailureMessage = %q, want %q", got, tc.want)
			}
		})
	}
}
