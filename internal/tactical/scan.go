// Package tactical scans a CS2 demo into the durable tactical document: the
// round index with its economy and deterministic classification, the per-round
// event list, the sampled position stream, and the map geometry derived from
// where players actually walked.
//
// Nothing here infers anything from rendered video and nothing calls a model:
// the demo is the only source, and every judgement the classifier makes is
// recorded as a reason on the round it applies to.
package tactical

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"sync"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"

	"github.com/google/uuid"

	"github.com/rechedev9/tickcut/internal/parser"
	"github.com/rechedev9/tickcut/internal/radarmap"
	"github.com/rechedev9/tickcut/internal/tacticalplan"
)

// DefaultSampleHZ is the position sampling rate. At 125 ms between samples a
// sprinting player moves about six pixels on a 1024 px radar, which is smoother
// than a 2D replay needs, and a full match still fits in a couple of megabytes.
const DefaultSampleHZ = 8

// MaxSampleHZ bounds the sampling rate. Higher rates multiply the blob size
// without changing what an analyst can see on a radar.
const MaxSampleHZ = 64

// DefaultCellSize is the occupancy grid resolution in world units. 64 units is
// roughly a player's shoulder width times two: fine enough to show corridors,
// coarse enough that a single match fills the walkable space.
const DefaultCellSize = 64

// Options configures a scan.
type Options struct {
	DemoPath string
	SHA256   string
	JobID    uuid.UUID
	// SampleHZ is the position sampling rate; zero selects DefaultSampleHZ.
	SampleHZ float64
	// CellSize is the occupancy grid resolution; zero selects DefaultCellSize.
	CellSize float64
	// MapName overrides the map read from the demo header, for demos that do
	// not carry one.
	MapName string
}

func (o Options) sampleHZ() (float64, error) {
	hz := o.SampleHZ
	if hz == 0 {
		hz = DefaultSampleHZ
	}
	if hz <= 0 || hz > MaxSampleHZ {
		return 0, fmt.Errorf("sample rate %v Hz must be in (0, %d]", hz, MaxSampleHZ)
	}
	return hz, nil
}

// Result is a completed scan: the document plus the position blob it describes.
type Result struct {
	Document  tacticalplan.Document
	Positions tacticalplan.Blob
}

// ScanFile opens a demo, scans it, and fills in the demo path and checksum.
func ScanFile(ctx context.Context, path string, opts Options) (Result, error) {
	sum, err := fileSHA256(path)
	if err != nil {
		return Result{}, err
	}
	// #nosec G304 -- the demo path is an explicit local input from the CLI or a job payload.
	f, err := os.Open(path)
	if err != nil {
		return Result{}, fmt.Errorf("open demo %q: %w", path, err)
	}
	defer f.Close()

	p := demoinfocs.NewParser(f)
	defer p.Close()

	opts.DemoPath = path
	if opts.SHA256 == "" {
		opts.SHA256 = sum
	}
	return ScanWithContext(ctx, p, opts)
}

// ScanWithContext drives Scan but aborts parsing when ctx is cancelled,
// mirroring the parser package: a watcher goroutine calls p.Cancel() and is
// joined before return, so Close() never races a Cancel() in flight.
func ScanWithContext(ctx context.Context, p demoinfocs.Parser, opts Options) (Result, error) {
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		select {
		case <-ctx.Done():
			p.Cancel()
		case <-stop:
		}
	})
	defer func() {
		close(stop)
		wg.Wait()
	}()

	result, err := Scan(p, opts)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Result{}, fmt.Errorf("scan tactical: %w", ctxErr)
	}
	return result, err
}

// Scan wires the event handlers on p, drives the parser to completion, and
// returns the tactical document and its position blob.
func Scan(p demoinfocs.Parser, opts Options) (Result, error) {
	hz, err := opts.sampleHZ()
	if err != nil {
		return Result{}, err
	}
	s := newScanner(p, opts, hz)
	s.register()

	if err := parser.ParseToEnd(p); err != nil {
		return Result{}, fmt.Errorf("parsing demo: %w", err)
	}
	return s.finish()
}

