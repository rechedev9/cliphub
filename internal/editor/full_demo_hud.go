package editor

import (
	"fmt"
	"os"

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
	renderer.Portrait = short.FullDemo.Effective.Options.Overlays.HUDPortrait != nil
	ass, err := renderer.ASS(*short.fullDemo.hud, customhud.Window{StartTick: item.SourceStartTick, SourceOffsetFrames: item.SourceOffsetFrames, Frames: item.EndFrame - item.StartFrame})
	if err != nil {
		return "", err
	}
	path := output + ".hud.ass"
	if err := os.WriteFile(path, []byte(ass), 0600); err != nil {
		return "", err
	}
	fontDir, err := mediafont.MaterializeHUD()
	if err != nil {
		return "", err
	}
	return customhud.ASSFilter(path, fontDir, false), nil
}

// Appended after voice inputs and camera effects. A single still is repeated
// by overlay's framesync; the main video alone determines the output length.
func fullDemoHUDPortrait(short ShortEdit, item recapplan.TimelineItem, command []string, video string) ([]string, string, error) {
	o := short.FullDemo.Effective.Options.Overlays
	if item.Role != "round" || o.HUDTheme != "focus" || o.HUDPortrait == nil {
		return command, video, nil
	}
	path, err := short.fullDemo.execution.assetPath(*o.HUDPortrait)
	if err != nil {
		return nil, "", err
	}
	index := fullDemoInputCount(command)
	command = append(command, "-i", path)
	video += fmt.Sprintf("[hudbase];[%d:v]format=rgba,scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih):color=black@0,setsar=1[hudportrait];[hudbase][hudportrait]overlay=x=%d:y=%d:eof_action=repeat:repeatlast=1:format=auto,format=yuv420p",
		index, customhud.PortraitWidth, customhud.PortraitHeight, customhud.PortraitWidth, customhud.PortraitHeight, customhud.PortraitX, customhud.PortraitY)
	return command, video, nil
}
