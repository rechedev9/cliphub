package tactical

import (
	"fmt"
	"math"
	"sort"

	"github.com/rechedev9/tickcut/internal/tacticalplan"
)

// The economy thresholds below are per player and are multiplied by the number
// of players a side fielded that round, so a 4-man side is judged on what four
// players could afford. They match the values the CS analysis ecosystem
// converged on, which keeps our buy types comparable with published numbers.
const (
	ecoMaxPerPlayer    = 1000
	fullMinPerPlayerCT = 4500
	fullMinPerPlayerT  = 4000
	brokeMaxPerPlayer  = 400
	defaultSideSize    = 5
)

// Round-shape thresholds. They are deliberately few and all expressed in
// seconds or world units, so a disagreement about a classification is an
// argument about a number rather than about hidden logic.
const (
	commitPlayers        = 3    // attackers reaching one site that make it a commit
	commitWindowSeconds  = 15.0 // how close together those arrivals have to be
	fastCommitSeconds    = 20.0 // a commit this early was not preceded by map control
	setupWindowSeconds   = 30.0 // how long after freeze end a defensive setup is still a setup
	executeWindowSecs    = 10.0 // utility thrown this close before the commit is the execute
	executeUtility       = 3    // pieces of utility that make it an execute
	splitSectorPlayers   = 2    // attackers per approach direction for a split
	approachGapRadians   = 1.2  // ~70 degrees of empty arc separates two approaches
	stackDefenders       = 3    // defenders on one site that make it a stack
	saveSurvivors        = 2    // survivors on a lost round that make it a save
	aggressionSeconds    = 25.0 // a defender's kill this early, away from a site
	plantApproachSeconds = 25.0 // how far back from a plant the site approach is read
)

type classifyInput struct {
	round    tacticalplan.Round
	acc      *roundAcc
	tickrate float64
	sites    siteMap
}

// classifyEconomy assigns both sides' buy types. The rules are the published
// ones: an eco is what a side could not avoid, a full buy is what a side can
// fight with, and a force buy is a half buy made while broke after a loss —
// which is why it is not a value band of its own.
func classifyEconomy(round tacticalplan.Round, previous *tacticalplan.Round) (tacticalplan.BuyType, tacticalplan.BuyType) {
	firstOfHalf := previous == nil || previous.Half != round.Half
	// Overtime halves start with a full buy, so labelling their first round a
	// pistol round would misreport every overtime economy.
	if firstOfHalf && round.Overtime == 0 {
		return tacticalplan.BuyPistol, tacticalplan.BuyPistol
	}
	return classifySideEconomy(round, previous, tacticalplan.SideCT),
		classifySideEconomy(round, previous, tacticalplan.SideT)
}

func classifySideEconomy(round tacticalplan.Round, previous *tacticalplan.Round, side tacticalplan.Side) tacticalplan.BuyType {
	size := sideSize(round, side)
	if size == 0 {
		return tacticalplan.BuyUnknown
	}
	value := round.Economy.EquipValue(side)
	if value <= 0 && round.Economy.SampleTick == 0 {
		return tacticalplan.BuyUnknown
	}
	if value <= ecoMaxPerPlayer*size {
		return tacticalplan.BuyEco
	}
	fullMin := fullMinPerPlayerCT
	if side == tacticalplan.SideT {
		fullMin = fullMinPerPlayerT
	}
	if value >= fullMin*size {
		return tacticalplan.BuyFull
	}
	money := round.Economy.CTMoney
	if side == tacticalplan.SideT {
		money = round.Economy.TMoney
	}
	if previous != nil && previous.Winner != "" && previous.Winner != side && money <= brokeMaxPerPlayer*size {
		return tacticalplan.BuyForce
	}
	return tacticalplan.BuySemi
}

func sideSize(round tacticalplan.Round, side tacticalplan.Side) int {
	n := 0
	for _, p := range round.Players {
		if p.Side == side {
			n++
		}
	}
	if n == 0 {
		return defaultSideSize
	}
	return n
}