type scanner struct {
	p    demoinfocs.Parser
	opts Options
	hz   float64

	mapName     string
	tickrate    float64
	sampleTicks int
	maxTick     int

	slots     map[uint64]uint8
	players   []*playerAcc
	teams     map[string]*teamAcc
	clanNames map[common.Team]string

	rounds       []*roundAcc
	current      *roundAcc
	half         int
	matchStarted bool

	lastEmittedTick int
	sampleGaps      int

	occupancy *tacticalplan.OccupancyBuilder
	warnings  []string
}

type playerAcc struct {
	steamID   uint64
	slot      uint8
	name      string
	startSide tacticalplan.Side
}

type teamAcc struct {
	key       string
	startSide tacticalplan.Side
	members   map[uint64]struct{}
}

type roundAcc struct {
	number        int
	tickStart     int
	tickFreezeEnd int
	tickEnd       int
	tickOfficial  int
	scoreCT       int
	scoreT        int
	winner        tacticalplan.Side
	endReason     string
	half          int
	overtime      int

	bomb    *tacticalplan.Bomb
	economy tacticalplan.Economy
	events  []tacticalplan.Event
	frames  []tacticalplan.Frame

	kills          []killRecord
	stats          map[uint8]*playerRoundAcc
	sides          map[uint8]tacticalplan.Side
	economySampled bool
}

type killRecord struct {
	tick       int
	killerSlot *uint8
	victimSlot uint8
	killerSide tacticalplan.Side
	victimSide tacticalplan.Side
	traded     bool
	pos        [3]float64
	teamKill   bool
}

type playerRoundAcc struct {
	kills      int
	deaths     int
	assists    int
	damage     int
	equipValue int
	money      int
	deathTick  int
	tradeKills int
	traded     bool
}

func newScanner(p demoinfocs.Parser, opts Options, hz float64) *scanner {
	return &scanner{
		p:               p,
		opts:            opts,
		hz:              hz,
		slots:           map[uint64]uint8{},
		teams:           map[string]*teamAcc{},
		clanNames:       map[common.Team]string{},
		half:            1,
		lastEmittedTick: math.MinInt32,
	}
}

