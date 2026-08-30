package renderplan

import (
	"fmt"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/editor"
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

// ValidateFullDemoPublish rejects a 9:16 pack for a locked Full Demo recap.
// Biblioteca must not mark gameplay-pov-60 recap ready at 1080×1920.
func ValidateFullDemoPublish(edit EditRequest, result editor.Result) error {
	if !edit.MatchRecap || edit.Format != FormatLandscape16x9 {
		return nil
	}
	for i, short := range result.Shorts {
		if short.OutputFormat == editor.OutputFormatShort9x16 {
			return fmt.Errorf("full demo short %d published as short-9x16", i)
		}
		art := short.PublishArtifact
		if art.Width == 0 && art.Height == 0 {
			art = short.OutputArtifact
		}
		if art.Width == 1080 && art.Height == 1920 {
			return fmt.Errorf("full demo short %d is 1080x1920; gameplay-pov-60 recap must be 1920x1080", i)
		}
	}
	return nil
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