// classifyRound decides the round's shape. It keeps the opening-duel facts the
// caller already filled in and adds the site, both sides' patterns, the tags,
// and a reason for every judgement.
func classifyRound(in classifyInput) tacticalplan.Class {
	class := in.round.Class
	class.Tags = []string{}
	class.Reasons = []string{}
	class.TSide = tacticalplan.TUnknown
	class.CTSide = tacticalplan.CTUnknown
	class.Site = tacticalplan.SiteNone

	freezeEnd := in.round.TickFreezeEnd
	if freezeEnd == 0 {
		freezeEnd = in.round.TickStart
	}
	class.FirstContactTick = firstContactTick(in.acc)
	// The rules below read first contact off the input round, so the fact has to
	// be installed before any of them run.
	in.round.Class.FirstContactTick = class.FirstContactTick

	commit, hasCommit := findCommit(in, tacticalplan.SideT)
	if !hasCommit {
		// A plant is proof that the site was taken, even when fewer attackers
		// survived to reach it than a commit normally needs.
		commit, hasCommit = commitFromPlant(in)
	}
	switch {
	case in.round.Bomb != nil && in.round.Bomb.PlantTick > 0 && in.round.Bomb.Site != tacticalplan.SiteNone:
		class.Site = in.round.Bomb.Site
		class.Reasons = append(class.Reasons, fmt.Sprintf("site from bomb plant on %s", class.Site))
	case hasCommit:
		class.Site = commit.site
		class.Reasons = append(class.Reasons, fmt.Sprintf("site from %d attackers within %.0fu of %s",
			commit.players, in.sites.radius, commit.site))
	case class.FirstContactTick > 0:
		if site, dist := nearestSiteOfFirstContact(in); site != tacticalplan.SiteNone && dist <= in.sites.radius {
			class.Site = site
			class.Reasons = append(class.Reasons, fmt.Sprintf("site from first contact %.0fu from %s", dist, site))
		}
	}

	class.TSide, class.Reasons = classifyTSide(in, commit, hasCommit, freezeEnd, class.Reasons)
	class.CTSide, class.Reasons = classifyCTSide(in, commit, hasCommit, freezeEnd, class.Reasons)
	class.Tags = roundTags(in, commit, hasCommit)
	sort.Strings(class.Tags)
	return class
}

type commitInfo struct {
	site    tacticalplan.Site
	tick    int
	players int
	sectors int
}

// siteEntry is the moment one attacker first reached a site, and the direction
// they came from.
type siteEntry struct {
	slot    uint8
	tick    int
	bearing float64
}

// findCommit locates the first time enough attackers reached one site inside a
// short window, which is the anchor every other timing judgement uses.
//
// Entries are counted as they arrive rather than as a head count in a single
// frame: an execute where the first man in trades immediately never has three
// live attackers on the site at once, but it is still an execute.
func findCommit(in classifyInput, side tacticalplan.Side) (commitInfo, bool) {
	sides := roundSides(in.round)
	best := commitInfo{site: tacticalplan.SiteNone}
	found := false
	window := int(commitWindowSeconds * in.tickrate)

	for _, site := range in.sites.bombsites() {
		center, ok := in.sites.center(site)
		if !ok {
			continue
		}
		entries := siteEntries(in, sides, side, center)
		if len(entries) < commitPlayers {
			continue
		}
		for i := 0; i+commitPlayers-1 < len(entries); i++ {
			last := i
			for last+1 < len(entries) && entries[last+1].tick-entries[i].tick <= window {
				last++
			}
			if last-i+1 < commitPlayers {
				continue
			}
			bearings := make([]float64, 0, last-i+1)
			for _, e := range entries[i : last+1] {
				bearings = append(bearings, e.bearing)
			}
			candidate := commitInfo{
				site:    site,
				tick:    entries[i+commitPlayers-1].tick,
				players: len(bearings),
				sectors: approachGroups(bearings),
			}
			if !found || candidate.tick < best.tick {
				best = candidate
				found = true
			}
			break
		}
	}
	return best, found
}

// commitFromPlant reconstructs the commitment from the plant when too few
// attackers survived to reach the site together. The commit tick is the first
// attacker's arrival in the approach window before the plant, so the timing
// still describes the execute rather than the plant that ended it.
func commitFromPlant(in classifyInput) (commitInfo, bool) {
	bomb := in.round.Bomb
	if bomb == nil || bomb.PlantTick == 0 || bomb.Site == tacticalplan.SiteNone {
		return commitInfo{site: tacticalplan.SiteNone}, false
	}
	commit := commitInfo{site: bomb.Site, tick: bomb.PlantTick}
	center, ok := in.sites.center(bomb.Site)
	if !ok {
		return commit, true
	}
	entries := siteEntries(in, roundSides(in.round), tacticalplan.SideT, center)
	from := bomb.PlantTick - int(plantApproachSeconds*in.tickrate)
	var bearings []float64
	for _, e := range entries {
		if e.tick < from || e.tick > bomb.PlantTick {
			continue
		}
		if len(bearings) == 0 {
			commit.tick = e.tick
		}
		bearings = append(bearings, e.bearing)
	}
	commit.players = len(bearings)
	commit.sectors = approachGroups(bearings)
	return commit, true
}

