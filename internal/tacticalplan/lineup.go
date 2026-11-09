package tacticalplan

import "slices"

// MaxLineupPlayers caps a lineup; a team fields five and rarely more than a dozen over a season.
const MaxLineupPlayers = 50

// Lineup is the players a team view keeps, by SteamID64: a round counts only
// when every player the team fielded in it is in the lineup. An empty lineup
// keeps every round.
type Lineup []string

// LineupRound is one round of a document, read from one side; an empty Side
// reads every player of the round.
type LineupRound struct {
	Document Document
	Round    Round
	Side     Side
}

// Keeps reports whether every player who played the side in the round is in
// the lineup.
func (l Lineup) Keeps(lr LineupRound) bool {
	if len(l) == 0 {
		return true
	}
	for _, pr := range lr.Round.Players {
		if lr.Side != "" && pr.Side != lr.Side {
			continue
		}
		p, ok := lr.Document.PlayerBySlot(pr.Slot)
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
		if side := d.TeamSide(teamKey, r); side != "" && l.Keeps(LineupRound{Document: d, Round: r, Side: side}) {
			return true
		}
	}
	return false
}
