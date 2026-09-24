package renderplan

import (
	"errors"
	"testing"

	"github.com/rechedev9/cliphub/internal/editor"
)

func TestRenderVariantFailureMessagePrefersResultError(t *testing.T) {
	got := RenderVariantFailureMessage(editor.Result{Error: "encoder failed"}, errors.New("process failed"))
	if got != "encoder failed" {
		t.Fatalf("RenderVariantFailureMessage = %q, want result error", got)
	}
}

func TestRenderVariantFailureMessageFallsBackToProcessError(t *testing.T) {
	got := RenderVariantFailureMessage(editor.Result{}, errors.New("process failed"))
	if got != "process failed" {
		t.Fatalf("RenderVariantFailureMessage = %q, want process error", got)
	}
}

func TestRenderVariantFailureMessageAllowsEmptyInput(t *testing.T) {
	got := RenderVariantFailureMessage(editor.Result{}, nil)
	if got != "" {
		t.Fatalf("RenderVariantFailureMessage = %q, want empty", got)
	}
}

func TestRenderVariantFailureMessageDropsFailureCode(t *testing.T) {
	resultErr := RenderVariantFailureMessage(editor.Result{Error: "failure_code=audio_master_exhausted substage=audio_master; audio_loudness_failed after three masters"}, nil)
	processErr := RenderVariantFailureMessage(editor.Result{}, errors.New("failure_code=delivery_verify_failed substage=delivery_verify; full_demo_output_invalid"))
	if resultErr != "audio_loudness_failed after three masters" || processErr != "full_demo_output_invalid" {
		t.Fatalf("messages = %q, %q", resultErr, processErr)
	}
}
