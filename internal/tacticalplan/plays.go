package tacticalplan

import (
	"fmt"
	"math"
	"sort"
)

// Stratbook rules. A play is the utility a team threw before it committed and
// where its players stood at a fixed second, so two rounds are the same play
// when those two pictures overlap enough. Every rule ships in the detection
// result, like the round classifier's thresholds, so a grouping can be argued
// about as a number.
const (
	// PlayPositionSeconds is when, after freeze end, a round's positions are read.
	PlayPositionSeconds = ScoutSetupSeconds
	// PlayMinGrenades is the least utility a round needs to be compared at all:
	// a round with less is a save or a dry peek, not a play.
	PlayMinGrenades = 2
	// PlayMinRounds is how many similar rounds make a repeated play.
	PlayMinRounds = 3
	// PlayMinUtilitySimilarity is the least overlap of grenades (kind and
	// landing spot, counted as a multiset) between a round and the play's seed.
	PlayMinUtilitySimilarity = 0.5
	// PlayMinPositionSimilarity is the least overlap of the callouts the team
	// held at PlayPositionSeconds. It is skipped when either round has no positions.
	PlayMinPositionSimilarity = 0.4
	// PlayCommonShare is the part of a play's rounds a grenade or a spot must
	// appear in to be listed as part of the play.
	PlayCommonShare = 0.5
	// PlayExampleLimit caps the example rounds kept for each detected play.
	PlayExampleLimit = 20
	// PlayGridCell names a landing spot by its grid cell, in world units, when
	// the map has no callouts.
	PlayGridCell = 256
	// PlayMaxGrenades caps the grenades one round contributes to a play.
	PlayMaxGrenades = 40
)

// PlayGrenade is one grenade of a play: what it was, when it landed after
// freeze end and where. Share is set on a detected play only: the part of its
// rounds that threw it. LineupID names the catalog lineup it matched, if any.
type PlayGrenade struct {
	Kind     EventKind  `json:"kind"`
	Seconds  float64    `json:"seconds"`
	Pos      [3]float64 `json:"pos"`
	Callout  string     `json:"callout,omitempty"`
	Player   string     `json:"player,omitempty"`
	Share    float64    `json:"share,omitempty"`
	LineupID string     `json:"lineup_id,omitempty"`
	Lineup   string     `json:"lineup,omitempty"`
}

// PlayPosition is where one player of a play stood at the play's second.
type PlayPosition struct {
	Player    string     `json:"player,omitempty"`
	SteamID64 string     `json:"steamid64,omitempty"`
	Pos       [3]float64 `json:"pos"`
	Callout   string     `json:"callout,omitempty"`
	Share     float64    `json:"share,omitempty"`
}

// PlaySketch is what one round shows of a team's play: the side, the round's
// own classification, its grenades up to the commit and its positions.
type PlaySketch struct {
	Map             string         `json:"map"`
	Side            Side           `json:"side"`
	Kind            string         `json:"kind"`
	Site            Site           `json:"site"`
	PositionSeconds float64        `json:"position_seconds"`
	Grenades        []PlayGrenade  `json:"grenades"`
	Positions       []PlayPosition `json:"positions"`
}

// PlayCutoffTick is the last tick whose utility belongs to a round's play: the
// plant when there was one, since what follows is the post-plant, else the end.
func PlayCutoffTick(r Round) int {
	if r.Bomb != nil && r.Bomb.PlantTick > 0 {
		return r.Bomb.PlantTick
	}
	return r.TickEnd
}

