package demooverlay

import "testing"

func TestUsesProgrammaticIntroChrome(t *testing.T) {
	if UsesProgrammaticIntroChrome(Document{Source: SourceFACEIT, Theme: ThemeFaceitOrange}) {
		t.Fatal("faceit-orange should keep bundled intro plates")
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