func (s *scanner) register() {
	p := s.p

	p.RegisterNetMessageHandler(func(info *msg.CSVCMsg_ServerInfo) {
		if name := info.GetMapName(); name != "" {
			s.mapName = name
		}
	})

	p.RegisterEventHandler(func(events.MatchStart) {
		// Everything before the match start is warmup or a knife round; a demo
		// that never announces one counts from its first round, the same
		// convention the roster scan uses.
		if len(s.rounds) > 0 {
			s.warn("discarded %d pre-match rounds at MatchStart", len(s.rounds))
		}
		s.rounds = nil
		s.current = nil
		s.half = 1
		s.matchStarted = true
	})

	p.RegisterEventHandler(func(events.GameHalfEnded) {
		s.half++
	})

	p.RegisterEventHandler(func(e events.TeamClanNameUpdated) {
		if e.TeamState == nil {
			return
		}
		if name := e.TeamState.ClanName(); name != "" {
			s.clanNames[e.TeamState.Team()] = name
		}
	})

	p.RegisterEventHandler(func(events.RoundStart) {
		gs := s.p.GameState()
		if gs.IsWarmupPeriod() {
			return
		}
		s.closeCurrent(s.tick())
		s.current = &roundAcc{
			number:    len(s.rounds) + 1,
			tickStart: s.tick(),
			scoreCT:   gs.TeamCounterTerrorists().Score(),
			scoreT:    gs.TeamTerrorists().Score(),
			half:      s.half,
			overtime:  gs.OvertimeCount(),
			stats:     map[uint8]*playerRoundAcc{},
			sides:     map[uint8]tacticalplan.Side{},
		}
		s.lastEmittedTick = math.MinInt32
	})

	p.RegisterEventHandler(func(events.RoundFreezetimeEnd) {
		if s.current == nil {
			return
		}
		s.current.tickFreezeEnd = s.tick()
	})

	p.RegisterEventHandler(func(e events.RoundEnd) {
		if s.current == nil {
			return
		}
		s.current.tickEnd = s.tick()
		s.current.winner = sideOf(e.Winner)
		s.current.endReason = endReasonSlug(e.Reason)
		if !s.current.economySampled {
			s.sampleEconomy()
		}
	})

	p.RegisterEventHandler(func(events.RoundEndOfficial) {
		s.closeCurrent(s.tick())
	})

	p.RegisterEventHandler(func(e events.Kill) { s.onKill(e) })
	p.RegisterEventHandler(func(e events.PlayerHurt) { s.onHurt(e) })
	p.RegisterEventHandler(func(e events.BombPlanted) { s.onPlant(e) })
	p.RegisterEventHandler(func(e events.BombDefused) { s.onDefuse(e) })
	p.RegisterEventHandler(func(e events.BombExplode) { s.onExplode(e) })

	p.RegisterEventHandler(func(e events.SmokeStart) {
		s.addUtility(tacticalplan.EventSmoke, e.GrenadeEvent)
	})
	p.RegisterEventHandler(func(e events.FlashExplode) {
		s.addUtility(tacticalplan.EventFlash, e.GrenadeEvent)
	})
	p.RegisterEventHandler(func(e events.HeExplode) {
		s.addUtility(tacticalplan.EventHE, e.GrenadeEvent)
	})
	p.RegisterEventHandler(func(e events.DecoyStart) {
		s.addUtility(tacticalplan.EventDecoy, e.GrenadeEvent)
	})
	p.RegisterEventHandler(func(e events.InfernoStart) {
		if e.Inferno == nil {
			return
		}
		thrower := e.Inferno.Thrower()
		pos := e.Inferno.Entity.Position()
		s.addEvent(tacticalplan.Event{
			Kind:   tacticalplan.EventMolotov,
			Tick:   s.tick(),
			Pos:    [3]float64{pos.X, pos.Y, pos.Z},
			Weapon: "molotov",
			Side:   sideOfPlayer(thrower),
		}, thrower)
	})

	p.RegisterEventHandler(func(events.FrameDone) { s.onFrame() })
}

func (s *scanner) tick() int {
	tick := s.p.GameState().IngameTick()
	if tick > s.maxTick {
		s.maxTick = tick
	}
	return tick
}

func (s *scanner) warn(format string, args ...any) {
	s.warnings = append(s.warnings, fmt.Sprintf(format, args...))
}

func (s *scanner) closeCurrent(tick int) {
	if s.current == nil {
		return
	}
	if s.current.tickEnd == 0 {
		s.current.tickEnd = tick
	}
	s.current.tickOfficial = tick
	s.rounds = append(s.rounds, s.current)
	s.current = nil
}

// slotFor assigns a stable index to a player the first time it is seen. The
// indices are provisional: finish() renumbers them so slot order follows the
// starting sides, which is what makes two scans of the same demo identical.
func (s *scanner) slotFor(p *common.Player) (uint8, bool) {
	if p == nil || p.SteamID64 == 0 {
		return 0, false
	}
	side := sideOf(p.Team)
	if slot, ok := s.slots[p.SteamID64]; ok {
		acc := s.players[slot]
		if p.Name != "" {
			acc.name = p.Name
		}
		return slot, true
	}
	if side == "" {
		return 0, false
	}
	if len(s.players) >= 16 {
		s.warn("more than 16 players seen; %q was ignored", p.Name)
		return 0, false
	}
	// #nosec G115 -- the hard 16-player bound above makes this conversion exact.
	slot := uint8(len(s.players))
	s.slots[p.SteamID64] = slot
	s.players = append(s.players, &playerAcc{
		steamID:   p.SteamID64,
		slot:      slot,
		name:      p.Name,
		startSide: side,
	})
	s.recordTeamMember(p, side)
	return slot, true
}

