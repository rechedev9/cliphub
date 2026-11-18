package demooverlay

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/rechedev9/cliphub/internal/customhud"
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

// The embedded fonts would make each thumbnail about 2 MB, so previews draw with the fallback font.
var pageFontFaces = regexp.MustCompile(`@font-face\{[^}]*\}`)

// PreviewSVG wraps a design page as a scalable SVG image, so an <img> shows the
// renderer's own page. The page body must be well-formed XHTML.
func PreviewSVG(page []byte) ([]byte, error) {
	html := pageFontFaces.ReplaceAllString(string(page), "")
	styleStart, styleEnd := strings.Index(html, "<style>"), strings.Index(html, "</style>")
	bodyStart, bodyEnd := strings.Index(html, "<body"), strings.LastIndex(html, "</body>")
	if styleStart < 0 || styleEnd < styleStart || bodyStart < styleEnd || bodyEnd < bodyStart {
		return nil, fmt.Errorf("overlay page has no style and body to preview")
	}
	var out strings.Builder
	fmt.Fprintf(&out, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d"><foreignObject width="%d" height="%d"><html xmlns="http://www.w3.org/1999/xhtml"><head>`,
		FrameWidth, FrameHeight, FrameWidth, FrameHeight, FrameWidth, FrameHeight)
	out.WriteString(html[styleStart : styleEnd+len("</style>")])
	out.WriteString("</head>")
	out.WriteString(html[bodyStart : bodyEnd+len("</body>")])
	out.WriteString("</html></foreignObject></svg>")
	return []byte(out.String()), nil
}
