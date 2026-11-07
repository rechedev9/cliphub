package prolineups

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// IndexVersion changes whenever the meaning of a field changes.
const IndexVersion = 1

// Rules a lineup must pass to be published: thrown often enough, by more than
// one player and in more than one map, so a one-off or a single habit stays out.
const (
	minLineupThrows  = 3
	minLineupPlayers = 2
	minLineupMaps    = 2
	// A cluster is a lineup when it holds this share of the throws of its kind
	// from around its spot, or is thrown alwaysLineup times: a spot where players
	// throw anything anyhow only yields chance repeats.
	minSpotShare = 0.2
	alwaysLineup = 10
	recentDays       = 90
	trendDays        = 30
	instaSeconds     = 6.0
	roundSeconds     = 115.0
	examplesPerItem  = 8
	topEntries       = 5
)

// MapIndex is the published lineup library of one map.
type MapIndex struct {
	Version    int       `json:"version"`
	Map        string    `json:"map"`
	BuiltAt    time.Time `json:"built_at"`
	RecentDays int       `json:"recent_days"`
	// Maps and Rounds count the pro maps and side rounds read, recent and all time.
	Maps         Counts   `json:"maps"`
	RoundsT      Counts   `json:"rounds_t"`
	RoundsCT     Counts   `json:"rounds_ct"`
	Lineups      []Lineup `json:"lineups"`
	Combos       []Combo  `json:"combos"`
	LatestPlayed string   `json:"latest_played"`
}

// Counts is a number over the recent window and over everything read.
type Counts struct {
	Recent int `json:"recent"`
	All    int `json:"all"`
}

// Lineup is one way to throw one grenade that pros repeat.
type Lineup struct {
	ID         string                 `json:"id"`
	Slug       string                 `json:"slug"`
	Title      string                 `json:"title"`
	Kind       tacticalplan.EventKind `json:"kind"`
	Side       tacticalplan.Side      `json:"side"`
	ThrowPlace string                 `json:"throw_place"`
	LandPlace  string                 `json:"land_place"`
	Origin     [3]float64             `json:"origin"`
	Yaw        float64                `json:"yaw"`
	Pitch      float64                `json:"pitch"`
	Landing    [3]float64             `json:"landing"`
	Movement   string                 `json:"movement"`
	Jump       bool                   `json:"jump"`
	Insta      bool                   `json:"insta"`
	AirTime    float64                `json:"air_time"`
	Throws     Counts                 `json:"throws"`
	Matches    Counts                 `json:"matches"`
	// RoundShare is the share of the side's recent rounds on the map with this throw.
	RoundShare float64 `json:"round_share"`
	// Trend is throws per 100 side rounds over the last 30 days and the 60 before.
	Trend      [2]float64    `json:"trend"`
	RoundWin   float64       `json:"round_win"`
	Attempts   int           `json:"attempts"`
	Landed     int           `json:"landed"`
	Clock      ClockStats    `json:"clock"`
	Teams      []Usage       `json:"teams"`
	Players    []Usage       `json:"players"`
	Events     []Usage       `json:"events"`
	PlantSites []Usage       `json:"plant_sites"`
	With       []Usage       `json:"with"`
	Examples   []Example     `json:"examples"`
	LastSeen   time.Time     `json:"last_seen"`
	TeamCount  int           `json:"team_count"`
	Rank       RankInLanding `json:"rank"`
}

// ClockStats is when in the round the lineup is thrown, as seconds after freeze end.
type ClockStats struct {
	Median float64 `json:"median"`
	// Buckets counts recent throws per 10 seconds of round time, the last one for after the clock.
	Buckets []int `json:"buckets"`
}

// Usage is how often one team, player, event, site or partner lineup appears.
type Usage struct {
	Key    string  `json:"key"`
	Name   string  `json:"name"`
	Count  int     `json:"count"`
	Share  float64 `json:"share"`
	Detail string  `json:"detail,omitempty"`
}

