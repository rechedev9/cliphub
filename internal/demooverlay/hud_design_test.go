package demooverlay

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/hudthemes"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

func designDocs() []Document {
	last20 := 20
	roster := Roster{TargetSteamID64: "pov", Map: "de_mirage", ScoreCT: 13, ScoreT: 9, Players: []RosterPlayer{
		{SteamID64: "mate", Name: "Teammate", Team: "T", Kills: 30, Deaths: 10, Rounds: 22, ADR: 101},
		{SteamID64: "pov", Name: "Chosen", Team: "T", Kills: 12, Deaths: 15, Rounds: 22, ADR: 70},
		{SteamID64: "rival", Name: "Rival", Team: "CT", Kills: 18, Deaths: 12, Rounds: 22, ADR: 88},
	}}
	faceit := map[string]Enrichment{"pov": {Nickname: "Chosen", ELO: 2100, SkillLevel: 9, Last20: &Last20{Matches: &last20}}}
	return []Document{BuildForSource(roster, "", nil), BuildForSource(roster, SourceFACEIT, faceit)}
}

var (
	pageStyle = regexp.MustCompile(`(?s)<style>.*?</style>|<title>.*?</title>`)
	pageTag   = regexp.MustCompile(`<[^>]*>`)
)

func visibleText(page []byte) string {
	return strings.Join(strings.Fields(pageTag.ReplaceAllString(pageStyle.ReplaceAllString(string(page), " "), " ")), " ")
}

func designPage(t *testing.T, doc Document, overlays recapplan.OverlayOptions, outro bool) []byte {
	t.Helper()
	doc.Theme, doc.Palette = NormalizeTheme(overlays.Theme), DesignPalette(overlays)
	page, err := DesignHTML(doc, DesignView{Outro: outro})
	if err != nil {
		t.Fatal(err)
	}
	return page
}

func TestHUDDesignShowsTheNeonContentInTheHUDColours(t *testing.T) {
	radius := 4
	neon := recapplan.OverlayOptions{Theme: recapplan.OverlayThemeNeon, HUDTheme: "prism"}
	for _, tc := range []struct {
		name     string
		overlays recapplan.OverlayOptions
		want     []string
	}{
		{"catalog design", recapplan.OverlayOptions{Theme: recapplan.OverlayThemeHUD, HUDTheme: "prism"}, []string{"#AC91FF", "#68EDBE", "#171429"}},
		{"own theme", recapplan.OverlayOptions{Theme: recapplan.OverlayThemeHUD, HUDTheme: "prism", HUDCustom: &hudthemes.Style{Name: "Mío", CT: "112233", T: "445566", Background: "0A0B0C", Radius: &radius}}, []string{"#112233", "#445566", "#0A0B0C", "4px"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, doc := range designDocs() {
				for _, outro := range []bool{false, true} {
					hud := designPage(t, doc, tc.overlays, outro)
					if got, want := visibleText(hud), visibleText(designPage(t, doc, neon, outro)); got != want {
						t.Fatalf("outro=%v source=%q: the HUD design changed the content\n got %s\nwant %s", outro, doc.Source, got, want)
					}
					for _, colour := range tc.want {
						if !strings.Contains(string(hud), colour) {
							t.Errorf("outro=%v: %s is not drawn", outro, colour)
						}
					}
					if strings.Contains(strings.ToLower(string(hud)), "#e69bff") || strings.Contains(strings.ToLower(string(hud)), "#d99aff") {
						t.Errorf("outro=%v: the neon frame leaked into the HUD design", outro)
					}
					if strings.Index(string(hud), `data-steamid="pov"`) > strings.Index(string(hud), `data-steamid="mate"`) && !outro {
						t.Error("the chosen player is not first in the intro")
					}
					if !strings.Contains(string(hud), `pov" data-steamid="pov"`) {
						t.Errorf("outro=%v: the chosen player is not marked", outro)
					}
				}
			}
		})
	}
}

// Glacier is a light design with dark text; an own theme on it with a dark background must stay legible.
func TestHUDDesignKeepsTextLegibleOnAnOwnBackground(t *testing.T) {
	dark := DesignPalette(recapplan.OverlayOptions{Theme: recapplan.OverlayThemeHUD, HUDTheme: "glacier", HUDCustom: &hudthemes.Style{Name: "Noche", CT: "2EC4B6", T: "FF6B6B", Background: "1B1F3B"}})
	light := DesignPalette(recapplan.OverlayOptions{Theme: recapplan.OverlayThemeHUD, HUDTheme: "arena", HUDCustom: &hudthemes.Style{Name: "Día", CT: "1165AB", T: "B63453", Background: "F2F4F7"}})
	for _, tc := range []struct {
		name    string
		palette *Palette
		light   bool
	}{{"dark background", dark, true}, {"light background", light, false}} {
		for _, colour := range []string{tc.palette.Text, tc.palette.Muted} {
			value, _ := strconv.ParseUint(colour, 16, 32)
			r, g, b := value>>16, value>>8&0xFF, value&0xFF
			if tc.light != (min(r, g, b) >= 0x90) || !tc.light && max(r, g, b) > 0x70 {
				t.Errorf("%s: text colour %s is not legible on %s", tc.name, colour, tc.palette.Background)
			}
		}
	}
	if catalog := DesignPalette(recapplan.OverlayOptions{Theme: recapplan.OverlayThemeHUD, HUDTheme: "glacier"}); catalog.Text != "12283D" {
		t.Errorf("a readable design lost its own text colour: %s", catalog.Text)
	}
}

func TestHUDDesignOverTheNativeHUDIsNeutral(t *testing.T) {
	if DesignPalette(recapplan.OverlayOptions{Theme: recapplan.OverlayThemeNeon, HUDTheme: "prism"}) != nil {
		t.Fatal("the neon design took a HUD palette")
	}
	native := recapplan.OverlayOptions{Theme: recapplan.OverlayThemeHUD}
	p := DesignPalette(native)
	if p == nil {
		t.Fatal("no palette over the native HUD")
	}
	for _, colour := range []string{p.Background, p.Text, p.Muted} {
		r, _ := strconv.ParseUint(colour[0:2], 16, 8)
		g, _ := strconv.ParseUint(colour[2:4], 16, 8)
		b, _ := strconv.ParseUint(colour[4:6], 16, 8)
		if max(r, g, b)-min(r, g, b) > 16 {
			t.Errorf("%s is not a neutral grey", colour)
		}
	}
	if page := designPage(t, designDocs()[0], native, false); !strings.Contains(string(page), "#"+p.Background) {
		t.Fatal("the neutral background is not drawn")
	}
}
