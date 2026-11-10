package tacticalplan

import (
	"slices"
	"sort"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/placename"
)

// LeaderboardExampleLimit caps the rounds a row carries to open in the Replay.
const LeaderboardExampleLimit = 30

// RoundRef is a round of a player to open in the Replay, with the slot the
// player had in that match so the Replay can keep them in focus.
type RoundRef struct {
	MatchID uuid.UUID `json:"match_id"`
	Round   int       `json:"round"`
	Tick    int       `json:"tick"`
	Slot    uint8     `json:"slot"`
}

// MatchReadOptions says what ReadMatch reads besides the metrics.
type MatchReadOptions struct {
	// Roles reads each player's role per side from Opening, the frames of the
	// opening window (OpeningFrames); nil frames leave the rounds unsampled.
	Roles   bool
	Opening map[int][]Frame
}

// MatchRead is one match, read once and kept: each player's totals, role
// counts and rounds per side. A leaderboard folds many of these, so a new
// filter never reads a match again.
type MatchRead struct {
	Rounds int
	// HasRoles is true when the role counts were read.
	HasRoles bool
	totals   map[Side]map[string]playerTotals
	roles    map[Side]map[string]RoleCounts
	rounds   map[Side]map[string][]RoundRef
	// keyed files the rounds of a player under the role and the position they showed.
	keyed map[Side]map[string]map[string][]RoundRef
	// teams names each player's team; set on pro reads only.
	teams map[string]string
}

// ReadMatch reads the totals of every player on each side of a match.
func ReadMatch(m MetricsMatch, opts MatchReadOptions) MatchRead {
	read := MatchRead{
		Rounds: len(m.Document.Rounds), HasRoles: opts.Roles,
		totals: map[Side]map[string]playerTotals{}, roles: map[Side]map[string]RoleCounts{},
		rounds: map[Side]map[string][]RoundRef{}, keyed: map[Side]map[string]map[string][]RoundRef{},
	}
	for _, side := range []Side{SideCT, SideT} {
		read.totals[side] = map[string]playerTotals{}
		read.roles[side] = map[string]RoleCounts{}
		read.rounds[side] = map[string][]RoundRef{}
		read.keyed[side] = map[string]map[string][]RoundRef{}
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
			ref := RoundRef{MatchID: m.MatchID, Round: r.Number, Tick: r.TickFreezeEnd, Slot: pr.Slot}
			read.rounds[pr.Side][steam] = append(read.rounds[pr.Side][steam], ref)
			if !opts.Roles || len(opts.Opening[r.Number]) == 0 {
				continue
			}
			rr, ok := readRoleRound(roundReadInput{doc: m.Document, zones: zones, frames: opts.Opening[r.Number], round: r, slot: pr.Slot, side: pr.Side})
			if !ok {
				continue
			}
			t := read.roles[pr.Side][steam]
			t.add(rr.counts)
			read.roles[pr.Side][steam] = t
			if read.keyed[pr.Side][steam] == nil {
				read.keyed[pr.Side][steam] = map[string][]RoundRef{}
			}
			for _, key := range rr.keys() {
				read.keyed[pr.Side][steam][key] = append(read.keyed[pr.Side][steam][key], ref)
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
	// MapRole keeps the players whose position on MapRoleSide is this callout
	// (or MapRoleAllround); the sample must hold that side only.
	MapRole     string
	MapRoleSide Side
	// LibraryRoles is each player's library role (awp, entry, support, rifler);
	// LibraryRole keeps the players with that one.
	LibraryRoles map[string]string
	LibraryRole  string
}

type leaderboardPlayer struct {
	totals   playerTotals
	roles    map[Side]*RoleCounts
	examples []RoundRef
	keyed    map[string][]RoundRef
	team     string
}

// LeaderboardSample folds MatchReads into per-player totals over the chosen
// sides. Fold the newest match first: the examples keep the first rounds seen.
type LeaderboardSample struct {
	Matches int
	Rounds  int
	roles   bool
	players map[string]*leaderboardPlayer
}

func appendCapped(dst []RoundRef, src []RoundRef) []RoundRef {
	for _, e := range src {
		if len(dst) >= LeaderboardExampleLimit {
			break
		}
		dst = append(dst, e)
	}
	return dst
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
				p = &leaderboardPlayer{totals: playerTotals{steamID: steam, name: t.name}, roles: map[Side]*RoleCounts{}, keyed: map[string][]RoundRef{}}
				s.players[steam] = p
			}
			if p.team == "" {
				p.team = m.teams[steam]
			}
			if !counted[steam] {
				counted[steam] = true
				p.totals.matches++
			}
			p.totals.merge(t)
			if tally, ok := m.roles[sd][steam]; ok {
				if p.roles[sd] == nil {
					p.roles[sd] = &RoleCounts{}
				}
				p.roles[sd].add(tally)
			}
			p.examples = appendCapped(p.examples, m.rounds[sd][steam])
			for key, refs := range m.keyed[sd][steam] {
				p.keyed[key] = appendCapped(p.keyed[key], refs)
			}
		}
	}
}

