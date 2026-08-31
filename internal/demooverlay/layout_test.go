package demooverlay

import (
	"bytes"
	"image/png"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/parser"
)

func TestDefaultLayoutKeepsNativeHUDChannelAndFullFrameOutro(t *testing.T) {
	l := DefaultLayout()
	if l.Width != 1920 || l.Height != 1080 {
		t.Fatalf("frame = %dx%d", l.Width, l.Height)
	}
	if l.Intro.LeftPanelX != 42 || l.Intro.RightPanelX != 1308 {
		t.Fatalf("panels x = %d / %d", l.Intro.LeftPanelX, l.Intro.RightPanelX)
	}
	if l.Intro.CenterGap != 703 {
		t.Fatalf("center gap = %d, want 703 so radar/score/health stay visible", l.Intro.CenterGap)
	}
	if !l.NativeHUDVisible() {
		t.Fatal("native HUD channel closed")
	}
	if !l.Outro.FullFrame {
		t.Fatal("outro must be full-frame")
	}
	if FadeFromBlackSeconds != 1 || IntroOverlayAfterFadeSeconds != 4 {
		t.Fatalf("fade/delay = %.1f / %.1f", FadeFromBlackSeconds, IntroOverlayAfterFadeSeconds)
	}
	if IntroFreezeSeconds != 15 || OutroSeconds != 8 || BannerHoldSeconds != 4 {
		t.Fatalf("freeze/outro/banner = %d / %d / %d", IntroFreezeSeconds, OutroSeconds, BannerHoldSeconds)
	}
	if IntroFreezeSeconds != parser.IntroFreezeSeconds {
		t.Fatalf("overlay freeze %d != parser %d", IntroFreezeSeconds, parser.IntroFreezeSeconds)
	}
	if BannerHoldSeconds != parser.OutroBannerSeconds {
		t.Fatalf("overlay banner %d != parser %d", BannerHoldSeconds, parser.OutroBannerSeconds)
	}
	if OutroSeconds != parser.OutroScoreboardSeconds {
		t.Fatalf("overlay outro %d != parser %d", OutroSeconds, parser.OutroScoreboardSeconds)
	}
}

func TestOverlayWindowsStartsAfterFadeAndLeavesBeforeLive(t *testing.T) {
	tests := []struct {
		name     string
		duration float64
		introIn  float64
		introEnd float64
		outroIn  float64
	}{
		{name: "long recap", duration: 600, introIn: 5, introEnd: 14, outroIn: 592},
		{name: "shorter than freeze", duration: 6, introIn: 5, introEnd: 6, outroIn: 6},
		{name: "empty", duration: 0, introIn: 0, introEnd: 0, outroIn: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is, ie, os, oe := OverlayWindows(tt.duration)
			if is != tt.introIn || ie != tt.introEnd {
				t.Fatalf("intro = %.1f-%.1f, want %.1f-%.1f", is, ie, tt.introIn, tt.introEnd)
			}
			if os != tt.outroIn || oe != tt.duration {
				t.Fatalf("outro = %.1f-%.1f, want %.1f-%.1f", os, oe, tt.outroIn, tt.duration)
			}
			if ie > 0 && os < ie {
				t.Fatalf("outro overlaps intro: %.1f < %.1f", os, ie)
			}
			if tt.duration >= IntroOverlayEnd() && is != IntroOverlayStart() {
				t.Fatalf("roster must wait until fade+4s, start=%.1f", is)
			}
			if tt.duration >= float64(IntroFreezeSeconds) && ie >= float64(IntroFreezeSeconds) {
				t.Fatalf("roster must leave before live, end=%.1f", ie)
			}
		})
	}
}

