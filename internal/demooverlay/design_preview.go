package demooverlay

import (
	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/htmlstill"
)

// PreviewDocument is the example match of the design previews: the players of
// the HUD preview with illustrative facts from a local demo.
func PreviewDocument() Document {
	roster := Roster{TargetSteamID64: customhud.ExampleTarget, Map: "de_mirage", ScoreCT: 13, ScoreT: 9, Rounds: 22}
	for i, p := range customhud.Example().Players {
		kills := 9 + (i*7)%17
		roster.Players = append(roster.Players, RosterPlayer{
			SteamID64: p.SteamID, Name: p.Name, Team: p.Side, Kills: kills, Deaths: 10 + i%7, Assists: 2 + i%5, Headshots: kills / 2,
			MVPs: i % 4, Rounds: roster.Rounds, ADR: 58 + float64((i*13)%47), HSPct: 34 + float64((i*11)%31), Rounds2K: i % 3, Rounds3K: i % 2,
		})
	}
	return BuildForSource(roster, "", nil)
}

// PreviewSVG wraps a design page as a scalable SVG image, so an <img> shows the
// renderer's own page. The page body must be well-formed XHTML.
func PreviewSVG(page []byte) ([]byte, error) {
	return htmlstill.PreviewSVG(page, htmlstill.Landscape)
}
