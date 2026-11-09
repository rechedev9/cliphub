package tacticalplan

import (
	"fmt"
	"math"
	"sort"

	"github.com/rechedev9/cliphub/internal/placename"
	"github.com/rechedev9/cliphub/internal/radarmap"
)

// AttackZone is a zone the team controlled in a default's rounds. Share is
// over the default's rounds.
type AttackZone struct {
	Zone  string `json:"zone"`
	Label string `json:"label"`
	PrepRounds
}

// AttackDefault is a spread the team stood in at AttackReadSeconds of a round it
// did not rush or execute: "2-1-2" is two players towards A, one mid and two
// towards B. Share is over the framed T rounds.
type AttackDefault struct {
	Formation string `json:"formation"`
	PrepRounds
	Zones []AttackZone `json:"zones"`
}

// AttackExecute is one way the team took a site on a buy matchup: the route most
// of its first players on site came through, "" when it could not be read or the
// round split. Share is over the matchup's rounds.
type AttackExecute struct {
	Buy         string `json:"buy"`
	OpponentBuy string `json:"opponent_buy"`
	Site        string `json:"site"`
	Route       string `json:"route"`
	RouteLabel  string `json:"route_label"`
	PrepRounds
	Rushes         Rate    `json:"rushes"`
	Plants         Rate    `json:"plants"`
	ContactSeconds float64 `json:"contact_seconds"`
}

// AttackSpot is a callout a T player held most in a window, with the median
// second of the round they first stood there.
type AttackSpot struct {
	PrepSpot
	ArrivalSeconds float64 `json:"arrival_seconds"`
}

// AttackPace is how a player moved in a window, by their mean speed over it:
// held still, walked or ran. Rates are over the window's paced rounds.
type AttackPace struct {
	Static Rate `json:"static"`
	Slow   Rate `json:"slow"`
	Fast   Rate `json:"fast"`
}

// AttackPlayerWindow is where a T player stood most in one window and how they moved.
type AttackPlayerWindow struct {
	PrepWindow
	Rounds int          `json:"rounds"`
	Spots  []AttackSpot `json:"spots"`
	Pace   AttackPace   `json:"pace"`
}

// AttackPlayer is one T player of the team and their positions per window.
type AttackPlayer struct {
	SteamID64 string               `json:"steamid64"`
	Name      string               `json:"name"`
	Role      string               `json:"role"`
	Rounds    int                  `json:"rounds"`
	Windows   []AttackPlayerWindow `json:"windows"`
}

type defaultAcc struct {
	prepGroup
	zones map[string]*prepGroup
}

// addDefault reads the round's formation at AttackReadSeconds; a map without
// both site centres, or a round that ended before, has none.
func (run *attackRun) addDefault(ar attackRound) {
	zones := siteZones(ar.m.Document)
	a, okA := zones[ZoneA]
	b, okB := zones[ZoneB]
	if !okA || !okB {
		return
	}
	for _, f := range ar.read.track {
		if f.seconds < AttackReadSeconds {
			continue
		}
		if len(f.pos) == 0 {
			return
		}
		var spread [3]int
		for _, at := range f.pos {
			dA := math.Hypot(at[0]-a.center[0], at[1]-a.center[1])
			dB := math.Hypot(at[0]-b.center[0], at[1]-b.center[1])
			switch lean := dA / math.Max(dA+dB, 1); {
			case lean < AttackLeanRatio:
				spread[0]++
			case lean > 1-AttackLeanRatio:
				spread[2]++
			default:
				spread[1]++
			}
		}
		key := fmt.Sprintf("%d-%d-%d", spread[0], spread[1], spread[2])
		acc, ok := run.defaults[key]
		if !ok {
			acc = &defaultAcc{zones: map[string]*prepGroup{}}
			run.defaults[key] = acc
		}
		acc.add(ar.won, evidence(ar.m.ScoutMatch, ar.r, f.tick))
		for _, hold := range ar.read.zones {
			g, ok := acc.zones[hold.Zone]
			if !ok {
				g = &prepGroup{}
				acc.zones[hold.Zone] = g
			}
			g.add(ar.won, ar.start)
		}
		return
	}
}