// siteEntries lists, in arrival order, the first time each of a side's players
// was seen alive inside a site.
func siteEntries(in classifyInput, sides map[uint8]tacticalplan.Side, side tacticalplan.Side, center [2]float64) []siteEntry {
	var entries []siteEntry
	seen := map[uint8]bool{}
	for _, frame := range in.acc.frames {
		for _, sample := range frame.Samples {
			if seen[sample.Slot] || sides[sample.Slot] != side || !sample.Flags.Has(tacticalplan.FlagAlive) {
				continue
			}
			if math.Hypot(sample.X-center[0], sample.Y-center[1]) > in.sites.radius {
				continue
			}
			seen[sample.Slot] = true
			entries = append(entries, siteEntry{
				slot:    sample.Slot,
				tick:    frame.Tick,
				bearing: math.Atan2(sample.Y-center[1], sample.X-center[0]),
			})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].tick < entries[j].tick })
	return entries
}

// approachGroups counts the distinct directions attackers arrived from. It
// clusters the bearings around the site rather than bucketing them into fixed
// quadrants, because a fixed grid splits a single group that happens to
// straddle a boundary and would report a straight A-main take as a split.
func approachGroups(bearings []float64) int {
	if len(bearings) == 0 {
		return 0
	}
	sorted := append([]float64(nil), bearings...)
	sort.Float64s(sorted)

	n := len(sorted)
	gaps := make([]float64, n)
	widest := 0
	for i := range sorted {
		next := sorted[(i+1)%n]
		gap := next - sorted[i]
		if i == n-1 {
			gap = next + 2*math.Pi - sorted[i]
		}
		gaps[i] = gap
		if gap > gaps[widest] {
			widest = i
		}
	}
	if gaps[widest] < approachGapRadians {
		// Everyone arrived from one direction.
		if n >= splitSectorPlayers {
			return 1
		}
		return 0
	}

	groups := 0
	size := 0
	// Walking from just after the widest gap keeps a group that straddles the
	// -pi/+pi seam whole.
	for step := 0; step < n; step++ {
		i := (widest + 1 + step) % n
		size++
		if gaps[i] >= approachGapRadians || step == n-1 {
			if size >= splitSectorPlayers {
				groups++
			}
			size = 0
		}
	}
	return groups
}

func classifyTSide(in classifyInput, commit commitInfo, hasCommit bool, freezeEnd int, reasons []string) (tacticalplan.TPattern, []string) {
	planted := in.round.Bomb != nil && in.round.Bomb.PlantTick > 0
	if !hasCommit && !planted {
		survivors := survivorCount(in.round, tacticalplan.SideT)
		if in.round.Winner == tacticalplan.SideCT && survivors >= saveSurvivors {
			return tacticalplan.TSave, append(reasons,
				fmt.Sprintf("T save: no site commitment and %d survivors on a lost round", survivors))
		}
		// An attack that never reached a site but did trade fights was still a
		// default: the map-control phase is where it ended.
		if contact := in.round.Class.FirstContactTick; contact > 0 {
			return tacticalplan.TDefault, append(reasons,
				fmt.Sprintf("default: first contact %.0fs after freeze end, no site was ever reached",
					seconds(contact-freezeEnd, in.tickrate)))
		}
		return tacticalplan.TUnknown, append(reasons, "no T site commitment and no contact was observed")
	}

	commitTick := commit.tick
	elapsed := seconds(commitTick-freezeEnd, in.tickrate)
	buy := in.round.Economy.TBuy

	utility := utilityBefore(in.acc, tacticalplan.SideT, commitTick, in.tickrate)
	switch {
	case (buy == tacticalplan.BuyEco || buy == tacticalplan.BuyForce) && elapsed <= fastCommitSeconds:
		return tacticalplan.TEcoRush, append(reasons,
			fmt.Sprintf("eco rush: %s buy committed %.0fs after freeze end", buy, elapsed))
	case commit.sectors >= 2:
		return tacticalplan.TSplit, append(reasons,
			fmt.Sprintf("split: attackers arrived from %d separate approaches", commit.sectors))
	case utility >= executeUtility:
		return tacticalplan.TExecute, append(reasons,
			fmt.Sprintf("execute: %d pieces of T utility in the %.0fs before the commit", utility, executeWindowSecs))
	case elapsed <= fastCommitSeconds:
		return tacticalplan.TFast, append(reasons,
			fmt.Sprintf("fast: committed %.0fs after freeze end with %d utility", elapsed, utility))
	default:
		return tacticalplan.TDefault, append(reasons,
			fmt.Sprintf("default: committed %.0fs after freeze end with %d utility", elapsed, utility))
	}
}