func (s *scanner) recordTeamMember(p *common.Player, side tacticalplan.Side) {
	// A team is identified by the side it started on, because that is the only
	// identity that survives the half-time swap. Clan names, when the demo has
	// them, become the display name.
	key := teamKeyForStartSide(side, s.half)
	team, ok := s.teams[key]
	if !ok {
		team = &teamAcc{key: key, startSide: startSideForKey(key), members: map[uint64]struct{}{}}
		s.teams[key] = team
	}
	team.members[p.SteamID64] = struct{}{}
}

func (s *scanner) onFrame() {
	if s.current == nil {
		return
	}
	tick := s.tick()
	s.sampleEconomyIfDue(tick)

	if s.sampleTicks <= 0 {
		// The tick rate is only known once the demo header has been read, so
		// the sampling interval is resolved on the first frame that can answer.
		rate := s.tickrateOrDefault()
		if rate <= 0 {
			return
		}
		s.sampleTicks = int(math.Round(rate / s.hz))
		if s.sampleTicks < 1 {
			s.sampleTicks = 1
		}
	}

	// Emitting on an elapsed-tick threshold rather than a modulo keeps the
	// stream regular even when the demo skips or repeats frames, which CS2
	// GOTV recordings do around round transitions.
	if s.lastEmittedTick != math.MinInt32 {
		gap := tick - s.lastEmittedTick
		if gap < s.sampleTicks {
			return
		}
		if gap > s.sampleTicks && s.sampleTicks > 0 {
			s.sampleGaps++
		}
	}
	s.lastEmittedTick = tick

	gs := s.p.GameState()
	frame := tacticalplan.Frame{Tick: tick}
	for _, pl := range gs.Participants().All() {
		slot, ok := s.slotFor(pl)
		if !ok {
			continue
		}
		side := sideOf(pl.Team)
		if side == "" {
			continue
		}
		pos := pl.Position()
		if pos.X == 0 && pos.Y == 0 && pos.Z == 0 && !pl.IsAlive() {
			// A dead player with no pawn has no position to plot.
			continue
		}
		s.current.sides[slot] = side
		frame.Samples = append(frame.Samples, tacticalplan.Sample{
			Slot:   slot,
			X:      pos.X,
			Y:      pos.Y,
			Z:      pos.Z,
			Yaw:    float64(pl.ViewDirectionX()),
			Health: pl.Health(),
			Flags:  sampleFlags(pl, side),
		})
		if pl.IsAlive() {
			s.occupancyAdd(pos.X, pos.Y, pos.Z, pl.LastPlaceName())
		}
	}
	if len(frame.Samples) > 0 {
		sort.Slice(frame.Samples, func(i, j int) bool { return frame.Samples[i].Slot < frame.Samples[j].Slot })
		s.current.frames = append(s.current.frames, frame)
	}
}

func (s *scanner) occupancyAdd(x, y, z float64, place string) {
	if s.occupancy == nil {
		cal, ok := radarmap.Lookup(s.mapNameOrDefault())
		if !ok {
			// An uncalibrated map still gets one level; finish() installs a
			// calibration derived from the observed bounds.
			cal = radarmap.Calibration{Map: s.mapNameOrDefault()}
		}
		cellSize := s.opts.CellSize
		if cellSize <= 0 {
			cellSize = DefaultCellSize
		}
		s.occupancy = tacticalplan.NewOccupancyBuilder(cal, cellSize)
	}
	s.occupancy.Add(x, y, z, place)
}

func (s *scanner) mapNameOrDefault() string {
	if s.opts.MapName != "" {
		return s.opts.MapName
	}
	return s.mapName
}

