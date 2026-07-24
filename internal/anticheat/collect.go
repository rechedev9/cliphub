package anticheat

import (
	"math"

	"github.com/golang/geo/r3"
	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"
)

// Tuning constants for the raw observations. They define what the metrics
// count; the baseline in score.go decides whether a counted value is unusual.
const (
	// fallbackTickRate is used when the demo header carries no usable rate.
	// Every CS2 matchmaking and most third-party demos record at 64.
	fallbackTickRate = 64.0

	// angleWindowSeconds is how much view-angle history each player keeps. It
	// only has to cover the pre-shot window analysed on a kill.
	angleWindowSeconds = 0.5

	// crosshairLockDegrees is how close the crosshair must sit to an enemy for
	// the aim to count as "on" that enemy. Roughly a head at mid range.
	crosshairLockDegrees = 5.0

	// wallTrackMaxUnits caps how far away an unseen enemy can be and still
	// count as tracked. Beyond this a chance alignment is too likely.
	wallTrackMaxUnits = 2200.0

	// preaimLockSeconds is how long the crosshair must stay locked onto an
	// enemy the player cannot see before the following kill counts as pre-aim.
	preaimLockSeconds = 0.3

	// reactionWindowSeconds bounds how long after an enemy becomes visible a
	// kill still counts as a reaction to that enemy appearing.
	reactionWindowSeconds = 2.0

	// roboticSettleMS is the settle time below which a flick is flagged as
	// evidence: no human aims through a fast angle change and fires this fast.
	roboticSettleMS = 60.0

	// roboticFlickDegPerSec is the peak angular speed a flick must reach
	// before its settle time is worth flagging at all.
	roboticFlickDegPerSec = 400.0

	// instantReactionMS is the reaction time below which a kill is flagged as
	// evidence. Trained human reaction to a visual cue bottoms out near 150 ms.
	instantReactionMS = 100.0
)

// angleSample is one tick of a player's view direction.
type angleSample struct {
	tick  int
	yaw   float64
	pitch float64
}

// track accumulates one player's observations across the whole demo.
//
// Long-lived state is deliberately bounded: view angles live in a fixed ring
// covering angleWindowSeconds, and everything else is a counter. A 40-minute
// demo therefore costs the same memory as a one-round one.
type track struct {
	steamID uint64
	name    string
	team    string

	ring    []angleSample
	ringPos int
	ringLen int

	aliveTicks     int
	wallTrackTicks int

	// preaimTicks counts, per enemy, how many consecutive sampled ticks the
	// crosshair has been locked onto that enemy while the enemy was not
	// visible to this player. It resets the moment the lock or the occlusion
	// breaks, so reading it at a kill yields the length of the run that led
	// straight into the kill.
	preaimTicks map[uint64]int

	// spottedSince records, per enemy, the tick that enemy last became visible
	// to this player. Cleared when visibility breaks and at every round start,
	// so a kill reads the age of the current sighting, not a stale one.
	spottedSince map[uint64]int

	kills []killObservation
}

// killObservation is everything one gun kill contributes to the metrics.
type killObservation struct {
	tick  int
	round int

	victimName string
	weapon     string

	headshot     bool
	throughSmoke bool
	penetrated   int

	// peakDegPerSec is the fastest view-angle change in the pre-shot window.
	peakDegPerSec float64
	// settleMS is the time from that peak to the kill. Human aim needs time
	// to correct after a fast angle change; an assisted aim does not.
	settleMS float64
	// jitter is the share of the pre-shot window's angle deltas that reversed
	// direction. Machine micro-correction reverses far more often than a hand.
	jitter float64
	// hasAngles reports whether the pre-shot window held enough samples for
	// peakDegPerSec, settleMS, and jitter to mean anything.
	hasAngles bool

	// preaimLocked is true when the crosshair had been locked onto the victim
	// through cover for at least preaimLockSeconds right before the kill.
	preaimLocked bool
	// visibleForMS is how long the victim had been visible to the killer when
	// the kill landed, or -1 when the victim was never visible.
	visibleForMS float64
}

// collector wires the demo event handlers and holds every track.
type collector struct {
	parser   demoinfocs.Parser
	tickRate float64
	ringSize int

	mapName string
	round   int
	rounds  int

	tracks       map[uint64]*track
	sampledTicks int
	lastTick     int
}

func newCollector(p demoinfocs.Parser) *collector {
	rate := p.TickRate()
	if rate <= 0 || math.IsNaN(rate) {
		rate = fallbackTickRate
	}
	ring := int(math.Ceil(rate * angleWindowSeconds))
	if ring < 8 {
		ring = 8
	}
	return &collector{
		parser:   p,
		tickRate: rate,
		ringSize: ring,
		tracks:   map[uint64]*track{},
		lastTick: -1,
	}
}

