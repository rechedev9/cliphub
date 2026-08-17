package tactical

import (
	"fmt"
	"sort"

	"github.com/rechedev9/cliphub/internal/radarmap"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// derivedRadarSize is the pixel resolution used when a map has no shipped
// calibration. It matches the size CS2 draws its own overviews at.
const derivedRadarSize = 1024

// derivedRadarMargin keeps the outermost observed positions off the very edge
// of a derived radar.
const derivedRadarMargin = 256

// finish turns the accumulated scan state into the document and the position
// blob. Slots are renumbered here, once, so that a document is byte-identical
// across runs of the same demo regardless of the order players first appeared.
func (s *scanner) finish() (Result, error) {
	s.closeCurrent(s.maxTick)

	tickrate := s.tickrateOrDefault()
	if tickrate <= 0 {
		return Result{}, fmt.Errorf("demo reports no tick rate")
	}
	if !s.matchStarted {
		s.warn("demo has no MatchStart; every non-warmup round was counted")
	}
	if s.sampleGaps > 0 {
		s.warn("%d position samples arrived later than the %d-tick interval", s.sampleGaps, s.sampleTicks)
	}
	if len(s.rounds) == 0 {
		return Result{}, fmt.Errorf("demo contains no completed rounds")
	}

	remap := s.renumberSlots()
	s.remapFrames(remap)
	doc := tacticalplan.NewDocument()
	doc.JobID = s.opts.JobID
	doc.Players = s.documentPlayers(remap)
	doc.Teams = s.documentTeams(remap)

	geometry := s.buildGeometry()
	sites := buildSiteMap(s.rounds, geometry)

	rounds := make([]tacticalplan.Round, 0, len(s.rounds))
	for i, acc := range s.rounds {
		round := s.buildRound(acc, remap)
		var previous *tacticalplan.Round
		if i > 0 {
			previous = &rounds[i-1]
		}
		round.Economy.CTBuy, round.Economy.TBuy = classifyEconomy(round, previous)
		round.Class = classifyRound(classifyInput{
			round:    round,
			acc:      acc,
			tickrate: tickrate,
			sites:    sites,
		})
		rounds = append(rounds, round)
	}
	doc.Rounds = rounds
	doc.Geometry = geometry

	blob, err := s.encodePositions(tickrate)
	if err != nil {
		return Result{}, err
	}
	doc.Positions = blob.Descriptor

	doc.Demo = tacticalplan.Demo{
		Path:            s.opts.DemoPath,
		SHA256:          s.opts.SHA256,
		Map:             s.mapNameOrDefault(),
		Tickrate:        tickrate,
		DurationTicks:   s.maxTick,
		Format:          s.demoFormat(),
		MaxRounds:       s.regulationLength(),
		OvertimeRounds:  s.overtimeLength(),
		RegulationEnded: s.regulationEndRound(),
	}
	doc.Warnings = s.warnings
	return Result{Document: doc, Positions: blob}, nil
}

// renumberSlots returns a mapping from the provisional slot assigned during the
// scan to the final one. Final slots run CT starters first, then T starters,
// each ordered by SteamID, which makes the identity table readable and stable.
func (s *scanner) renumberSlots() map[uint8]uint8 {
	ordered := make([]*playerAcc, len(s.players))
	copy(ordered, s.players)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].startSide != ordered[j].startSide {
			return ordered[i].startSide == tacticalplan.SideCT
		}
		return ordered[i].steamID < ordered[j].steamID
	})
	remap := make(map[uint8]uint8, len(ordered))
	for i, acc := range ordered {
		remap[acc.slot] = uint8(i)
	}
	return remap
}

func (s *scanner) documentPlayers(remap map[uint8]uint8) []tacticalplan.Player {
	players := make([]tacticalplan.Player, 0, len(s.players))
	for _, acc := range s.players {
		players = append(players, tacticalplan.Player{
			Slot:      remap[acc.slot],
			SteamID64: steamIDString(acc.steamID),
			Name:      acc.name,
			TeamKey:   teamKeyForStartSide(acc.startSide, 1),
			StartSide: acc.startSide,
		})
	}
	sort.Slice(players, func(i, j int) bool { return players[i].Slot < players[j].Slot })
	return players
}

