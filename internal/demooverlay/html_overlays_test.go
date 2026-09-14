package demooverlay

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNeonHTMLUsesRealScopesAndEscapedDOM(t *testing.T) {
	total, recent := 7229, 20
	hs := 69.0
	card := PlayerCard{Name: `<script>bad()</script>`, SteamID64: "target", LifetimeMatches: &total, Last20: &Last20{Matches: &recent, HSPct: &hs}, HSPct: 12, HasHSPct: true}
	v := neonCardView(card, true)
	if v.Matches != "7,229" || v.RecentLabel != "Last 20 Matches" || v.HS != "69%" {
		t.Fatalf("scopes mixed: %+v", v)
	}
	card.Last20 = nil
	if got := neonCardView(card, true); got.HS != "—" {
		t.Fatal("demo headshots leaked into recent FACEIT stats")
	}
	doc := Document{Source: SourceFACEIT, TargetSteamID64: "target", Intro: Intro{Left: []PlayerCard{{Name: "Teammate", SteamID64: "other"}, card}}}
	html, err := NeonIntroHTML(doc, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(html), "<canvas") || strings.Contains(string(html), "<script>") || !strings.Contains(string(html), "&lt;script&gt;") {
		t.Fatal("unsafe markup or canvas")
	}
	if strings.Index(string(html), `data-steamid="target"`) > strings.Index(string(html), `data-steamid="other"`) {
		t.Fatal("POV not first")
	}
	if doc.Intro.Left[0].SteamID64 != "other" {
		t.Fatal("preview mutated roster order")
	}
}

