package demooverlay

import (
	"bytes"
	"image/color"
	"image/png"
	"testing"
)

func TestUsesProgrammaticIntroChrome(t *testing.T) {
	if !UsesProgrammaticIntroChrome(Document{Source: SourceFACEIT, Theme: ThemeFaceitOrange}) {
		t.Fatal("faceit-orange should use live programmatic chrome")
	}
	if !UsesProgrammaticIntroChrome(Document{Source: SourceFACEIT, Theme: ThemeNeonViolet}) {
		t.Fatal("neon-violet should use programmatic intro chrome")
	}
}

func TestResolveTheme(t *testing.T) {
	tests := []struct {
		name string
		doc  Document
		want string
	}{
		{name: "default orange", doc: Document{Source: SourceFACEIT}, want: ThemeFaceitOrange},
		{name: "explicit violet", doc: Document{Source: SourceFACEIT, Theme: ThemeNeonViolet}, want: ThemeNeonViolet},
		{name: "explicit orange", doc: Document{Theme: ThemeFaceitOrange}, want: ThemeFaceitOrange},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveTheme(tc.doc)
			if got.Name != tc.want {
				t.Fatalf("theme = %q, want %q", got.Name, tc.want)
			}
		})
	}
}

func TestRenderIntroChromePNG(t *testing.T) {
	data, err := renderIntroChromePNG(Document{Source: SourceFACEIT, Theme: ThemeNeonViolet})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 8 || string(data[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("expected PNG header, got %d bytes", len(data))
	}
}

// Non-FACEIT docs keep the generic drawtext column on DefaultLayout geometry,
// so the generated chrome must stay a bare panel for them or badges draw twice.
func TestRenderIntroChromePNGSkipsCardsForNonFACEITSources(t *testing.T) {
	roster := Intro{
		Left:  []PlayerCard{{Name: "left", Kills: 20, Deaths: 10}},
		Right: []PlayerCard{{Name: "right", Kills: 12, Deaths: 18}},
	}
	render := func(doc Document, l IntroLayout) color.Color {
		t.Helper()
		data, err := renderIntroChromePNG(doc)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		// Inside the first roster row, away from the panel border and accent.
		return img.At(l.LeftPanelX+l.PanelWidth/2, l.PanelTop+l.HeaderH+60)
	}
	generic := DefaultLayout().Intro
	bare := render(Document{Source: SourcePremier, Theme: ThemeNeonViolet}, generic)
	withRoster := render(Document{Source: SourcePremier, Theme: ThemeNeonViolet, Intro: roster}, generic)
	if bare != withRoster {
		t.Fatalf("premier intro chrome painted a FACEIT card: %v != %v", withRoster, bare)
	}
	faceit := faceitIntroLayout()
	faceitBare := render(Document{Source: SourceFACEIT, Theme: ThemeNeonViolet}, faceit)
	faceitRoster := render(Document{Source: SourceFACEIT, Theme: ThemeNeonViolet, Intro: roster}, faceit)
	if faceitBare == faceitRoster {
		t.Fatal("faceit intro chrome missing card in the first roster row")
	}
}
