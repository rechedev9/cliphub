package library

import "math"

// Map preference model. Each part is in [-1, 1] and the score is
// round(100 × (PreferenceVetoWeight·veto + PreferencePlayWeight·play + PreferenceWinWeight·win)):
//
//	veto = (picks − bans) / (picks + bans + PreferenceVetoPrior)
//	play = 2 × matches / (most matches on any of the team's maps) − 1, or 0 with no matches
//	win  = 2 × (wins + ties/2 + 1) / (matches + 2) − 1
//
// Vetoes weigh most because a pick or a ban is the team saying what it wants;
// the priors keep one series or one match from reading as a strong preference.
const (
	PreferenceVetoWeight = 0.5
	PreferencePlayWeight = 0.25
	PreferenceWinWeight  = 0.25
	PreferenceVetoPrior  = 2
)

// MapPreference is how much a team seems to want a map, from -100 (always
// avoids it) to 100 (always goes for it), with the parts and samples behind it.
type MapPreference struct {
	Score   int     `json:"score"`
	Veto    float64 `json:"veto"`
	Play    float64 `json:"play"`
	Win     float64 `json:"win"`
	Vetoes  int     `json:"vetoes"`
	Matches int     `json:"matches"`
}

func mapPreference(r MapRecord, mostMatches int) MapPreference {
	picks, bans := r.Veto.Picked.Count, r.Veto.Banned.Count
	p := MapPreference{Vetoes: picks + bans, Matches: r.Matches}
	p.Veto = round4(float64(picks-bans) / float64(picks+bans+PreferenceVetoPrior))
	if mostMatches > 0 {
		p.Play = round4(2*float64(r.Matches)/float64(mostMatches) - 1)
	}
	p.Win = round4(2*(float64(r.Wins)+float64(r.Ties)/2+1)/float64(r.Matches+2) - 1)
	p.Score = int(math.Round(100 * (PreferenceVetoWeight*p.Veto + PreferencePlayWeight*p.Play + PreferenceWinWeight*p.Win)))
	return p
}

// scorePreferences sets the preference of every record of one team.
func scorePreferences(records []MapRecord) {
	most := 0
	for _, r := range records {
		most = max(most, r.Matches)
	}
	for i := range records {
		records[i].Preference = mapPreference(records[i], most)
	}
}
