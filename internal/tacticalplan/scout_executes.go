package tacticalplan

import "math"

// ScoutExecute is the executes and splits on one bombsite.
type ScoutExecute struct {
	Site Site `json:"site"`
	ScoutRounds
	Patterns []ScoutPatternRow `json:"patterns"`
	// Refs holds every execute round, Examples only the first ones.
	Refs         []Evidence `json:"refs"`
	Plants       Rate       `json:"plants"`
	PlantTime    Histogram  `json:"plant_time"`
	FirstContact Histogram  `json:"first_contact"`
	// Mean grenades of each kind the team threw inside the execute window.
	MeanUtility map[EventKind]float64 `json:"mean_utility"`
	Utility     []ScoutUtilityRow     `json:"utility"`
}

type executeAcc struct {
	all          roundGroup
	refs         []Evidence
	patterns     *patternGroups
	plants       int
	plantTime    []float64
	firstContact []float64
	utility      *utilityTable
	kindTotals   map[EventKind]int
}

func newExecuteAcc() *executeAcc {
	return &executeAcc{patterns: newPatternGroups(), utility: newUtilityTable(), kindTotals: map[EventKind]int{}}
}

func (e *executeAcc) add(m ScoutMatch, r Round, won bool, team map[uint8]bool) {
	doc := m.Document
	anchor := r.Class.FirstContactTick
	if r.Bomb != nil && r.Bomb.PlantTick > 0 {
		anchor = r.Bomb.PlantTick
		e.plants++
		if secs, ok := secondsSince(r, anchor, doc.Demo.Tickrate); ok {
			e.plantTime = append(e.plantTime, secs)
		}
	}
	if secs, ok := secondsSince(r, r.Class.FirstContactTick, doc.Demo.Tickrate); ok && r.Class.FirstContactTick > 0 {
		e.firstContact = append(e.firstContact, secs)
	}
	example := evidence(m, r, anchor)
	e.refs = append(e.refs, example)
	e.all.add(won, example)
	e.patterns.add(SideT, string(r.Class.TSide), r.Class.Site, won, example)
	if anchor <= 0 {
		anchor = r.TickEnd
	}
	if doc.Demo.Tickrate > 0 {
		from := max(anchor-int(ScoutExecuteWindowSeconds*doc.Demo.Tickrate), r.TickFreezeEnd)
		for kind, n := range e.utility.addRound(m, r, team, from, anchor) {
			e.kindTotals[kind] += n
		}
	}
}

func finishExecutes(sites map[Site]*executeAcc) []ScoutExecute {
	total := 0
	for _, acc := range sites {
		total += acc.all.rounds
	}
	out := []ScoutExecute{}
	for _, site := range []Site{SiteA, SiteB} {
		acc, ok := sites[site]
		if !ok {
			continue
		}
		mean := map[EventKind]float64{}
		for _, kind := range []EventKind{EventSmoke, EventFlash, EventMolotov, EventHE} {
			mean[kind] = math.Round(float64(acc.kindTotals[kind])/float64(acc.all.rounds)*10) / 10
		}
		out = append(out, ScoutExecute{
			Site:         site,
			ScoutRounds:  acc.all.result(total),
			Patterns:     acc.patterns.rows(),
			Refs:         acc.refs,
			Plants:       newRate(acc.plants, acc.all.rounds),
			PlantTime:    newHistogram(acc.plantTime),
			FirstContact: newHistogram(acc.firstContact),
			MeanUtility:  mean,
			Utility:      acc.utility.result(acc.all.rounds),
		})
	}
	return out
}
