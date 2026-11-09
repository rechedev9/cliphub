package tacticalplan

import (
	"slices"
)

// LeaderboardExampleLimit caps the rounds a row carries to open in the Replay.
const LeaderboardExampleLimit = 30

// MatchReadOptions says what ReadMatch reads besides the metrics.
type MatchReadOptions struct {
	// Roles reads each player's role per side from Opening, the frames of the
	// opening window (OpeningFrames); nil frames leave the rounds unsampled.
	Roles   bool
	Opening map[int][]Frame
}

// MatchRead is one match, read once and kept: each player's totals, role
// tallies and rounds per side. A leaderboard folds many of these, so a new
// filter never reads a match again.
type MatchRead struct {
	Rounds int
	// HasRoles is true when the role tallies were read.
	HasRoles bool
	totals   map[Side]map[string]playerTotals
	roles    map[Side]map[string]roleTally
	rounds   map[Side]map[string][]Evidence
}

// ReadMatch reads the totals of every player on each side of a match.
func ReadMatch(m MetricsMatch, opts MatchReadOptions) MatchRead {
	read := MatchRead{
		Rounds: len(m.Document.Rounds), HasRoles: opts.Roles,
		totals: map[Side]map[string]playerTotals{}, roles: map[Side]map[string]roleTally{}, rounds: map[Side]map[string][]Evidence{},
	}
	for _, side := range []Side{SideCT, SideT} {
		read.totals[side] = map[string]playerTotals{}
		read.roles[side] = map[string]roleTally{}
		read.rounds[side] = map[string][]Evidence{}
		report := Metrics([]MetricsMatch{m}, MetricsOptions{Side: side})
		for _, p := range report.Players {
			t := playerTotals{steamID: p.SteamID64, name: p.Name}
			t.add(p)
			read.totals[side][p.SteamID64] = t
		}
	}
	steamBySlot := map[uint8]string{}
	for _, p := range m.Document.Players {
		if p.SteamID64 != "" {
			steamBySlot[p.Slot] = p.SteamID64
		}
	}
	zones := siteZones(m.Document)
	for _, r := range m.Document.Rounds {
		for _, pr := range r.Players {
			steam, ok := steamBySlot[pr.Slot]
			if !ok || read.rounds[pr.Side] == nil {
				continue
			}
			read.rounds[pr.Side][steam] = append(read.rounds[pr.Side][steam], Evidence{MatchID: m.MatchID, Round: r.Number, Tick: r.TickFreezeEnd})
			if !opts.Roles || len(opts.Opening[r.Number]) == 0 {
				continue
			}
			if tally, ok := readRoleRound(m.Document, zones, opts.Opening[r.Number], r, pr.Slot, pr.Side); ok {
				t := read.roles[pr.Side][steam]
				t.add(tally)
				read.roles[pr.Side][steam] = t
			}
		}
	}
	return read
}

// LeaderboardOptions selects and orders the rows of a LeaderboardSample.
type LeaderboardOptions struct {
	Metric    string
	MinRounds int
	Limit     int
	// Role keeps the players whose role on its side is this one; the sample
	// must hold that side only.
	Role string
}

type leaderboardPlayer struct {
	totals   playerTotals
	roles    map[Side]*roleTally
	examples []Evidence
}

// LeaderboardSample folds MatchReads into per-player totals over the chosen
// sides. Fold the newest match first: the examples keep the first rounds seen.
type LeaderboardSample struct {
	Matches int
	Rounds  int
	roles   bool
	players map[string]*leaderboardPlayer
}

// Add folds one match, counting the rounds of side only, or of both when side is empty.
func (s *LeaderboardSample) Add(m MatchRead, side Side) {
	if s.players == nil {
		s.players = map[string]*leaderboardPlayer{}
	}
	s.Matches++
	s.Rounds += m.Rounds
	s.roles = s.roles || m.HasRoles
	counted := map[string]bool{}
	sides := []Side{SideCT, SideT}
	if side != "" {
		sides = []Side{side}
	}
	for _, sd := range sides {
		for steam, t := range m.totals[sd] {
			p := s.players[steam]
			if p == nil {
				p = &leaderboardPlayer{totals: playerTotals{steamID: steam, name: t.name}, roles: map[Side]*roleTally{}}
				s.players[steam] = p
			}
			if !counted[steam] {
				counted[steam] = true
				p.totals.matches++
			}
			p.totals.merge(t)
			if tally, ok := m.roles[sd][steam]; ok {
				if p.roles[sd] == nil {
					p.roles[sd] = &roleTally{}
				}
				p.roles[sd].add(tally)
			}
			for _, e := range m.rounds[sd][steam] {
				if len(p.examples) < LeaderboardExampleLimit {
					p.examples = append(p.examples, e)
				}
			}
		}
	}
}

func (p *leaderboardPlayer) roleOn(side Side) string {
	if t := p.roles[side]; t != nil {
		return t.role(side)
	}
	return ""
}

// Rows ranks the players with at least MinRounds rounds on the metric, best
// first. A player without the metric, or without the role, is left out.
func (s *LeaderboardSample) Rows(opts LeaderboardOptions) []LeaderboardRow {
	rows := []LeaderboardRow{}
	if !slices.Contains(LeaderboardMetrics, opts.Metric) {
		return rows
	}
	for _, p := range s.players {
		values := p.totals.values()
		v, ok := values[opts.Metric]
		if p.totals.rounds < opts.MinRounds || !ok {
			continue
		}
		if opts.Role != "" && p.roleOn(RoleSide(opts.Role)) != opts.Role {
			continue
		}
		row := LeaderboardRow{
			SteamID64: p.totals.steamID, Name: p.totals.name, Rounds: p.totals.rounds, Matches: p.totals.matches,
			Value: v, Metrics: values, Examples: p.examples,
		}
		if s.roles {
			row.Roles = &PlayerRoles{CT: p.roleOn(SideCT), T: p.roleOn(SideT)}
		}
		rows = append(rows, row)
	}
	return rankRows(rows, opts.Metric, opts.Limit)
}