// Example is one recent throw to watch in a pro match.
type Example struct {
	Match    string    `json:"match"`
	MapKey   string    `json:"map_key"`
	Event    string    `json:"event"`
	PlayedAt time.Time `json:"played_at"`
	Team     string    `json:"team"`
	Player   string    `json:"player"`
	Round    int       `json:"round"`
	Clock    float64   `json:"clock"`
	Tick     int       `json:"tick"`
}

// RankInLanding places a lineup among those landing at the same callout.
type RankInLanding struct {
	Position int `json:"position"`
	Of       int `json:"of"`
}

// BuildIndex groups a map's throws into lineups and combos as of now.
func BuildIndex(mapName string, throws []Throw, now time.Time) MapIndex {
	idx := MapIndex{Version: IndexVersion, Map: mapName, BuiltAt: now.UTC(), RecentDays: recentDays, Lineups: []Lineup{}, Combos: []Combo{}}
	recentFrom := now.AddDate(0, 0, -recentDays)
	rounds := sideRounds(throws, recentFrom, &idx)
	clusters := clusterThrows(throws)
	grid := throwGrid(throws)
	memberOf := map[int]string{}
	for _, c := range clusters {
		l, ok := buildLineup(c, throws, grid, rounds, now)
		if !ok {
			continue
		}
		for _, i := range c.members {
			memberOf[i] = l.ID
		}
		idx.Lineups = append(idx.Lineups, l)
	}
	slices.SortFunc(idx.Lineups, func(a, b Lineup) int {
		return cmp.Or(cmp.Compare(b.Throws.Recent, a.Throws.Recent), cmp.Compare(b.Throws.All, a.Throws.All), cmp.Compare(a.ID, b.ID))
	})
	uniqueNames(idx.Lineups)
	addPartners(idx.Lineups, throws, memberOf, recentFrom)
	rankInLanding(idx.Lineups)
	idx.Combos = buildCombos(idx.Lineups, throws, memberOf, recentFrom)
	return idx
}

// roundKey is one side's round of one pro map.
type roundKey struct {
	mapKey string
	round  int
	side   tacticalplan.Side
}

// roundInfo is the team that played a side round and when the map was played.
type roundInfo struct {
	team     string
	playedAt time.Time
}

type roundTally struct {
	recent map[roundKey]roundInfo
	all    map[roundKey]bool
}

// sideRounds counts the side rounds seen. A round with no grenade of a side is
// missed, which only happens in rounds lost before anyone threw anything.
func sideRounds(throws []Throw, recentFrom time.Time, idx *MapIndex) roundTally {
	tally := roundTally{recent: map[roundKey]roundInfo{}, all: map[roundKey]bool{}}
	maps, recentMaps := map[string]bool{}, map[string]bool{}
	latest := time.Time{}
	for _, t := range throws {
		k := roundKey{t.MapKey, t.Round, t.Side}
		tally.all[k] = true
		maps[t.MapKey] = true
		if !t.PlayedAt.Before(recentFrom) {
			tally.recent[k] = roundInfo{team: t.Team, playedAt: t.PlayedAt}
			recentMaps[t.MapKey] = true
		}
		if t.PlayedAt.After(latest) {
			latest = t.PlayedAt
		}
	}
	idx.Maps = Counts{Recent: len(recentMaps), All: len(maps)}
	for k := range tally.all {
		if k.side == tacticalplan.SideT {
			idx.RoundsT.All++
		} else {
			idx.RoundsCT.All++
		}
	}
	for k := range tally.recent {
		if k.side == tacticalplan.SideT {
			idx.RoundsT.Recent++
		} else {
			idx.RoundsCT.Recent++
		}
	}
	if !latest.IsZero() {
		idx.LatestPlayed = latest.UTC().Format(time.RFC3339)
	}
	return tally
}

