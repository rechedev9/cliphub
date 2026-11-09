package tacticalplan

import (
	"math"
	"sort"

	"github.com/rechedev9/cliphub/internal/radarmap"
)

// PrepSpotLimit caps the positions listed for one player in one time window.
const PrepSpotLimit = 5

// PrepWindow is a span of seconds after freeze end; a zero ToSeconds has no end.
type PrepWindow struct {
	FromSeconds int `json:"from_seconds"`
	ToSeconds   int `json:"to_seconds"`
}

func (w PrepWindow) contains(seconds float64) bool {
	return seconds >= float64(w.FromSeconds) && (w.ToSeconds == 0 || seconds < float64(w.ToSeconds))
}

// PrepWindows are the windows CT positions are read in: the Replay's quick
// tramos on the round clock, 1:55 to 1:00, 1:00 to 0:40 and 0:40 to the end.
var PrepWindows = []PrepWindow{{0, 55}, {55, 75}, {75, 0}}

// PrepThresholds are the rules the prep applied.
type PrepThresholds struct {
	Windows            []PrepWindow `json:"windows"`
	SpotLimit          int          `json:"spot_limit"`
	ExampleLimit       int          `json:"example_limit"`
	SiteContactPlayers int          `json:"site_contact_players"`
	TempoBucketSeconds int          `json:"tempo_bucket_seconds"`
}

// PrepRounds is a set of rounds with the first examples and every round's
// ref, so the UI can open each one in the replay.
type PrepRounds struct {
	ScoutRounds
	Refs []Evidence `json:"refs"`
}

// PrepSpot is a callout a CT player held, with every round's ref. Pos is the
// world centroid of the samples that named it; Floor is "lower" when that
// centroid lies on the lower radar of a two-level map.
type PrepSpot struct {
	ScoutSpot
	Refs  []Evidence `json:"refs"`
	Pos   [3]float64 `json:"pos"`
	Floor string     `json:"floor,omitempty"`
}

// PrepMatchup is the team's buy against the rival's buy on one side. Share is
// over the team's rounds on that side.
type PrepMatchup struct {
	Side        Side    `json:"side"`
	Buy         BuyType `json:"buy"`
	OpponentBuy BuyType `json:"opponent_buy"`
	PrepRounds
}

// PrepTempo is the pace of the team's rounds on one buy and side: the site
// contact (T) or the first duel (CT), and the plant.
type PrepTempo struct {
	Side Side    `json:"side"`
	Buy  BuyType `json:"buy"`
	PrepRounds
	Tempo Histogram `json:"tempo"`
	Plant Histogram `json:"plant"`
}

// PrepPlayerWindow is where a CT player stood most in one window: per round,
// the callout of most of their frames there, and the share of rounds it won.
type PrepPlayerWindow struct {
	PrepWindow
	Rounds int        `json:"rounds"`
	Spots  []PrepSpot `json:"spots"`
}

// PrepPlayer is one CT player of the team and their positions per window.
type PrepPlayer struct {
	SteamID64 string             `json:"steamid64"`
	Name      string             `json:"name"`
	Role      string             `json:"role"`
	Rounds    int                `json:"rounds"`
	Windows   []PrepPlayerWindow `json:"windows"`
}

// Prep is what to expect from a team on one map: how it fares in each buy
// matchup, how fast it plays on each buy, and where each CT player stands.
type Prep struct {
	Map        string         `json:"map"`
	Thresholds PrepThresholds `json:"thresholds"`
	Matches    int            `json:"matches"`
	RoundCount int            `json:"round_count"`
	// Framed counts the CT rounds with positions, which the players read.
	Framed    int           `json:"framed"`
	Matchups  []PrepMatchup `json:"matchups"`
	Tempo     []PrepTempo   `json:"tempo"`
	CTPlayers []PrepPlayer  `json:"ct_players"`
}

// PrepOptions selects the matches and names the roles of the team's players.
type PrepOptions struct {
	Filter ScoutFilter
	// Roles is the inferred role per SteamID; a missing one reads "unknown".
	Roles map[string]string
}

type matchupKey struct {
	side             Side
	buy, opponentBuy BuyType
}

type tempoKey struct {
	side Side
	buy  BuyType
}

// prepGroup is a roundGroup that also keeps every round's ref.
type prepGroup struct {
	roundGroup
	refs []Evidence
}

func (g *prepGroup) add(won bool, e Evidence) {
	g.roundGroup.add(won, e)
	g.refs = append(g.refs, e)
}

