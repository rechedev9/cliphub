package main

import (
	"cmp"
	"fmt"
	"math"
	"net/url"
	"slices"
	"strconv"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	maxGrenadeSpots = 10
	// spotAfterFacts is how many follow-up facts a spot row names; the detail names them all.
	spotAfterFacts  = 3
	clockBinSeconds = 10
	// replayLeadSeconds opens a replay this long before the throw, as the web's spotReplayRefs.
	replayLeadSeconds = 3
	defaultTickrate   = 64
	// afterPlantSpan names the clock bin of the throws after the bomb went down.
	afterPlantSpan = "Tras la plant"
)

// spotRead is what readSpots needs beyond the kept grenades.
type spotRead struct {
	in    teamGrenadesIn
	limit int
	// names maps a callout token to the team's name for it.
	names map[string]string
}

type clockPlayerOut struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name" jsonschema:"untrusted player name from the demo"`
	Throws    int    `json:"throws"`
}

type clockBinOut struct {
	Span    string           `json:"span" jsonschema:"the round clock span, such as 1:55-1:45"`
	Throws  int              `json:"throws"`
	Players []clockPlayerOut `json:"players"`
}

type followUpOut struct {
	Kind   string `json:"kind" jsonschema:"play (the round's playbook category and site), fight (where the team's first duel after it was) or zone (a callout a player stayed in for 3 s)"`
	Value  string `json:"value" jsonschema:"callout, or category@site for a play; untrusted callout names"`
	Rounds int    `json:"rounds" jsonschema:"of the spot's rounds"`
	Won    int    `json:"won"`
}

type grenadeSpotOut struct {
	Key        string        `json:"key"`
	Kind       string        `json:"kind"`
	LandPlace  string        `json:"land_place" jsonschema:"untrusted callout name"`
	ThrowPlace string        `json:"throw_place,omitempty" jsonschema:"untrusted callout name"`
	Rounds     int           `json:"rounds" jsonschema:"rounds it was thrown in, over the response's rounds"`
	Throws     int           `json:"throws"`
	Won        int           `json:"won" jsonschema:"of its rounds"`
	Throwers   []throwerOut  `json:"throwers"`
	After      []followUpOut `json:"after" jsonschema:"what the team did in the 20 s after it, each over the spot's rounds; the most common first"`
}

type spotRoundOut struct {
	Ref       string   `json:"ref" jsonschema:"match_id/round"`
	Thrower   string   `json:"thrower" jsonschema:"untrusted player name from the demo"`
	Seconds   float64  `json:"seconds" jsonschema:"when it went off, seconds after freeze end"`
	ThrowTick *int     `json:"throw_tick,omitempty"`
	Won       bool     `json:"won"`
	Play      string   `json:"play,omitempty" jsonschema:"the round's category@site"`
	Fight     string   `json:"fight,omitempty"`
	Zones     []string `json:"zones"`
}

type grenadeDetailOut struct {
	grenadeSpotOut
	Setpos     string         `json:"setpos,omitempty" jsonschema:"console line that stands you where the throw nearest the spot's centre was thrown, looking the same way; absent for older analyses"`
	RoundList  []spotRoundOut `json:"round_list" jsonschema:"the rounds it was thrown in, newest match first"`
	StudioPath string         `json:"studio_path" jsonschema:"opens every round of the spot superposed in Studio's Replay, from the throw"`
}

type invertedRoundOut struct {
	Ref  string `json:"ref" jsonschema:"match_id/round"`
	Won  bool   `json:"won"`
	Play string `json:"play,omitempty" jsonschema:"the round's category@site"`
}

