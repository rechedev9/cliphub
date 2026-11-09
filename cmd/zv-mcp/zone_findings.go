package main

import (
	"math"
	"sort"
	"strconv"

	"github.com/rechedev9/cliphub/internal/placename"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// maxLandingZones caps the landing zones listed per grenade kind.
const maxLandingZones = 5

var grenadeLabels = map[string]string{"smoke": "Humos", "flash": "Flashes", "he": "HE", "molotov": "Molotovs", "decoy": "Señuelos"}

type landingOut struct {
	Callout    string  `json:"callout" jsonschema:"CS2 place token"`
	Label      string  `json:"label"`
	Count      int     `json:"count"`
	Pct        float64 `json:"pct" jsonschema:"over the grenades of this kind"`
	StudioPath string  `json:"studio_path" jsonschema:"opens the rounds of these grenades in the Replay with the zone marked"`
}

// landingZones counts where one kind of grenade went off, by callout, and links
// each zone and the whole kind to their rounds with the zones marked.
func landingZones(kind string, gs []tacticalplan.TeamGrenade) ([]landingOut, string) {
	type acc struct {
		count int
		refs  []tuiclient.Evidence
	}
	byZone := map[string]*acc{}
	all := []tuiclient.Evidence{}
	for _, g := range gs {
		ref := tuiclient.Evidence{MatchID: g.MatchID.String(), Round: g.Round, Tick: g.Tick}
		all = append(all, ref)
		if g.LandPlace == "" {
			continue
		}
		a, ok := byZone[g.LandPlace]
		if !ok {
			a = &acc{}
			byZone[g.LandPlace] = a
		}
		a.count++
		a.refs = append(a.refs, ref)
	}
	zones := make([]string, 0, len(byZone))
	for z := range byZone {
		zones = append(zones, z)
	}
	sort.Slice(zones, func(i, j int) bool {
		if byZone[zones[i]].count != byZone[zones[j]].count {
			return byZone[zones[i]].count > byZone[zones[j]].count
		}
		return zones[i] < zones[j]
	})
	if len(zones) > maxLandingZones {
		zones = zones[:maxLandingZones]
	}
	out := []landingOut{}
	for _, z := range zones {
		a := byZone[z]
		label := placename.Label(z)
		link := findingLink{title: grenadeLabels[kind] + " en " + label, refs: a.refs, zones: []string{z}}
		out = append(out, landingOut{Callout: cleanName(z), Label: cleanName(label), Count: a.count,
			Pct: ratioPct(a.count, len(gs)), StudioPath: link.path()})
	}
	return out, findingLink{title: grenadeLabels[kind], refs: all, zones: zones}.path()
}

func ratioPct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return math.Round(float64(n)*1000/float64(total)) / 10
}

// openingTramo is the window player_positions reads, as the Replay's tramo writes it.
func openingTramo(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	return "0-" + strconv.Itoa(seconds)
}
