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