func (c *collector) register(p demoinfocs.Parser) {
	p.RegisterNetMessageHandler(func(info *msg.CSVCMsg_ServerInfo) {
		if name := info.GetMapName(); name != "" {
			c.mapName = name
		}
	})

	// Warmup and knife rounds distort every metric here, so the match start
	// discards whatever was collected before it, exactly like the roster scan.
	p.RegisterEventHandler(func(events.MatchStart) {
		c.tracks = map[uint64]*track{}
		c.sampledTicks = 0
		c.round = 0
		c.rounds = 0
	})

	p.RegisterEventHandler(func(events.RoundStart) {
		c.round++
		for _, t := range c.tracks {
			clear(t.preaimTicks)
			clear(t.spottedSince)
		}
	})

	p.RegisterEventHandler(func(events.RoundEnd) { c.rounds++ })

	p.RegisterEventHandler(func(events.FrameDone) { c.sample() })

	p.RegisterEventHandler(func(e events.Kill) { c.recordKill(e) })
}

// sample snapshots every living player's view angles and their aim relative to
// every living enemy. It runs once per in-game tick even when the demo
// dispatches several frames for the same tick.
func (c *collector) sample() {
	gs := c.parser.GameState()
	tick := gs.IngameTick()
	if tick == c.lastTick {
		return
	}
	c.lastTick = tick

	alive := make([]*common.Player, 0, 10)
	for _, pl := range gs.Participants().Playing() {
		if pl != nil && pl.SteamID64 != 0 && pl.IsAlive() {
			alive = append(alive, pl)
		}
	}
	if len(alive) == 0 {
		return
	}
	c.sampledTicks++

	for _, pl := range alive {
		t := c.track(pl)
		yaw, pitch := viewAngles(pl)
		t.push(angleSample{tick: tick, yaw: yaw, pitch: pitch}, c.ringSize)
		t.aliveTicks++

		eyes, _ := pl.PositionEyes()
		view := viewVector(yaw, pitch)

		tracking := false
		for _, enemy := range alive {
			if enemy.Team == pl.Team || enemy.SteamID64 == pl.SteamID64 {
				continue
			}
			if enemy.IsSpottedBy(pl) {
				if _, ok := t.spottedSince[enemy.SteamID64]; !ok {
					t.spottedSince[enemy.SteamID64] = tick
				}
				delete(t.preaimTicks, enemy.SteamID64)
				continue
			}
			delete(t.spottedSince, enemy.SteamID64)

			target, _ := enemy.PositionEyes()
			offset := target.Sub(eyes)
			distance := offset.Norm()
			if distance > wallTrackMaxUnits || distance == 0 {
				delete(t.preaimTicks, enemy.SteamID64)
				continue
			}
			if angleBetween(view, offset) > crosshairLockDegrees {
				delete(t.preaimTicks, enemy.SteamID64)
				continue
			}
			t.preaimTicks[enemy.SteamID64]++
			tracking = true
		}
		if tracking {
			t.wallTrackTicks++
		}
	}
}

// recordKill folds one kill into the killer's track. Only gun kills count:
// grenade, knife, Zeus, and world kills say nothing about aim, and a
// team kill says nothing about information advantage.
func (c *collector) recordKill(e events.Kill) {
	if e.Killer == nil || e.Victim == nil || e.Weapon == nil {
		return
	}
	if e.Killer.SteamID64 == 0 || e.Killer.SteamID64 == e.Victim.SteamID64 {
		return
	}
	if e.Killer.Team == e.Victim.Team {
		return
	}
	if !isGunKill(e.Weapon) {
		return
	}

	t := c.track(e.Killer)
	tick := c.parser.GameState().IngameTick()
	obs := killObservation{
		tick:         tick,
		round:        c.round,
		victimName:   e.Victim.Name,
		weapon:       e.Weapon.String(),
		headshot:     e.IsHeadshot,
		throughSmoke: e.ThroughSmoke,
		penetrated:   e.PenetratedObjects,
		visibleForMS: -1,
	}

	obs.peakDegPerSec, obs.settleMS, obs.jitter, obs.hasAngles = t.preShotAim(tick, c.tickRate)
	obs.preaimLocked = float64(t.preaimTicks[e.Victim.SteamID64]) >= preaimLockSeconds*c.tickRate
	if since, ok := t.spottedSince[e.Victim.SteamID64]; ok {
		obs.visibleForMS = float64(tick-since) / c.tickRate * 1000
	}

	t.kills = append(t.kills, obs)
}

func (c *collector) track(pl *common.Player) *track {
	t, ok := c.tracks[pl.SteamID64]
	if !ok {
		t = &track{
			steamID:      pl.SteamID64,
			ring:         make([]angleSample, c.ringSize),
			preaimTicks:  map[uint64]int{},
			spottedSince: map[uint64]int{},
		}
		c.tracks[pl.SteamID64] = t
	}
	t.name = pl.Name
	if label := teamLabel(pl.Team); label != "" {
		t.team = label
	}
	return t
}