func (s *scanner) sampleEconomyIfDue(tick int) {
	r := s.current
	if r == nil || r.economySampled || r.tickFreezeEnd == 0 || s.tickrateOrDefault() <= 0 {
		return
	}
	if float64(tick-r.tickFreezeEnd) < economyDelaySeconds*s.tickrateOrDefault() {
		return
	}
	s.sampleEconomy()
}

// economyDelaySeconds is how long after freeze-time end the equipment value is
// read. Players keep buying for a few seconds past the freeze, and by the end
// of the buy window they have already thrown utility and died, so both edges
// misreport what the team actually took into the round.
const economyDelaySeconds = 7

func (s *scanner) sampleEconomy() {
	r := s.current
	if r == nil || r.economySampled {
		return
	}
	r.economySampled = true
	r.economy.SampleTick = s.tick()
	for _, pl := range s.p.GameState().Participants().All() {
		slot, ok := s.slotFor(pl)
		if !ok {
			continue
		}
		side := sideOf(pl.Team)
		if side == "" {
			continue
		}
		value := pl.EquipmentValueCurrent()
		money := pl.Money()
		acc := r.playerStats(slot)
		acc.equipValue = value
		acc.money = money
		r.sides[slot] = side
		if side == tacticalplan.SideCT {
			r.economy.CTEquipValue += value
			r.economy.CTMoney += money
		} else {
			r.economy.TEquipValue += value
			r.economy.TMoney += money
		}
	}
}

func (r *roundAcc) playerStats(slot uint8) *playerRoundAcc {
	acc, ok := r.stats[slot]
	if !ok {
		acc = &playerRoundAcc{}
		r.stats[slot] = acc
	}
	return acc
}

func (s *scanner) onKill(e events.Kill) {
	r := s.current
	if r == nil || e.Victim == nil {
		return
	}
	tick := s.tick()
	victimSlot, ok := s.slotFor(e.Victim)
	if !ok {
		return
	}
	victimSide := sideOf(e.Victim.Team)
	rec := killRecord{
		tick:       tick,
		victimSlot: victimSlot,
		victimSide: victimSide,
		pos:        parser.PlayerPosition(e.Victim),
	}
	// The event's place is where the shot came from when there is a killer, and
	// where the victim fell otherwise (world damage, a disconnect).
	place := e.Victim.LastPlaceName()
	if killerSlot, ok := s.slotFor(e.Killer); ok {
		rec.killerSlot = &killerSlot
		rec.killerSide = sideOf(e.Killer.Team)
		rec.teamKill = rec.killerSide == victimSide
		rec.pos = parser.PlayerPosition(e.Killer)
		place = e.Killer.LastPlaceName()
	}

	// A trade is the reverse kill: this killer is the player who killed a
	// teammate of theirs within the window. Identity is matched on slot, never
	// on name, so a mid-match name change cannot break it.
	if rec.killerSlot != nil && !rec.teamKill {
		window := int(tradeWindowSeconds * s.tickrateOrDefault())
		for i := range r.kills {
			prev := &r.kills[i]
			if prev.teamKill || prev.killerSlot == nil {
				continue
			}
			if *prev.killerSlot != victimSlot || tick-prev.tick > window {
				continue
			}
			if prev.victimSide != rec.killerSide {
				continue
			}
			prev.traded = true
			s.markTraded(r, prev.victimSlot)
			r.playerStats(*rec.killerSlot).tradeKills++
		}
	}

	r.kills = append(r.kills, rec)

	victimStats := r.playerStats(victimSlot)
	victimStats.deaths++
	victimStats.deathTick = tick
	r.sides[victimSlot] = victimSide
	if rec.killerSlot != nil && !rec.teamKill {
		killerStats := r.playerStats(*rec.killerSlot)
		killerStats.kills++
		r.sides[*rec.killerSlot] = rec.killerSide
	}
	if assistSlot, ok := s.slotFor(e.Assister); ok && sideOf(e.Assister.Team) != victimSide {
		r.playerStats(assistSlot).assists++
	}

	event := tacticalplan.Event{
		Tick:          tick,
		Kind:          tacticalplan.EventKill,
		TargetSlot:    &victimSlot,
		Weapon:        parser.WeaponName(e.Weapon),
		Pos:           rec.pos,
		TargetPos:     parser.PlayerPosition(e.Victim),
		Place:         place,
		Headshot:      e.IsHeadshot,
		Wallbang:      e.PenetratedObjects > 0,
		ThroughSmoke:  e.ThroughSmoke,
		AttackerBlind: e.AttackerBlind,
		NoScope:       e.NoScope,
	}
	if rec.killerSlot != nil {
		slot := *rec.killerSlot
		event.ActorSlot = &slot
		event.Side = rec.killerSide
	} else {
		event.Side = victimSide.Opponent()
	}
	r.events = append(r.events, event)
}