func TestIntroFilterOmitsEmptyFACEITColumns(t *testing.T) {
	demoOnly := Build(Roster{
		TargetSteamID64: "1",
		Players: []RosterPlayer{
			{SteamID64: "1", Name: "donk666", Team: "CT", Kills: 2, Deaths: 1},
			{SteamID64: "2", Name: "enemy", Team: "T", Kills: 1, Deaths: 2},
		},
	}, nil)
	got := introFilter(demoOnly, "/fonts/Montserrat-ExtraBold.ttf")
	for _, banned := range []string{"ELO ", "LVL ", "Matches ", "Win% ", "Swing "} {
		if strings.Contains(got, banned) {
			t.Fatalf("demo-only intro invented %q:\n%s", banned, got)
		}
	}
	if !strings.Contains(got, "donk666") || !strings.Contains(got, "2/1/0") || !strings.Contains(got, "K/D/A") {
		t.Fatalf("demo-only intro missing demo facts:\n%s", got)
	}

	elo := 4370
	enriched := Build(Roster{
		TargetSteamID64: "1",
		Players: []RosterPlayer{
			{SteamID64: "1", Name: "donk666", Team: "CT", Kills: 2, Deaths: 1},
		},
	}, map[string]Enrichment{"1": {ELO: elo, SkillLevel: 10}})
	got = introFilter(enriched, "/fonts/Montserrat-ExtraBold.ttf")
	if !strings.Contains(got, "4370") || !strings.Contains(got, "10") {
		t.Fatalf("enriched intro missing FACEIT facts:\n%s", got)
	}
}

func TestStatColumnOffsetsGiveKDAMoreWidthThanKD(t *testing.T) {
	offsets := statColumnOffsets(400, 8)
	if len(offsets) != 8 {
		t.Fatalf("len = %d", len(offsets))
	}
	kdaW := offsets[5] - offsets[4]
	kdW := offsets[6] - offsets[5]
	if kdaW <= kdW {
		t.Fatalf("K/D/A width %d <= K/D width %d (offsets=%v)", kdaW, kdW, offsets)
	}
	for i := 1; i < len(offsets); i++ {
		if offsets[i] <= offsets[i-1] {
			t.Fatalf("column %d did not advance: %v", i, offsets)
		}
	}
}

func TestStillFilterGraphAlwaysUsesLabeledInput(t *testing.T) {
	got := stillFilterGraph("drawtext=text='x'", nil)
	if !strings.HasPrefix(got, "[0:v]") {
		t.Fatalf("graph = %q, want [0:v] prefix", got)
	}
}

func TestIntroChromeKeepsTransparentHUDChannel(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(introChromePNG))
	if err != nil {
		t.Fatalf("decode intro chrome: %v", err)
	}
	if img.Bounds().Dx() != FrameWidth || img.Bounds().Dy() != FrameHeight {
		t.Fatalf("intro chrome = %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
	_, _, _, a := img.At(960, 540).RGBA()
	if a != 0 {
		t.Fatalf("intro chrome center alpha = %d, want 0 so native HUD stays visible", a)
	}
	_, _, _, panelA := img.At(200, 200).RGBA()
	if panelA == 0 {
		t.Fatal("intro chrome left panel is transparent")
	}
	l := DefaultLayout()
	if !l.NativeHUDVisible() {
		t.Fatal("native HUD channel closed")
	}
}

func TestOutroChromeIsFullFrameAndFilterUsesDemoScoreline(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(outroChromePNG))
	if err != nil {
		t.Fatalf("decode outro chrome: %v", err)
	}
	if img.Bounds().Dx() != FrameWidth || img.Bounds().Dy() != FrameHeight {
		t.Fatalf("outro chrome = %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
	doc := Build(Roster{
		TargetSteamID64: "1",
		ScoreCT:         13,
		ScoreT:          8,
		Players: []RosterPlayer{
			{SteamID64: "1", Name: "donk666", Team: "CT", Kills: 23, Deaths: 14, Assists: 4, ADR: 101.6, Rating: 1.35, MVPs: 4},
			{SteamID64: "2", Name: "KingwayO", Team: "T", Kills: 18, Deaths: 16},
		},
	}, nil)
	got := outroFilter(doc, "/fonts/Montserrat-ExtraBold.ttf")
	if !strings.Contains(got, "13") || !strings.Contains(got, "8") {
		t.Fatalf("outro missing scoreline:\n%s", got)
	}
	if !strings.Contains(got, "23/14/4") {
		t.Fatalf("outro missing K/D/A:\n%s", got)
	}
}
