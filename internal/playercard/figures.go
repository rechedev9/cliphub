package playercard

import "fmt"

// figure is one shown number: its text and its value against a typical player's.
type figure struct {
	text string
	lead float64
}

// Typical values a figure is measured against to find the player's best one.
// Five players share each round's opening duel, so a typical player opens 0.1 per round.
const (
	typicalRating       = 1.0
	typicalKD           = 1.0
	typicalADR          = 80.0
	typicalKAST         = 70.0
	typicalHS           = 50.0
	typicalEntriesRound = 0.1
	typicalUtilityRound = 6.0
)

// bestLabels are the figures the MVP template compares, in the order a tie keeps.
var bestLabels = []string{"RATING CLIPHUB", "K/D", "ADR", "KAST", "HS", "ENTRADAS", "UTILIDAD"}

func figureOf(card Card, label string) (figure, bool) {
	rounds := float64(card.Rounds)
	switch label {
	case "RATING CLIPHUB":
		if card.Rating != nil {
			return figure{fmt.Sprintf("%.2f", *card.Rating), *card.Rating / typicalRating}, true
		}
	case "K/D":
		kd := float64(card.Kills) / float64(max(card.Deaths, 1))
		return figure{fmt.Sprintf("%.2f", kd), kd / typicalKD}, true
	case "ADR":
		if card.ADR != nil {
			return figure{fmt.Sprintf("%.1f", *card.ADR), *card.ADR / typicalADR}, true
		}
	case "KAST":
		if card.KAST != nil {
			return figure{fmt.Sprintf("%.0f%%", *card.KAST), *card.KAST / typicalKAST}, true
		}
	case "HS":
		if card.HSPct != nil {
			return figure{fmt.Sprintf("%.0f%%", *card.HSPct), *card.HSPct / typicalHS}, true
		}
	case "ENTRADAS":
		if card.OpeningKills != nil {
			return figure{fmt.Sprint(*card.OpeningKills), perRound(*card.OpeningKills, rounds) / typicalEntriesRound}, true
		}
	case "UTILIDAD":
		if card.UtilityDamage != nil {
			return figure{fmt.Sprint(*card.UtilityDamage), perRound(*card.UtilityDamage, rounds) / typicalUtilityRound}, true
		}
	}
	return figure{}, false
}

func perRound(total int, rounds float64) float64 {
	if rounds <= 0 {
		return 0
	}
	return float64(total) / rounds
}

// bestStat is the figure furthest above a typical player's.
func bestStat(card Card) *stat {
	var best *stat
	lead := 0.0
	for _, label := range bestLabels {
		f, ok := figureOf(card, label)
		if ok && (best == nil || f.lead > lead) {
			best, lead = &stat{Label: label, Value: f.text}, f.lead
		}
	}
	return best
}