func buildLineup(c *cluster, throws []Throw, grid map[gridKey][]int, rounds roundTally, now time.Time) (Lineup, bool) {
	members := make([]Throw, 0, len(c.members))
	for _, i := range c.members {
		members = append(members, throws[i])
	}
	players, maps := map[string]bool{}, map[string]bool{}
	for _, t := range members {
		players[t.SteamID64] = true
		maps[t.MapKey] = true
	}
	if len(members) < minLineupThrows || len(players) < minLineupPlayers || len(maps) < minLineupMaps {
		return Lineup{}, false
	}
	if len(members) < alwaysLineup && float64(len(members)) < minSpotShare*float64(spotThrows(c, throws, grid)) {
		return Lineup{}, false
	}
	recentFrom := now.AddDate(0, 0, -recentDays)
	side := majority(members, func(t Throw) string { return string(t.Side) })
	l := Lineup{
		Kind: c.kind, Side: tacticalplan.Side(side), Origin: round3(c.origin), Yaw: round2(c.yaw()), Pitch: round2(c.pitch), Landing: round3(c.landing),
		ThrowPlace: majority(members, func(t Throw) string { return t.ThrowPlace }), LandPlace: majority(members, func(t Throw) string { return t.LandPlace }),
		Movement: majority(members, func(t Throw) string { return t.Movement }), Jump: strings.HasSuffix(c.technique, "+jump"),
	}
	l.ID = lineupID(l)
	l.Slug, l.Title = names(l)
	l.Attempts, l.Landed = attempts(c, throws, grid)
	recent := []Throw{}
	matches, recentMatches, recentRounds, wonRounds := map[string]bool{}, map[string]bool{}, map[roundKey]bool{}, map[roundKey]bool{}
	airs, clocks := []float64{}, []float64{}
	for _, t := range members {
		matches[t.MapKey] = true
		airs = append(airs, t.AirTime)
		if t.PlayedAt.After(l.LastSeen) {
			l.LastSeen = t.PlayedAt
		}
		if t.PlayedAt.Before(recentFrom) {
			continue
		}
		recent = append(recent, t)
		recentMatches[t.MapKey] = true
		clocks = append(clocks, t.Clock)
		k := roundKey{t.MapKey, t.Round, t.Side}
		recentRounds[k] = true
		if t.RoundWon {
			wonRounds[k] = true
		}
	}
	l.Throws = Counts{Recent: len(recent), All: len(members)}
	l.Matches = Counts{Recent: len(recentMatches), All: len(matches)}
	l.AirTime = round2(median(airs))
	l.Clock = clockStats(clocks)
	l.Insta = len(clocks) > 0 && l.Clock.Median <= instaSeconds
	sideTotal := 0
	for k := range rounds.recent {
		if k.side == l.Side {
			sideTotal++
		}
	}
	l.RoundShare = ratio(len(recentRounds), sideTotal)
	l.RoundWin = ratio(len(wonRounds), len(recentRounds))
	l.Trend = trend(recent, rounds, l.Side, now)
	l.Teams = usage(perRound(recent), func(t Throw) string { return t.Team }, teamRounds(rounds, l.Side))
	l.TeamCount = len(distinct(members, func(t Throw) string { return t.Team }))
	l.Players = usage(recent, func(t Throw) string { return t.Player }, nil)
	l.Events = usage(recent, func(t Throw) string { return t.Event }, nil)
	l.PlantSites = usage(perRound(recent), func(t Throw) string { return string(t.PlantSite) }, nil)
	l.Examples = examples(members)
	return l, true
}

