package parser

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"sync"

	"github.com/golang/geo/r3"
	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"

	"github.com/rechedev9/cliphub/internal/demosafe"
	"github.com/rechedev9/cliphub/internal/radarmap"
	"github.com/rechedev9/cliphub/internal/radaroverlay"
)

const (
	// radarUtilityLeadSeconds keeps a smoke or fire thrown before a window that still burns in it.
	radarUtilityLeadSeconds = 25
	radarDuplicateTicks     = 4
)

// RadarTrackOptions names the demo, its map and the capture windows to sample.
type RadarTrackOptions struct {
	DemoSHA256 string
	TickRate   int
	Map        string
	Windows    []radaroverlay.Window
}

// ExtractRadarTrack reads the demo up to the last window and keeps, for each window, the living
// players SamplesPerSecond times a second, the bomb, the round's deaths and the utility that went
// off. It reads only what CS2 recorded; a map without a radar calibration is refused.
func ExtractRadarTrack(ctx context.Context, demo io.Reader, opts RadarTrackOptions) (radaroverlay.Track, error) {
	cal, ok := radarmap.Lookup(opts.Map)
	track := radaroverlay.Track{Version: radaroverlay.Version, DemoSHA256: opts.DemoSHA256, TickRate: opts.TickRate, Map: opts.Map, Calibration: cal}
	if !ok {
		return track, fmt.Errorf("radar track: %q has no radar calibration", opts.Map)
	}
	if opts.TickRate < 1 || len(opts.Windows) == 0 {
		return track, fmt.Errorf("radar track needs a tick rate and at least one window")
	}
	s := newRadarSampler(opts)
	p := demoinfocs.NewParser(demo)
	defer p.Close()
	s.register(p)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		select {
		case <-ctx.Done():
			p.Cancel()
		case <-stop:
		}
	})
	err := demosafe.Recover(func() error {
		for {
			more, err := p.ParseNextFrame()
			if err != nil {
				return err
			}
			tick := p.GameState().IngameTick()
			if !more || tick > s.lastTick {
				return nil
			}
			s.frame(p.GameState(), tick)
		}
	})
	close(stop)
	wg.Wait()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return track, fmt.Errorf("radar track: %w", ctxErr)
	}
	if err != nil && !errors.Is(err, demoinfocs.ErrUnexpectedEndOfDemo) {
		return track, fmt.Errorf("radar track: %w", err)
	}
	track.Segments = s.segments()
	return track, track.Validate()
}

type radarSampler struct {
	windows  []radaroverlay.Window
	step     int
	lead     int
	lastTick int
	segs     []*radarSegment
	slots    map[uint64]int
	deaths   []radaroverlay.Death
	utility  []radaroverlay.Utility
	// open maps a burning smoke's entity or a fire's inferno to its index in utility.
	openSmokes map[int]int
	openFires  map[int64]int
	planted    *radaroverlay.Bomb
	// bombGone hides the C4 from its defuse or explosion to the next round.
	bombGone bool
}

type radarSegment struct {
	seg  radaroverlay.Segment
	pov  uint64
	next int
}

func newRadarSampler(opts RadarTrackOptions) *radarSampler {
	s := &radarSampler{
		windows: opts.Windows, step: max(1, opts.TickRate/radaroverlay.SamplesPerSecond), lead: radarUtilityLeadSeconds * opts.TickRate,
		slots: map[uint64]int{}, openSmokes: map[int]int{}, openFires: map[int64]int{},
	}
	for _, w := range opts.Windows {
		pov, _ := strconv.ParseUint(w.POVSteamID64, 10, 64)
		s.segs = append(s.segs, &radarSegment{
			seg: radaroverlay.Segment{ID: w.ID, TickStart: w.TickStart, TickEnd: w.TickEnd, POV: -1, Frames: []radaroverlay.Frame{}},
			pov: pov, next: w.TickStart,
		})
		s.lastTick = max(s.lastTick, w.TickEnd)
	}
	return s
}