func (run *attackRun) defaultRows(framed int) []AttackDefault {
	out := make([]AttackDefault, 0, len(run.defaults))
	for key, acc := range run.defaults {
		zones := make([]AttackZone, 0, len(acc.zones))
		for zone, g := range acc.zones {
			zones = append(zones, AttackZone{Zone: zone, Label: placename.Label(zone), PrepRounds: g.result(acc.rounds)})
		}
		sort.Slice(zones, func(i, j int) bool {
			if zones[i].Rounds != zones[j].Rounds {
				return zones[i].Rounds > zones[j].Rounds
			}
			return zones[i].Zone < zones[j].Zone
		})
		if len(zones) > AttackZoneLimit {
			zones = zones[:AttackZoneLimit]
		}
		out = append(out, AttackDefault{Formation: key, PrepRounds: acc.result(framed), Zones: zones})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Rounds != out[j].Rounds {
			return out[i].Rounds > out[j].Rounds
		}
		return out[i].Formation < out[j].Formation
	})
	return out
}

type attackExecuteKey struct {
	band  bandKey
	site  string
	route string
}

type attackExecuteAcc struct {
	prepGroup
	rushes, plants int
	contact        []float64
}

func (run *attackRun) addExecute(ar attackRound) {
	key := attackExecuteKey{band: ar.band, site: ar.site, route: entryRoute(ar.read, ar.site)}
	acc, ok := run.executes[key]
	if !ok {
		acc = &attackExecuteAcc{}
		run.executes[key] = acc
	}
	acc.add(ar.won, ar.start)
	if ar.tempo == TempoRush {
		acc.rushes++
	}
	if ar.r.Bomb != nil && ar.r.Bomb.PlantTick > 0 {
		acc.plants++
	}
	if ar.read.hasTempo {
		acc.contact = append(acc.contact, ar.read.tempo)
	}
}

// entryRoute is the callout most of the first players on the site stood in
// last before stepping onto it; a tie goes to the first name.
func entryRoute(read playbookRead, site string) string {
	target := Site(site)
	tick, ok := read.presence[target]
	if !isBombsite(target) || !ok {
		return ""
	}
	at := -1
	for i, f := range read.track {
		if f.tick == tick {
			at = i
			break
		}
	}
	if at < 0 {
		return ""
	}
	votes := map[string]int{}
	for slot, callout := range read.track[at].callouts {
		if SiteFromPlace(callout) != target {
			continue
		}
		for i := at - 1; i >= 0; i-- {
			if c, ok := read.track[i].callouts[slot]; ok && SiteFromPlace(c) != target {
				votes[c]++
				break
			}
		}
	}
	best := ""
	for route, n := range votes {
		if best == "" || n > votes[best] || (n == votes[best] && route < best) {
			best = route
		}
	}
	return best
}

func (run *attackRun) executeRows() []AttackExecute {
	byBand := map[bandKey][]AttackExecute{}
	for key, acc := range run.executes {
		label := ""
		if key.route != "" {
			label = placename.Label(key.route)
		}
		byBand[key.band] = append(byBand[key.band], AttackExecute{
			Buy: key.band.buy, OpponentBuy: key.band.opponent, Site: key.site, Route: key.route, RouteLabel: label,
			PrepRounds: acc.result(run.matchups[key.band].rounds), Rushes: newRate(acc.rushes, acc.rounds),
			Plants: newRate(acc.plants, acc.rounds), ContactSeconds: median(acc.contact),
		})
	}
	bands := make([]bandKey, 0, len(byBand))
	for k := range byBand {
		bands = append(bands, k)
	}
	sort.Slice(bands, func(i, j int) bool { return bands[i].less(bands[j]) })
	out := []AttackExecute{}
	for _, k := range bands {
		rows := byBand[k]
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].Rounds != rows[j].Rounds {
				return rows[i].Rounds > rows[j].Rounds
			}
			if rows[i].Site != rows[j].Site {
				return rows[i].Site < rows[j].Site
			}
			return rows[i].Route < rows[j].Route
		})
		if len(rows) > AttackExecuteLimit {
			rows = rows[:AttackExecuteLimit]
		}
		out = append(out, rows...)
	}
	return out
}

type attackSpotAcc struct {
	prepSpotAcc
	arrival []float64
}

type attackWindowAcc struct {
	rounds, paced, static, slow, fast int
	spots                             map[string]*attackSpotAcc
}

type attackPlayerAcc struct {
	name    string
	rounds  int
	windows []*attackWindowAcc
}

