package parser

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"sync"

	demoinfocs "github.com/rechedev9/demo-parser-rsn/pkg/demoinfocs"
	"github.com/rechedev9/demo-parser-rsn/pkg/demoinfocs/common"
	"github.com/rechedev9/demo-parser-rsn/pkg/demoinfocs/events"

	"github.com/rechedev9/cliphub/internal/demosafe"
	"github.com/rechedev9/cliphub/internal/worldoverlay"
)

// worldGrenadeLeadSeconds samples grenades thrown shortly before a window, so a trail already in flight shows.
const worldGrenadeLeadSeconds = 4

// WorldTrackOptions names the demo and the capture windows to sample.
type WorldTrackOptions struct {
	DemoSHA256 string
	TickRate   int
	Windows    []worldoverlay.Window
}

// ExtractWorldTrack reads the demo up to the last window and keeps, for each window and tick,
// the POV's eye and view, the other living players and the grenades in flight. It reads only
// what CS2 recorded and leaves a value out when the demo does not have it.
func ExtractWorldTrack(ctx context.Context, demo io.Reader, opts WorldTrackOptions) (worldoverlay.Track, error) {
	track := worldoverlay.Track{Version: worldoverlay.Version, DemoSHA256: opts.DemoSHA256, TickRate: opts.TickRate}
	if opts.TickRate < 1 || len(opts.Windows) == 0 {
		return track, fmt.Errorf("world track needs a tick rate and at least one window")
	}
	sampler := newWorldSampler(opts)
	p, err := demosafe.NewParser(demo)
	if err != nil {
		return track, fmt.Errorf("world track: %w", err)
	}
	defer p.Close()
	p.RegisterEventHandler(func(e events.PlayerHurt) {
		if e.Attacker != nil && e.Player != nil {
			sampler.hurt(e.Attacker.SteamID64, e.Player.SteamID64, p.GameState().IngameTick())
		}
	})
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		select {
		case <-ctx.Done():
			p.Cancel()
		case <-stop:
		}
	})
	err = demosafe.Recover(func() error {
		for {
			more, err := p.ParseNextFrame()
			if err != nil {
				return err
			}
			tick := p.GameState().IngameTick()
			if !more || tick > sampler.lastTick {
				return nil
			}
			sampler.sample(p.GameState(), tick)
		}
	})
	close(stop)
	wg.Wait()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return track, fmt.Errorf("world track: %w", ctxErr)
	}
	if err != nil && !errors.Is(err, demoinfocs.ErrUnexpectedEndOfDemo) {
		return track, fmt.Errorf("world track: %w", err)
	}
	track.Segments = sampler.segments()
	return track, track.Validate()
}

type worldSampler struct {
	windows  []worldoverlay.Window
	lead     int
	lastTick int
	segs     []*worldSegment
}

type worldSegment struct {
	seg      worldoverlay.Segment
	pov      uint64
	hits     map[uint64]int
	slots    map[uint64]int
	grenades map[int64]*worldoverlay.Grenade
	order    []int64
}

func newWorldSampler(opts WorldTrackOptions) *worldSampler {
	s := &worldSampler{windows: opts.Windows, lead: worldGrenadeLeadSeconds * opts.TickRate}
	for _, w := range opts.Windows {
		pov, _ := strconv.ParseUint(w.POVSteamID64, 10, 64)
		s.segs = append(s.segs, &worldSegment{
			seg: worldoverlay.Segment{ID: w.ID, POV: w.POVSteamID64, TickStart: w.TickStart, TickEnd: w.TickEnd, Names: []string{}, Frames: []worldoverlay.Frame{}},
			pov: pov, hits: map[uint64]int{}, slots: map[uint64]int{}, grenades: map[int64]*worldoverlay.Grenade{},
		})
		s.lastTick = max(s.lastTick, w.TickEnd)
	}
	return s
}

// hurt notes the tick a window's POV damaged a player.
func (s *worldSampler) hurt(attacker, victim uint64, tick int) {
	for i, w := range s.windows {
		if seg := s.segs[i]; seg.pov == attacker && tick >= w.TickStart && tick <= w.TickEnd {
			seg.hits[victim] = tick
		}
	}
}

func (s *worldSampler) sample(gs demoinfocs.GameState, tick int) {
	var playing []*common.Player
	for i, w := range s.windows {
		inFrame := tick >= w.TickStart && tick <= w.TickEnd
		inLead := tick >= w.TickStart-s.lead && tick <= w.TickEnd
		if !inLead {
			continue
		}
		if playing == nil {
			playing = gs.Participants().Playing()
		}
		seg := s.segs[i]
		if inFrame {
			seg.addFrame(tick, playing)
		}
		seg.addGrenades(tick, gs.GrenadeProjectiles())
	}
}

// addFrame records one tick, replacing an earlier frame of the same tick.
func (w *worldSegment) addFrame(tick int, playing []*common.Player) {
	var pov *common.Player
	for _, p := range playing {
		if p != nil && p.SteamID64 == w.pov {
			pov = p
		}
	}
	frame := worldoverlay.Frame{Tick: tick, Camera: worldCamera(pov)}
	for _, p := range playing {
		if p == nil || p == pov || !safeAlive(p) {
			continue
		}
		eye, ok := safeEyePosition(p)
		if !ok {
			continue
		}
		frame.Players = append(frame.Players, worldoverlay.Player{
			Slot: w.slot(p), Side: teamLabel(p.Team), Health: safeHealth(p), Eye: roundVec(eye),
			Spotted: pov != nil && safeSpottedBy(p, pov), Hit: w.hitAt(p.SteamID64, tick),
		})
	}
	sort.Slice(frame.Players, func(i, j int) bool { return frame.Players[i].Slot < frame.Players[j].Slot })
	if n := len(w.seg.Frames); n > 0 && w.seg.Frames[n-1].Tick == tick {
		w.seg.Frames[n-1] = frame
		return
	}
	w.seg.Frames = append(w.seg.Frames, frame)
}