// readSpots adds the clock, the spots, the spot read in full and the inverted rounds.
func readSpots(out *teamGrenadesOut, kept tacticalplan.TeamGrenades, sr spotRead) {
	in, limit := sr.in, sr.limit
	out.Clock = clockBins(kept.Grenades, kept.RoundList)
	spots := tacticalplan.GrenadeSpots(kept.Grenades, kept.RoundList)
	out.Spots = make([]grenadeSpotOut, 0, min(len(spots), maxGrenadeSpots))
	for _, s := range spots[:min(len(spots), maxGrenadeSpots)] {
		out.Spots = append(out.Spots, grenadeSpotRow(s, spotAfterFacts))
	}
	matching := kept.Grenades
	if in.Spot != "" {
		matching = slices.DeleteFunc(slices.Clone(kept.Grenades), func(g tacticalplan.TeamGrenade) bool { return g.Spot != in.Spot })
		for _, s := range spots {
			if s.Key == in.Spot {
				detail := spotDetail(s, matching, spotContext{rounds: kept.RoundList, names: sr.names})
				out.Spot = &detail
			}
		}
	}
	thrown := map[string]bool{}
	for _, g := range matching {
		thrown[roundRef(g.MatchID.String(), g.Round)] = true
	}
	out.MatchingRounds = len(thrown)
	if !in.Invert {
		return
	}
	out.MatchingRounds = 0
	out.Inverted = []invertedRoundOut{}
	for _, r := range kept.RoundList {
		ref := roundRef(r.MatchID.String(), r.Round)
		if thrown[ref] {
			continue
		}
		out.MatchingRounds++
		if len(out.Inverted) < limit {
			out.Inverted = append(out.Inverted, invertedRoundOut{Ref: ref, Won: r.Won, Play: r.Play()})
		}
	}
}

func grenadeSpotRow(s tacticalplan.GrenadeSpot, facts int) grenadeSpotOut {
	out := grenadeSpotOut{Key: cleanName(s.Key), Kind: string(s.Kind), LandPlace: cleanName(s.LandPlace), ThrowPlace: cleanName(s.ThrowPlace),
		Rounds: s.FollowUp.Rounds, Throws: s.Throws, Won: s.FollowUp.Won, Throwers: []throwerOut{}, After: []followUpOut{}}
	for _, t := range s.Throwers[:min(len(s.Throwers), maxGrenadeThrowers)] {
		out.Throwers = append(out.Throwers, throwerOut{SteamID64: cleanName(t.SteamID64), Name: cleanName(t.Name), Count: t.Throws})
	}
	for _, f := range s.FollowUp.Facts[:min(len(s.FollowUp.Facts), facts)] {
		out.After = append(out.After, followUpOut{Kind: f.Kind, Value: cleanName(f.Value), Rounds: f.Rounds, Won: f.Won})
	}
	return out
}

type spotContext struct {
	rounds []tacticalplan.GrenadeRound
	names  map[string]string
}

func spotDetail(s tacticalplan.GrenadeSpot, grenades []tacticalplan.TeamGrenade, sc spotContext) grenadeDetailOut {
	out := grenadeDetailOut{grenadeSpotOut: grenadeSpotRow(s, len(s.FollowUp.Facts)), RoundList: []spotRoundOut{}}
	byRef := map[string]tacticalplan.GrenadeRound{}
	for _, r := range sc.rounds {
		byRef[roundRef(r.MatchID.String(), r.Round)] = r
	}
	v := url.Values{}
	seen := map[string]bool{}
	for _, g := range grenades {
		ref := roundRef(g.MatchID.String(), g.Round)
		if seen[ref] {
			continue
		}
		seen[ref] = true
		r := byRef[ref]
		out.RoundList = append(out.RoundList, spotRoundOut{Ref: ref, Thrower: cleanName(g.ThrowerName), Seconds: g.Seconds, ThrowTick: g.ThrowTick,
			Won: g.RoundWon, Play: r.Play(), Fight: cleanName(g.After.Fight), Zones: cleanNames(g.After.Zones)})
		if len(out.RoundList) <= maxPlaylistRounds {
			tick := g.Tick
			if g.ThrowTick != nil {
				tick = *g.ThrowTick
			}
			v.Add("r", g.MatchID.String()+":"+strconv.Itoa(g.Round)+":"+strconv.Itoa(max(0, tick-leadTicks(g.Tickrate))))
		}
	}
	v.Set("t", fmt.Sprintf("%s · %d rondas", spotTitle(s, sc.names), min(len(out.RoundList), maxPlaylistRounds)))
	v.Set("s", "1")
	out.StudioPath = "/replay?" + v.Encode()
	if g, ok := representativeThrow(grenades); ok {
		out.Setpos = setposCommand(tuiclient.Lineup{Origin: *g.Origin, Pitch: tacticalplan.SetangPitch(g.View[1]), Yaw: tacticalplan.SetangYaw(g.View[0])})
	}
	return out
}