func (s *scanner) markTraded(r *roundAcc, slot uint8) {
	r.playerStats(slot).traded = true
}

// tradeWindowSeconds matches the window the rest of the product already uses
// for KAST, so two views of the same demo never disagree about what a trade is.
const tradeWindowSeconds = 5.0

func (s *scanner) onHurt(e events.PlayerHurt) {
	r := s.current
	if r == nil || e.Player == nil || e.Attacker == nil {
		return
	}
	if e.Attacker.Team == e.Player.Team {
		return
	}
	slot, ok := s.slotFor(e.Attacker)
	if !ok {
		return
	}
	r.playerStats(slot).damage += e.HealthDamageTaken
}

func (s *scanner) onPlant(e events.BombPlanted) {
	r := s.current
	if r == nil {
		return
	}
	tick := s.tick()
	bomb := &tacticalplan.Bomb{PlantTick: tick, Site: siteFromBombsite(e.Site)}
	pos := parser.PlayerPosition(e.Player)
	if slot, ok := s.slotFor(e.Player); ok {
		bomb.PlanterSlot = &slot
	}
	r.bomb = bomb
	r.events = append(r.events, tacticalplan.Event{
		Tick:      tick,
		Kind:      tacticalplan.EventPlant,
		ActorSlot: bomb.PlanterSlot,
		Side:      tacticalplan.SideT,
		Pos:       pos,
		Site:      bomb.Site,
		Place:     placeOf(e.Player),
	})
}

func (s *scanner) onDefuse(e events.BombDefused) {
	r := s.current
	if r == nil {
		return
	}
	tick := s.tick()
	if r.bomb == nil {
		r.bomb = &tacticalplan.Bomb{Site: siteFromBombsite(e.Site)}
	}
	r.bomb.DefuseTick = tick
	if slot, ok := s.slotFor(e.Player); ok {
		r.bomb.DefuserSlot = &slot
	}
	r.events = append(r.events, tacticalplan.Event{
		Tick:      tick,
		Kind:      tacticalplan.EventDefuse,
		ActorSlot: r.bomb.DefuserSlot,
		Side:      tacticalplan.SideCT,
		Pos:       parser.PlayerPosition(e.Player),
		Site:      r.bomb.Site,
		Place:     placeOf(e.Player),
	})
}

func (s *scanner) onExplode(e events.BombExplode) {
	r := s.current
	if r == nil {
		return
	}
	tick := s.tick()
	if r.bomb == nil {
		r.bomb = &tacticalplan.Bomb{Site: siteFromBombsite(e.Site)}
	}
	r.bomb.ExplodeTick = tick
	r.events = append(r.events, tacticalplan.Event{
		Tick: tick,
		Kind: tacticalplan.EventExplode,
		Side: tacticalplan.SideT,
		Site: r.bomb.Site,
	})
}