func (g *prepGroup) allRefs() []Evidence {
	if g.refs == nil {
		return []Evidence{}
	}
	return g.refs
}

func (g *prepGroup) result(total int) PrepRounds {
	return PrepRounds{ScoutRounds: g.roundGroup.result(total), Refs: g.allRefs()}
}

type tempoAcc struct {
	group        prepGroup
	tempo, plant []float64
}

type prepWindowAcc struct {
	rounds int
	spots  map[string]*prepSpotAcc
}

// prepSpotAcc sums the positions of the samples that named the spot.
type prepSpotAcc struct {
	prepGroup
	sum     [3]float64
	samples int
}

func (a *prepSpotAcc) centroid() [3]float64 {
	if a.samples == 0 {
		return [3]float64{}
	}
	var out [3]float64
	for i, v := range a.sum {
		out[i] = math.Round(v / float64(a.samples))
	}
	return out
}

type prepPlayerAcc struct {
	name    string
	rounds  int
	windows []*prepWindowAcc
}

type prepRun struct {
	sideRounds map[Side]int
	matchups   map[matchupKey]*prepGroup
	tempo      map[tempoKey]*tempoAcc
	players    map[string]*prepPlayerAcc
}

// BuildPrep reads the team's rounds over the selected matches. Matches are
// read in Select order, which is the order of every example list.
func BuildPrep(matches []FramedMatch, opts PrepOptions) Prep {
	f := opts.Filter
	prep := Prep{
		Thresholds: PrepThresholds{
			Windows: PrepWindows, SpotLimit: PrepSpotLimit, ExampleLimit: ScoutExampleLimit,
			SiteContactPlayers: SiteContactPlayers, TempoBucketSeconds: timingBucketSeconds,
		},
	}
	if len(f.Maps) == 1 {
		prep.Map = f.Maps[0]
	}
	run := prepRun{
		sideRounds: map[Side]int{}, matchups: map[matchupKey]*prepGroup{},
		tempo: map[tempoKey]*tempoAcc{}, players: map[string]*prepPlayerAcc{},
	}
	selected := selectFramed(matches, f)
	prep.Matches = len(selected)
	for _, m := range selected {
		src := openFrames(m)
		for _, r := range f.rounds(m.ScoutMatch) {
			prep.RoundCount++
			if run.addRound(m, r, src.round(r.Number)) {
				prep.Framed++
			}
		}
	}
	prep.Matchups = run.matchupRows()
	prep.Tempo = run.tempoRows()
	prep.CTPlayers = run.playerRows(opts.Roles, prep.Map)
	return prep
}

// addRound counts one round; it reports whether CT positions were read from it.
func (run *prepRun) addRound(m FramedMatch, r Round, frames []Frame) bool {
	doc := m.Document
	side := doc.TeamSide(m.TeamKey, r)
	won := r.Winner == side
	team := teamSlots(r, side)
	start := evidence(m.ScoutMatch, r, 0)
	run.sideRounds[side]++
	buy := r.Economy.Buy(side)

	mk := matchupKey{side: side, buy: buy, opponentBuy: r.Economy.Buy(side.Opponent())}
	g, ok := run.matchups[mk]
	if !ok {
		g = &prepGroup{}
		run.matchups[mk] = g
	}
	g.add(won, start)

	track := teamTrack(doc, r, team, frames)
	tk := tempoKey{side: side, buy: buy}
	acc, ok := run.tempo[tk]
	if !ok {
		acc = &tempoAcc{}
		run.tempo[tk] = acc
	}
	acc.group.add(won, start)
	if tick := roundTempo(doc, r, side, team, sitePresence(track)); tick > 0 {
		if secs, ok := secondsSince(r, tick, doc.Demo.Tickrate); ok {
			acc.tempo = append(acc.tempo, secs)
		}
	}
	if r.Bomb != nil && r.Bomb.PlantTick > 0 {
		if secs, ok := secondsSince(r, r.Bomb.PlantTick, doc.Demo.Tickrate); ok {
			acc.plant = append(acc.plant, secs)
		}
	}
	if side != SideCT || len(track) == 0 {
		return false
	}
	run.addPositions(m, r, team, track)
	return true
}

