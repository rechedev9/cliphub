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
	if l.Intro.LeftPanelX != 45 || l.Intro.RightPanelX != 1311 {
		t.Fatalf("panels x = %d / %d", l.Intro.LeftPanelX, l.Intro.RightPanelX)
	}
	if l.Intro.LeftPanelCropX != 42 || l.Intro.RightPanelCropX != 1308 {
		t.Fatalf("panel crop x = %d / %d", l.Intro.LeftPanelCropX, l.Intro.RightPanelCropX)
	}
	if l.Intro.CenterGap != 703 {
		t.Fatalf("center gap = %d, want 703 so radar/score/health stay visible", l.Intro.CenterGap)
	}
	leftMargin := l.Intro.LeftPanelX
	rightMargin := l.Width - l.Intro.RightPanelX - l.Intro.PanelWidth
	if rightMargin-leftMargin > 1 {
		t.Fatalf("intro margins asymmetric: left=%d right=%d", leftMargin, rightMargin)
	}
	if l.Intro.AvatarSize != 64 || l.Intro.HeaderH != 36 || l.Intro.RowHeight != 196 {
		t.Fatalf("intro card = avatar %d header %d row %d", l.Intro.AvatarSize, l.Intro.HeaderH, l.Intro.RowHeight)
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

func TestIntroFilterHeadersFollowSource(t *testing.T) {
	roster := Roster{
		TargetSteamID64: "1",
		Map:             "de_mirage",
		ClanNameCT:      "Vitality",
		ClanNameT:       "G2",
		Players: []RosterPlayer{
			{SteamID64: "1", Name: "donk666", Team: "CT", Kills: 23, Deaths: 14, Assists: 4, ADR: 101.6, Rating: 1.35, HSPct: 52, Headshots: 12},
			{SteamID64: "2", Name: "enemy", Team: "T", Kills: 10, Deaths: 18},
		},
	}
	leaked := map[string]Enrichment{"1": {Nickname: "faceit-donk", ELO: 4370, SkillLevel: 10}}
	font := "/fonts/Montserrat-ExtraBold.ttf"
	tests := []struct {
		source string
		want   []string
		forbid []string
	}{
		{
			source: SourcePremier,
			want:   []string{"Vitality", "CS2 PREMIER", "Mirage", "donk666", "POV", "G2"},
			forbid: []string{"23/14/4", "ADR", "RATING", "4370", "faceit-donk", "ELO ", "LVL ", "LINEUP", "PLAYERS"},
		},
		{
			source: SourceProfessional,
			want:   []string{"Vitality", "Mirage", "donk666", "POV", "G2"},
			forbid: []string{"23/14/4", "ADR", "RATING", "4370", "faceit-donk", "ELO ", "LVL ", "PREMIER", "PLAYERS", "HS%"},
		},
		{
			source: SourceFACEIT,
			want:   []string{"Vitality", "faceit-donk", "4370", "10", "POV"},
			forbid: []string{"23/14/4", "PREMIER", "LINEUP", "Mirage"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.source, func(t *testing.T) {
			got := introFilter(BuildForSource(roster, tc.source, leaked), font, false)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Fatalf("missing %q:\n%s", want, got)
				}
			}
			for _, banned := range tc.forbid {
				if strings.Contains(got, banned) {
					t.Fatalf("invented %q:\n%s", banned, got)
				}
			}
		})
	}
}

