package editor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/htmlstill"
	"github.com/rechedev9/cliphub/internal/playercard"
)

const (
	// The opening card holds the first seconds and leaves before the first kill when it can.
	openingCardSeconds    = 2.4
	openingCardMinSeconds = 1.2
	openingCardBeforeKill = 0.3
	// playerCardEntrySeconds is the card's brief fade in.
	playerCardEntrySeconds = 0.3
)

// PlayerCards are the drawn card PNGs of each slot of a one-player 9:16 Short; empty is no card.
type PlayerCards struct {
	Opening string `json:"opening,omitempty"`
	End     string `json:"end,omitempty"`
	Cover   string `json:"cover,omitempty"`
}

// preparePlayerCards draws each chosen template once into outDir with the overlay renderer.
func preparePlayerCards(path, outDir string) (*PlayerCards, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read player cards: %w", err)
	}
	var req playercard.Request
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("decode player cards: %w", err)
	}
	var image []byte
	if req.ImagePath != "" {
		if image, err = os.ReadFile(req.ImagePath); err != nil {
			return nil, fmt.Errorf("read player image: %w", err)
		}
	}
	drawn := map[string]string{}
	draw := func(id string) (string, error) {
		if id == "" || drawn[id] != "" {
			return drawn[id], nil
		}
		html, err := playercard.HTML(id, req.Card, image)
		if err != nil {
			return "", err
		}
		out := filepath.Join(outDir, "player-card-"+id+".png")
		if err := htmlstill.Render(html, out, htmlstill.Portrait); err != nil {
			return "", fmt.Errorf("draw the %s player card: %w", id, err)
		}
		drawn[id] = out
		return out, nil
	}
	var cards PlayerCards
	for _, slot := range []struct {
		id  string
		dst *string
	}{{req.Opening, &cards.Opening}, {req.End, &cards.End}, {req.Cover, &cards.Cover}} {
		if *slot.dst, err = draw(slot.id); err != nil {
			return nil, err
		}
	}
	return &cards, nil
}

// shortPlayerCards are the Short's cards, or nil when it is not a one-player 9:16 Short.
func shortPlayerCards(short ShortEdit) *PlayerCards {
	if short.Overlays.PlayerCards == nil || short.OutputFormat != OutputFormatShort9x16 || short.MultiPOV() {
		return nil
	}
	return short.Overlays.PlayerCards
}

func openingPlayerCard(short ShortEdit) string {
	if cards := shortPlayerCards(short); cards != nil {
		return cards.Opening
	}
	return ""
}

func endPlayerCard(short ShortEdit) string {
	if cards := shortPlayerCards(short); cards != nil {
		return cards.End
	}
	return ""
}

func coverPlayerCard(short ShortEdit) string {
	if cards := shortPlayerCards(short); cards != nil {
		return cards.Cover
	}
	return ""
}

// openingCardWindow ends the opening card before the first kill, never under openingCardMinSeconds.
func openingCardWindow(short ShortEdit) (float64, bool) {
	end := min(openingCardSeconds, short.DurationSeconds)
	for _, kill := range short.Kills {
		end = min(end, max(kill.TimeSeconds-openingCardBeforeKill, openingCardMinSeconds))
	}
	return end, end >= openingCardMinSeconds
}

// generatedPlayerCardEffects lays the opening card over the first seconds and the end card over the closing span.
func generatedPlayerCardEffects(short ShortEdit) []Effect {
	width, height := outputDimensions(short)
	card := func(path string, start, end, fadeOut float64, source string) Effect {
		return Effect{Type: EffectImage, Path: path, X: "0", Y: "0", Width: width, Height: height,
			StartSeconds: start, EndSeconds: end, FadeInSeconds: playerCardEntrySeconds, FadeOutSeconds: fadeOut, Source: source}
	}
	var effects []Effect
	if path := openingPlayerCard(short); path != "" {
		if end, ok := openingCardWindow(short); ok {
			effects = append(effects, card(path, 0, end, playerCardEntrySeconds, "player-card-opening"))
		}
	}
	if path := endPlayerCard(short); path != "" {
		if start, end, ok := statsCardWindow(short); ok {
			effects = append(effects, card(path, start, end, 0, "player-card-end"))
		}
	}
	return effects
}
