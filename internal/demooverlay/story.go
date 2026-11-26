package demooverlay

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"image"
	"image/draw"
	"image/png"
	"math"
	"net/http"
	"os"
	"path/filepath"
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
	SubscribeBottomEdge  = 910
	SubscribeSlidePixels = 900
	SubscribeLabel       = "SUBSCRIBE"
	SubscribeBellFPS     = 20
	subscribeBellFrames  = 18
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

type SubscribeSprite struct {
	CellTop    int
	CellHeight int
	Frames     int
}

func SubscribeSpriteFor(size htmlstill.Size) SubscribeSprite {
	if size.Height > size.Width {
		return SubscribeSprite{CellTop: 1253, CellHeight: 230, Frames: subscribeBellFrames}
	}
	return SubscribeSprite{CellTop: SubscribeBottomEdge - 48 - 75, CellHeight: 150, Frames: subscribeBellFrames}
}

func (s SubscribeSprite) Height() int {
	return s.CellHeight * (s.Frames + 1)
}

func subscribeBellSwings(frames int) []template.CSS {
	swings := []template.CSS{"transform:rotate(0deg)"}
	for k := range frames {
		angle := 20 * math.Exp(-float64(k)/8) * math.Sin(math.Pi*float64(k+1)/3)
		swings = append(swings, template.CSS(fmt.Sprintf("transform:rotate(%.1fdeg)", angle)))
	}
	return swings
}

func (s SubscribeSprite) rowsPerPage(size htmlstill.Size) int {
	return size.Height / s.CellHeight
}

func SubscribeHTML(palette *Palette, card SubscribeCard, size htmlstill.Size) ([][]byte, error) {
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
	sprite := SubscribeSpriteFor(size)
	swings, rows := subscribeBellSwings(sprite.Frames), sprite.rowsPerPage(size)
	var pages [][]byte
	for first := 0; first < len(swings); first += rows {
		page, err := storyPage("subscribe.html", struct {
			pageFonts
			Palette                   *Palette
			Label                     string
			Layout                    string
			Width, Height, CellHeight int
			Swings                    []template.CSS
			Branded                   bool
			Logo                      template.URL
			ChannelName               string
		}{fonts, palette, SubscribeLabel, layout, size.Width, size.Height, sprite.CellHeight, swings[first:min(first+rows, len(swings))], logo != "" || card.ChannelName != "", logo, card.ChannelName})
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	return pages, nil
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

func RenderSubscribeSprite(palette *Palette, card SubscribeCard, size htmlstill.Size, outPath string) (SubscribeSprite, error) {
	pages, err := SubscribeHTML(palette, card, size)
	if err != nil {
		return SubscribeSprite{}, err
	}
	sprite := SubscribeSpriteFor(size)
	if err := os.MkdirAll(filepath.Dir(outPath), 0750); err != nil {
		return SubscribeSprite{}, err
	}
	dir, err := os.MkdirTemp(filepath.Dir(outPath), ".subscribe-sprite-")
	if err != nil {
		return SubscribeSprite{}, err
	}
	defer os.RemoveAll(dir)
	sheet := image.NewNRGBA(image.Rect(0, 0, size.Width, sprite.Height()))
	for i, page := range pages {
		pagePath := filepath.Join(dir, fmt.Sprintf("page-%d.png", i))
		if err := htmlstill.Render(page, pagePath, size); err != nil {
			return SubscribeSprite{}, err
		}
		still, err := decodePNG(pagePath)
		if err != nil {
			return SubscribeSprite{}, err
		}
		top := i * sprite.rowsPerPage(size) * sprite.CellHeight
		draw.Draw(sheet, image.Rect(0, top, size.Width, sprite.Height()), still, image.Point{}, draw.Src)
	}
	return sprite, encodePNG(sheet, outPath)
}

func decodePNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func encodePNG(img image.Image, path string) error {
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return err
	}
	return os.WriteFile(path, out.Bytes(), 0600)
}
