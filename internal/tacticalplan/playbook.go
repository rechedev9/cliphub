package tacticalplan

import (
	"sort"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/placename"
)

// Playbook categories. Each round gets exactly one, tried in the order of
// playbookCategories, so the shares of a side add up to the whole.
const (
	PlaybookSave       = "save"
	PlaybookFake       = "fake"
	PlaybookRush       = "rush"
	PlaybookExecute    = "execute"
	PlaybookMapControl = "map_control"
	PlaybookStack      = "stack"
	PlaybookAggression = "aggression"
	PlaybookRetake     = "retake"
	PlaybookDefault    = "default"
)

// Playbook rules, beside the map control and site contact rules.
const (
	// PlaybookRushSeconds is the latest site contact of a rush.
	PlaybookRushSeconds = 25
	// PlaybookFakeUtility is the grenades landing on the other bombsite before
	// the team reached the site it played that make the round a fake.
	PlaybookFakeUtility = 2
)

// playbookCategories lists each side's categories in priority order.
var playbookCategories = map[Side][]string{
	SideT:  {PlaybookSave, PlaybookFake, PlaybookRush, PlaybookExecute, PlaybookMapControl, PlaybookDefault},
	SideCT: {PlaybookSave, PlaybookStack, PlaybookAggression, PlaybookRetake, PlaybookDefault},
}

// PlaybookThresholds are the rules the playbook applied.
type PlaybookThresholds struct {
	RushSeconds             int `json:"rush_seconds"`
	FakeUtility             int `json:"fake_utility"`
	SiteContactPlayers      int `json:"site_contact_players"`
	MapControlPlayers       int `json:"map_control_players"`
	MapControlSeconds       int `json:"map_control_seconds"`
	MapControlWindowSeconds int `json:"map_control_window_seconds"`
	TempoBucketSeconds      int `json:"tempo_bucket_seconds"`
}

// PlaybookGroup is a set of rounds with every round's ref, so the UI can open
// each one in the replay.
type PlaybookGroup struct {
	Rounds  int        `json:"rounds"`
	Share   Rate       `json:"share"`
	WinRate Rate       `json:"win_rate"`
	Refs    []Evidence `json:"refs"`
}

// PlaybookCategory is the rounds of one category.
type PlaybookCategory struct {
	Category string `json:"category"`
	PlaybookGroup
}

// PlaybookSite is the rounds played on one site. Share is over every round.
type PlaybookSite struct {
	Site Site `json:"site"`
	PlaybookGroup
}

// PlaybookZone is the rounds in which the team controlled one zone. Share is
// over the rounds with positions, the only ones a hold can be read from.
type PlaybookZone struct {
	Zone  string `json:"zone"`
	Label string `json:"label"`
	PlaybookGroup
}

// PlaybookRound is one round of the playbook. TempoSeconds is nil when the
// team never reached a site (T) or the round had no duel (CT).
type PlaybookRound struct {
	MatchID      uuid.UUID  `json:"match_id"`
	Round        int        `json:"round"`
	Tick         int        `json:"tick"`
	Category     string     `json:"category"`
	Pattern      string     `json:"pattern"`
	Site         Site       `json:"site"`
	FakeSite     Site       `json:"fake_site,omitempty"`
	Won          bool       `json:"won"`
	Buy          BuyType    `json:"buy"`
	OpponentBuy  BuyType    `json:"opponent_buy"`
	TempoSeconds *float64   `json:"tempo_seconds"`
	Framed       bool       `json:"framed"`
	Zones        []ZoneHold `json:"zones"`
}

// Playbook is what a team does on one side of one map, round by round.
type Playbook struct {
	Map        string             `json:"map"`
	Side       Side               `json:"side"`
	Thresholds PlaybookThresholds `json:"thresholds"`
	Matches    int                `json:"matches"`
	RoundCount int                `json:"round_count"`
	WinRate    Rate               `json:"win_rate"`
	// Framed counts the rounds with positions; map control needs them.
	Framed     int                `json:"framed"`
	Categories []PlaybookCategory `json:"categories"`
	Tempo      Histogram          `json:"tempo"`
	Sites      []PlaybookSite     `json:"sites"`
	Zones      []PlaybookZone     `json:"zones"`
	Rounds     []PlaybookRound    `json:"rounds"`
}

