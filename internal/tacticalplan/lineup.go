package tacticalplan

import "slices"

// MaxLineupPlayers caps a lineup; a team fields five and rarely more than a dozen over a season.
const MaxLineupPlayers = 50

// Lineup is the players a team view keeps, by SteamID64: a round counts only
// when every player the team fielded in it is in the lineup. An empty lineup
// keeps every round.
type Lineup []string

// Keeps reports whether every player who played side in round r is in the
// lineup; with no side, every player of the round must be.
func (l Lineup) Keeps(d Document, r Round, side Side) bool {
	if len(l) == 0 {
		return true
	}
	for _, pr := range r.Players {
		if side != "" && pr.Side != side {
			continue
		}
		p, ok := d.PlayerBySlot(pr.Slot)
		if !ok || !slices.Contains(l, p.SteamID64) {
			return false
		}
	}
	return true
}

// PlayedIn reports whether the team (teamKey) played at least one round of
// the document with only lineup players, so a match it never fielded them in
// leaves the sample instead of counting with no rounds.
func (l Lineup) PlayedIn(d Document, teamKey string) bool {
	if len(l) == 0 {
		return true
	}
	for _, r := range d.Rounds {
		if side := d.TeamSide(teamKey, r); side != "" && l.Keeps(d, r, side) {
			return true
		}
	}
	return false
}
