package tacticalplan

import (
	"sort"
	"time"
)

// Attack prep rules. They ship inside every attack prep, next to the numbers.
const (
	// AttackReadSeconds is when a default's spread is read: after the setup, before most takes.
	AttackReadSeconds = 25
	// AttackLeanRatio puts a player on a site's side of the map when their distance
	// to that site is below this share of their distances to both sites.
	AttackLeanRatio = 0.4
	// AttackStaticSpeed and AttackFastSpeed split a window's pace, in world units per second.
	AttackStaticSpeed = 40
	AttackFastSpeed   = 150
	// AttackExecuteLimit caps the executes listed per buy matchup, most used first.
	AttackExecuteLimit = 5
	// AttackZoneLimit caps the map control zones listed per default.
	AttackZoneLimit = 3
	// AttackMinRounds is the T rounds below which the prep warns the sample is thin: two halves.
	AttackMinRounds = 24
	// AttackStaleDays is the age past which a sample may no longer say how the team plays today.
	AttackStaleDays = 21
	// AttackRosterMatches is the matches a player needs to count as a regular, not a stand-in.
	AttackRosterMatches = 3
)

// Buy bands of a matchup: semi and force buys are one half buy.
const (
	BandPistol = "pistol"
	BandEco    = "eco"
	BandHalf   = "half"
	BandFull   = "full"
)

var bandOrder = map[string]int{BandPistol: 0, BandEco: 1, BandHalf: 2, BandFull: 3}

// Where a T round went: one site, both, or no site at all.
const (
	AttackSiteA    = "A"
	AttackSiteB    = "B"
	AttackSiteBoth = "both"
	AttackSiteNone = "none"
)

// AttackSites lists the site keys in report order.
var AttackSites = []string{AttackSiteA, AttackSiteB, AttackSiteBoth, AttackSiteNone}

// How a T round reached its site.
const (
	TempoRush     = "rush"
	TempoSetStrat = "set_strat"
	TempoMidRound = "mid_round"
	TempoNoSite   = "no_site"
)

// AttackTempos lists the tempo keys in report order.
var AttackTempos = []string{TempoRush, TempoSetStrat, TempoMidRound, TempoNoSite}

// Warning kinds of a prep.
const (
	WarningStale         = "stale"
	WarningFewRounds     = "few_rounds"
	WarningRosterChanged = "roster_changed"
	// WarningCapped is set by the endpoint when it read fewer matches than the filter selects.
	WarningCapped = "capped"
)

// AttackThresholds are the rules the attack prep applied.
type AttackThresholds struct {
	Windows            []PrepWindow `json:"windows"`
	RushSeconds        int          `json:"rush_seconds"`
	ReadSeconds        int          `json:"read_seconds"`
	LeanRatio          float64      `json:"lean_ratio"`
	StaticSpeed        int          `json:"static_speed"`
	FastSpeed          int          `json:"fast_speed"`
	SiteContactPlayers int          `json:"site_contact_players"`
	SpotLimit          int          `json:"spot_limit"`
	ExecuteLimit       int          `json:"execute_limit"`
	MinRounds          int          `json:"min_rounds"`
	StaleDays          int          `json:"stale_days"`
}

// AttackCell is the rounds of one site or tempo key. Share is over the rounds of its row.
type AttackCell struct {
	Key string `json:"key"`
	PrepRounds
}

// AttackMatchup is the team's buy band against the rival's on T, with where
// and how fast its rounds went. Share is over every T round.
type AttackMatchup struct {
	Buy         string `json:"buy"`
	OpponentBuy string `json:"opponent_buy"`
	PrepRounds
	Sites []AttackCell `json:"sites"`
	Tempo []AttackCell `json:"tempo"`
}

// PrepWarning is what to distrust in a prep: an old sample, few rounds, a lineup change.
type PrepWarning struct {
	Kind   string   `json:"kind"`
	Days   int      `json:"days,omitempty"`
	Rounds int      `json:"rounds,omitempty"`
	Joined []string `json:"joined,omitempty"`
	Left   []string `json:"left,omitempty"`
	// Matches and Available are a capped warning's matches read and matches the filter selects.
	Matches   int `json:"matches,omitempty"`
	Available int `json:"available,omitempty"`
}