// SketchRound reads the play a team ran in one round. frame is the positions
// frame to read, or nil when the match has no blob; its tick sets PositionSeconds.
func SketchRound(doc Document, r Round, teamKey string, frame *Frame) (PlaySketch, error) {
	side := doc.TeamSide(teamKey, r)
	if side == "" {
		return PlaySketch{}, fmt.Errorf("team %q is not in the document", teamKey)
	}
	s := PlaySketch{
		Map: doc.Demo.Map, Side: side, Kind: sidePattern(r, side), Site: r.Class.Site,
		Grenades: playGrenades(doc, r, side), Positions: []PlayPosition{},
	}
	if frame != nil {
		s.Positions = playPositions(doc, r, side, *frame)
		if secs, ok := secondsSince(r, frame.Tick, doc.Demo.Tickrate); ok {
			s.PositionSeconds = math.Round(secs*10) / 10
		}
	}
	return s, nil
}

// playGrenades lists the side's grenades from freeze end to the play cutoff,
// in the order they landed.
func playGrenades(doc Document, r Round, side Side) []PlayGrenade {
	team := teamSlots(r, side)
	cutoff := PlayCutoffTick(r)
	out := []PlayGrenade{}
	// Some demos report one flash twice at the same tick; a play counts it once.
	type grenadeKey struct {
		kind EventKind
		slot uint8
		tick int
	}
	seen := map[grenadeKey]bool{}
	for _, ev := range r.Events {
		if len(out) >= PlayMaxGrenades {
			break
		}
		if !isScoutUtility(ev.Kind) || ev.ActorSlot == nil || !team[*ev.ActorSlot] {
			continue
		}
		if ev.Tick < r.TickFreezeEnd || (cutoff > 0 && ev.Tick > cutoff) {
			continue
		}
		key := grenadeKey{kind: ev.Kind, slot: *ev.ActorSlot, tick: ev.Tick}
		if seen[key] {
			continue
		}
		seen[key] = true
		g := PlayGrenade{Kind: ev.Kind, Pos: ev.Pos, Callout: NearestCallout(doc.Geometry, ev.Pos)}
		if secs, ok := secondsSince(r, ev.Tick, doc.Demo.Tickrate); ok {
			g.Seconds = math.Round(secs*10) / 10
		}
		_, g.Player = playerIdentity(doc, *ev.ActorSlot)
		out = append(out, g)
	}
	return out
}

// playPositions lists the side's alive players in a frame, by slot.
func playPositions(doc Document, r Round, side Side, frame Frame) []PlayPosition {
	team := teamSlots(r, side)
	out := []PlayPosition{}
	for _, sample := range frame.Samples {
		if !team[sample.Slot] || !sample.Flags.Has(FlagAlive) {
			continue
		}
		pos := [3]float64{sample.X, sample.Y, sample.Z}
		p, _ := doc.PlayerBySlot(sample.Slot)
		out = append(out, PlayPosition{Player: p.Name, SteamID64: p.SteamID64, Pos: pos, Callout: NearestCallout(doc.Geometry, pos)})
	}
	return out
}

// FrameAt decodes a round's frames from a position blob and returns the first
// frame at or after tick, or the last frame when tick is past the round.
func FrameAt(blob []byte, doc Document, round, tick int) (Frame, bool, error) {
	header, err := DecodeHeader(blob)
	if err != nil {
		return Frame{}, false, err
	}
	offset, ok := doc.Positions.Offset(round)
	if !ok || offset.FrameCount == 0 {
		return Frame{}, false, nil
	}
	frames, err := DecodeFrames(blob, offset.ByteOffset, offset.FrameCount, header)
	if err != nil || len(frames) == 0 {
		return Frame{}, false, err
	}
	for _, f := range frames {
		if f.Tick >= tick {
			return f, true, nil
		}
	}
	return frames[len(frames)-1], true, nil
}

// spotName is the token a position is compared by: its callout, or its grid
// cell when the map has no callouts.
func spotName(g MapGeometry, pos [3]float64) string {
	if c := NearestCallout(g, pos); c != "" {
		return c
	}
	return fmt.Sprintf("cell:%d:%d", int(math.Floor(pos[0]/PlayGridCell)), int(math.Floor(pos[1]/PlayGridCell)))
}