func (s *radarSampler) register(p demoinfocs.Parser) {
	tick := func() int { return p.GameState().IngameTick() }
	p.RegisterEventHandler(func(events.RoundStart) { s.roundStart(tick()) })
	p.RegisterEventHandler(func(e events.Kill) {
		if e.Victim == nil {
			return
		}
		if pos, ok := safeFeet(e.Victim); ok {
			s.deaths = append(s.deaths, radaroverlay.Death{Slot: s.slot(e.Victim), Side: teamLabel(e.Victim.Team), Tick: tick(), X: pos[0], Y: pos[1], Z: pos[2]})
		}
	})
	p.RegisterEventHandler(func(e events.SmokeStart) {
		s.openSmokes[e.GrenadeEntityID] = s.addUtility(radaroverlay.KindSmoke, e.GrenadeEvent, tick())
	})
	p.RegisterEventHandler(func(e events.SmokeExpired) {
		if at, ok := s.openSmokes[e.GrenadeEntityID]; ok {
			s.utility[at].EndTick = tick()
			delete(s.openSmokes, e.GrenadeEntityID)
		}
	})
	p.RegisterEventHandler(func(e events.FlashExplode) { s.addUtility(radaroverlay.KindFlash, e.GrenadeEvent, tick()) })
	p.RegisterEventHandler(func(e events.HeExplode) { s.addUtility(radaroverlay.KindHE, e.GrenadeEvent, tick()) })
	p.RegisterEventHandler(func(e events.InfernoStart) {
		if e.Inferno == nil || e.Inferno.Entity == nil {
			return
		}
		pos, ok := safeInfernoPosition(e.Inferno)
		if !ok {
			return
		}
		s.openFires[e.Inferno.UniqueID()] = s.addUtility(radaroverlay.KindFire, events.GrenadeEvent{Position: pos, Thrower: safeInfernoThrower(e.Inferno)}, tick())
	})
	p.RegisterEventHandler(func(e events.InfernoExpired) {
		if e.Inferno != nil {
			s.endFire(e.Inferno.UniqueID(), tick())
		}
	})
	p.RegisterEventHandler(func(e events.BombPlanted) {
		bomb := radaroverlay.Bomb{State: radaroverlay.BombPlanted, Carrier: -1}
		if b := p.GameState().Bomb(); b != nil && !zeroVec(b.LastOnGroundPosition) {
			bomb.X, bomb.Y, bomb.Z = round1(b.LastOnGroundPosition.X), round1(b.LastOnGroundPosition.Y), round1(b.LastOnGroundPosition.Z)
		} else if pos, ok := safeFeet(e.Player); ok {
			bomb.X, bomb.Y, bomb.Z = pos[0], pos[1], pos[2]
		} else {
			return
		}
		s.planted = &bomb
	})
	p.RegisterEventHandler(func(events.BombDefused) { s.planted, s.bombGone = nil, true })
	p.RegisterEventHandler(func(events.BombExplode) { s.planted, s.bombGone = nil, true })
}

// roundStart clears the last round off the radar: its deaths, its burning utility and the bomb.
func (s *radarSampler) roundStart(tick int) {
	for i := range s.deaths {
		if s.deaths[i].Until == 0 {
			s.deaths[i].Until = tick
		}
	}
	for id, at := range s.openSmokes {
		s.utility[at].EndTick = tick
		delete(s.openSmokes, id)
	}
	for id := range s.openFires {
		s.endFire(id, tick)
	}
	s.planted, s.bombGone = nil, false
}

func (s *radarSampler) addUtility(kind string, e events.GrenadeEvent, tick int) int {
	// CS2 demos can report one detonation twice, from the game event and from the entity.
	for i := len(s.utility) - 1; i >= 0 && s.utility[i].Tick >= tick-radarDuplicateTicks; i-- {
		if u := s.utility[i]; u.Kind == kind && math.Abs(u.X-e.Position.X) < 32 && math.Abs(u.Y-e.Position.Y) < 32 {
			return i
		}
	}
	side := ""
	if e.Thrower != nil {
		side = teamLabel(e.Thrower.Team)
	}
	s.utility = append(s.utility, radaroverlay.Utility{
		Kind: kind, Side: side, Tick: tick, X: round1(e.Position.X), Y: round1(e.Position.Y), Z: round1(e.Position.Z),
	})
	return len(s.utility) - 1
}

func (s *radarSampler) endFire(id int64, tick int) {
	if at, ok := s.openFires[id]; ok {
		s.utility[at].EndTick = tick
		delete(s.openFires, id)
	}
}

// frame samples every window due a frame at tick and ends fires whose last flame went out:
// the inferno entity outlives its flames by seconds.
func (s *radarSampler) frame(gs demoinfocs.GameState, tick int) {
	if len(s.openFires) > 0 {
		for _, inf := range gs.Infernos() {
			if _, open := s.openFires[inf.UniqueID()]; open && safeFlamesOut(inf) {
				s.endFire(inf.UniqueID(), tick)
			}
		}
	}
	var playing []*common.Player
	var bomb *radaroverlay.Bomb
	sampled := false
	for i, w := range s.windows {
		seg := s.segs[i]
		if tick < w.TickStart || tick > w.TickEnd || tick < seg.next {
			continue
		}
		if !sampled {
			playing, bomb, sampled = gs.Participants().Playing(), s.bomb(gs), true
		}
		seg.add(s, tick, playing, bomb)
		seg.next = tick + s.step
	}
}

