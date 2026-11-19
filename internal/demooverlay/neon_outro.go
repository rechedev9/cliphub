package demooverlay

import (
	"fmt"
	"html/template"
	"strconv"
	"strings"
)

type neonScoreRow struct {
	Name, SteamID, ELO, Level string
	Avatar                    template.URL
	POV                       bool
	Stats                     []string
}
type neonScoreTeam struct {
	Name, Side string
	Score      int
	Winner     bool
	AverageELO string
	Players    []neonScoreRow
}

// scoreboardHTML uses match facts for the scoreboard and labels profile ELO as
// current. The FACEIT Data API does not supply the reference site's swing metric.
func scoreboardHTML(doc Document, previewGrey bool) ([]byte, error) {
	fonts, err := loadPageFonts()
	if err != nil {
		return nil, err
	}
	data := struct {
		pageFonts
		PreviewGrey bool
		Map         string
		FACEIT      bool
		Teams       []neonScoreTeam
		Palette     *Palette
	}{pageFonts: fonts, PreviewGrey: previewGrey, Map: strings.ToUpper(strings.TrimPrefix(doc.Map, "de_")), FACEIT: NormalizeSource(doc.Source) == SourceFACEIT, Palette: doc.Palette}
	avatars := map[string]template.URL{}
	for _, panel := range [][]PlayerCard{doc.Intro.Left, doc.Intro.Right} {
		for _, card := range panel {
			avatars[card.SteamID64] = neonAvatarURL(card.AvatarFile)
		}
	}
	for _, team := range doc.Outro.Teams {
		t := neonScoreTeam{Name: team.Name, Side: team.Side, Score: team.Score, Winner: team.Score == max(doc.ScoreCT, doc.ScoreT), AverageELO: "—"}
		if team.AverageELO != nil {
			t.AverageELO = strconv.Itoa(*team.AverageELO)
		}
		for _, card := range team.Players {
			row := neonScoreRow{Name: card.Name, SteamID: card.SteamID64, POV: doc.IsPOV(card), Avatar: avatars[card.SteamID64], ELO: "—", Level: ""}
			if card.ELO != nil {
				row.ELO = strconv.Itoa(*card.ELO)
			}
			if card.SkillLevel != nil {
				row.Level = strconv.Itoa(*card.SkillLevel)
			}
			kd := "—"
			if card.Deaths > 0 {
				kd = fmt.Sprintf("%.2f", float64(card.Kills)/float64(card.Deaths))
			}
			adr, kr, hs := "—", "—", "—"
			if card.HasADR {
				adr = fmt.Sprintf("%.1f", card.ADR)
			}
			if card.Rounds > 0 {
				kr = fmt.Sprintf("%.2f", float64(card.Kills)/float64(card.Rounds))
			}
			if card.HasHSPct {
				hs = fmt.Sprintf("%.1f%%", card.HSPct)
			}
			row.Stats = []string{strconv.Itoa(card.Kills), strconv.Itoa(card.Deaths), strconv.Itoa(card.Assists), adr, kd, kr, strconv.Itoa(card.Headshots), hs, strconv.Itoa(card.Rounds5K), strconv.Itoa(card.Rounds4K), strconv.Itoa(card.Rounds3K), strconv.Itoa(card.Rounds2K), strconv.Itoa(card.MVPs)}
			t.Players = append(t.Players, row)
		}
		data.Teams = append(data.Teams, t)
	}
	return executePage(designFile(doc, "outro"), data)
}
