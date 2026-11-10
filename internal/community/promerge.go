package community

import (
	"cmp"
	"fmt"
	"math"
	"slices"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// ProPart is one downloaded demo of a pro map: its header and its position blob.
type ProPart struct {
	Header ProAnalysisHeader
	Blob   []byte
}

const maxMergedSlots = 16

// MergeProParts joins the parts of a map split by a server restart into one
// analysis. A replayed round is kept from the later part, as the server's lineups do.
func MergeProParts(parts []ProPart) (ProAnalysisHeader, []byte, error) {
	if len(parts) == 0 {
		return ProAnalysisHeader{}, nil, fmt.Errorf("merge pro parts: no parts")
	}
	if len(parts) == 1 {
		return parts[0].Header, parts[0].Blob, nil
	}
	parts = slices.Clone(parts)
	slices.SortStableFunc(parts, func(a, b ProPart) int {
		return cmp.Compare(firstRound(a.Header.Document), firstRound(b.Header.Document))
	})
	owner := map[int]int{}
	for i, p := range parts {
		for _, r := range p.Header.Document.Rounds {
			if j, ok := owner[r.Number]; !ok || firstRound(p.Header.Document) > firstRound(parts[j].Header.Document) {
				owner[r.Number] = i
			}
		}
	}
	m := newProMerge(parts[0].Header)
	for i, p := range parts {
		if err := m.add(p, func(n int) bool { return owner[n] == i }); err != nil {
			return ProAnalysisHeader{}, nil, fmt.Errorf("merge part %s: %w", p.Header.Entry, err)
		}
		// Parts prepared at different times may hold different sidecars; the one with the veto wins, as on the server.
		if len(p.Header.Match.Veto) > len(m.header.Match.Veto) {
			m.header.Match = p.Header.Match
		}
	}
	return m.finish()
}

func firstRound(d tacticalplan.Document) int {
	first := math.MaxInt
	for _, r := range d.Rounds {
		first = min(first, r.Number)
	}
	return first
}

type proMerge struct {
	header ProAnalysisHeader
	doc    tacticalplan.Document
	frames []tacticalplan.RoundFrames
	bySID  map[string]uint8
	end    int
}

func newProMerge(base ProAnalysisHeader) *proMerge {
	doc := base.Document
	doc.Rounds = []tacticalplan.Round{}
	doc.Teams = slices.Clone(doc.Teams)
	for i := range doc.Teams {
		doc.Teams[i].Slots = slices.Clone(doc.Teams[i].Slots)
		doc.Teams[i].SteamIDs = slices.Clone(doc.Teams[i].SteamIDs)
	}
	doc.Players = slices.Clone(doc.Players)
	doc.Warnings = slices.Clone(doc.Warnings)
	// The merged map spans several demos, so no single .dem can attach to it.
	doc.Demo.SHA256 = ""
	m := &proMerge{header: base, doc: doc, bySID: map[string]uint8{}}
	for _, p := range doc.Players {
		m.bySID[playerKey(p)] = p.Slot
	}
	return m
}

func (m *proMerge) add(p ProPart, keep func(int) bool) error {
	doc := p.Header.Document
	teams := m.teamKeys(doc)
	slots := map[uint8]uint8{}
	for _, pl := range doc.Players {
		slot, ok := m.bySID[playerKey(pl)]
		if !ok {
			free, err := m.freeSlot()
			if err != nil {
				return err
			}
			slot = free
			m.bySID[playerKey(pl)] = slot
			m.addPlayer(pl, slot, teams[pl.TeamKey])
		}
		slots[pl.Slot] = slot
	}
	rounds := []tacticalplan.Round{}
	start := math.MaxInt
	for _, r := range doc.Rounds {
		if keep(r.Number) {
			rounds = append(rounds, r)
			start = min(start, r.TickStart)
		}
	}
	shift := 0
	if len(m.doc.Rounds) > 0 && start <= m.end {
		shift = m.end - start + int(doc.Demo.Tickrate*5)
	}
	rm := roundRemap{slots: slots, shift: shift}
	for _, r := range rounds {
		frames, err := partFrames(p, r.Number)
		if err != nil {
			return err
		}
		for i := range frames {
			frames[i].Tick += shift
			for j := range frames[i].Samples {
				frames[i].Samples[j].Slot = rm.slot(frames[i].Samples[j].Slot)
			}
			m.end = max(m.end, frames[i].Tick)
		}
		moved := remapRound(r, rm)
		m.end = max(m.end, moved.TickEnd)
		m.doc.Rounds = append(m.doc.Rounds, moved)
		m.frames = append(m.frames, tacticalplan.RoundFrames{Round: r.Number, Frames: frames})
	}
	m.doc.AnalysisVersion = min(m.doc.AnalysisVersion, doc.AnalysisVersion)
	m.doc.UtilityEffects = m.doc.UtilityEffects && doc.UtilityEffects
	m.doc.Demo.RegulationEnded = max(m.doc.Demo.RegulationEnded, doc.Demo.RegulationEnded)
	if doc.Geometry.SampleCount > m.doc.Geometry.SampleCount {
		m.doc.Geometry = doc.Geometry
	}
	if p.Header.Entry != m.header.Entry {
		m.doc.Warnings = append(m.doc.Warnings, doc.Warnings...)
	}
	return nil
}

// teamKeys maps each team key of a part to the merged team with the most shared players.
func (m *proMerge) teamKeys(doc tacticalplan.Document) map[string]string {
	out := map[string]string{}
	for _, t := range doc.Teams {
		best, shared := "", 0
		for _, mt := range m.doc.Teams {
			n := 0
			for _, sid := range t.SteamIDs {
				if slices.Contains(mt.SteamIDs, sid) {
					n++
				}
			}
			if n > shared || (n == shared && best == "" && mt.Key == t.Key) {
				best, shared = mt.Key, n
			}
		}
		out[t.Key] = best
	}
	return out
}

// playerKey is a player's SteamID, or the name for a bot that has none.
func playerKey(p tacticalplan.Player) string {
	if p.SteamID64 == "" || p.SteamID64 == "0" {
		return "name:" + p.Name
	}
	return p.SteamID64
}

func (m *proMerge) freeSlot() (uint8, error) {
	for s := range uint8(maxMergedSlots) {
		if !slices.ContainsFunc(m.doc.Players, func(p tacticalplan.Player) bool { return p.Slot == s }) {
			return s, nil
		}
	}
	return 0, fmt.Errorf("the parts field more than %d players", maxMergedSlots)
}

func (m *proMerge) addPlayer(p tacticalplan.Player, slot uint8, teamKey string) {
	p.Slot, p.TeamKey = slot, teamKey
	m.doc.Players = append(m.doc.Players, p)
	for i := range m.doc.Teams {
		if m.doc.Teams[i].Key == teamKey {
			m.doc.Teams[i].Slots = append(m.doc.Teams[i].Slots, slot)
			m.doc.Teams[i].SteamIDs = append(m.doc.Teams[i].SteamIDs, p.SteamID64)
		}
	}
}

func partFrames(p ProPart, round int) ([]tacticalplan.Frame, error) {
	desc := p.Header.Document.Positions
	off, ok := desc.Offset(round)
	if !ok || off.FrameCount == 0 {
		return []tacticalplan.Frame{}, nil
	}
	return tacticalplan.DecodeFrames(p.Blob, off.ByteOffset, off.FrameCount, desc)
}

func (m *proMerge) finish() (ProAnalysisHeader, []byte, error) {
	slices.SortFunc(m.doc.Rounds, func(a, b tacticalplan.Round) int { return cmp.Compare(a.Number, b.Number) })
	slices.SortFunc(m.frames, func(a, b tacticalplan.RoundFrames) int { return cmp.Compare(a.Round, b.Round) })
	sampleTicks := max(m.doc.Positions.SampleTicks, 1)
	blob, err := tacticalplan.EncodePositions(m.frames, sampleTicks, m.doc.Demo.Tickrate)
	if err != nil {
		return ProAnalysisHeader{}, nil, err
	}
	m.doc.Positions = blob.Descriptor
	m.doc.Demo.DurationTicks = max(m.doc.Demo.DurationTicks, m.end)
	m.header.Document, m.header.Positions = m.doc, blob.Descriptor
	return m.header, blob.Data, nil
}

type roundRemap struct {
	slots map[uint8]uint8
	shift int
}

func (rm roundRemap) slot(s uint8) uint8 {
	if v, ok := rm.slots[s]; ok {
		return v
	}
	return s
}

func (rm roundRemap) slotPtr(s *uint8) *uint8 {
	if s == nil {
		return nil
	}
	v := rm.slot(*s)
	return &v
}

func (rm roundRemap) tick(t int) int {
	if t == 0 {
		return 0
	}
	return t + rm.shift
}

// remapRound copies a round with its player slots renumbered and its ticks moved.
func remapRound(r tacticalplan.Round, rm roundRemap) tacticalplan.Round {
	r.TickStart, r.TickFreezeEnd, r.TickEnd, r.TickOfficial = r.TickStart+rm.shift, r.TickFreezeEnd+rm.shift, r.TickEnd+rm.shift, rm.tick(r.TickOfficial)
	r.Economy.SampleTick = rm.tick(r.Economy.SampleTick)
	if r.Bomb != nil {
		b := *r.Bomb
		b.PlantTick, b.DefuseTick, b.ExplodeTick = rm.tick(b.PlantTick), rm.tick(b.DefuseTick), rm.tick(b.ExplodeTick)
		b.PlanterSlot, b.DefuserSlot = rm.slotPtr(b.PlanterSlot), rm.slotPtr(b.DefuserSlot)
		r.Bomb = &b
	}
	r.Class.OpeningSlot = rm.slotPtr(r.Class.OpeningSlot)
	r.Class.OpeningTick, r.Class.FirstContactTick = rm.tick(r.Class.OpeningTick), rm.tick(r.Class.FirstContactTick)
	r.Class.Tags, r.Class.Reasons = slices.Clone(r.Class.Tags), slices.Clone(r.Class.Reasons)
	r.Players = slices.Clone(r.Players)
	for i := range r.Players {
		r.Players[i].Slot = rm.slot(r.Players[i].Slot)
		r.Players[i].DeathTick = rm.tick(r.Players[i].DeathTick)
	}
	r.Hits = slices.Clone(r.Hits)
	for i := range r.Hits {
		h := &r.Hits[i]
		h.Tick, h.ActorSlot, h.TargetSlot = h.Tick+rm.shift, rm.slot(h.ActorSlot), rm.slot(h.TargetSlot)
	}
	r.Events = slices.Clone(r.Events)
	for i := range r.Events {
		r.Events[i] = remapEvent(r.Events[i], rm)
	}
	return r
}

func remapEvent(e tacticalplan.Event, rm roundRemap) tacticalplan.Event {
	e.Tick, e.EndTick = e.Tick+rm.shift, rm.tick(e.EndTick)
	e.ActorSlot, e.TargetSlot = rm.slotPtr(e.ActorSlot), rm.slotPtr(e.TargetSlot)
	if e.Throw != nil {
		t := *e.Throw
		t.Tick = rm.tick(t.Tick)
		e.Throw = &t
	}
	if e.Effect != nil {
		eff := *e.Effect
		eff.Blinded = slices.Clone(eff.Blinded)
		for i := range eff.Blinded {
			eff.Blinded[i].Slot = rm.slot(eff.Blinded[i].Slot)
		}
		e.Effect = &eff
	}
	return e
}