func TestIntroFilterIncludesMonogramWithoutAvatar(t *testing.T) {
	doc := BuildForSource(Roster{
		TargetSteamID64: "1",
		ClanNameCT:      "NAVI",
		Players: []RosterPlayer{
			{SteamID64: "1", Name: "s1mple", Team: "CT", Kills: 20, Deaths: 10},
		},
	}, SourceProfessional, nil)
	got := introFilter(doc, "/fonts/Montserrat-ExtraBold.ttf", false)
	if !strings.Contains(got, "text='S'") {
		t.Fatalf("monogram initial missing:\n%s", got)
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
	got := introFilter(demoOnly, "/fonts/Montserrat-ExtraBold.ttf", false)
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
	got = introFilter(enriched, "/fonts/Montserrat-ExtraBold.ttf", false)
	if !strings.Contains(got, "4370") || !strings.Contains(got, "10") {
		t.Fatalf("enriched intro missing FACEIT facts:\n%s", got)
	}
}

func TestStatColumnOffsetsSpreadFourNebulaColumns(t *testing.T) {
	offsets := statColumnOffsets(400, 4)
	if len(offsets) != 4 {
		t.Fatalf("len = %d", len(offsets))
	}
	adrW := offsets[3] - offsets[2]
	matchW := offsets[1] - offsets[0]
	if adrW <= matchW {
		t.Fatalf("ADR/HS width %d <= Matches width %d (offsets=%v)", adrW, matchW, offsets)
	}
	for i := 1; i < len(offsets); i++ {
		if offsets[i] <= offsets[i-1] {
			t.Fatalf("column %d did not advance: %v", i, offsets)
		}
	}
}

func TestIntroFilterLast20UsesNebulaFourColumns(t *testing.T) {
	matches := 1577
	win := 80.0
	kd := 1.66
	kr := 0.93
	adr := 96.7
	doc := Build(Roster{
		TargetSteamID64: "1",
		Players: []RosterPlayer{
			{SteamID64: "1", Name: "ZywOo", Team: "CT", Kills: 23, Deaths: 10, Headshots: 12, HSPct: 52, ADR: 101.6},
			{SteamID64: "2", Name: "enemy", Team: "T", Kills: 10, Deaths: 18},
		},
	}, map[string]Enrichment{
		"1": {Nickname: "ZywOo", Country: "fr", ELO: 3500, SkillLevel: 10, Last20: &Last20{
			Matches: &matches, WinPct: &win, KD: &kd, KR: &kr, ADR: &adr,
		}},
	})
	got := introFilter(doc, "/fonts/Montserrat-ExtraBold.ttf", false)
	for _, want := range []string{"Counter-Terrorists", "3500 avg ELO", "ZywOo", "3500", "10", "Last 20 matches", "Matches", "Win rate", "ADR", "K/D / K/R", "1,66 / 0,93", "POV"} {
		if !strings.Contains(got, want) {
			t.Fatalf("intro missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Swing") {
		t.Fatalf("Last 20 intro invented swing:\n%s", got)
	}
}

func TestIntroLast20ADRDoesNotFallBackToMatch(t *testing.T) {
	matches := 20
	win := 57.0
	tests := []struct {
		name      string
		last20    *Last20
		matchADR  float64
		wantADR   string
		forbidADR string
	}{
		{
			name:      "last20 adr present",
			last20:    &Last20{Matches: &matches, WinPct: &win, ADR: floatPtr(96.7)},
			matchADR:  101.6,
			wantADR:   "96,70",
			forbidADR: "101,6",
		},
		{
			name:      "last20 without adr",
			last20:    &Last20{Matches: &matches, WinPct: &win},
			matchADR:  101.6,
			wantADR:   "",
			forbidADR: "101,6",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := Build(Roster{
				TargetSteamID64: "1",
				Players: []RosterPlayer{
					{SteamID64: "1", Name: "donk666", Team: "CT", Kills: 23, Deaths: 14, ADR: tt.matchADR},
					{SteamID64: "2", Name: "enemy", Team: "T", Kills: 10, Deaths: 18},
				},
			}, map[string]Enrichment{"1": {Last20: tt.last20}})
			got := introFilter(doc, "/fonts/Montserrat-ExtraBold.ttf", false)
			if !strings.Contains(got, "Last 20 matches") || !strings.Contains(got, "ADR") {
				t.Fatalf("missing Last 20 ADR column:\n%s", got)
			}
			if tt.wantADR != "" && !strings.Contains(got, tt.wantADR) {
				t.Fatalf("missing Last 20 ADR %q:\n%s", tt.wantADR, got)
			}
			if tt.forbidADR != "" && strings.Contains(got, tt.forbidADR) {
				t.Fatalf("match ADR leaked into Last 20 grid:\n%s", got)
			}
			_, value := introADRHS(doc.Intro.Left[0])
			if value != tt.wantADR {
				t.Fatalf("introADRHS = %q, want %q", value, tt.wantADR)
			}
		})
	}
}

func floatPtr(v float64) *float64 { return &v }

func TestOutroGridColumnsKeepsNebulaOrder(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{name: "demo facts", in: []string{ColName, ColKDA, ColRating, ColADR, ColHS, ColHSPct, ColMVP}, want: []string{ColRating, ColKDA, ColADR, ColHSPct}},
		{name: "with faceit", in: []string{ColName, ColELO, ColLevel, ColKDA}, want: []string{ColKDA, ColELO, ColLevel}},
		{name: "empty", in: nil, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := outroGridColumns(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestOutroFilterDrawsScoreboardColumns(t *testing.T) {
	doc := BuildForSource(Roster{
		TargetSteamID64: "1",
		ClanNameCT:      "Vitality",
		ClanNameT:       "G2",
		ScoreCT:         13,
		ScoreT:          8,
		Players: []RosterPlayer{
			{SteamID64: "1", Name: "donk666", Team: "CT", Kills: 23, Deaths: 14, Assists: 4, ADR: 101.6, Rating: 1.35, Headshots: 12, HSPct: 52},
			{SteamID64: "2", Name: "KingwayO", Team: "T", Kills: 18, Deaths: 16},
		},
	}, SourceProfessional, nil)
	outroLayout := DefaultLayout().Outro
	got := outroFilter(doc, "/fonts/Montserrat-ExtraBold.ttf", outroLayout, false)
	for _, want := range []string{"Vitality", "G2", "RATING", "K/D/A", "ADR", "HS%", "23/14/4", "1,35", "POV"} {
		if !strings.Contains(got, want) {
			t.Fatalf("outro missing %q:\n%s", want, got)
		}
	}
}

func TestStillFilterGraphAlwaysUsesLabeledInput(t *testing.T) {
	got := stillFilterGraph(stillFilterGraphOptions{text: "drawtext=text='x'"})
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
	_, _, _, a := img.At(10, 10).RGBA()
	if a == 0xFFFF {
		t.Fatal("outro chrome is fully opaque; gameplay cannot show through")
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
	outroLayout := DefaultLayout().Outro
	got := outroFilter(doc, "/fonts/Montserrat-ExtraBold.ttf", outroLayout, false)
	if !strings.Contains(got, "13") || !strings.Contains(got, "8") {
		t.Fatalf("outro missing scoreline:\n%s", got)
	}
	if !strings.Contains(got, "23/14/4") {
		t.Fatalf("outro missing K/D/A:\n%s", got)
	}
}