// push appends one view-angle sample, overwriting the oldest once the ring is
// full.
func (t *track) push(s angleSample, size int) {
	t.ring[t.ringPos] = s
	t.ringPos = (t.ringPos + 1) % size
	if t.ringLen < size {
		t.ringLen++
	}
}

// window returns the retained view-angle samples in chronological order.
func (t *track) window() []angleSample {
	out := make([]angleSample, 0, t.ringLen)
	size := len(t.ring)
	start := (t.ringPos - t.ringLen + size) % size
	for i := 0; i < t.ringLen; i++ {
		out = append(out, t.ring[(start+i)%size])
	}
	return out
}

// preShotAim measures the view-angle motion leading into a shot: the fastest
// angular speed reached, how long before the shot that peak happened, and how
// often the yaw reversed direction over the window.
//
// hasAngles is false when the window holds fewer than three samples, which
// happens on POV demos and at the very start of a life; the caller then drops
// this kill from the aim metrics instead of scoring noise.
func (t *track) preShotAim(killTick int, tickRate float64) (peakDegPerSec, settleMS, jitter float64, hasAngles bool) {
	samples := t.window()
	if len(samples) < 3 {
		return 0, 0, 0, false
	}

	peakTick := killTick
	var reversals, deltas int
	var prevYawDelta float64
	for i := 1; i < len(samples); i++ {
		prev, cur := samples[i-1], samples[i]
		dt := float64(cur.tick-prev.tick) / tickRate
		if dt <= 0 || cur.tick > killTick {
			continue
		}
		speed := angularDistance(prev.yaw, prev.pitch, cur.yaw, cur.pitch) / dt
		if speed > peakDegPerSec {
			peakDegPerSec = speed
			peakTick = cur.tick
		}

		yawDelta := shortestArc(cur.yaw - prev.yaw)
		if deltas > 0 && yawDelta != 0 && prevYawDelta != 0 && math.Signbit(yawDelta) != math.Signbit(prevYawDelta) {
			reversals++
		}
		if yawDelta != 0 {
			prevYawDelta = yawDelta
		}
		deltas++
	}
	if deltas < 2 {
		return 0, 0, 0, false
	}

	settleMS = float64(killTick-peakTick) / tickRate * 1000
	if settleMS < 0 {
		settleMS = 0
	}
	return peakDegPerSec, settleMS, float64(reversals) / float64(deltas), true
}

// viewAngles returns the player's yaw and pitch in degrees, with pitch
// normalised to [-180, 180) because the demo reports it as 270..90.
func viewAngles(pl *common.Player) (yaw, pitch float64) {
	yaw = float64(pl.ViewDirectionX())
	pitch = float64(pl.ViewDirectionY())
	if pitch >= 180 {
		pitch -= 360
	}
	return yaw, pitch
}

// viewVector converts yaw/pitch in degrees into a unit direction vector in the
// game's coordinate space, where a positive pitch aims downward.
func viewVector(yawDeg, pitchDeg float64) r3.Vector {
	yaw := yawDeg * math.Pi / 180
	pitch := pitchDeg * math.Pi / 180
	return r3.Vector{
		X: math.Cos(yaw) * math.Cos(pitch),
		Y: math.Sin(yaw) * math.Cos(pitch),
		Z: -math.Sin(pitch),
	}
}

// angleBetween returns the angle in degrees between two direction vectors. A
// zero-length vector yields 180 so it can never be mistaken for a lock.
func angleBetween(a, b r3.Vector) float64 {
	na, nb := a.Norm(), b.Norm()
	if na == 0 || nb == 0 {
		return 180
	}
	cos := a.Dot(b) / (na * nb)
	return math.Acos(math.Min(1, math.Max(-1, cos))) * 180 / math.Pi
}

// angularDistance returns the angle in degrees between two view directions.
func angularDistance(yaw1, pitch1, yaw2, pitch2 float64) float64 {
	return angleBetween(viewVector(yaw1, pitch1), viewVector(yaw2, pitch2))
}

// shortestArc maps a yaw difference into (-180, 180] so wrapping past 0/360
// does not read as a full-circle spin.
func shortestArc(delta float64) float64 {
	for delta > 180 {
		delta -= 360
	}
	for delta <= -180 {
		delta += 360
	}
	return delta
}

// isGunKill reports whether the weapon is a firearm, the only kind of kill
// whose aim and information usage these metrics can interpret.
func isGunKill(eq *common.Equipment) bool {
	switch eq.Class() {
	case common.EqClassPistols, common.EqClassSMG, common.EqClassHeavy, common.EqClassRifle:
		return eq.Type != common.EqZeus
	default:
		return false
	}
}

// teamLabel collapses the demo team enum to the "CT"/"T" labels the rest of
// the product uses, and to "" for anything else.
func teamLabel(t common.Team) string {
	switch t {
	case common.TeamCounterTerrorists:
		return "CT"
	case common.TeamTerrorists:
		return "T"
	default:
		return ""
	}
}