func classifyCTSide(in classifyInput, commit commitInfo, hasCommit bool, freezeEnd int, reasons []string) (tacticalplan.CTPattern, []string) {
	planted := in.round.Bomb != nil && in.round.Bomb.PlantTick > 0
	if planted {
		defused := in.round.Bomb.DefuseTick > 0
		alive := aliveAt(in, tacticalplan.SideCT, in.round.Bomb.PlantTick)
		if defused || alive > 0 {
			return tacticalplan.CTRetake, append(reasons,
				fmt.Sprintf("retake: %d defenders alive at the plant, defused=%t", alive, defused))
		}
		return tacticalplan.CTHold, append(reasons,
			"hold: the site was lost before the plant and no defender was left to retake")
	}

	if stack, site, tick := findStack(in, freezeEnd); stack {
		return tacticalplan.CTStack, append(reasons,
			fmt.Sprintf("stack: %d defenders on %s %.0fs after freeze end, before first contact",
				stackDefenders, site, seconds(tick-freezeEnd, in.tickrate)))
	}

	if slot, tick, ok := firstDefenderKill(in); ok {
		elapsed := seconds(tick-freezeEnd, in.tickrate)
		if elapsed <= aggressionSeconds {
			if site, dist := nearestSiteOfKill(in, tick); site == tacticalplan.SiteNone || dist > in.sites.radius {
				return tacticalplan.CTAggression, append(reasons,
					fmt.Sprintf("aggression: defender slot %d took a kill %.0fs in, away from any site", slot, elapsed))
			}
		}
	}

	survivors := survivorCount(in.round, tacticalplan.SideCT)
	if in.round.Winner == tacticalplan.SideT && survivors >= saveSurvivors && !planted {
		return tacticalplan.CTSave, append(reasons,
			fmt.Sprintf("CT save: %d survivors on a lost round with no plant", survivors))
	}
	_ = hasCommit
	return tacticalplan.CTHold, append(reasons, "hold: defenders stayed in their default setup")
}

func roundTags(in classifyInput, commit commitInfo, hasCommit bool) []string {
	tags := []string{}
	round := in.round
	if round.Overtime > 0 {
		tags = append(tags, tacticalplan.TagOvertime)
	}
	if round.Economy.CTBuy == tacticalplan.BuyPistol && round.Economy.TBuy == tacticalplan.BuyPistol {
		tags = append(tags, tacticalplan.TagPistol)
	}
	if round.EndReason == "time_expired" {
		tags = append(tags, tacticalplan.TagTimeout)
	}
	if round.Bomb != nil && round.Bomb.PlantTick > 0 {
		tags = append(tags, tacticalplan.TagPostPlant)
		if round.Winner == tacticalplan.SideCT {
			tags = append(tags, tacticalplan.TagRetakeWon)
		}
	}
	if round.Class.OpeningTraded {
		tags = append(tags, tacticalplan.TagOpeningTraded)
	}
	for _, p := range round.Players {
		if p.Kills >= 5 {
			tags = append(tags, tacticalplan.TagAce)
			break
		}
	}
	if isAntiEco(round.Economy) {
		tags = append(tags, tacticalplan.TagAntiEco)
	}
	if !hasCommit && (round.Bomb == nil || round.Bomb.PlantTick == 0) {
		if survivorCount(round, tacticalplan.SideT) >= saveSurvivors && round.Winner == tacticalplan.SideCT {
			tags = append(tags, tacticalplan.TagFullSave)
		}
	}
	_ = commit
	return tags
}