func (w *worldSegment) hitAt(steamID uint64, tick int) bool {
	hit, ok := w.hits[steamID]
	return ok && hit == tick
}

func (w *worldSegment) slot(p *common.Player) int {
	if slot, ok := w.slots[p.SteamID64]; ok {
		return slot
	}
	w.slots[p.SteamID64] = len(w.seg.Names)
	w.seg.Names = append(w.seg.Names, p.Name)
	return w.slots[p.SteamID64]
}

func (w *worldSegment) addGrenades(tick int, projectiles map[int]*common.GrenadeProjectile) {
	for _, g := range projectiles {
		if g == nil || g.Entity == nil || g.WeaponInstance == nil {
			continue
		}
		kind := worldGrenadeKind(g.WeaponInstance.Type)
		if kind == "" {
			continue
		}
		id := g.UniqueID()
		track, ok := w.grenades[id]
		if !ok {
			side := ""
			if g.Thrower != nil {
				side = teamLabel(g.Thrower.Team)
			}
			track = &worldoverlay.Grenade{Kind: kind, Side: side}
			w.grenades[id] = track
			w.order = append(w.order, id)
		}
		pos, ok := safeProjectilePosition(g)
		if !ok {
			continue
		}
		point := worldoverlay.Point{Tick: tick, Pos: pos}
		if n := len(track.Points); n > 0 && track.Points[n-1].Tick == tick {
			track.Points[n-1] = point
			continue
		}
		track.Points = append(track.Points, point)
	}
}

func (s *worldSampler) segments() []worldoverlay.Segment {
	out := make([]worldoverlay.Segment, 0, len(s.segs))
	for _, w := range s.segs {
		for _, id := range w.order {
			if g := w.grenades[id]; len(g.Points) >= 2 && g.Points[len(g.Points)-1].Tick >= w.seg.TickStart {
				w.seg.Grenades = append(w.seg.Grenades, *g)
			}
		}
		out = append(out, w.seg)
	}
	return out
}

// worldCamera is the POV's eye; nil while it is dead or its pawn lacks the eye offset.
func worldCamera(pov *common.Player) *worldoverlay.Camera {
	if pov == nil || !safeAlive(pov) {
		return nil
	}
	eye, ok := safeEyePosition(pov)
	if !ok {
		return nil
	}
	yaw, pitch, ok := safeViewAngles(pov)
	if !ok {
		return nil
	}
	return &worldoverlay.Camera{
		Eye: roundVec(eye), Yaw: round2(yaw), Pitch: round2(pitch),
		FOV: safePawnInt(pov, "m_pCameraServices.m_iFOV"), Scoped: safeScoped(pov), Side: teamLabel(pov.Team),
	}
}

func worldGrenadeKind(t common.EquipmentType) string {
	switch t {
	case common.EqSmoke:
		return worldoverlay.KindSmoke
	case common.EqFlash:
		return worldoverlay.KindFlash
	case common.EqHE:
		return worldoverlay.KindHE
	case common.EqMolotov, common.EqIncendiary:
		return worldoverlay.KindFire
	case common.EqDecoy:
		return worldoverlay.KindDecoy
	default:
		return ""
	}
}

func safeAlive(p *common.Player) (alive bool) {
	defer func() {
		if recover() != nil {
			alive = false
		}
	}()
	return p.IsAlive()
}

func safeHealth(p *common.Player) (hp int) {
	defer func() {
		if recover() != nil {
			hp = 0
		}
	}()
	return p.Health()
}

func safeScoped(p *common.Player) (scoped bool) {
	defer func() {
		if recover() != nil {
			scoped = false
		}
	}()
	return p.IsScoped()
}

// safeSpottedBy reads CS2's m_bSpottedByMask; a demo without it reads as not spotted.
func safeSpottedBy(p, observer *common.Player) (spotted bool) {
	defer func() {
		if recover() != nil {
			spotted = false
		}
	}()
	return p.IsSpottedBy(observer)
}

func safePawnInt(p *common.Player, name string) (out int) {
	defer func() {
		if recover() != nil {
			out = 0
		}
	}()
	pawn := p.PlayerPawnEntity()
	if pawn == nil {
		return 0
	}
	prop := pawn.Property(name)
	if prop == nil {
		return 0
	}
	return prop.Value().Int()
}

func safeProjectilePosition(g *common.GrenadeProjectile) (out worldoverlay.Vec, ok bool) {
	defer func() {
		if recover() != nil {
			out, ok = worldoverlay.Vec{}, false
		}
	}()
	v := g.Position()
	return roundVec([3]float64{v.X, v.Y, v.Z}), true
}

func roundVec(v [3]float64) worldoverlay.Vec {
	return worldoverlay.Vec{round1(v[0]), round1(v[1]), round1(v[2])}
}