func (s *scanner) addUtility(kind tacticalplan.EventKind, e events.GrenadeEvent) {
	s.addEvent(tacticalplan.Event{
		Kind:   kind,
		Tick:   s.tick(),
		Pos:    [3]float64{e.Position.X, e.Position.Y, e.Position.Z},
		Weapon: parser.WeaponName(e.Grenade),
		Side:   sideOfPlayer(e.Thrower),
		Place:  placeOf(e.Thrower),
	}, e.Thrower)
}

func (s *scanner) addEvent(ev tacticalplan.Event, actor *common.Player) {
	if s.current == nil {
		return
	}
	if slot, ok := s.slotFor(actor); ok {
		ev.ActorSlot = &slot
	}
	s.current.events = append(s.current.events, ev)
}

func (s *scanner) tickrateOrDefault() float64 {
	if s.tickrate > 0 {
		return s.tickrate
	}
	if rate := s.p.TickRate(); rate > 0 {
		s.tickrate = rate
		return rate
	}
	return 0
}

func sampleFlags(p *common.Player, side tacticalplan.Side) tacticalplan.SampleFlags {
	var flags tacticalplan.SampleFlags
	if p.IsAlive() {
		flags |= tacticalplan.FlagAlive
	}
	if p.IsBlinded() {
		flags |= tacticalplan.FlagBlinded
	}
	if p.IsDucking() {
		flags |= tacticalplan.FlagDucking
	}
	if p.IsScoped() {
		flags |= tacticalplan.FlagScoped
	}
	if p.IsAirborne() {
		flags |= tacticalplan.FlagAirborne
	}
	if side == tacticalplan.SideT {
		flags |= tacticalplan.FlagSideT
	}
	for _, w := range p.Weapons() {
		if w != nil && w.Type == common.EqBomb {
			flags |= tacticalplan.FlagHasBomb
			break
		}
	}
	return flags
}

func sideOf(t common.Team) tacticalplan.Side {
	switch parser.TeamLabel(t) {
	case "CT":
		return tacticalplan.SideCT
	case "T":
		return tacticalplan.SideT
	default:
		return ""
	}
}

func sideOfPlayer(p *common.Player) tacticalplan.Side {
	if p == nil {
		return ""
	}
	return sideOf(p.Team)
}

func placeOf(p *common.Player) string {
	if p == nil {
		return ""
	}
	return p.LastPlaceName()
}

func siteFromBombsite(site events.Bombsite) tacticalplan.Site {
	switch site {
	case events.BombsiteA:
		return tacticalplan.SiteA
	case events.BombsiteB:
		return tacticalplan.SiteB
	default:
		return tacticalplan.SiteNone
	}
}

func teamKeyForStartSide(side tacticalplan.Side, half int) string {
	// In the second half a player's current side is the opposite of the side
	// their team started on.
	if half%2 == 0 {
		side = side.Opponent()
	}
	if side == tacticalplan.SideCT {
		return "ct-start"
	}
	return "t-start"
}

func startSideForKey(key string) tacticalplan.Side {
	if key == "ct-start" {
		return tacticalplan.SideCT
	}
	return tacticalplan.SideT
}

func endReasonSlug(reason events.RoundEndReason) string {
	switch reason {
	case events.RoundEndReasonTargetBombed:
		return "bomb_exploded"
	case events.RoundEndReasonBombDefused:
		return "bomb_defused"
	case events.RoundEndReasonCTWin:
		return "ct_eliminated_t"
	case events.RoundEndReasonTerroristsWin:
		return "t_eliminated_ct"
	case events.RoundEndReasonTargetSaved:
		return "time_expired"
	case events.RoundEndReasonTerroristsSurrender:
		return "t_surrender"
	case events.RoundEndReasonCTSurrender:
		return "ct_surrender"
	case events.RoundEndReasonDraw:
		return "draw"
	default:
		return "unknown"
	}
}

func fileSHA256(path string) (string, error) {
	// #nosec G304 -- the demo path is an explicit local input.
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open demo %q: %w", path, err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("checksum demo %q: %w", path, err)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func steamIDString(id uint64) string { return strconv.FormatUint(id, 10) }