// AttackPrep is how a team attacks one map: where each buy matchup goes, how
// fast, its defaults, its executes per matchup and each T player's positions.
type AttackPrep struct {
	Map        string           `json:"map"`
	Thresholds AttackThresholds `json:"thresholds"`
	Matches    int              `json:"matches"`
	RoundCount int              `json:"round_count"`
	// Framed counts the T rounds with positions; defaults and players read only them.
	Framed       int             `json:"framed"`
	LastPlayedAt *time.Time      `json:"last_played_at,omitempty"`
	DaysSince    int             `json:"days_since_last_match"`
	Sites        []AttackCell    `json:"sites"`
	Tempo        []AttackCell    `json:"tempo"`
	Matchups     []AttackMatchup `json:"matchups"`
	Defaults     []AttackDefault `json:"defaults"`
	Executes     []AttackExecute `json:"executes"`
	Players      []AttackPlayer  `json:"players"`
	Warnings     []PrepWarning   `json:"warnings"`
}

// AttackOptions selects the matches, names the players' roles and dates the sample.
type AttackOptions struct {
	Filter ScoutFilter
	// Roles is the inferred role per SteamID; a missing one reads "unknown".
	Roles map[string]string
	// Now dates the sample's age; the zero time skips the stale warning.
	Now time.Time
}

type bandKey struct{ buy, opponent string }

func (k bandKey) less(o bandKey) bool {
	if k.buy != o.buy {
		return bandOrder[k.buy] < bandOrder[o.buy]
	}
	return bandOrder[k.opponent] < bandOrder[o.opponent]
}

// cellGroups counts a row's rounds per key.
type cellGroups struct {
	all  prepGroup
	keys map[string]*prepGroup
}

func newCellGroups() *cellGroups { return &cellGroups{keys: map[string]*prepGroup{}} }

func (c *cellGroups) add(key string, ar attackRound) {
	c.all.add(ar.won, ar.start)
	g, ok := c.keys[key]
	if !ok {
		g = &prepGroup{}
		c.keys[key] = g
	}
	g.add(ar.won, ar.start)
}

func (c *cellGroups) cells(order []string) []AttackCell {
	out := make([]AttackCell, 0, len(order))
	for _, key := range order {
		g, ok := c.keys[key]
		if !ok {
			g = &prepGroup{}
		}
		out = append(out, AttackCell{Key: key, PrepRounds: g.result(c.all.rounds)})
	}
	return out
}

type matchupAcc struct {
	prepGroup
	sites, tempo *cellGroups
	// framed counts the rounds with positions, the only ones an execute is read from.
	framed int
}

type attackRun struct {
	sites, tempo *cellGroups
	matchups     map[bandKey]*matchupAcc
	defaults     map[string]*defaultAcc
	executes     map[attackExecuteKey]*attackExecuteAcc
	players      map[string]*attackPlayerAcc
}

// framedRound is one round of a match with its decoded frames.
type framedRound struct {
	m      FramedMatch
	r      Round
	frames []Frame
}

// attackRound is one T round as the attack prep reads it.
type attackRound struct {
	m        FramedMatch
	r        Round
	read     playbookRead
	category string
	site     string
	tempo    string
	band     bandKey
	won      bool
	start    Evidence
}

