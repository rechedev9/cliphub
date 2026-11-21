package subtitles

import (
	"fmt"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/mediafont"
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

func drawnWidth(t *testing.T, line string, fontSize int) float64 {
	t.Helper()
	faces, err := mediafont.HUDFaces()
	if err != nil {
		t.Fatal(err)
	}
	for _, face := range faces {
		if face.Family != mediafont.FamilyName {
			continue
		}
		parsed, err := sfnt.Parse(face.Data)
		if err != nil {
			t.Fatal(err)
		}
		ppem := fixed.Int26_6(float64(fontSize) * mediafont.ASSEmScale(face.Data) * 64)
		var buf sfnt.Buffer
		var advance fixed.Int26_6
		for _, r := range line {
			glyph, err := parsed.GlyphIndex(&buf, r)
			if err != nil || glyph == 0 {
				t.Fatalf("%s has no glyph for %q", face.Family, r)
			}
			step, err := parsed.GlyphAdvance(&buf, glyph, ppem, font.HintingNone)
			if err != nil {
				t.Fatal(err)
			}
			advance += step
		}
		outline, shadow := max(2, fontSize/14), 1
		return float64(advance)/64 + float64(2*outline+shadow)
	}
	t.Fatalf("no %s face", mediafont.FamilyName)
	return 0
}

func widestLine(f Format) string {
	var b strings.Builder
	for i := range f.NameRunes + f.LineRunes {
		b.WriteRune(rune('A' + i%26))
	}
	return b.String()
}

func TestPortraitSafeZoneIsTheAreaShortsTikTokAndReelsAllLeaveClear(t *testing.T) {
	if got, want := PortraitSafeZone(1080, 1920), (Zone{Left: 120, Top: 288, Right: 888, Bottom: 1248}); got != want {
		t.Fatalf("safe zone = %+v, want %+v", got, want)
	}
	if got, want := PortraitSafeZone(720, 1280), (Zone{Left: 80, Top: 192, Right: 592, Bottom: 832}); got != want {
		t.Fatalf("safe zone of a 720x1280 frame = %+v, want %+v", got, want)
	}
}

func TestPortraitBlockOfTwoSubtitulosOfTwoWidestLinesStaysInsideTheSafeZone(t *testing.T) {
	for _, frame := range [][2]int{{1080, 1920}, {720, 1280}} {
		width, height := frame[0], frame[1]
		zone, placed, format := PortraitSafeZone(width, height), PlacementOf(width, height), FormatOf(width, height)
		box := placed.Box
		if box.Left < zone.Left || box.Right > zone.Right || box.Top < zone.Top || box.Bottom > zone.Bottom {
			t.Errorf("%dx%d: block %+v leaves the safe zone %+v", width, height, box, zone)
		}
		line := widestLine(format)
		if wide := drawnWidth(t, line, placed.FontSize); wide > float64(box.Right-box.Left) {
			t.Errorf("%dx%d: a line of %d capitals is %.1f px wide at size %d and the block has %d px, so it would wrap into a third line",
				width, height, len(line), wide, placed.FontSize, box.Right-box.Left)
		}
		script, err := ASS(Script{Width: width, Height: height, DurationSeconds: 5, Cues: []Cue{talk("a", 0, 2)}})
		if err != nil {
			t.Fatal(err)
		}
		style := fmt.Sprintf(",%d,&H00FFFFFF,", placed.FontSize)
		margins := fmt.Sprintf(",2,%d,%d,%d,1\n", box.Left, width-box.Right, height-box.Bottom)
		if !strings.Contains(script, style) || !strings.Contains(script, margins) {
			t.Errorf("%dx%d: the script does not draw at size %d inside %+v:\n%s", width, height, placed.FontSize, box, script)
		}
	}
}
