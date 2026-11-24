package editor

import (
	"fmt"
	"strings"

	"github.com/rechedev9/cliphub/internal/overlaylang"
)

type overlayText struct {
	clutchWon      string
	matchStats     string
	headshots      string
	rating         string
	faceitLevel    string
	winRate        string
	bestMap        string
	winProbability string
	ctPlate        string
	tPlate         string
	with           string
	on             string
}

var spanishOverlayText = overlayText{
	clutchWon:      "GANADO",
	matchStats:     "ESTADÍSTICAS DE LA PARTIDA",
	headshots:      "HS %.0f %%",
	rating:         "RATING DE CLIPHUB %.2f",
	faceitLevel:    "FACEIT NIVEL %d",
	winRate:        "%.0f %% DE VICTORIAS",
	bestMap:        "MEJOR MAPA: ",
	winProbability: "PROB. DE VICTORIA",
	ctPlate:        "CT %d %%",
	tPlate:         "%d %% T",
	with:           "con",
	on:             "en",
}

var englishOverlayText = overlayText{
	clutchWon:      "WON",
	matchStats:     "MATCH STATS",
	headshots:      "HS %.0f%%",
	rating:         "CLIPHUB RATING %.2f",
	faceitLevel:    "FACEIT LEVEL %d",
	winRate:        "%.0f%% WIN RATE",
	bestMap:        "BEST MAP: ",
	winProbability: "WIN PROBABILITY",
	ctPlate:        "CT %d%%",
	tPlate:         "%d%% T",
	with:           "with",
	on:             "on",
}

func overlayTexts(language overlaylang.Language) overlayText {
	if language == overlaylang.English {
		return englishOverlayText
	}
	return spanishOverlayText
}

type headlineFacts struct {
	mapName string
	kills   int
	weapon  string
}

func (t overlayText) headline(facts headlineFacts) string {
	parts := []string{"Highlight"}
	if facts.kills > 0 {
		parts = []string{fmt.Sprintf("%dK", facts.kills)}
	}
	if facts.weapon != "" {
		parts = append(parts, t.with, facts.weapon)
	}
	if facts.mapName != "" {
		parts = append(parts, t.on, prettifyMapName(facts.mapName))
	}
	return strings.Join(parts, " ")
}

func (t overlayText) smokeHeadline(mapName string, smoke SmokeCue) string {
	destination := smoke.Destination
	if destination == "" {
		destination = utilityDisplayName(smoke.Type)
	}
	if smoke.FromArea != "" && smoke.Destination != "" {
		destination = smoke.FromArea + " -> " + smoke.Destination
	}
	if mapName != "" {
		return destination + " " + t.on + " " + prettifyMapName(mapName)
	}
	return destination
}