// isAntiEco reports the matchup, not a buy type: a full buy facing a side that
// could not answer it.
func isAntiEco(e tacticalplan.Economy) bool {
	poor := func(b tacticalplan.BuyType) bool {
		return b == tacticalplan.BuyEco || b == tacticalplan.BuySemi || b == tacticalplan.BuyForce
	}
	return (e.CTBuy == tacticalplan.BuyFull && poor(e.TBuy)) ||
		(e.TBuy == tacticalplan.BuyFull && poor(e.CTBuy))
}

func firstContactTick(acc *roundAcc) int {
	for _, k := range acc.kills {
		if !k.teamKill {
			return k.tick
		}
	}
	return 0
}

func nearestSiteOfFirstContact(in classifyInput) (tacticalplan.Site, float64) {
	for _, k := range in.acc.kills {
		if k.teamKill {
			continue
		}
		return in.sites.nearest(k.pos[0], k.pos[1])
	}
	return tacticalplan.SiteNone, math.Inf(1)
}

func nearestSiteOfKill(in classifyInput, tick int) (tacticalplan.Site, float64) {
	for _, k := range in.acc.kills {
		if k.tick != tick {
			continue
		}
		return in.sites.nearest(k.pos[0], k.pos[1])
	}
	return tacticalplan.SiteNone, math.Inf(1)
}

func firstDefenderKill(in classifyInput) (uint8, int, bool) {
	for _, k := range in.acc.kills {
		if k.teamKill || k.killerSlot == nil {
			continue
		}
		if k.killerSide != tacticalplan.SideCT {
			continue
		}
		return *k.killerSlot, k.tick, true
	}
	return 0, 0, false
}

// findStack looks for three defenders on one site during the setup phase only.
// Three defenders on a site after the fight has started is a retake or a
// collapse, not a stack, and counting it as one would make every round look
// stacked.
func findStack(in classifyInput, freezeEnd int) (bool, tacticalplan.Site, int) {
	sides := roundSides(in.round)
	limit := freezeEnd + int(setupWindowSeconds*in.tickrate)
	if contact := in.round.Class.FirstContactTick; contact > 0 && contact < limit {
		limit = contact
	}
	for _, site := range in.sites.bombsites() {
		center, ok := in.sites.center(site)
		if !ok {
			continue
		}
		for _, frame := range in.acc.frames {
			if frame.Tick > limit {
				break
			}
			n := 0
			for _, sample := range frame.Samples {
				if sides[sample.Slot] != tacticalplan.SideCT || !sample.Flags.Has(tacticalplan.FlagAlive) {
					continue
				}
				if math.Hypot(sample.X-center[0], sample.Y-center[1]) <= in.sites.radius {
					n++
				}
			}
			if n >= stackDefenders {
				return true, site, frame.Tick
			}
		}
	}
	return false, tacticalplan.SiteNone, 0
}

func aliveAt(in classifyInput, side tacticalplan.Side, tick int) int {
	sides := roundSides(in.round)
	best := -1
	alive := 0
	for _, frame := range in.acc.frames {
		if frame.Tick > tick {
			break
		}
		if frame.Tick < best {
			continue
		}
		best = frame.Tick
		alive = 0
		for _, sample := range frame.Samples {
			if sides[sample.Slot] == side && sample.Flags.Has(tacticalplan.FlagAlive) {
				alive++
			}
		}
	}
	return alive
}

func utilityBefore(acc *roundAcc, side tacticalplan.Side, tick int, tickrate float64) int {
	if tickrate <= 0 {
		return 0
	}
	from := tick - int(executeWindowSecs*tickrate)
	n := 0
	for _, ev := range acc.events {
		if ev.Side != side || ev.Tick < from || ev.Tick > tick {
			continue
		}
		switch ev.Kind {
		case tacticalplan.EventSmoke, tacticalplan.EventFlash, tacticalplan.EventHE,
			tacticalplan.EventMolotov, tacticalplan.EventDecoy:
			n++
		}
	}
	return n
}

func survivorCount(round tacticalplan.Round, side tacticalplan.Side) int {
	n := 0
	for _, p := range round.Players {
		if p.Side == side && p.Survived {
			n++
		}
	}
	return n
}

func roundSides(round tacticalplan.Round) map[uint8]tacticalplan.Side {
	sides := make(map[uint8]tacticalplan.Side, len(round.Players))
	for _, p := range round.Players {
		sides[p.Slot] = p.Side
	}
	return sides
}

func seconds(ticks int, tickrate float64) float64 {
	if tickrate <= 0 {
		return 0
	}
	return float64(ticks) / tickrate
}
