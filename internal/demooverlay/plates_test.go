package demooverlay

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/mediafont"
)

func TestResolvePlatePath(t *testing.T) {
	dir := t.TempDir()
	writeTestPlate(t, dir, "premier-intro.jpg", color.RGBA{R: 255, G: 0, B: 0, A: 255})

	tests := []struct {
		name      string
		source    string
		assetsDir string
		kind      string
		wantName  string
	}{
		{name: "premier intro", source: SourcePremier, assetsDir: dir, kind: "intro", wantName: "premier-intro.jpg"},
		{name: "missing file", source: SourcePremier, assetsDir: dir, kind: "outro", wantName: ""},
		{name: "empty dir", source: SourcePremier, assetsDir: "", kind: "intro", wantName: ""},
		{name: "unknown source", source: "casual", assetsDir: dir, kind: "intro", wantName: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			switch tc.kind {
			case "intro":
				got = IntroPlatePath(tc.source, tc.assetsDir)
			case "outro":
				got = OutroPlatePath(tc.source, tc.assetsDir)
			default:
				t.Fatalf("unknown kind %q", tc.kind)
			}
			if tc.wantName == "" {
				if got != "" {
					t.Fatalf("got %q, want empty", got)
				}
				return
			}
			if filepath.Base(got) != tc.wantName {
				t.Fatalf("got %q, want %q", got, tc.wantName)
			}
		})
	}
}

func TestOutroLayoutForSourceKeepsDefaultGrid(t *testing.T) {
	base := DefaultLayout().Outro
	for _, source := range []string{"", SourcePremier, SourceProfessional, SourceFACEIT} {
		got := OutroLayoutForSource(source)
		if got != base {
			t.Fatalf("source %q layout = %+v, want default %+v", source, got, base)
		}
	}
}

func TestIntroTextInsetIndentsFACEITPlateRows(t *testing.T) {
	layout := DefaultLayout().Intro
	tests := []struct {
		name     string
		source   string
		hasPlate bool
		wantMin  int
	}{
		{name: "faceit plate", source: SourceFACEIT, hasPlate: true, wantMin: layout.AvatarXOff + layout.AvatarSize + 8},
		{name: "faceit chrome", source: SourceFACEIT, hasPlate: false, wantMin: layout.CardInset},
		{name: "premier plate", source: SourcePremier, hasPlate: true, wantMin: layout.CardInset},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IntroTextInset(layout, tc.source, tc.hasPlate)
			if got < tc.wantMin {
				t.Fatalf("inset = %d, want >= %d", got, tc.wantMin)
			}
		})
	}
}

func TestIntroPlateFilterClausesUsesPanelGeometry(t *testing.T) {
	l := DefaultLayout()
	clauses, next := IntroPlateFilterClauses("[0:v]", 1, l)
	if next != "pl_done" {
		t.Fatalf("next = %q", next)
	}
	joined := strings.Join(clauses, ";")
	for _, want := range []string{
		"[1:v]",
		"scale=1920:1080:force_original_aspect_ratio=increase",
		"crop=1920:1080",
		"split=2[pcovL][pcovR]",
		"[pcovL]crop=563:1020:42:28[plL]",
		"[pcovR]crop=563:1020:1308:28[plR]",
		"[0:v][plL]overlay=x=42:y=28",
		"[pl_left][plR]overlay=x=1308:y=28",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in:\n%s", want, joined)
		}
	}
}