// BuildAttack reads the team's T rounds over the selected matches. Matches are
// read in Select order, which is the order of every example list.
func BuildAttack(matches []FramedMatch, opts AttackOptions) AttackPrep {
	f := opts.Filter
	f.Side = SideT
	prep := AttackPrep{
		Thresholds: AttackThresholds{
			Windows: PrepWindows, RushSeconds: PlaybookRushSeconds, ReadSeconds: AttackReadSeconds, LeanRatio: AttackLeanRatio,
			StaticSpeed: AttackStaticSpeed, FastSpeed: AttackFastSpeed, SiteContactPlayers: SiteContactPlayers,
			SpotLimit: PrepSpotLimit, ExecuteLimit: AttackExecuteLimit, MinRounds: AttackMinRounds, StaleDays: AttackStaleDays,
		},
		Warnings: []PrepWarning{},
	}
	if len(f.Maps) == 1 {
		prep.Map = f.Maps[0]
	}
	run := attackRun{
		sites: newCellGroups(), tempo: newCellGroups(), matchups: map[bandKey]*matchupAcc{},
		defaults: map[string]*defaultAcc{}, executes: map[attackExecuteKey]*attackExecuteAcc{}, players: map[string]*attackPlayerAcc{},
	}
	selected := selectFramed(matches, f)
	prep.Matches = len(selected)
	rosters := make([]matchRoster, 0, len(selected))
	for _, m := range selected {
		src := openFrames(m)
		rounds := f.rounds(m.ScoutMatch)
		rosters = append(rosters, rosterOf(m, rounds))
		for _, r := range rounds {
			prep.RoundCount++
			if run.addRound(framedRound{m: m, r: r, frames: src.round(r.Number)}) {
				prep.Framed++
			}
		}
	}
	prep.Sites = run.sites.cells(AttackSites)
	prep.Tempo = run.tempo.cells(AttackTempos)
	prep.Matchups = run.matchupRows(prep.RoundCount)
	prep.Defaults = run.defaultRows(prep.Framed)
	prep.Executes = run.executeRows()
	prep.Players = run.playerRows(opts.Roles, prep.Map)
	if prep.RoundCount < AttackMinRounds {
		prep.Warnings = append(prep.Warnings, PrepWarning{Kind: WarningFewRounds, Rounds: prep.RoundCount})
	}
	if len(selected) > 0 {
		played := selected[0].PlayedAt
		prep.LastPlayedAt = &played
		var warnings []PrepWarning
		prep.DaysSince, warnings = sampleWarnings(rosters, opts.Now)
		prep.Warnings = append(prep.Warnings, warnings...)
	}
	return prep
}

// addRound counts one T round; it reports whether positions were read from it.
func (run *attackRun) addRound(in framedRound) bool {
	read, framed := readPlaybookRound(in.m.Document, in.r, SideT, in.frames)
	category, _ := read.category()
	site := attackSite(in.r, attackRead{presence: read.presence, category: category})
	ar := attackRound{
		m: in.m, r: in.r, read: read, category: category, site: site,
		tempo: attackTempo(category, site), band: bandKey{buyBand(in.r.Economy.TBuy), buyBand(in.r.Economy.CTBuy)},
		won: in.r.Winner == SideT, start: evidence(in.m.ScoutMatch, in.r, 0),
	}
	run.sites.add(ar.site, ar)
	run.tempo.add(ar.tempo, ar)
	if ar.band.buy != "" && ar.band.opponent != "" {
		acc, ok := run.matchups[ar.band]
		if !ok {
			acc = &matchupAcc{sites: newCellGroups(), tempo: newCellGroups()}
			run.matchups[ar.band] = acc
		}
		acc.add(ar.won, ar.start)
		acc.sites.add(ar.site, ar)
		acc.tempo.add(ar.tempo, ar)
		if framed {
			acc.framed++
			if ar.site != AttackSiteNone && (ar.tempo == TempoRush || ar.tempo == TempoSetStrat) {
				run.addExecute(ar)
			}
		}
	}
	if !framed {
		return false
	}
	if category == PlaybookDefault || category == PlaybookMapControl {
		run.addDefault(ar)
	}
	run.addPlayers(ar)
	return true
}

func (run *attackRun) matchupRows(total int) []AttackMatchup {
	keys := make([]bandKey, 0, len(run.matchups))
	for k := range run.matchups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].less(keys[j]) })
	out := make([]AttackMatchup, 0, len(keys))
	for _, k := range keys {
		acc := run.matchups[k]
		out = append(out, AttackMatchup{
			Buy: k.buy, OpponentBuy: k.opponent, PrepRounds: acc.result(total),
			Sites: acc.sites.cells(AttackSites), Tempo: acc.tempo.cells(AttackTempos),
		})
	}
	return out
}

