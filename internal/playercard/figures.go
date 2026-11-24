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

// bestStats are the figures the MVP template compares, in the order a tie keeps.
var bestStats = []string{statRating, statKD, statADR, statKAST, statHS, statEntries, statUtility}

func figureOf(card Card, key string) (figure, bool) {
	rounds := float64(card.Rounds)
	switch key {
	case statRating:
		if card.Rating != nil {
			return figure{fmt.Sprintf("%.2f", *card.Rating), *card.Rating / typicalRating}, true
		}
	case statKD:
		kd := float64(card.Kills) / float64(max(card.Deaths, 1))
		return figure{fmt.Sprintf("%.2f", kd), kd / typicalKD}, true
	case statADR:
		if card.ADR != nil {
			return figure{fmt.Sprintf("%.1f", *card.ADR), *card.ADR / typicalADR}, true
		}
	case statKAST:
		if card.KAST != nil {
			return figure{fmt.Sprintf("%.0f%%", *card.KAST), *card.KAST / typicalKAST}, true
		}
	case statHS:
		if card.HSPct != nil {
			return figure{fmt.Sprintf("%.0f%%", *card.HSPct), *card.HSPct / typicalHS}, true
		}
	case statEntries:
		if card.OpeningKills != nil {
			return figure{fmt.Sprint(*card.OpeningKills), perRound(*card.OpeningKills, rounds) / typicalEntriesRound}, true
		}
	case statUtility:
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