// lineupID is stable across rebuilds while the lineup's centre stays put.
func lineupID(l Lineup) string {
	jump := 0
	if l.Jump {
		jump = 1
	}
	return fmt.Sprintf("%s-%s-%d-%d-%d-%d-%d", l.Kind, strings.ToLower(string(l.Side)), int(math.Round(l.Origin[0]/32)), int(math.Round(l.Origin[1]/32)),
		int(math.Round(l.Landing[0]/64)), int(math.Round(l.Landing[1]/64)), jump)
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func round3(v [3]float64) [3]float64 {
	return [3]float64{math.Round(v[0]), math.Round(v[1]), math.Round(v[2])}
}

func ratio(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return round2(float64(n) / float64(d))
}

func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := slices.Clone(v)
	slices.Sort(s)
	if len(s)%2 == 1 {
		return s[len(s)/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}

func clockStats(clocks []float64) ClockStats {
	buckets := make([]int, int(math.Ceil(roundSeconds/10))+1)
	for _, c := range clocks {
		i := int(max(c, 0) / 10)
		if c > roundSeconds {
			i = len(buckets) - 1
		}
		buckets[min(i, len(buckets)-1)]++
	}
	return ClockStats{Median: round2(median(clocks)), Buckets: buckets}
}

func majority(ts []Throw, key func(Throw) string) string {
	counts := map[string]int{}
	best := ""
	for _, t := range ts {
		k := key(t)
		counts[k]++
		if counts[k] > counts[best] || (counts[k] == counts[best] && k < best) {
			best = k
		}
	}
	return best
}

func distinct(ts []Throw, key func(Throw) string) map[string]bool {
	out := map[string]bool{}
	for _, t := range ts {
		if k := key(t); k != "" {
			out[k] = true
		}
	}
	return out
}

// perRound keeps one throw per side round, so round outcomes count once.
func perRound(ts []Throw) []Throw {
	seen := map[roundKey]bool{}
	out := []Throw{}
	for _, t := range ts {
		k := roundKey{t.MapKey, t.Round, t.Side}
		if !seen[k] {
			seen[k] = true
			out = append(out, t)
		}
	}
	return out
}

// usage ranks the top keys; with per-key round totals, Detail is the share of
// that key's side rounds with the throw.
func usage(ts []Throw, key func(Throw) string, totals map[string]int) []Usage {
	counts := map[string]int{}
	for _, t := range ts {
		if k := key(t); k != "" {
			counts[k]++
		}
	}
	out := []Usage{}
	for k, n := range counts {
		u := Usage{Key: k, Name: k, Count: n, Share: ratio(n, len(ts))}
		if total := totals[k]; total > 0 {
			u.Detail = fmt.Sprintf("%.0f", 100*float64(n)/float64(total))
		}
		out = append(out, u)
	}
	slices.SortFunc(out, func(a, b Usage) int { return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(a.Key, b.Key)) })
	return out[:min(len(out), topEntries)]
}

// teamRounds counts each team's recent rounds on a side of the map.
func teamRounds(rounds roundTally, side tacticalplan.Side) map[string]int {
	out := map[string]int{}
	for k, info := range rounds.recent {
		if k.side == side && info.team != "" {
			out[info.team]++
		}
	}
	return out
}

// trend is throws per 100 side rounds in the last 30 days and in the rest of
// the recent window, so a rising or fading lineup shows.
func trend(recent []Throw, rounds roundTally, side tacticalplan.Side, now time.Time) [2]float64 {
	split := now.AddDate(0, 0, -trendDays)
	var newThrows, oldThrows, newRounds, oldRounds int
	for _, t := range recent {
		if t.PlayedAt.Before(split) {
			oldThrows++
		} else {
			newThrows++
		}
	}
	for k, info := range rounds.recent {
		if k.side != side {
			continue
		}
		if info.playedAt.Before(split) {
			oldRounds++
		} else {
			newRounds++
		}
	}
	per100 := func(n, d int) float64 {
		if d == 0 {
			return 0
		}
		return math.Round(1000*float64(n)/float64(d)) / 10
	}
	return [2]float64{per100(newThrows, newRounds), per100(oldThrows, oldRounds)}
}

func examples(members []Throw) []Example {
	s := slices.Clone(members)
	slices.SortFunc(s, func(a, b Throw) int { return cmp.Or(b.PlayedAt.Compare(a.PlayedAt), cmp.Compare(a.Round, b.Round)) })
	out := []Example{}
	for _, t := range s[:min(len(s), examplesPerItem)] {
		out = append(out, Example{Match: t.Match, MapKey: t.MapKey, Event: t.Event, PlayedAt: t.PlayedAt, Team: t.Team, Player: t.Player, Round: t.Round, Clock: round2(t.Clock), Tick: t.ThrowTick})
	}
	return out
}