// multisetSimilarity is the weighted Jaccard index of two token counts: the
// shared count over the combined count. Two empty sets are not similar.
func multisetSimilarity(a, b map[string]int) float64 {
	shared, combined := 0, 0
	for k, n := range a {
		shared += min(n, b[k])
		combined += max(n, b[k])
	}
	for k, n := range b {
		if _, ok := a[k]; !ok {
			combined += n
		}
	}
	if combined == 0 {
		return 0
	}
	return float64(shared) / float64(combined)
}

// PlayThresholds are the rules a detection applied.
type PlayThresholds struct {
	PositionSeconds       float64   `json:"position_seconds"`
	MinGrenades           int       `json:"min_grenades"`
	MinRounds             int       `json:"min_rounds"`
	MinUtilitySimilarity  float64   `json:"min_utility_similarity"`
	MinPositionSimilarity float64   `json:"min_position_similarity"`
	CommonShare           float64   `json:"common_share"`
	ExampleLimit          int       `json:"example_limit"`
	SkippedBuys           []BuyType `json:"skipped_buys"`
}

// DetectedPlay is a group of a team's rounds on one map and side that share
// their utility and positions. Seed is the round every member was compared to.
type DetectedPlay struct {
	Map       string         `json:"map"`
	Side      Side           `json:"side"`
	Kind      string         `json:"kind"`
	Site      Site           `json:"site"`
	Rounds    int            `json:"rounds"`
	Share     Rate           `json:"share"`
	WinRate   Rate           `json:"win_rate"`
	Grenades  []PlayGrenade  `json:"grenades"`
	Positions []PlayPosition `json:"positions"`
	Seed      Evidence       `json:"seed"`
	Examples  []Evidence     `json:"examples"`
}

// PlayDetection is the repeated plays of a team over the selected matches.
type PlayDetection struct {
	Thresholds PlayThresholds `json:"thresholds"`
	Matches    int            `json:"matches"`
	// Rounds is the team's rounds the filter kept; Compared is those with
	// enough utility and a buy worth comparing.
	Rounds   int            `json:"rounds"`
	Compared int            `json:"compared"`
	Plays    []DetectedPlay `json:"plays"`
}

// playRound is one round prepared for comparison.
type playRound struct {
	match    ScoutMatch
	round    Round
	side     Side
	won      bool
	utility  map[string]int
	spots    map[string]int
	grenades []PlayGrenade
	places   []PlayPosition
	order    int
}

func (p playRound) evidence() Evidence {
	return evidence(p.match, p.round, p.round.TickFreezeEnd)
}

// similar applies both thresholds to a pair of rounds.
func similar(a, b playRound) bool {
	if multisetSimilarity(a.utility, b.utility) < PlayMinUtilitySimilarity {
		return false
	}
	if len(a.spots) == 0 || len(b.spots) == 0 {
		return true
	}
	return multisetSimilarity(a.spots, b.spots) >= PlayMinPositionSimilarity
}

type playGroupKey struct {
	mapName string
	side    Side
}

// DetectPlays groups each map and side's rounds of the scouted team around
// seeds: the round with the most similar rounds left becomes a seed, its
// similar rounds join it, and the pass repeats until no seed has enough. Every
// member is similar to its seed, so a group cannot drift through a chain of
// near misses. Matches arrive in Select order, which breaks every tie.
func DetectPlays(matches []ScoutMatch, f ScoutFilter) PlayDetection {
	det := PlayDetection{
		Thresholds: PlayThresholds{
			PositionSeconds: PlayPositionSeconds, MinGrenades: PlayMinGrenades, MinRounds: PlayMinRounds,
			MinUtilitySimilarity: PlayMinUtilitySimilarity, MinPositionSimilarity: PlayMinPositionSimilarity,
			CommonShare: PlayCommonShare, ExampleLimit: PlayExampleLimit, SkippedBuys: []BuyType{BuyEco},
		},
		Plays: []DetectedPlay{},
	}
	selected := f.Select(matches)
	det.Matches = len(selected)
	groups := map[playGroupKey][]playRound{}
	totals := map[playGroupKey]int{}
	keys := []playGroupKey{}
	order := 0
	for _, m := range selected {
		for _, r := range f.rounds(m) {
			side := m.Document.TeamSide(m.TeamKey, r)
			key := playGroupKey{mapName: m.Document.Demo.Map, side: side}
			if _, ok := totals[key]; !ok {
				keys = append(keys, key)
			}
			totals[key]++
			det.Rounds++
			pr, ok := preparePlayRound(m, r, side)
			if !ok {
				continue
			}
			pr.order = order
			order++
			det.Compared++
			groups[key] = append(groups[key], pr)
		}
	}
	for _, key := range keys {
		det.Plays = append(det.Plays, seedPlays(groups[key], totals[key])...)
	}
	sort.SliceStable(det.Plays, func(i, j int) bool {
		a, b := det.Plays[i], det.Plays[j]
		if a.Rounds != b.Rounds {
			return a.Rounds > b.Rounds
		}
		if a.Map != b.Map {
			return a.Map < b.Map
		}
		return a.Side < b.Side
	})
	return det
}

