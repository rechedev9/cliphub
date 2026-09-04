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

	// settlePeakFloorDegPerSec is the peak angular speed a pre-shot window must
	// reach before its settle time means anything. Below it the crosshair never
	// really turned, so "time since the peak" is noise — and a player holding a
	// static angle would otherwise record a settle of 0 ms, the most suspicious
	// value the metric has, for the most ordinary kill in the game.
	//
	// 15 °/s is a quarter of a degree per tick at 64: above hand jitter, far
	// below any deliberate turn. It has to stay low. Professional flick speed
	// sits at 162 °/s for the *90th* percentile of a player's kills, so a floor
	// anywhere near that discards almost every kill and the metric dies: at
	// 100 °/s no player in a 15-map professional corpus kept enough sample to
	// score at all.
	settlePeakFloorDegPerSec = 15.0

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

	// everSpotted remembers which enemies became visible at any point in the
	// current round. spottedSince cannot answer that — it is cleared the moment
	// visibility breaks — and "was never seen this round" is what the unspotted
	// kill metric claims to measure.
	everSpotted map[uint64]struct{}

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
	// to correct after a fast angle change; an assisted aim does not. It only
	// means anything when hasSettle is true.
	settleMS float64
	// hasSettle reports whether the crosshair actually turned before the shot.
	// Without a real peak there is nothing to settle from, and the kill is left
	// out of the settle metric instead of entering it as a 0 ms outlier.
	hasSettle bool
	// jitter is the share of the pre-shot window's angle deltas that reversed
	// direction. Machine micro-correction reverses far more often than a hand.
	jitter float64
	// hasAngles reports whether the pre-shot window held enough samples for
	// peakDegPerSec, settleMS, and jitter to mean anything.
	hasAngles bool

	// preaimLocked is true when the crosshair had been locked onto the victim
	// through cover for at least preaimLockSeconds right before the kill.
	preaimLocked bool
	// visibleForMS is how long the victim had been continuously visible to the
	// killer when the kill landed, or -1 when the victim was not visible at
	// that moment. It is the reaction-time input, not a "never seen" flag.
	visibleForMS float64
	// victimEverSpotted is true when the victim had been visible to the killer
	// at any point in the round. A victim who was visible earlier and merely
	// stepped out of the spotted mask is not an unseen kill.
	victimEverSpotted bool
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
			clear(t.everSpotted)
		}
	})

	p.RegisterEventHandler(func(events.RoundEnd) { c.rounds++ })

	p.RegisterEventHandler(func(events.FrameDone) { c.sample() })

	p.RegisterEventHandler(func(e events.Kill) { c.recordKill(e) })
}

// playerTick is one living player's state for a single sampled tick.
//
// Eye position and view angles are resolved once per player here instead of
// once per ordered player pair: on a given tick a player's eye position is the
// same for every observer looking at them, while PositionEyes resolves the
// pawn entity and walks its property table on every call. In a 5v5 that is 10
// resolves per tick instead of 60.
type playerTick struct {
	pl    *common.Player
	eyes  r3.Vector
	yaw   float64
	pitch float64
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

	alive := make([]playerTick, 0, 10)
	for _, pl := range gs.Participants().Playing() {
		if pl == nil || pl.SteamID64 == 0 || !pl.IsAlive() {
			continue
		}
		yaw, pitch := viewAngles(pl)
		// The bool is discarded: a player without a pawn contributes the zero
		// vector, which the distance guard in sampleTick filters out. Skipping
		// them here instead would change what the metrics count.
		eyes, _ := pl.PositionEyes()
		alive = append(alive, playerTick{pl: pl, eyes: eyes, yaw: yaw, pitch: pitch})
	}

	// Visibility stays the demoinfocs call: the spotted mask lives behind
	// entity-handle resolution and a split bitmask in library internals.
	c.sampleTick(tick, alive, (*common.Player).IsSpottedBy)
}

// sampleTick folds one sampled tick into the tracks of the players alive on
// it. sample resolves the demo state; this half is bookkeeping over playerTick
// values, so it can be driven from a synthetic fixture in tests.
//
// spottedBy reports whether enemy is currently visible to observer. The demo
// path passes (*common.Player).IsSpottedBy.
func (c *collector) sampleTick(tick int, alive []playerTick, spottedBy func(enemy, observer *common.Player) bool) {
	if len(alive) == 0 {
		return
	}
	c.sampledTicks++

	for i := range alive {
		ob := &alive[i]
		t := c.track(ob.pl)
		t.push(angleSample{tick: tick, yaw: ob.yaw, pitch: ob.pitch}, c.ringSize)
		t.aliveTicks++

		view := viewVector(ob.yaw, ob.pitch)

		tracking := false
		for j := range alive {
			enemy := &alive[j]
			if enemy.pl.Team == ob.pl.Team || enemy.pl.SteamID64 == ob.pl.SteamID64 {
				continue
			}
			id := enemy.pl.SteamID64
			if spottedBy(enemy.pl, ob.pl) {
				if _, ok := t.spottedSince[id]; !ok {
					t.spottedSince[id] = tick
				}
				t.everSpotted[id] = struct{}{}
				delete(t.preaimTicks, id)
				continue
			}
			delete(t.spottedSince, id)

			offset := enemy.eyes.Sub(ob.eyes)
			distance := offset.Norm()
			if distance > wallTrackMaxUnits || distance == 0 {
				delete(t.preaimTicks, id)
				continue
			}
			if angleBetween(view, offset) > crosshairLockDegrees {
				delete(t.preaimTicks, id)
				continue
			}
			t.preaimTicks[id]++
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
	obs.hasSettle = obs.hasAngles && obs.peakDegPerSec >= settlePeakFloorDegPerSec
	obs.preaimLocked = float64(t.preaimTicks[e.Victim.SteamID64]) >= preaimLockSeconds*c.tickRate
	if since, ok := t.spottedSince[e.Victim.SteamID64]; ok {
		obs.visibleForMS = float64(tick-since) / c.tickRate * 1000
	}
	_, obs.victimEverSpotted = t.everSpotted[e.Victim.SteamID64]

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
			everSpotted:  map[uint64]struct{}{},
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