func (w *radarSegment) add(s *radarSampler, tick int, playing []*common.Player, bomb *radaroverlay.Bomb) {
	frame := radaroverlay.Frame{Tick: tick, Bomb: bomb}
	for _, p := range playing {
		if p == nil {
			continue
		}
		slot := s.slot(p)
		if p.SteamID64 != 0 && p.SteamID64 == w.pov {
			w.seg.POV = slot
		}
		if !safeAlive(p) {
			continue
		}
		pos, ok := safeFeet(p)
		if !ok {
			continue
		}
		yaw, _, _ := safeViewAngles(p)
		frame.Players = append(frame.Players, radaroverlay.Player{Slot: slot, Side: teamLabel(p.Team), X: pos[0], Y: pos[1], Z: pos[2], Yaw: round1(yaw)})
	}
	sort.Slice(frame.Players, func(i, j int) bool { return frame.Players[i].Slot < frame.Players[j].Slot })
	w.seg.Frames = append(w.seg.Frames, frame)
}

// bomb is where the C4 is: planted, carried by a living player or lying on the ground.
func (s *radarSampler) bomb(gs demoinfocs.GameState) (out *radaroverlay.Bomb) {
	if s.planted != nil {
		planted := *s.planted
		return &planted
	}
	if s.bombGone {
		return nil
	}
	defer func() {
		if recover() != nil {
			out = nil
		}
	}()
	b := gs.Bomb()
	if b == nil {
		return nil
	}
	if b.Carrier != nil {
		return &radaroverlay.Bomb{State: radaroverlay.BombCarried, Carrier: s.slot(b.Carrier)}
	}
	if zeroVec(b.LastOnGroundPosition) {
		return nil
	}
	at := b.LastOnGroundPosition
	return &radaroverlay.Bomb{State: radaroverlay.BombDropped, Carrier: -1, X: round1(at.X), Y: round1(at.Y), Z: round1(at.Z)}
}

// slot numbers a player for the whole demo; a bot without a SteamID is told apart by its entity.
func (s *radarSampler) slot(p *common.Player) int {
	key := p.SteamID64
	if key == 0 {
		key = 1<<63 | uint64(p.EntityID)
	}
	if slot, ok := s.slots[key]; ok {
		return slot
	}
	s.slots[key] = len(s.slots)
	return s.slots[key]
}

// segments gives each window the round's deaths and the utility that shows during it.
func (s *radarSampler) segments() []radaroverlay.Segment {
	out := make([]radaroverlay.Segment, 0, len(s.segs))
	for _, w := range s.segs {
		seg := w.seg
		for _, d := range s.deaths {
			if d.Tick <= seg.TickEnd && (d.Until == 0 || d.Until > seg.TickStart) {
				seg.Deaths = append(seg.Deaths, d)
			}
		}
		for _, u := range s.utility {
			if u.Tick > seg.TickEnd || u.Tick < seg.TickStart-s.lead {
				continue
			}
			lasting := u.Kind == radaroverlay.KindSmoke || u.Kind == radaroverlay.KindFire
			if (lasting && u.EndTick != 0 && u.EndTick <= seg.TickStart) || (!lasting && u.Tick < seg.TickStart-s.step) {
				continue
			}
			seg.Utility = append(seg.Utility, u)
		}
		out = append(out, seg)
	}
	return out
}

func safeFeet(p *common.Player) (out [3]float64, ok bool) {
	if p == nil {
		return out, false
	}
	defer func() {
		if recover() != nil {
			out, ok = [3]float64{}, false
		}
	}()
	v := p.Position()
	if v.X == 0 && v.Y == 0 && v.Z == 0 {
		return out, false
	}
	return [3]float64{round1(v.X), round1(v.Y), round1(v.Z)}, true
}

func safeInfernoPosition(inf *common.Inferno) (out r3.Vector, ok bool) {
	defer func() {
		if recover() != nil {
			out, ok = r3.Vector{}, false
		}
	}()
	return inf.Entity.Position(), true
}

func safeInfernoThrower(inf *common.Inferno) (out *common.Player) {
	defer func() {
		if recover() != nil {
			out = nil
		}
	}()
	return inf.Thrower()
}

func safeFlamesOut(inf *common.Inferno) (out bool) {
	defer func() {
		if recover() != nil {
			out = false
		}
	}()
	fires := inf.Fires()
	return len(fires.List()) > 0 && len(fires.Active().List()) == 0
}

func zeroVec(v r3.Vector) bool { return v.X == 0 && v.Y == 0 && v.Z == 0 }