// addPositions reads, per CT player and window, the callout of most of their
// alive frames; a tie goes to the callout reached first.
func (run *prepRun) addPositions(m FramedMatch, r Round, team map[uint8]bool, track []teamFrame) {
	for slot := range team {
		id, name := playerIdentity(m.Document, slot)
		seen := false
		for i, w := range PrepWindows {
			counts := map[string]int{}
			firstTick := map[string]int{}
			best := ""
			for _, f := range track {
				callout, ok := f.callouts[slot]
				if !ok || !w.contains(f.seconds) {
					continue
				}
				counts[callout]++
				if _, ok := firstTick[callout]; !ok {
					firstTick[callout] = f.tick
				}
				if best == "" || counts[callout] > counts[best] {
					best = callout
				}
			}
			if best == "" {
				continue
			}
			acc := run.player(id, name)
			seen = true
			win := acc.windows[i]
			win.rounds++
			spot, ok := win.spots[best]
			if !ok {
				spot = &prepSpotAcc{}
				win.spots[best] = spot
			}
			spot.add(false, evidence(m.ScoutMatch, r, firstTick[best]))
			for _, f := range track {
				if f.callouts[slot] != best || !w.contains(f.seconds) {
					continue
				}
				at := f.pos[slot]
				for k := range at {
					spot.sum[k] += at[k]
				}
				spot.samples++
			}
		}
		if seen {
			run.players[id].rounds++
		}
	}
}

func (run *prepRun) player(id, name string) *prepPlayerAcc {
	acc, ok := run.players[id]
	if !ok {
		acc = &prepPlayerAcc{name: name, windows: make([]*prepWindowAcc, len(PrepWindows))}
		for i := range acc.windows {
			acc.windows[i] = &prepWindowAcc{spots: map[string]*prepSpotAcc{}}
		}
		run.players[id] = acc
	}
	return acc
}

func (run *prepRun) matchupRows() []PrepMatchup {
	out := make([]PrepMatchup, 0, len(run.matchups))
	for k, g := range run.matchups {
		out = append(out, PrepMatchup{Side: k.side, Buy: k.buy, OpponentBuy: k.opponentBuy, PrepRounds: g.result(run.sideRounds[k.side])})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Side != b.Side {
			return a.Side == SideT
		}
		if a.Buy != b.Buy {
			return buyOrder[a.Buy] < buyOrder[b.Buy]
		}
		return buyOrder[a.OpponentBuy] < buyOrder[b.OpponentBuy]
	})
	return out
}

func (run *prepRun) tempoRows() []PrepTempo {
	out := make([]PrepTempo, 0, len(run.tempo))
	for k, acc := range run.tempo {
		out = append(out, PrepTempo{
			Side: k.side, Buy: k.buy, PrepRounds: acc.group.result(run.sideRounds[k.side]),
			Tempo: newHistogram(acc.tempo), Plant: newHistogram(acc.plant),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Side != out[j].Side {
			return out[i].Side == SideT
		}
		return buyOrder[out[i].Buy] < buyOrder[out[j].Buy]
	})
	return out
}

func (run *prepRun) playerRows(roles map[string]string, mapName string) []PrepPlayer {
	cal, calibrated := radarmap.Lookup(mapName)
	out := make([]PrepPlayer, 0, len(run.players))
	for id, acc := range run.players {
		role := roles[id]
		if role == "" {
			role = "unknown"
		}
		p := PrepPlayer{SteamID64: id, Name: acc.name, Role: role, Rounds: acc.rounds, Windows: make([]PrepPlayerWindow, 0, len(PrepWindows))}
		for i, w := range PrepWindows {
			win := acc.windows[i]
			spots := make([]PrepSpot, 0, len(win.spots))
			for callout, g := range win.spots {
				spot := ScoutSpot{Callout: callout, Share: newRate(g.rounds, win.rounds), Examples: g.examples}
				row := PrepSpot{ScoutSpot: spot, Refs: g.allRefs(), Pos: g.centroid()}
				if calibrated && g.samples > 0 && cal.Level(row.Pos[2]) == radarmap.LevelLower {
					row.Floor = radarmap.LevelLower
				}
				spots = append(spots, row)
			}
			sort.Slice(spots, func(i, j int) bool {
				if spots[i].Share.Count != spots[j].Share.Count {
					return spots[i].Share.Count > spots[j].Share.Count
				}
				return spots[i].Callout < spots[j].Callout
			})
			if len(spots) > PrepSpotLimit {
				spots = spots[:PrepSpotLimit]
			}
			p.Windows = append(p.Windows, PrepPlayerWindow{PrepWindow: w, Rounds: win.rounds, Spots: spots})
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Rounds != out[j].Rounds {
			return out[i].Rounds > out[j].Rounds
		}
		return out[i].SteamID64 < out[j].SteamID64
	})
	return out
}
