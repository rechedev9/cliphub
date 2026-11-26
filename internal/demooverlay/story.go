package demooverlay

import (
	"bytes"
	"embed"
	"html/template"
	"strings"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/killplan"
)

const (
	HalftimeSeconds     = 4.0
	HalftimeCardTop     = 224
	HalftimeCardHeight  = 124
	HalftimeSlidePixels = 48
	HalftimeLabel       = "DESCANSO"

	SubscribeSeconds     = 6.0
	SubscribeBottomEdge  = 1036
	SubscribeSlidePixels = 480
	SubscribeLabel       = "SUSCRÍBETE"
)

//go:embed halftime.html subscribe.html
var storyAssets embed.FS

type Halftime struct {
	Round int             `json:"round"`
	Teams [2]HalftimeTeam `json:"teams"`
}

type HalftimeTeam struct {
	Name  string `json:"name"`
	Side  string `json:"side"`
	Score int    `json:"score"`
}

func BuildHalftime(doc Document, rounds []killplan.RoundResult, names customhud.Teams) *Halftime {
	first := ""
	for i, round := range rounds {
		if round.Number != i+1 || (round.Winner != "CT" && round.Winner != "T") {
			return nil
		}
		side := round.TargetSide
		if side != "CT" && side != "T" {
			continue
		}
		if first == "" {
			first = side
		}
		if side == first {
			continue
		}
		if rounds[i-1].TargetSide != first || !regulationSwitch(rounds[:i], first) {
			return nil
		}
		won := 0
		for _, played := range rounds[:i] {
			if played.Winner == first {
				won++
			}
		}
		second := oppositeSide(first)
		clanPOV, clanRival := halftimeClanNames(doc)
		return &Halftime{Round: round.Number, Teams: [2]HalftimeTeam{
			{Name: firstNonEmpty(halftimeStartName(names, first), clanPOV, second), Side: second, Score: won},
			{Name: firstNonEmpty(halftimeStartName(names, second), clanRival, first), Side: first, Score: i - won},
		}}
	}
	return nil
}

func regulationSwitch(firstHalf []killplan.RoundResult, side string) bool {
	for _, round := range firstHalf {
		if round.TargetSide != side {
			return false
		}
	}
	return true
}

func halftimeStartName(names customhud.Teams, startSide string) string {
	if startSide == "CT" {
		return strings.TrimSpace(names.CTStart)
	}
	return strings.TrimSpace(names.TStart)
}

func halftimeClanNames(doc Document) (pov, rival string) {
	found := false
	for _, team := range doc.Outro.Teams {
		if team.Side != "CT" && team.Side != "T" {
			continue
		}
		name := strings.TrimSpace(team.Name)
		if name == sideDisplayName(team.Side) {
			name = ""
		}
		mine := false
		for _, card := range team.Players {
			mine = mine || doc.IsPOV(card)
		}
		if mine {
			pov, found = name, true
		} else {
			rival = name
		}
	}
	if !found {
		return "", ""
	}
	return pov, rival
}

type halftimeTeamView struct {
	Name, Side string
	Score      int
	Lead       bool
}

func HalftimeHTML(halftime Halftime, palette *Palette) ([]byte, error) {
	fonts, err := loadPageFonts()
	if err != nil {
		return nil, err
	}
	data := struct {
		pageFonts
		Palette *Palette
		Label   string
		Teams   []halftimeTeamView
	}{pageFonts: fonts, Palette: palette, Label: HalftimeLabel}
	for i, team := range halftime.Teams {
		data.Teams = append(data.Teams, halftimeTeamView{Name: team.Name, Side: team.Side, Score: team.Score, Lead: team.Score > halftime.Teams[1-i].Score})
	}
	return storyPage("halftime.html", data)
}

func SubscribeHTML(palette *Palette) ([]byte, error) {
	fonts, err := loadPageFonts()
	if err != nil {
		return nil, err
	}
	return storyPage("subscribe.html", struct {
		pageFonts
		Palette *Palette
		Label   string
	}{fonts, palette, SubscribeLabel})
}

func storyPage(file string, data any) ([]byte, error) {
	page, err := template.ParseFS(storyAssets, file)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := page.Execute(&out, data); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func RenderHalftimePNG(halftime Halftime, palette *Palette, outPath string) error {
	html, err := HalftimeHTML(halftime, palette)
	if err != nil {
		return err
	}
	return renderHTMLStill(html, outPath)
}

func RenderSubscribePNG(palette *Palette, outPath string) error {
	html, err := SubscribeHTML(palette)
	if err != nil {
		return err
	}
	return renderHTMLStill(html, outPath)
}
