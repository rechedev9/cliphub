package playercard

import (
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/overlaylang"
)

const (
	statRating  = "rating"
	statKD      = "kd"
	statADR     = "adr"
	statKAST    = "kast"
	statHS      = "hs"
	statEntries = "entries"
	statUtility = "utility"
)

type cardText struct {
	Lang, Title, Rating, Level string
	won, lost, tied            string
	roles                      map[string]string
	stats                      map[string]string
}

var spanishCardText = cardText{
	Lang: "es", Title: "Carta de jugador", Rating: "RATING CLIPHUB", Level: "NIVEL",
	won: "VICTORIA %d-%d", lost: "DERROTA %d-%d", tied: "EMPATE %d-%d",
	roles: map[string]string{library.RoleAWP: "AWPER", library.RoleEntry: "ENTRY", library.RoleSupport: "SOPORTE", library.RoleRifler: "RIFLER"},
	stats: map[string]string{statRating: "RATING CLIPHUB", statKD: "K/D", statADR: "ADR", statKAST: "KAST", statHS: "HS", statEntries: "ENTRADAS", statUtility: "UTILIDAD"},
}

var englishCardText = cardText{
	Lang: "en", Title: "Player card", Rating: "CLIPHUB RATING", Level: "LEVEL",
	won: "WIN %d-%d", lost: "LOSS %d-%d", tied: "DRAW %d-%d",
	roles: map[string]string{library.RoleAWP: "AWPER", library.RoleEntry: "ENTRY", library.RoleSupport: "SUPPORT", library.RoleRifler: "RIFLER"},
	stats: map[string]string{statRating: "CLIPHUB RATING", statKD: "K/D", statADR: "ADR", statKAST: "KAST", statHS: "HS", statEntries: "ENTRIES", statUtility: "UTILITY"},
}

func cardTexts(language overlaylang.Language) cardText {
	if language == overlaylang.English {
		return englishCardText
	}
	return spanishCardText
}

func (t cardText) matchStats(card Card, keys ...string) []stat {
	var out []stat
	for _, key := range keys {
		if f, ok := figureOf(card, key); ok {
			out = append(out, stat{Label: t.stats[key], Value: f.text})
		}
	}
	return out
}

func (t cardText) bestStat(card Card) *stat {
	var best *stat
	lead := 0.0
	for _, key := range bestStats {
		f, ok := figureOf(card, key)
		if ok && (best == nil || f.lead > lead) {
			best, lead = &stat{Label: t.stats[key], Value: f.text}, f.lead
		}
	}
	return best
}