type refGroup struct {
	rounds, wins int
	refs         []Evidence
}

func (g *refGroup) add(won bool, e Evidence) {
	g.rounds++
	if won {
		g.wins++
	}
	g.refs = append(g.refs, e)
}

func (g *refGroup) result(total int) PlaybookGroup {
	refs := g.refs
	if refs == nil {
		refs = []Evidence{}
	}
	return PlaybookGroup{Rounds: g.rounds, Share: newRate(g.rounds, total), WinRate: newRate(g.wins, g.rounds), Refs: refs}
}

// selectFramed applies the scout filter's match selection and order.
func selectFramed(matches []FramedMatch, f ScoutFilter) []FramedMatch {
	plain := make([]ScoutMatch, 0, len(matches))
	byID := make(map[uuid.UUID]FramedMatch, len(matches))
	for _, m := range matches {
		plain = append(plain, m.ScoutMatch)
		byID[m.MatchID] = m
	}
	selected := f.Select(plain)
	out := make([]FramedMatch, 0, len(selected))
	for _, sm := range selected {
		out = append(out, byID[sm.MatchID])
	}
	return out
}

// playbookRead is what one round shows once its frames are read.
type playbookRead struct {
	doc      Document
	round    Round
	side     Side
	team     map[uint8]bool
	presence map[Site]int
	zones    []ZoneHold
	tempo    float64
	hasTempo bool
}

// readPlaybookRound reads a round's team track, holds and tempo.
func readPlaybookRound(doc Document, r Round, side Side, frames []Frame) (playbookRead, bool) {
	team := teamSlots(r, side)
	track := teamTrack(doc, r, team, frames)
	read := playbookRead{doc: doc, round: r, side: side, team: team, presence: sitePresence(track), zones: controlledZones(track)}
	if tick := roundTempo(doc, r, side, team, read.presence); tick > 0 {
		read.tempo, read.hasTempo = secondsSince(r, tick, doc.Demo.Tickrate)
	}
	return read, len(track) > 0
}

// category gives the round its playbook category, and the faked site of a fake.
func (p playbookRead) category() (string, Site) {
	r := p.round
	if p.side == SideCT {
		switch r.Class.CTSide {
		case CTSave:
			return PlaybookSave, ""
		case CTStack:
			return PlaybookStack, ""
		case CTAggression:
			return PlaybookAggression, ""
		case CTRetake:
			return PlaybookRetake, ""
		default:
			return PlaybookDefault, ""
		}
	}
	planted := r.Bomb != nil && r.Bomb.PlantTick > 0
	if r.Class.TSide == TSave || (r.Economy.TBuy == BuyEco && !planted && !isBombsite(r.Class.Site)) {
		return PlaybookSave, ""
	}
	if fake := p.fakeSite(); fake != "" {
		return PlaybookFake, fake
	}
	switch {
	case (p.hasTempo && p.tempo <= PlaybookRushSeconds) || r.Class.TSide == TFast || r.Class.TSide == TEcoRush:
		return PlaybookRush, ""
	case r.Class.TSide == TExecute || r.Class.TSide == TSplit:
		return PlaybookExecute, ""
	case holdsMapZone(p.zones):
		return PlaybookMapControl, ""
	default:
		return PlaybookDefault, ""
	}
}

// fakeSite is the other bombsite when the team showed there (players or
// PlaybookFakeUtility grenades) before reaching the site it played, else "".
func (p playbookRead) fakeSite() Site {
	r := p.round
	target := r.Class.Site
	if !isBombsite(target) {
		return ""
	}
	other := SiteA
	if target == SiteA {
		other = SiteB
	}
	commit := PlayCutoffTick(r)
	if t, ok := p.presence[target]; ok && t < commit {
		commit = t
	}
	if t, ok := p.presence[other]; ok && t < commit {
		return other
	}
	commitSecs, ok := secondsSince(r, commit, p.doc.Demo.Tickrate)
	if !ok {
		return ""
	}
	n := 0
	for _, g := range playGrenades(p.doc, r, p.side) {
		if g.Seconds < commitSecs && SiteFromPlace(g.Callout) == other {
			n++
		}
	}
	if n >= PlaybookFakeUtility {
		return other
	}
	return ""
}