func (run *attackRun) player(id, name string) *attackPlayerAcc {
	acc, ok := run.players[id]
	if !ok {
		acc = &attackPlayerAcc{name: name, windows: make([]*attackWindowAcc, len(PrepWindows))}
		for i := range acc.windows {
			acc.windows[i] = &attackWindowAcc{spots: map[string]*attackSpotAcc{}}
		}
		run.players[id] = acc
	}
	return acc
}

// addPlayers reads, per T player and window, the callout of most of their alive
// frames (a tie goes to the callout reached first) and their mean speed.
func (run *attackRun) addPlayers(ar attackRound) {
	track := ar.read.track
	for slot := range ar.read.team {
		id, name := playerIdentity(ar.m.Document, slot)
		seen := false
		for i, w := range PrepWindows {
			best, firstTick, pace := windowSpot(track, slotWindow{slot: slot, w: w})
			if best == "" {
				continue
			}
			acc := run.player(id, name)
			seen = true
			win := acc.windows[i]
			win.rounds++
			spot, ok := win.spots[best]
			if !ok {
				spot = &attackSpotAcc{}
				win.spots[best] = spot
			}
			spot.add(false, evidence(ar.m.ScoutMatch, ar.r, firstTick))
			for _, f := range track {
				if f.callouts[slot] == best && w.contains(f.seconds) {
					at := f.pos[slot]
					for k := range at {
						spot.sum[k] += at[k]
					}
					spot.samples++
				}
			}
			for _, f := range track {
				if f.callouts[slot] == best {
					spot.arrival = append(spot.arrival, f.seconds)
					break
				}
			}
			if pace >= 0 {
				win.paced++
				switch {
				case pace < AttackStaticSpeed:
					win.static++
				case pace < AttackFastSpeed:
					win.slow++
				default:
					win.fast++
				}
			}
		}
		if seen {
			run.players[id].rounds++
		}
	}
}

type slotWindow struct {
	slot uint8
	w    PrepWindow
}

// windowSpot is the slot's most held callout in the window, the tick it first
// stood there, and its mean speed over the window, or -1 under a second of frames.
func windowSpot(track []teamFrame, at slotWindow) (string, int, float64) {
	slot := at.slot
	counts := map[string]int{}
	firstTick := map[string]int{}
	best := ""
	var path, from, to float64
	var last [3]float64
	moved := false
	for _, f := range track {
		callout, ok := f.callouts[slot]
		if !ok || !at.w.contains(f.seconds) {
			continue
		}
		counts[callout]++
		if _, ok := firstTick[callout]; !ok {
			firstTick[callout] = f.tick
		}
		if best == "" || counts[callout] > counts[best] {
			best = callout
		}
		pos := f.pos[slot]
		if moved {
			path += math.Hypot(pos[0]-last[0], pos[1]-last[1])
		} else {
			from = f.seconds
		}
		last, to, moved = pos, f.seconds, true
	}
	if to-from < 1 {
		return best, firstTick[best], -1
	}
	return best, firstTick[best], path / (to - from)
}

func (run *attackRun) playerRows(roles map[string]string, mapName string) []AttackPlayer {
	cal, calibrated := radarmap.Lookup(mapName)
	out := make([]AttackPlayer, 0, len(run.players))
	for id, acc := range run.players {
		role := roles[id]
		if role == "" {
			role = "unknown"
		}
		p := AttackPlayer{SteamID64: id, Name: acc.name, Role: role, Rounds: acc.rounds, Windows: make([]AttackPlayerWindow, 0, len(PrepWindows))}
		for i, w := range PrepWindows {
			win := acc.windows[i]
			spots := make([]AttackSpot, 0, len(win.spots))
			for callout, g := range win.spots {
				spot := ScoutSpot{Callout: callout, Share: newRate(g.rounds, win.rounds), Examples: g.examples}
				row := PrepSpot{ScoutSpot: spot, Refs: g.allRefs(), Pos: g.centroid()}
				if calibrated && g.samples > 0 && cal.Level(row.Pos[2]) == radarmap.LevelLower {
					row.Floor = radarmap.LevelLower
				}
				spots = append(spots, AttackSpot{PrepSpot: row, ArrivalSeconds: median(g.arrival)})
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
			pace := AttackPace{Static: newRate(win.static, win.paced), Slow: newRate(win.slow, win.paced), Fast: newRate(win.fast, win.paced)}
			p.Windows = append(p.Windows, AttackPlayerWindow{PrepWindow: w, Rounds: win.rounds, Spots: spots, Pace: pace})
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
