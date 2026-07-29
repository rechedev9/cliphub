package renderplan

import (
	"fmt"

	"github.com/rechedev9/fragforge/internal/artifacts"
	"github.com/rechedev9/fragforge/internal/editor"
)

// ValidateRenderVariantRunResult returns an error when the editor wrote a
// structured failure result after the process completed.
func ValidateRenderVariantRunResult(result editor.Result) error {
	if result.Error != "" {
		return fmt.Errorf("render result error: %s", result.Error)
	}
	return validateRenderShortIdentities(result.Shorts)
}

// ValidateRenderVariantUploadResult returns an error when a successful render
// result lacks any Shorts to materialize.
func ValidateRenderVariantUploadResult(result editor.Result) error {
	if result.Error != "" {
		return nil
	}
	return validateRenderShortIdentities(result.Shorts)
}

func validateRenderShortIdentities(shorts []editor.ShortResult) error {
	if len(shorts) == 0 {
		return fmt.Errorf("render result has no shorts")
	}
	seen := make(map[string]struct{}, len(shorts))
	for i, short := range shorts {
		if err := artifacts.ValidateArtifactToken("render segment id", short.SegmentID); err != nil {
			return fmt.Errorf("render result short %d: %w", i, err)
		}
		if _, duplicate := seen[short.SegmentID]; duplicate {
			return fmt.Errorf("render result contains duplicate segment id %q", short.SegmentID)
		}
		seen[short.SegmentID] = struct{}{}
	}
	return nil
}