// grenadeKindLabel mirrors GRENADE_KIND_LABEL in the web, since the title shows in the Replay.
var grenadeKindLabel = map[tacticalplan.EventKind]string{
	tacticalplan.EventSmoke: "Humo", tacticalplan.EventFlash: "Flash", tacticalplan.EventHE: "HE",
	tacticalplan.EventMolotov: "Molotov", tacticalplan.EventDecoy: "Señuelo",
}

// leadTicks is replayLeadSeconds at the demo's tick rate.
func leadTicks(tickrate float64) int {
	if tickrate <= 0 {
		tickrate = defaultTickrate
	}
	return int(math.Round(replayLeadSeconds * tickrate))
}

// spotTitle names a spot as the web's spotLabel does, with the team's callout
// names (the web's calloutLabel): "Humo a Ramp desde T Spawn".
func spotTitle(s tacticalplan.GrenadeSpot, names map[string]string) string {
	label := func(token string) string { return cleanName(cmp.Or(names[token], token)) }
	title := grenadeKindLabel[s.Kind]
	if s.LandPlace != "" {
		title += " a " + label(s.LandPlace)
	}
	if s.ThrowPlace != "" {
		title += " desde " + label(s.ThrowPlace)
	}
	return title
}

// representativeThrow is the recorded throw nearest the spot's mean origin, so the setpos is a throw that happened.
func representativeThrow(grenades []tacticalplan.TeamGrenade) (tacticalplan.TeamGrenade, bool) {
	var x, y float64
	n := 0
	for _, g := range grenades {
		if g.Origin != nil && g.View != nil {
			x, y, n = x+g.Origin[0], y+g.Origin[1], n+1
		}
	}
	if n == 0 {
		return tacticalplan.TeamGrenade{}, false
	}
	x, y = x/float64(n), y/float64(n)
	best, bestDist := tacticalplan.TeamGrenade{}, math.Inf(1)
	for _, g := range grenades {
		if g.Origin == nil || g.View == nil {
			continue
		}
		if d := math.Hypot(g.Origin[0]-x, g.Origin[1]-y); d < bestDist {
			best, bestDist = g, d
		}
	}
	return best, true
}

// clockBins counts throws per clockBinSeconds of the round clock, as the web's
// clockBins; throws after the plant go in a last "Tras la plant" bin.
func clockBins(grenades []tacticalplan.TeamGrenade, rounds []tacticalplan.GrenadeRound) []clockBinOut {
	bins := int(math.Ceil(float64(roundClockSeconds) / clockBinSeconds))
	plants := map[string]float64{}
	for _, r := range rounds {
		if r.PlantSeconds != nil {
			plants[roundRef(r.MatchID.String(), r.Round)] = *r.PlantSeconds
		}
	}
	counts := make([]map[string]*clockPlayerOut, bins+1)
	for _, g := range grenades {
		i := min(bins-1, max(0, int(g.Seconds/clockBinSeconds)))
		if plant, ok := plants[roundRef(g.MatchID.String(), g.Round)]; ok && g.Seconds >= plant {
			i = bins
		}
		if counts[i] == nil {
			counts[i] = map[string]*clockPlayerOut{}
		}
		p, ok := counts[i][g.ThrowerSteamID64]
		if !ok {
			p = &clockPlayerOut{SteamID64: cleanName(g.ThrowerSteamID64), Name: cleanName(g.ThrowerName)}
			counts[i][g.ThrowerSteamID64] = p
		}
		p.Throws++
	}
	out := []clockBinOut{}
	for i, players := range counts {
		if players == nil {
			continue
		}
		span := afterPlantSpan
		if i < bins {
			span = roundClock(i*clockBinSeconds) + "-" + roundClock((i+1)*clockBinSeconds)
		}
		bin := clockBinOut{Span: span, Players: []clockPlayerOut{}}
		for _, p := range players {
			bin.Throws += p.Throws
			bin.Players = append(bin.Players, *p)
		}
		slices.SortFunc(bin.Players, func(a, b clockPlayerOut) int {
			return cmp.Or(cmp.Compare(b.Throws, a.Throws), cmp.Compare(a.SteamID64, b.SteamID64))
		})
		out = append(out, bin)
	}
	return out
}