func (p *leaderboardPlayer) countsOn(side Side) RoleCounts {
	if t := p.roles[side]; t != nil {
		return *t
	}
	return RoleCounts{}
}

func (p *leaderboardPlayer) roleOn(side Side) string { return p.countsOn(side).role(side) }

// breakdown is the player's roles and positions per side, or nil without a read.
func (p *leaderboardPlayer) breakdown() *PlayerBreakdown {
	ct, t := p.countsOn(SideCT).breakdown(SideCT), p.countsOn(SideT).breakdown(SideT)
	if ct == nil && t == nil {
		return nil
	}
	return &PlayerBreakdown{CT: ct, T: t}
}

// rowExamples are the rounds a row opens: the ones that showed the filtered
// role or position, else the newest of the side.
func (p *leaderboardPlayer) rowExamples(opts LeaderboardOptions) []RoundRef {
	switch {
	case opts.Role != "":
		return p.keyed[roleKeyPrefix+opts.Role]
	case opts.MapRole != "" && opts.MapRole != MapRoleAllround:
		return p.keyed[placeKeyPrefix+opts.MapRole]
	}
	return p.examples
}

// PlayerExamples is the rounds each player's row opens under the role and map
// role of opts, by SteamID64, whatever the player's metrics or rounds.
func (s *LeaderboardSample) PlayerExamples(opts LeaderboardOptions) map[string][]RoundRef {
	out := map[string][]RoundRef{}
	for steam, p := range s.players {
		if refs := p.rowExamples(opts); len(refs) > 0 {
			out[steam] = refs
		}
	}
	return out
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
		if opts.MapRole != "" && p.countsOn(opts.MapRoleSide).mapRole() != opts.MapRole {
			continue
		}
		libraryRole := opts.LibraryRoles[p.totals.steamID]
		if opts.LibraryRole != "" && libraryRole != opts.LibraryRole {
			continue
		}
		row := LeaderboardRow{
			SteamID64: p.totals.steamID, Name: p.totals.name, Rounds: p.totals.rounds, Matches: p.totals.matches,
			Value: v, Metrics: values, Examples: p.rowExamples(opts), Team: p.team, LibraryRole: libraryRole,
		}
		if s.roles {
			row.Roles = &PlayerRoles{CT: p.roleOn(SideCT), T: p.roleOn(SideT)}
			row.Breakdown = p.breakdown()
		}
		rows = append(rows, row)
	}
	return rankRows(rows, opts.Metric, opts.Limit)
}

// MapRoleOptions lists the map roles the players with at least minRounds
// rounds have on a side, most common first: what a map role filter can pick.
func (s *LeaderboardSample) MapRoleOptions(side Side, minRounds int) []MapRoleOption {
	counts := map[string]int{}
	for _, p := range s.players {
		if p.totals.rounds < minRounds {
			continue
		}
		if role := p.countsOn(side).mapRole(); role != "" {
			counts[role]++
		}
	}
	out := make([]MapRoleOption, 0, len(counts))
	for key, n := range counts {
		label := "Todoterreno"
		if key != MapRoleAllround {
			label = placename.Label(key)
		}
		out = append(out, MapRoleOption{Key: key, Label: label, Players: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Players != out[j].Players {
			return out[i].Players > out[j].Players
		}
		return out[i].Key < out[j].Key
	})
	return out
}