func preparePlayRound(m ScoutMatch, r Round, side Side) (playRound, bool) {
	if r.Economy.Buy(side) == BuyEco {
		return playRound{}, false
	}
	grenades := playGrenades(m.Document, r, side)
	if len(grenades) < PlayMinGrenades {
		return playRound{}, false
	}
	pr := playRound{
		match: m, round: r, side: side, won: r.Winner == side,
		utility: map[string]int{}, spots: map[string]int{}, grenades: grenades,
	}
	for _, g := range grenades {
		pr.utility[string(g.Kind)+"@"+spotName(m.Document.Geometry, g.Pos)]++
	}
	if frame, ok := m.Setup[r.Number]; ok {
		pr.places = playPositions(m.Document, r, side, frame)
		for _, p := range pr.places {
			pr.spots[spotName(m.Document.Geometry, p.Pos)]++
		}
	}
	return pr, true
}

// seedPlays runs the seed pass over one map and side.
func seedPlays(rounds []playRound, total int) []DetectedPlay {
	n := len(rounds)
	neighbours := make([][]int, n)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if similar(rounds[i], rounds[j]) {
				neighbours[i] = append(neighbours[i], j)
				neighbours[j] = append(neighbours[j], i)
			}
		}
	}
	taken := make([]bool, n)
	out := []DetectedPlay{}
	for {
		seed, best := -1, 0
		for i := 0; i < n; i++ {
			if taken[i] {
				continue
			}
			free := 0
			for _, j := range neighbours[i] {
				if !taken[j] {
					free++
				}
			}
			if free > best {
				seed, best = i, free
			}
		}
		if seed < 0 || best+1 < PlayMinRounds {
			return out
		}
		members := []playRound{rounds[seed]}
		taken[seed] = true
		for _, j := range neighbours[seed] {
			if !taken[j] {
				taken[j] = true
				members = append(members, rounds[j])
			}
		}
		out = append(out, summarizePlay(members, total))
	}
}

// summarizePlay turns a seed and its members into the play they share.
func summarizePlay(members []playRound, total int) DetectedPlay {
	seed := members[0]
	sort.SliceStable(members[1:], func(i, j int) bool { return members[1+i].order < members[1+j].order })
	p := DetectedPlay{
		Map: seed.match.Document.Demo.Map, Side: seed.side, Rounds: len(members),
		Share: newRate(len(members), total), Seed: seed.evidence(), Examples: []Evidence{},
	}
	kinds, sites := map[string]int{}, map[string]int{}
	won := 0
	for _, m := range members {
		kinds[sidePattern(m.round, m.side)]++
		sites[string(m.round.Class.Site)]++
		if m.won {
			won++
		}
		if len(p.Examples) < PlayExampleLimit {
			p.Examples = append(p.Examples, m.evidence())
		}
	}
	p.Kind = mostCommon(kinds, sidePattern(seed.round, seed.side))
	p.Site = Site(mostCommon(sites, string(seed.round.Class.Site)))
	p.WinRate = newRate(won, len(members))
	p.Grenades = commonGrenades(members)
	p.Positions = commonPositions(members)
	return p
}

