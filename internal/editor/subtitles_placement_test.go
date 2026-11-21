package editor

import (
	"testing"

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