func screenshotFixture(t *testing.T, w, h int, c color.NRGBA) *ScreenshotFile {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
	var body bytes.Buffer
	if err := png.Encode(&body, img); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(body.Bytes())
	file := filepath.Join(t.TempDir(), "screenshot.png")
	if err := os.WriteFile(file, body.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return &ScreenshotFile{Path: file, SHA256: hex.EncodeToString(hash[:])}
}

func TestScreenshotHTMLBindsOriginalImageContent(t *testing.T) {
	left := screenshotFixture(t, 436, 513, color.NRGBA{R: 255, A: 255})
	_, err := ScreenshotHTML(Screenshots{Team1: left}, false, false)
	if err == nil {
		t.Fatal("accepted missing second team")
	}
	if err := os.WriteFile(left.Path, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ScreenshotHTML(Screenshots{Scoreboard: left}, true, false); err == nil {
		t.Fatal("accepted changed screenshot bytes")
	}
}

func TestChromiumHTMLOverlaysPreserveDimensionsAlphaAndImageAspect(t *testing.T) {
	if os.Getenv("ZV_OVERLAY_RENDERER_PATH") == "" {
		t.Skip("set ZV_OVERLAY_RENDERER_PATH to exercise bundled Chromium")
	}
	left := screenshotFixture(t, 400, 200, color.NRGBA{R: 255, A: 255})
	right := screenshotFixture(t, 200, 400, color.NRGBA{B: 255, A: 255})
	s := Screenshots{Team1: left, Team2: right, Scoreboard: left}
	for _, outro := range []bool{false, true} {
		html, err := ScreenshotHTML(s, outro, false)
		if err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(t.TempDir(), "overlay.png")
		if err := renderHTMLStill(html, out); err != nil {
			t.Fatal(err)
		}
		f, err := os.Open(out)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dx() != 1920 || img.Bounds().Dy() != 1080 {
			t.Fatalf("wrong dimensions: %v", img.Bounds())
		}
		pixel := func(x, y int) color.NRGBA { return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA) }
		if pixel(1, 1).A != 0 {
			t.Fatal("overlay background is opaque")
		}
		if outro {
			if pixel(960, 540).R != 255 {
				t.Fatal("scoreboard image missing")
			}
		} else {
			if pixel(100, 540).R != 255 || pixel(1600, 540).B != 255 {
				t.Fatal("team images missing or swapped")
			}
			if pixel(100, 200).A != 0 || pixel(960, 540).A != 0 {
				t.Fatal("screenshot stretched or gameplay center covered")
			}
		}
	}
	doc := Document{Theme: ThemeNeonViolet, Intro: Intro{Left: []PlayerCard{{Name: "Actual player", SteamID64: "1"}}}}
	out := filepath.Join(t.TempDir(), "neon.png")
	if err := renderNeonIntroStill(doc, out, false); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, alpha := img.At(960, 540).RGBA()
	if alpha != 0 {
		t.Fatal("generated neon covered gameplay center")
	}
	_, _, _, alpha = img.At(160, 180).RGBA()
	if alpha == 0 {
		t.Fatal("generated player card missing")
	}
}

// This is a renderer-level regression fixture for the two truth models. Local
// cards expose this-match facts only; FACEIT cards can additionally show the
// independently supplied profile values. Set FULL_DEMO_NEON_OUT to retain the
// four PNGs for visual review.
func TestChromiumRendersLocalAndFACEITNeonTruthModels(t *testing.T) {
	if os.Getenv("ZV_OVERLAY_RENDERER_PATH") == "" {
		t.Skip("set ZV_OVERLAY_RENDERER_PATH to exercise bundled Chromium")
	}
	roster := Roster{TargetSteamID64: "76561198386265483", Map: "de_cache", ScoreCT: 13, ScoreT: 7,
		ClanNameCT: "Spirit", ClanNameT: "DENDELE", Players: []RosterPlayer{
			{SteamID64: "76561198386265483", Name: "donk", Team: "CT", Kills: 24, Deaths: 10, Assists: 10, Headshots: 19, Rounds: 20, ADR: 134, HSPct: 79.2, Rating: 2.05, Rounds2K: 2, Rounds5K: 2},
			{SteamID64: "76561199063238565", Name: "magixx", Team: "CT", Kills: 15, Deaths: 8, Assists: 2, Headshots: 7, Rounds: 20, ADR: 66.2, HSPct: 46.7, Rating: 1.26, Rounds2K: 5},
			{SteamID64: "76561198809462276", Name: "koala", Team: "T", Kills: 14, Deaths: 12, Assists: 0, Headshots: 8, Rounds: 20, ADR: 84.1, HSPct: 57.1, Rating: 1.03, Rounds2K: 4},
		}}
	last20 := 20
	faceitDoc := BuildForSource(roster, SourceFACEIT, map[string]Enrichment{
		"76561198386265483": {Nickname: "donk", ELO: 4370, SkillLevel: 10, Last20: &Last20{Matches: &last20}},
	})
	faceitDoc.Theme = ThemeNeonViolet
	localDoc := BuildForSource(roster, "", nil)
	localDoc.Theme = ThemeNeonViolet
	dir := t.TempDir()
	if requested := os.Getenv("FULL_DEMO_NEON_OUT"); requested != "" {
		dir = requested
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name string
		doc  Document
	}{{"local", localDoc}, {"faceit", faceitDoc}} {
		t.Run(tc.name, func(t *testing.T) {
			for _, item := range []struct {
				name  string
				outro bool
			}{{"intro", false}, {"outro", true}} {
				var html []byte
				var err error
				if item.outro {
					html, err = NeonOutroHTML(tc.doc, true)
				} else {
					html, err = NeonIntroHTML(tc.doc, true)
				}
				if err != nil {
					t.Fatal(err)
				}
				out := filepath.Join(dir, tc.name+"-neon-"+item.name+".png")
				if err := renderHTMLStill(html, out); err != nil {
					t.Fatal(err)
				}
				f, err := os.Open(out)
				if err != nil {
					t.Fatal(err)
				}
				img, decodeErr := png.Decode(f)
				_ = f.Close()
				if decodeErr != nil || img.Bounds().Dx() != FrameWidth || img.Bounds().Dy() != FrameHeight {
					t.Fatalf("%s dimensions: %v, %v", item.name, img.Bounds(), decodeErr)
				}
			}
		})
	}
}
