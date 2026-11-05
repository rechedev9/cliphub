package tacticalplan

import "sort"

// BombsiteReport is a team's record on each bombsite of each map, as defender
// and as attacker. Every rate carries its count and denominator (the n).
type BombsiteReport struct {
	Filter            ScoutFilter   `json:"filter"`
	MinReliableSample int           `json:"min_reliable_sample"`
	MatchCount        int           `json:"match_count"`
	Maps              []BombsiteMap `json:"maps"`
}

// BombsiteMap is one map: the team's rounds on each side and both sites.
type BombsiteMap struct {
	Map      string         `json:"map"`
	Matches  int            `json:"matches"`
	CTRounds int            `json:"ct_rounds"`
	TRounds  int            `json:"t_rounds"`
	Sites    []BombsiteSite `json:"sites"`
}

// BombsiteSite is one bombsite seen from both sides.
type BombsiteSite struct {
	Site Site            `json:"site"`
	CT   BombsiteDefense `json:"ct"`
	T    BombsiteAttack  `json:"t"`
}

// BombsiteDefense is the team defending a site. Hit is over the team's CT
// rounds on the map; Held, PlantsAllowed and Won are over the hit rounds;
// RetakesAttempted is over the plants and RetakesWon over the attempts.
type BombsiteDefense struct {
	Hit              Rate       `json:"hit"`
	Held             Rate       `json:"held"`
	PlantsAllowed    Rate       `json:"plants_allowed"`
	Won              Rate       `json:"won"`
	RetakesAttempted Rate       `json:"retakes_attempted"`
	RetakesWon       Rate       `json:"retakes_won"`
	Examples         []Evidence `json:"examples"`
}

// BombsiteAttack is the team attacking a site. Attacks is over the team's T
// rounds on the map; Plants and Won are over the attacks; PostPlantWon is
// over the plants.
type BombsiteAttack struct {
	Attacks      Rate       `json:"attacks"`
	Plants       Rate       `json:"plants"`
	Won          Rate       `json:"won"`
	PostPlantWon Rate       `json:"post_plant_won"`
	Examples     []Evidence `json:"examples"`
}

// targetSite is the bombsite the attackers went for: the plant site when the
// bomb went down, else the site the scan classified from the attackers'
// commit or the first duel. A save without a plant targets no site.
func targetSite(r Round) Site {
	if r.Bomb != nil && r.Bomb.PlantTick > 0 && isBombsite(r.Bomb.Site) {
		return r.Bomb.Site
	}
	if r.Class.TSide == TSave || !isBombsite(r.Class.Site) {
		return SiteNone
	}
	return r.Class.Site
}

func isBombsite(s Site) bool { return s == SiteA || s == SiteB }

func bombPlanted(r Round) bool { return r.Bomb != nil && r.Bomb.PlantTick > 0 }

type siteCounts struct {
	rounds, planted, won, plantedWon, retakes, retakesWon int
	examples                                              []Evidence
}

type bombsiteMapAcc struct {
	matches     map[string]bool
	sideRounds  map[Side]int
	perSiteSide map[Side]map[Site]*siteCounts
}

func newBombsiteMapAcc() *bombsiteMapAcc {
	acc := &bombsiteMapAcc{matches: map[string]bool{}, sideRounds: map[Side]int{}, perSiteSide: map[Side]map[Site]*siteCounts{}}
	for _, side := range []Side{SideCT, SideT} {
		acc.perSiteSide[side] = map[Site]*siteCounts{SiteA: {}, SiteB: {}}
	}
	return acc
}

func (a *bombsiteMapAcc) add(m ScoutMatch, r Round, side Side) {
	a.sideRounds[side]++
	site := targetSite(r)
	if !isBombsite(site) {
		return
	}
	c := a.perSiteSide[side][site]
	won := r.Winner == side
	c.rounds++
	c.examples = appendExample(c.examples, evidence(m, r, 0))
	if won {
		c.won++
	}
	if !bombPlanted(r) {
		return
	}
	c.planted++
	if won {
		c.plantedWon++
	}
	if side == SideCT && r.Class.CTSide == CTRetake {
		c.retakes++
		if won {
			c.retakesWon++
		}
	}
}

func (a *bombsiteMapAcc) finish(name string) BombsiteMap {
	out := BombsiteMap{Map: name, Matches: len(a.matches), CTRounds: a.sideRounds[SideCT], TRounds: a.sideRounds[SideT]}
	for _, site := range []Site{SiteA, SiteB} {
		ct, t := a.perSiteSide[SideCT][site], a.perSiteSide[SideT][site]
		out.Sites = append(out.Sites, BombsiteSite{
			Site: site,
			CT: BombsiteDefense{
				Hit:              newRate(ct.rounds, out.CTRounds),
				Held:             newRate(ct.won-ct.plantedWon, ct.rounds),
				PlantsAllowed:    newRate(ct.planted, ct.rounds),
				Won:              newRate(ct.won, ct.rounds),
				RetakesAttempted: newRate(ct.retakes, ct.planted),
				RetakesWon:       newRate(ct.retakesWon, ct.retakes),
				Examples:         nonNilEvidence(ct.examples),
			},
			T: BombsiteAttack{
				Attacks:      newRate(t.rounds, out.TRounds),
				Plants:       newRate(t.planted, t.rounds),
				Won:          newRate(t.won, t.rounds),
				PostPlantWon: newRate(t.plantedWon, t.planted),
				Examples:     nonNilEvidence(t.examples),
			},
		})
	}
	return out
}

func nonNilEvidence(list []Evidence) []Evidence {
	if list == nil {
		return []Evidence{}
	}
	return list
}

// Bombsites builds the per-map, per-site record of a team over its matches,
// with the rounds the filter keeps. Rounds without a winner are left out.
func Bombsites(matches []ScoutMatch, f ScoutFilter) BombsiteReport {
	selected := f.Select(matches)
	report := BombsiteReport{Filter: f, MinReliableSample: MinReliableSample, MatchCount: len(selected), Maps: []BombsiteMap{}}
	maps := map[string]*bombsiteMapAcc{}
	for _, m := range selected {
		doc := m.Document
		acc, ok := maps[doc.Demo.Map]
		if !ok {
			acc = newBombsiteMapAcc()
			maps[doc.Demo.Map] = acc
		}
		acc.matches[m.MatchID.String()] = true
		for _, r := range f.rounds(m) {
			side := doc.TeamSide(m.TeamKey, r)
			if side == "" || r.Winner == "" {
				continue
			}
			acc.add(m, r, side)
		}
	}
	for name, acc := range maps {
		report.Maps = append(report.Maps, acc.finish(name))
	}
	sort.Slice(report.Maps, func(i, j int) bool {
		a, b := report.Maps[i], report.Maps[j]
		if a.CTRounds+a.TRounds != b.CTRounds+b.TRounds {
			return a.CTRounds+a.TRounds > b.CTRounds+b.TRounds
		}
		return a.Map < b.Map
	})
	return report
}
