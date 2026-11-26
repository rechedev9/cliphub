package demooverlay

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/htmlstill"
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
	SubscribeSlidePixels = 900
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

type SubscribeCard struct {
	LogoPath    string
	ChannelName string
}

func SubscribeHTML(palette *Palette, card SubscribeCard, size htmlstill.Size) ([]byte, error) {
	fonts, err := loadPageFonts()
	if err != nil {
		return nil, err
	}
	logo, err := subscribeLogoURL(card.LogoPath)
	if err != nil {
		return nil, err
	}
	layout := "landscape"
	if size.Height > size.Width {
		layout = "portrait"
	}
	return storyPage("subscribe.html", struct {
		pageFonts
		Palette       *Palette
		Label         string
		Layout        string
		Width, Height int
		Branded       bool
		Logo          template.URL
		ChannelName   string
	}{fonts, palette, SubscribeLabel, layout, size.Width, size.Height, logo != "" || card.ChannelName != "", logo, card.ChannelName})
}

func subscribeLogoURL(path string) (template.URL, error) {
	if path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("subscribe logo: %w", err)
	}
	kind := http.DetectContentType(data)
	if kind != "image/png" && kind != "image/jpeg" {
		return "", fmt.Errorf("subscribe logo must be a PNG or JPG, got %s", kind)
	}
	return template.URL("data:" + kind + ";base64," + base64.StdEncoding.EncodeToString(data)), nil
}

func storyPage(file string, data any) ([]byte, error) {
	return htmlPage(storyAssets, file, data)
}

func htmlPage(assets embed.FS, file string, data any) ([]byte, error) {
	page, err := template.ParseFS(assets, file)
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

func RenderSubscribePNG(palette *Palette, card SubscribeCard, size htmlstill.Size, outPath string) error {
	html, err := SubscribeHTML(palette, card, size)
	if err != nil {
		return err
	}
	return htmlstill.Render(html, outPath, size)
}
