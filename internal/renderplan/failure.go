package renderplan

import (
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/obs"
)

// RenderVariantFailureMessage returns the durable failure message for a render
// variant state, preferring the editor's structured result error when present.
// Studio shows and matches this text, so the remote-only failure_code prefix
// is removed.
func RenderVariantFailureMessage(result editor.Result, err error) string {
	if result.Error != "" {
		return obs.StripFailurePrefix(result.Error)
	}
	if err != nil {
		return obs.StripFailurePrefix(err.Error())
	}
	return ""
}