// buyBand folds a buy into the matchup bands; an unknown buy has none.
func buyBand(b BuyType) string {
	switch b {
	case BuyPistol:
		return BandPistol
	case BuyEco:
		return BandEco
	case BuySemi, BuyForce:
		return BandHalf
	case BuyFull:
		return BandFull
	default:
		return ""
	}
}

// attackRead is what attackSite reads of a round besides its class.
type attackRead struct {
	presence map[Site]int
	category string
}

// attackSite is both when the team stood on both sites before the plant of a
// round that was no fake (TSplit is one site from two corridors); otherwise the round's site.
func attackSite(r Round, read attackRead) string {
	tickA, onA := read.presence[SiteA]
	tickB, onB := read.presence[SiteB]
	planted := r.Bomb != nil && r.Bomb.PlantTick > 0
	switch {
	case onA && onB && planted && tickA <= r.Bomb.PlantTick && tickB <= r.Bomb.PlantTick && read.category != PlaybookFake:
		return AttackSiteBoth
	case r.Class.Site == SiteA:
		return AttackSiteA
	case r.Class.Site == SiteB:
		return AttackSiteB
	case onA:
		return AttackSiteA
	case onB:
		return AttackSiteB
	default:
		return AttackSiteNone
	}
}

// attackTempo reads the playbook category as the pace of the take: a rush, a set
// strat (execute or fake), a mid-round take after map control or a default, or no site.
func attackTempo(category, site string) string {
	if site == AttackSiteNone || category == PlaybookSave {
		return TempoNoSite
	}
	switch category {
	case PlaybookRush:
		return TempoRush
	case PlaybookExecute, PlaybookFake:
		return TempoSetStrat
	default:
		return TempoMidRound
	}
}

func dayOf(t time.Time) int64 { return t.UTC().Unix() / 86400 }

type rosterSpan struct {
	name        string
	matches     int
	first, last int64
}

// matchRoster is the team's players in the rounds a match kept, by SteamID64.
type matchRoster struct {
	day     int64
	players map[string]string
}

// rosterOf names the team's players in the kept rounds, so a lineup filter
// leaves its benched players out of the roster warning.
func rosterOf(m FramedMatch, rounds []Round) matchRoster {
	out := matchRoster{day: dayOf(m.PlayedAt), players: map[string]string{}}
	for _, r := range rounds {
		for _, pr := range r.Players {
			if p, ok := m.Document.PlayerBySlot(pr.Slot); ok && p.TeamKey == m.TeamKey && p.SteamID64 != "" {
				out.players[p.SteamID64] = p.Name
			}
		}
	}
	return out
}

// sampleWarnings is what to distrust in the sample: its age and lineup changes
// inside it, plus the days since the newest match. rosters is not empty.
func sampleWarnings(rosters []matchRoster, now time.Time) (int, []PrepWarning) {
	out := []PrepWarning{}
	newest, oldest := rosters[0].day, rosters[0].day
	spans := map[string]*rosterSpan{}
	for _, m := range rosters {
		day := m.day
		newest, oldest = max(newest, day), min(oldest, day)
		for id, name := range m.players {
			s, ok := spans[id]
			if !ok {
				s = &rosterSpan{name: name, first: day, last: day}
				spans[id] = s
			}
			s.matches++
			s.first, s.last = min(s.first, day), max(s.last, day)
		}
	}
	days := 0
	if !now.IsZero() {
		days = max(int(dayOf(now)-newest), 0)
		if days >= AttackStaleDays {
			out = append(out, PrepWarning{Kind: WarningStale, Days: days})
		}
	}
	joined, left := []string{}, []string{}
	for _, s := range spans {
		if s.matches < AttackRosterMatches {
			continue
		}
		if s.first > oldest {
			joined = append(joined, s.name)
		}
		if s.last < newest {
			left = append(left, s.name)
		}
	}
	if len(joined) > 0 || len(left) > 0 {
		sort.Strings(joined)
		sort.Strings(left)
		out = append(out, PrepWarning{Kind: WarningRosterChanged, Joined: joined, Left: left})
	}
	return days, out
}