// mostCommon returns the most counted key; the seed's own value wins a tie,
// then the smallest key.
func mostCommon(counts map[string]int, seed string) string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	best, bestCount := seed, counts[seed]
	for _, k := range keys {
		if counts[k] > bestCount {
			best, bestCount = k, counts[k]
		}
	}
	return best
}

type commonAcc struct {
	rounds  int
	seconds []float64
	sum     [3]float64
	count   int
	names   map[string]int
	callout string
	kind    EventKind
}

func (c *commonAcc) add(pos [3]float64, name string) {
	c.count++
	for i := range pos {
		c.sum[i] += pos[i]
	}
	if name != "" {
		c.names[name]++
	}
}

func (c *commonAcc) mean() [3]float64 {
	var out [3]float64
	for i := range out {
		out[i] = math.Round(c.sum[i]/float64(c.count)*10) / 10
	}
	return out
}

func shareOf(rounds, total int) float64 {
	return math.Round(float64(rounds)/float64(total)*100) / 100
}

// commonGrenades lists the grenade tokens at least PlayCommonShare of the
// members threw, with their median second and mean landing point.
func commonGrenades(members []playRound) []PlayGrenade {
	accs := map[string]*commonAcc{}
	for _, m := range members {
		seen := map[string]bool{}
		for _, g := range m.grenades {
			spot := spotName(m.match.Document.Geometry, g.Pos)
			token := string(g.Kind) + "@" + spot
			acc, ok := accs[token]
			if !ok {
				acc = &commonAcc{names: map[string]int{}, kind: g.Kind, callout: g.Callout}
				accs[token] = acc
			}
			acc.add(g.Pos, g.Player)
			acc.seconds = append(acc.seconds, g.Seconds)
			if !seen[token] {
				seen[token] = true
				acc.rounds++
			}
		}
	}
	out := []PlayGrenade{}
	for _, acc := range accs {
		share := float64(acc.rounds) / float64(len(members))
		if share < PlayCommonShare {
			continue
		}
		out = append(out, PlayGrenade{
			Kind: acc.kind, Seconds: median(acc.seconds), Pos: acc.mean(), Callout: acc.callout,
			Player: mostCommon(acc.names, ""), Share: shareOf(acc.rounds, len(members)),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Seconds != out[j].Seconds {
			return out[i].Seconds < out[j].Seconds
		}
		if out[i].Kind != out[j].Kind {
			return utilityOrder[out[i].Kind] < utilityOrder[out[j].Kind]
		}
		return out[i].Callout < out[j].Callout
	})
	return out
}

// commonPositions lists the spots at least PlayCommonShare of the members
// held, by player when one player held it most often.
func commonPositions(members []playRound) []PlayPosition {
	accs := map[string]*commonAcc{}
	for _, m := range members {
		seen := map[string]bool{}
		for _, p := range m.places {
			spot := spotName(m.match.Document.Geometry, p.Pos)
			acc, ok := accs[spot]
			if !ok {
				acc = &commonAcc{names: map[string]int{}, callout: p.Callout}
				accs[spot] = acc
			}
			acc.add(p.Pos, p.Player)
			if !seen[spot] {
				seen[spot] = true
				acc.rounds++
			}
		}
	}
	out := []PlayPosition{}
	for _, acc := range accs {
		share := float64(acc.rounds) / float64(len(members))
		if share < PlayCommonShare {
			continue
		}
		out = append(out, PlayPosition{
			Player: mostCommon(acc.names, ""), Pos: acc.mean(), Callout: acc.callout, Share: shareOf(acc.rounds, len(members)),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Share != out[j].Share {
			return out[i].Share > out[j].Share
		}
		return out[i].Callout < out[j].Callout
	})
	return out
}
