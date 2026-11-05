package tacticalplan

import (
	"math"
	"sort"
	"strings"
)

// A team controls a zone (a callout) when MapControlPlayers alive players hold it
// for MapControlSeconds unbroken, within MapControlWindowSeconds of freeze end.
const (
	MapControlPlayers       = 2
	MapControlSeconds       = 5
	MapControlWindowSeconds = 40
)

// ZoneHold is one zone a team controlled in a round: when the hold started,
// how long its longest unbroken stretch lasted, and the most players in it.
type ZoneHold struct {
	Zone        string  `json:"zone"`
	FromSeconds float64 `json:"from_seconds"`
	Seconds     float64 `json:"seconds"`
	Players     int     `json:"players"`
}

// isSpawnZone reports a spawn callout. Every team stands there together at the
// start of each round, which is not taking anything.
func isSpawnZone(callout string) bool {
	return strings.Contains(strings.ToLower(callout), "spawn")
}

type zoneRun struct {
	startSecs, lastSecs, best, firstFrom float64
	held                                 bool
	players                              int
}

// controlledZones reads the zones a team held from its track, earliest first.
// A missing frame does not break a run; a frame with fewer players does.
func controlledZones(track []teamFrame) []ZoneHold {
	runs := map[string]*zoneRun{}
	active := map[string]bool{}
	for _, f := range track {
		if f.seconds > MapControlWindowSeconds {
			break
		}
		counts := f.counts()
		for zone, n := range counts {
			if n < MapControlPlayers || isSpawnZone(zone) {
				continue
			}
			run, ok := runs[zone]
			if !ok {
				run = &zoneRun{}
				runs[zone] = run
			}
			if !active[zone] {
				run.startSecs = f.seconds
				active[zone] = true
			}
			run.lastSecs = f.seconds
			run.players = max(run.players, n)
			length := run.lastSecs - run.startSecs
			if length >= MapControlSeconds && !run.held {
				run.held, run.firstFrom = true, run.startSecs
			}
			run.best = math.Max(run.best, length)
		}
		for zone := range active {
			if counts[zone] < MapControlPlayers {
				delete(active, zone)
			}
		}
	}
	out := []ZoneHold{}
	for zone, run := range runs {
		if !run.held {
			continue
		}
		out = append(out, ZoneHold{
			Zone: zone, FromSeconds: math.Round(run.firstFrom*10) / 10, Seconds: math.Round(run.best*10) / 10, Players: run.players,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FromSeconds != out[j].FromSeconds {
			return out[i].FromSeconds < out[j].FromSeconds
		}
		return out[i].Zone < out[j].Zone
	})
	return out
}

// holdsMapZone reports a hold outside the bombsites, which is map control
// rather than a site take.
func holdsMapZone(holds []ZoneHold) bool {
	for _, h := range holds {
		if !isBombsite(SiteFromPlace(h.Zone)) {
			return true
		}
	}
	return false
}