func (s *scanner) documentTeams(remap map[uint8]uint8) []tacticalplan.Team {
	keys := make([]string, 0, len(s.teams))
	for key := range s.teams {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	teams := make([]tacticalplan.Team, 0, len(keys))
	for _, key := range keys {
		acc := s.teams[key]
		team := tacticalplan.Team{
			Key:       acc.key,
			Name:      s.teamDisplayName(acc),
			StartSide: acc.startSide,
		}
		for _, player := range s.players {
			if _, ok := acc.members[player.steamID]; !ok {
				continue
			}
			team.Slots = append(team.Slots, remap[player.slot])
			team.SteamIDs = append(team.SteamIDs, steamIDString(player.steamID))
		}
		sort.Slice(team.Slots, func(i, j int) bool { return team.Slots[i] < team.Slots[j] })
		sort.Strings(team.SteamIDs)
		teams = append(teams, team)
	}
	return teams
}

func (s *scanner) teamDisplayName(acc *teamAcc) string {
	// Clan names are read from the side a team was on in the first half, which
	// is the half the key is defined by.
	side := acc.startSide
	for team, name := range s.clanNames {
		if sideOf(team) == side && name != "" {
			return name
		}
	}
	if side == tacticalplan.SideCT {
		return "CT start"
	}
	return "T start"
}

func (s *scanner) buildRound(acc *roundAcc, remap map[uint8]uint8) tacticalplan.Round {
	round := tacticalplan.Round{
		Number:        acc.number,
		TickStart:     acc.tickStart,
		TickFreezeEnd: acc.tickFreezeEnd,
		TickEnd:       acc.tickEnd,
		TickOfficial:  acc.tickOfficial,
		ScoreCTBefore: acc.scoreCT,
		ScoreTBefore:  acc.scoreT,
		Winner:        acc.winner,
		EndReason:     acc.endReason,
		Half:          acc.half,
		Overtime:      acc.overtime,
		Economy:       acc.economy,
		Events:        make([]tacticalplan.Event, 0, len(acc.events)),
		Players:       make([]tacticalplan.PlayerRound, 0, len(acc.stats)),
	}
	if acc.bomb != nil {
		bomb := *acc.bomb
		bomb.PlanterSlot = remapSlotPtr(bomb.PlanterSlot, remap)
		bomb.DefuserSlot = remapSlotPtr(bomb.DefuserSlot, remap)
		round.Bomb = &bomb
	}

	openingSlot, openingDeathSlot, openingTick, openingSide, openingTraded := openingDuel(acc)
	for _, ev := range acc.events {
		ev.ActorSlot = remapSlotPtr(ev.ActorSlot, remap)
		ev.TargetSlot = remapSlotPtr(ev.TargetSlot, remap)
		if ev.Kind == tacticalplan.EventKill && ev.Tick == openingTick {
			ev.Opening = true
		}
		round.Events = append(round.Events, ev)
	}
	for i := range acc.kills {
		if !acc.kills[i].traded {
			continue
		}
		tick := acc.kills[i].tick
		for j := range round.Events {
			if round.Events[j].Kind == tacticalplan.EventKill && round.Events[j].Tick == tick {
				round.Events[j].Traded = true
			}
		}
	}
	sort.SliceStable(round.Events, func(i, j int) bool { return round.Events[i].Tick < round.Events[j].Tick })

	slots := make([]uint8, 0, len(acc.stats))
	for slot := range acc.stats {
		slots = append(slots, slot)
	}
	sort.Slice(slots, func(i, j int) bool { return remap[slots[i]] < remap[slots[j]] })
	for _, slot := range slots {
		stats := acc.stats[slot]
		final := remap[slot]
		round.Players = append(round.Players, tacticalplan.PlayerRound{
			Slot:         final,
			Side:         acc.sides[slot],
			Kills:        stats.kills,
			Deaths:       stats.deaths,
			Assists:      stats.assists,
			Damage:       stats.damage,
			EquipValue:   stats.equipValue,
			Money:        stats.money,
			DeathTick:    stats.deathTick,
			Survived:     stats.deaths == 0,
			OpeningKill:  openingSlot != nil && *openingSlot == final,
			OpeningDeath: openingDeathSlot != nil && *openingDeathSlot == final,
			Traded:       stats.traded,
			TradeKills:   stats.tradeKills,
		})
	}

	// The classifier fills in the rest; the opening duel is a fact, not a
	// judgement, so it is recorded here.
	round.Class = tacticalplan.Class{
		OpeningSlot:   openingSlot,
		OpeningSide:   openingSide,
		OpeningTick:   openingTick,
		OpeningTraded: openingTraded,
	}
	return round
}

// openingDuel returns the round's first non-team kill. Its slots are already
// final: remapFrames rewrote the kill records before any round was built.
func openingDuel(acc *roundAcc) (*uint8, *uint8, int, tacticalplan.Side, bool) {
	for i := range acc.kills {
		k := acc.kills[i]
		if k.teamKill {
			continue
		}
		victim := k.victimSlot
		var killer *uint8
		side := k.victimSide.Opponent()
		if k.killerSlot != nil {
			slot := *k.killerSlot
			killer = &slot
			side = k.killerSide
		}
		return killer, &victim, k.tick, side, k.traded
	}
	return nil, nil, 0, "", false
}

func remapSlotPtr(slot *uint8, remap map[uint8]uint8) *uint8 {
	if slot == nil {
		return nil
	}
	mapped := remap[*slot]
	return &mapped
}

func (s *scanner) buildGeometry() tacticalplan.MapGeometry {
	mapName := s.mapNameOrDefault()
	if s.occupancy == nil {
		s.warn("no player positions were sampled; the map geometry is empty")
		return tacticalplan.MapGeometry{Map: mapName, Source: tacticalplan.GeometrySourceOccupancy}
	}
	cal, ok := radarmap.Lookup(mapName)
	if !ok {
		derived, err := radarmap.DeriveCalibration(mapName, s.occupancy.Bounds(), derivedRadarSize, derivedRadarMargin)
		if err != nil {
			s.warn("map %q has no calibration and none could be derived: %v", mapName, err)
		} else {
			cal = derived
			s.warn("map %q has no shipped radar calibration; framing was derived from observed play and is not comparable across demos", mapName)
		}
	}
	return s.occupancy.Build(mapName, cal, 1)
}

func (s *scanner) encodePositions(tickrate float64) (tacticalplan.Blob, error) {
	rounds := make([]tacticalplan.RoundFrames, 0, len(s.rounds))
	for _, acc := range s.rounds {
		rounds = append(rounds, tacticalplan.RoundFrames{Round: acc.number, Frames: acc.frames})
	}
	sampleTicks := s.sampleTicks
	if sampleTicks <= 0 {
		sampleTicks = 1
	}
	blob, err := tacticalplan.EncodePositions(rounds, sampleTicks, tickrate)
	if err != nil {
		return tacticalplan.Blob{}, fmt.Errorf("encode positions: %w", err)
	}
	return blob, nil
}

// remapFrames rewrites the provisional slots inside the sampled frames and the
// kill records. It runs before encoding and before classification so the blob,
// the document, and every recorded reason agree on what a slot means.
func (s *scanner) remapFrames(remap map[uint8]uint8) {
	for _, acc := range s.rounds {
		for i := range acc.kills {
			acc.kills[i].victimSlot = remap[acc.kills[i].victimSlot]
			if acc.kills[i].killerSlot != nil {
				mapped := remap[*acc.kills[i].killerSlot]
				acc.kills[i].killerSlot = &mapped
			}
		}
		for i := range acc.frames {
			for j := range acc.frames[i].Samples {
				acc.frames[i].Samples[j].Slot = remap[acc.frames[i].Samples[j].Slot]
			}
			sort.Slice(acc.frames[i].Samples, func(a, b int) bool {
				return acc.frames[i].Samples[a].Slot < acc.frames[i].Samples[b].Slot
			})
		}
	}
}

// demoFormat reports the demo generation. The parser this package builds on
// only reads Source 2 recordings, so a demo that got this far is a CS2 demo.
func (s *scanner) demoFormat() string { return "cs2" }

// regulationLength reports the detected regulation round count: twice the
// number of rounds played in the first half.
func (s *scanner) regulationLength() int {
	first := 0
	for _, acc := range s.rounds {
		if acc.overtime == 0 && acc.half == 1 {
			first++
		}
	}
	if first == 0 {
		return 0
	}
	return first * 2
}

func (s *scanner) overtimeLength() int {
	halves := map[int]int{}
	for _, acc := range s.rounds {
		if acc.overtime > 0 {
			halves[acc.half]++
		}
	}
	longest := 0
	for _, n := range halves {
		if n > longest {
			longest = n
		}
	}
	return longest
}

func (s *scanner) regulationEndRound() int {
	last := 0
	for _, acc := range s.rounds {
		if acc.overtime == 0 {
			last = acc.number
		}
	}
	return last
}
