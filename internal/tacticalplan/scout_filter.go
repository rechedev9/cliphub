package tacticalplan

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// ScoutMatch is one match of the scouted team: its document, the key the team
// played under in it, and the match-level facts the filter reads.
type ScoutMatch struct {
	MatchID    uuid.UUID
	TeamKey    string
	PlayedAt   time.Time
	OpponentID string
	Opponent   string
	// OpponentRank is the rival's HLTV rank; 0 when it is not known.
	OpponentRank int
	// OwnTeam reports that one of the user's own teams played the match.
	OwnTeam  bool
	Document Document
	// Setup holds, per round number, the frame read ScoutSetupSeconds after
	// freeze end. It is nil when the match has no position blob.
	Setup map[int]Frame
}

// Whose picks matches by who played them: the user's own team, or only others.
type Whose string

// Whose values; the empty one keeps every match.
const (
	WhoseOwn    Whose = "own"
	WhoseOthers Whose = "others"
)

// OpponentTops are the HLTV rank bands the opponent level offers.
var OpponentTops = []int{5, 15, 30, 100, 200}

// ScoutFilter selects matches (map, date, rival, rival level, whose) and rounds (side,
// lineup, the criteria of Rounds). Zero fields do not filter; From and To are inclusive.
type ScoutFilter struct {
	Maps      []string   `json:"maps,omitempty"`
	Side      Side       `json:"side,omitempty"`
	From      *time.Time `json:"from,omitempty"`
	To        *time.Time `json:"to,omitempty"`
	Opponents []string   `json:"opponents,omitempty"`
	Lineup    Lineup     `json:"lineup,omitempty"`
	// MaxMatches keeps the most recent matches the lineup played; the loader applies it.
	MaxMatches int `json:"max_matches,omitempty"`
	// OpponentTop keeps the matches against a rival ranked this high or higher.
	OpponentTop int   `json:"opponent_top,omitempty"`
	Whose       Whose `json:"whose,omitempty"`
	// Rounds holds the round criteria (buys, AWP, advantage, range); its team,
	// side and lineup are the scout's own, and slots never apply across demos.
	Rounds Filter `json:"rounds"`
	// PlayRounds, when not nil, keeps only these rounds per match: a Libreta play's examples.
	PlayRounds map[uuid.UUID][]int `json:"-"`
}

// Select returns the matches the filter keeps, most recent first. The order is
// the order every example list follows, so the newest rounds come first.
func (f ScoutFilter) Select(matches []ScoutMatch) []ScoutMatch {
	kept, _ := f.SelectRated(matches)
	return kept
}

// SelectRated is Select plus the matches dropped only because the rival has no
// known rank, so a view can say how many the opponent level could not rate.
func (f ScoutFilter) SelectRated(matches []ScoutMatch) (kept, unrated []ScoutMatch) {
	kept = make([]ScoutMatch, 0, len(matches))
	for _, m := range matches {
		if !f.keepsMatch(m) {
			continue
		}
		if f.OpponentTop > 0 && m.OpponentRank <= 0 {
			unrated = append(unrated, m)
			continue
		}
		if f.OpponentTop > 0 && m.OpponentRank > f.OpponentTop {
			continue
		}
		kept = append(kept, m)
	}
	newestFirst(kept)
	newestFirst(unrated)
	return kept, unrated
}

func (f ScoutFilter) keepsMatch(m ScoutMatch) bool {
	switch {
	case len(f.Maps) > 0 && !containsString(f.Maps, m.Document.Demo.Map),
		f.From != nil && m.PlayedAt.Before(*f.From),
		f.To != nil && m.PlayedAt.After(*f.To),
		len(f.Opponents) > 0 && !containsString(f.Opponents, m.OpponentID),
		f.Whose == WhoseOwn && !m.OwnTeam,
		f.Whose == WhoseOthers && m.OwnTeam,
		f.PlayRounds != nil && len(f.PlayRounds[m.MatchID]) == 0:
		return false
	}
	return true
}

func newestFirst(out []ScoutMatch) {
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].PlayedAt.Equal(out[j].PlayedAt) {
			return out[i].PlayedAt.After(out[j].PlayedAt)
		}
		return out[i].MatchID.String() < out[j].MatchID.String()
	})
}

func (f ScoutFilter) rounds(m ScoutMatch) []Round {
	return f.RoundFilter(m).Apply(m.Document)
}

// RoundFilter is the round filter of one match: the round criteria, followed
// from the scouted team's key, on the scout's side and lineup.
func (f ScoutFilter) RoundFilter(m ScoutMatch) Filter {
	out := f.Rounds
	out.TeamKey, out.Side, out.Lineup = m.TeamKey, f.Side, f.Lineup
	out.Slots, out.Subjects = nil, nil
	if f.PlayRounds != nil {
		out.Numbers = append([]int{}, f.PlayRounds[m.MatchID]...)
	}
	return out
}

// ParseWhose reads the whose-matches value of a query.
func ParseWhose(raw string) (Whose, error) {
	switch v := Whose(raw); v {
	case "", WhoseOwn, WhoseOthers:
		return v, nil
	default:
		return "", fmt.Errorf("whose must be own or others, not %q", raw)
	}
}

// ParseOpponentTop reads the opponent level of a query: one of OpponentTops, or 0.
func ParseOpponentTop(n int) (int, error) {
	if n == 0 || containsInt(OpponentTops, n) {
		return n, nil
	}
	return 0, fmt.Errorf("opponent_top must be one of %v", OpponentTops)
}

func containsInt(list []int, v int) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