// BuildPlaybook classifies the team's rounds on the filter's side and counts
// categories, sites, tempo and zones. Refs follow Select order.
func BuildPlaybook(matches []FramedMatch, f ScoutFilter) Playbook {
	pb := Playbook{
		Side: f.Side,
		Thresholds: PlaybookThresholds{
			RushSeconds: PlaybookRushSeconds, FakeUtility: PlaybookFakeUtility, SiteContactPlayers: SiteContactPlayers,
			MapControlPlayers: MapControlPlayers, MapControlSeconds: MapControlSeconds,
			MapControlWindowSeconds: MapControlWindowSeconds, TempoBucketSeconds: timingBucketSeconds,
		},
		Rounds: []PlaybookRound{},
	}
	if len(f.Maps) == 1 {
		pb.Map = f.Maps[0]
	}
	categories := map[string]*refGroup{}
	sites := map[Site]*refGroup{}
	zones := map[string]*refGroup{}
	tempo := []float64{}
	wins := 0
	selected := selectFramed(matches, f)
	pb.Matches = len(selected)
	for _, m := range selected {
		src := openFrames(m)
		for _, r := range f.rounds(m.ScoutMatch) {
			side := m.Document.TeamSide(m.TeamKey, r)
			read, framed := readPlaybookRound(m.Document, r, side, src.round(r.Number))
			category, fake := read.category()
			won := r.Winner == side
			ref := evidence(m.ScoutMatch, r, 0)
			row := PlaybookRound{
				MatchID: m.MatchID, Round: r.Number, Tick: ref.Tick, Category: category, Pattern: sidePattern(r, side),
				Site: r.Class.Site, FakeSite: fake, Won: won, Buy: r.Economy.Buy(side), OpponentBuy: r.Economy.Buy(side.Opponent()),
				Framed: framed, Zones: read.zones,
			}
			if read.hasTempo {
				secs := read.tempo
				row.TempoSeconds = &secs
				tempo = append(tempo, secs)
			}
			pb.Rounds = append(pb.Rounds, row)
			pb.RoundCount++
			if won {
				wins++
			}
			if framed {
				pb.Framed++
			}
			groupOf(categories, category).add(won, ref)
			groupOf(sites, r.Class.Site).add(won, ref)
			for _, hold := range read.zones {
				groupOf(zones, hold.Zone).add(won, ref)
			}
		}
	}
	pb.WinRate = newRate(wins, pb.RoundCount)
	pb.Tempo = newHistogram(tempo)
	pb.Categories = []PlaybookCategory{}
	for _, name := range playbookCategories[f.Side] {
		pb.Categories = append(pb.Categories, PlaybookCategory{Category: name, PlaybookGroup: groupOf(categories, name).result(pb.RoundCount)})
	}
	pb.Sites = []PlaybookSite{}
	for _, site := range []Site{SiteA, SiteB, SiteMid} {
		pb.Sites = append(pb.Sites, PlaybookSite{Site: site, PlaybookGroup: groupOf(sites, site).result(pb.RoundCount)})
	}
	pb.Zones = make([]PlaybookZone, 0, len(zones))
	for zone, g := range zones {
		pb.Zones = append(pb.Zones, PlaybookZone{Zone: zone, Label: placename.Label(zone), PlaybookGroup: g.result(pb.Framed)})
	}
	sort.Slice(pb.Zones, func(i, j int) bool {
		if pb.Zones[i].Rounds != pb.Zones[j].Rounds {
			return pb.Zones[i].Rounds > pb.Zones[j].Rounds
		}
		return pb.Zones[i].Zone < pb.Zones[j].Zone
	})
	return pb
}

func groupOf[K comparable](groups map[K]*refGroup, key K) *refGroup {
	g, ok := groups[key]
	if !ok {
		g = &refGroup{}
		groups[key] = g
	}
	return g
}
