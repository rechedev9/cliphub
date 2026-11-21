package editor

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/brandkit"
	"github.com/rechedev9/cliphub/internal/radaroverlay"
	"github.com/rechedev9/cliphub/internal/subtitles"
)

func TestPortraitSubtitulosKeepClearOfTheOtherOverlays(t *testing.T) {
	const width, height = 1080, 1920
	box := subtitles.PlacementOf(width, height).Box
	const killfeedRows, killfeedRowStep = 5, 66
	if bottom := centeredKillfeedTop + killfeedRows*killfeedRowStep; box.Top < bottom {
		t.Errorf("block %+v reaches the centered kill feed, which ends at y %d", box, bottom)
	}
	counter := generatedKillCounterEffects(ShortEdit{KillCounter: true, DurationSeconds: 10, Kills: []KillCue{{TimeSeconds: 2}}})
	if len(counter) != 1 || counter[0].Y != "h*0.62" {
		t.Fatalf("kill counter = %+v, want one pop on the h*0.62 line", counter)
	}
	if lane := height * 62 / 100; box.Bottom > lane {
		t.Errorf("block %+v reaches the kill counter, drawn from y %d", box, lane)
	}
	for _, position := range radaroverlay.Positions {
		for _, size := range radaroverlay.Sizes {
			radar := radaroverlay.Place(radaroverlay.Placement{Position: position, Size: size, Width: width, Height: height})
			if radar.X < box.Right && box.Left < radar.X+radar.W && radar.Y < box.Bottom && box.Top < radar.Y+radar.H {
				t.Errorf("block %+v overlaps the %s radar at %s %+v", box, size, position, radar)
			}
		}
	}
}

func TestAPushedUpPortraitRadarStaysClearOfTheSubtitulos(t *testing.T) {
	const width, height = 1080, 1920
	block := subtitles.PlacementOf(width, height).Box
	lifted := 0
	inCorner := map[string]string{radaroverlay.PositionBottomLeft: brandkit.CornerBottomLeft, radaroverlay.PositionBottomRight: brandkit.CornerBottomRight}
	for position, corner := range inCorner {
		for _, size := range radaroverlay.Sizes {
			short := ShortEdit{OutputFormat: OutputFormatShort9x16, DurationSeconds: 20, SubtitlesASS: "short.subtitles.ass"}
			short.Overlays.Radar = &RadarOverlay{Position: position, Size: size}
			short.Overlays.Watermark = &BrandWatermark{Path: "logo.png", Corner: corner, Width: 400, Height: 250}
			short.Overlays.Banner = &SponsorBanner{Path: "banner.png", Width: 864, Height: 230}
			short.Effects = generatedBrandEffects(short)
			if len(short.Effects) != 2 {
				t.Fatalf("brand effects = %+v, want the watermark and the banner", short.Effects)
			}
			radar := radarBox(short)
			if radar.X < block.Right && block.Left < radar.X+radar.W && radar.Y < block.Bottom && block.Top < radar.Y+radar.H {
				t.Errorf("block %+v overlaps the %s radar at %s %+v, pushed up by the watermark and the banner", block, size, position, radar)
			}
			if radar.Y < centeredKillfeedTop {
				t.Errorf("the %s radar at %s %+v rose into the hook title and the vertical HUD", size, position, radar)
			}
			short.SubtitlesASS = ""
			if plain := radarBox(short); plain.Y > radar.Y {
				lifted++
			}
		}
	}
	if lifted == 0 {
		t.Error("no radar had to rise above the block; the case no longer pushes one into it")
	}
}
