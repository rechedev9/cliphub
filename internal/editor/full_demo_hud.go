package editor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/mediafont"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

func validateFullDemoHUD(evidence *FullDemoHUDEvidence, document recapplan.Document) error {
	theme := document.Options.Overlays.HUDTheme
	if theme == "" {
		if evidence != nil {
			return fmt.Errorf("native Full Demo must not carry custom HUD evidence")
		}
		return nil
	}
	if evidence == nil || evidence.RendererVersion != customhud.Version || evidence.Theme != theme || evidence.DemoSHA256 != document.Input.DemoSHA256 || !recapplan.ValidHash(evidence.TelemetrySHA256) || evidence.SnapshotCount < 1 {
		return fmt.Errorf("custom HUD evidence differs from its approved render")
	}
	return nil
}

func fullDemoHUDFilter(short ShortEdit, item recapplan.TimelineItem, output string) (string, error) {
	if short.FullDemo == nil || short.FullDemo.Effective.Options.Overlays.HUDTheme == "" || item.Role != "round" {
		return "", nil
	}
	if short.fullDemo == nil || short.fullDemo.hud == nil {
		return "", fmt.Errorf("custom HUD telemetry is missing")
	}
	renderer, err := customhud.NewRenderer(short.FullDemo.Effective.Options.Overlays.HUDTheme)
	if err != nil {
		return "", err
	}
	ass, err := renderer.ASS(*short.fullDemo.hud, customhud.Window{StartTick: item.SourceStartTick, SourceOffsetFrames: item.SourceOffsetFrames, Frames: item.EndFrame - item.StartFrame})
	if err != nil {
		return "", err
	}
	path := output + ".hud.ass"
	if err := os.WriteFile(path, []byte(ass), 0600); err != nil {
		return "", err
	}
	fontPath, err := mediafont.Materialize()
	if err != nil {
		return "", err
	}
	return customhud.ASSFilter(path, filepath.Dir(fontPath), false), nil
}
