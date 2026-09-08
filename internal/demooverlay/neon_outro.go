package demooverlay

import (
	"bytes"
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
	Name       string
	Score      int
	Winner     bool
	AverageELO string
	Players    []neonScoreRow
}

// NeonOutroHTML uses match facts for the scoreboard and labels profile ELO as
// current. The FACEIT Data API does not supply the reference site's swing metric.
func NeonOutroHTML(doc Document, previewGrey bool) ([]byte, error) {
	data := struct {
		RegularFont, MediumFont, BoldFont template.URL
		PreviewGrey                       bool
		Map                               string
		FACEIT                            bool
		Teams                             []neonScoreTeam
	}{PreviewGrey: previewGrey, Map: strings.ToUpper(strings.TrimPrefix(doc.Map, "de_")), FACEIT: NormalizeSource(doc.Source) == SourceFACEIT}
	var err error
	data.RegularFont, err = neonAssetURL("assets/fonts/Roboto-Regular.ttf", "font/ttf")
	if err != nil {
		return nil, err
	}
	data.MediumFont, err = neonAssetURL("assets/fonts/Roboto-Medium.ttf", "font/ttf")
	if err != nil {
		return nil, err
	}
	data.BoldFont, err = neonAssetURL("assets/fonts/Roboto-Bold.ttf", "font/ttf")
	if err != nil {
		return nil, err
	}
	avatars := map[string]template.URL{}
	for _, panel := range [][]PlayerCard{doc.Intro.Left, doc.Intro.Right} {
		for _, card := range panel {
			avatars[card.SteamID64] = neonAvatarURL(card.AvatarFile)
		}
	}
	for _, team := range doc.Outro.Teams {
		t := neonScoreTeam{Name: team.Name, Score: team.Score, Winner: team.Score == max(doc.ScoreCT, doc.ScoreT), AverageELO: "—"}
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
	raw, err := neonAssets.ReadFile("neon-outro.html")
	if err != nil {
		return nil, err
	}
	t, err := template.New("neon-outro").Parse(string(raw))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := t.Execute(&out, data); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