func TestLockedIntroPlateGeometryTables(t *testing.T) {
	tests := []struct {
		source string
		want   IntroPlateGeometry
	}{
		{
			source: SourceProfessional,
			want: IntroPlateGeometry{
				TeamNameY: 126, TeamNameXOff: 130, TeamNameSize: 30, SubtitleY: 160,
				RowNameCenterY: [5]int{293, 454, 613, 772, 932},
			},
		},
		{
			source: SourcePremier,
			want: IntroPlateGeometry{
				TeamNameY: 168, TeamNameXOff: 140, TeamNameSize: 28, SubtitleY: 198,
				RowNameCenterY: [5]int{293, 452, 603, 753, 908},
			},
		},
		{
			source: SourceFACEIT,
			want: IntroPlateGeometry{
				TeamNameY: 78, TeamNameXOff: 120, TeamNameSize: 28, SubtitleY: 104,
				RowNameCenterY: [5]int{276, 437, 598, 764, 929},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.source, func(t *testing.T) {
			got, ok := IntroPlateGeo(tc.source, true)
			if !ok {
				t.Fatal("expected geometry")
			}
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestIntroPlatePOVBadgeInset(t *testing.T) {
	l := DefaultLayout()
	tagX := introPlatePOVBadgeX(l.Intro.LeftPanelX, l.Intro.PanelWidth)
	rightEdge := tagX + introPlatePOVBadgeW
	panelRight := l.Intro.LeftPanelX + l.Intro.PanelWidth
	const wantPad = 12
	innerBorder := 20
	if rightEdge > panelRight-innerBorder-wantPad {
		t.Fatalf("POV badge right=%d exceeds panel inner border pad (panelRight=%d)", rightEdge, panelRight)
	}
}

func TestIntroPlateTeamHeaderInFilter(t *testing.T) {
	roster := Roster{
		TargetSteamID64: "1",
		Map:             "de_mirage", ScoreCT: 13, ScoreT: 8,
		ClanNameCT: "Vitality", ClanNameT: "G2",
		Players: []RosterPlayer{
			{SteamID64: "1", Name: "donk666", Team: "CT", Kills: 23},
		},
	}
	font := "/fonts/Montserrat-ExtraBold.ttf"
	tests := []struct {
		source string
		wantY  int
		wantPOV bool
	}{
		{source: SourceProfessional, wantY: 126, wantPOV: true},
		{source: SourcePremier, wantY: 168, wantPOV: true},
		{source: SourceFACEIT, wantY: 78, wantPOV: true},
	}
	for _, tc := range tests {
		t.Run(tc.source, func(t *testing.T) {
			doc := BuildForSource(roster, tc.source, nil)
			got := introFilter(doc, font, true)
			if !strings.Contains(got, fmt.Sprintf("y=%d", tc.wantY)) {
				t.Fatalf("missing team header y=%d in filter", tc.wantY)
			}
			if tc.wantPOV && !strings.Contains(got, "POV") {
				t.Fatal("missing POV badge")
			}
			tagX := introPlatePOVBadgeX(DefaultLayout().Intro.LeftPanelX, DefaultLayout().Intro.PanelWidth)
			if tc.wantPOV && !strings.Contains(got, fmt.Sprintf("x=%d", tagX)) {
				t.Fatalf("missing POV x=%d in filter", tagX)
			}
		})
	}
}

func TestFaceitIntroRow0CircleCenter(t *testing.T) {
	geo, ok := IntroPlateGeo(SourceFACEIT, true)
	if !ok {
		t.Fatal("expected geometry")
	}
	row0 := geo.RowNameCenterY[0]
	if row0 < 268 || row0 > 284 {
		t.Fatalf("FACEIT row0 center=%d, want ~276 from avatar ring measure", row0)
	}
	pitch := geo.RowNameCenterY[1] - geo.RowNameCenterY[0]
	if pitch < 155 || pitch > 168 {
		t.Fatalf("FACEIT row pitch=%d, want ~161", pitch)
	}
}

func TestLockedOutroPlateGeometryTables(t *testing.T) {
	proOutro := OutroPlateGeometry{
		HeaderY:   118,
		ColLabelY: 248,
		RowNameCenterY: [outroPlateMaxRows]int{
			329, 410, 490, 571, 653, 732, 811, 893, 972, 1052,
		},
	}
	tests := []struct {
		source string
		want   OutroPlateGeometry
	}{
		{source: SourceProfessional, want: proOutro},
		{
			source: SourcePremier,
			want: OutroPlateGeometry{
				HeaderY:   118,
				ColLabelY: 248,
				RowNameCenterY: [outroPlateMaxRows]int{
					329, 410, 490, 571, 651, 732, 811, 893, 972, 1052,
				},
			},
		},
		{
			source: SourceFACEIT,
			want: OutroPlateGeometry{
				HeaderY:   128,
				ColLabelY: 278,
				RowNameCenterY: [outroPlateMaxRows]int{
					316, 397, 477, 558, 637, 721, 804, 887, 976, 1054,
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.source, func(t *testing.T) {
			got, ok := OutroPlateGeo(tc.source, true)
			if !ok {
				t.Fatal("expected geometry")
			}
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestOutroPlateRowYPacking(t *testing.T) {
	geo, ok := OutroPlateGeo(SourceProfessional, true)
	if !ok {
		t.Fatal("expected geometry")
	}
	// Row 0 band is frame y=297..363; name+stat must stay inside.
	nameY := geo.rowNameY(0)
	statY := geo.rowStatY(0)
	if nameY < 297 || nameY > 340 {
		t.Fatalf("row 0 nameY=%d outside upper band", nameY)
	}
	if statY < 320 || statY+16 > 363 {
		t.Fatalf("row 0 statY=%d extends past band bottom 363", statY)
	}
	if geo.rowCount() != 10 {
		t.Fatalf("rowCount=%d, want 10", geo.rowCount())
	}
}

func TestOutroPlateBackdropClausesUsesHighOpacity(t *testing.T) {
	clauses, next := OutroPlateBackdropClauses("[0:v]", 1)
	if next != "plated" {
		t.Fatalf("next = %q", next)
	}
	joined := strings.Join(clauses, ";")
	for _, want := range []string{
		"scale=1920:1080:force_original_aspect_ratio=increase",
		"colorchannelmixer=aa=0.95",
		"overlay=x=0:y=0",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in:\n%s", want, joined)
		}
	}
}

func TestStillFilterGraphFallsBackWithoutPlate(t *testing.T) {
	got := stillFilterGraph(stillFilterGraphOptions{text: "drawtext=text='x'"})
	if !strings.HasPrefix(got, "[0:v]") {
		t.Fatalf("graph = %q, want [0:v] prefix", got)
	}
	if strings.Contains(got, "plL") {
		t.Fatalf("unexpected plate overlay in fallback graph: %s", got)
	}
}

func TestStillFilterGraphCompositesIntroPlate(t *testing.T) {
	got := stillFilterGraph(stillFilterGraphOptions{
		text:       "drawtext=text='x'",
		plateInput: 1,
		introPlate: true,
		layout:     DefaultLayout(),
	})
	for _, want := range []string{"[1:v]", "[plL]", "overlay=x=42:y=28", "drawtext=text='x'"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestIntroFilterSkipsFACEITMonogramWhenPlatePresent(t *testing.T) {
	doc := BuildForSource(Roster{
		TargetSteamID64: "1",
		Players: []RosterPlayer{
			{SteamID64: "1", Name: "ZywOo", Team: "CT", Kills: 20, Deaths: 10},
		},
	}, SourceFACEIT, map[string]Enrichment{"1": {Nickname: "ZywOo", ELO: 3500, SkillLevel: 10}})
	withPlate := introFilter(doc, "/fonts/Montserrat-ExtraBold.ttf", true)
	withoutPlate := introFilter(doc, "/fonts/Montserrat-ExtraBold.ttf", false)
	if strings.Contains(withPlate, "text='Z'") {
		t.Fatalf("FACEIT plate intro should not draw monogram:\n%s", withPlate)
	}
	if !strings.Contains(withoutPlate, "text='Z'") {
		t.Fatalf("FACEIT chrome intro should keep monogram:\n%s", withoutPlate)
	}
}

func TestRenderPlatePreviewPNGs(t *testing.T) {
	if os.Getenv("PLATES_PREVIEW") == "" {
		t.Skip("set PLATES_PREVIEW=1 to write real-plate preview PNGs")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	font, err := mediafont.Materialize()
	if err != nil {
		t.Fatalf("materialize font: %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	plates := filepath.Join(repoRoot, "data", "overlay-assets", "plates")
	if _, err := os.Stat(plates); err != nil {
		t.Skipf("real plates missing at %s", plates)
	}
	outDir := filepath.Join(repoRoot, ".qa-frames", "plates-preview")
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		t.Fatal(err)
	}
	roster := Roster{
		TargetSteamID64: "1",
		Map:             "de_mirage",
		ScoreCT:         13,
		ScoreT:          8,
		ClanNameCT:      "Vitality",
		ClanNameT:       "G2",
		Players: []RosterPlayer{
			{SteamID64: "1", Name: "donk666", Team: "CT", Kills: 23, Deaths: 14, Assists: 4, ADR: 101.6, Rating: 1.35, HSPct: 52, Headshots: 12},
			{SteamID64: "2", Name: "KingwayO", Team: "T", Kills: 18, Deaths: 16, Assists: 3, Rating: 1.05},
			{SteamID64: "3", Name: "mezii", Team: "CT", Kills: 15, Deaths: 12, Assists: 5},
			{SteamID64: "4", Name: "apEX", Team: "CT", Kills: 12, Deaths: 15, Assists: 7},
			{SteamID64: "5", Name: "ZywOo", Team: "CT", Kills: 20, Deaths: 10, Assists: 2},
			{SteamID64: "6", Name: "huNter", Team: "T", Kills: 14, Deaths: 17, Assists: 4},
		},
	}
	for _, source := range []string{SourceProfessional, SourcePremier, SourceFACEIT} {
		doc := BuildForSource(roster, source, nil)
		intro := filepath.Join(outDir, source+"-intro.png")
		outro := filepath.Join(outDir, source+"-outro.png")
		if err := RenderPNGs(ffmpeg, font, doc, intro, outro, RenderOptions{OverlayAssetsDir: plates, PreviewGreyBase: true}); err != nil {
			t.Fatalf("%s: %v", source, err)
		}
	}
}

func writeTestPlate(t *testing.T, dir, name string, c color.Color) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 36))
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			img.Set(x, y, c)
		}
	}
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create plate: %v", err)
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		_ = f.Close()
		t.Fatalf("encode plate: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close plate: %v", err)
	}
}
