package parser

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

// gameInfo carries the per-event game state needed to convert a demoinfocs
// Kill into a RawKill. It exists so BuildRawKill can be tested without a live
// parser.
type gameInfo struct {
	Tick     int
	Round    int
	ClutchVs int
}

// alivePlayer is one living player when a kill happens, so clutchVs needs no live parser.
type alivePlayer struct {
	SteamID64 uint64
	Team      common.Team
}

// clutchVs returns how many enemies the killer faced alone, counting the
// victim, or 0 when a teammate was still alive.
func clutchVs(alive []alivePlayer, killer, victim alivePlayer) int {
	if killer.Team != common.TeamCounterTerrorists && killer.Team != common.TeamTerrorists {
		return 0
	}
	enemies := 0
	for _, p := range alive {
		if p.SteamID64 == killer.SteamID64 || p.SteamID64 == victim.SteamID64 {
			continue
		}
		if p.Team == killer.Team {
			return 0
		}
		if p.Team == victim.Team {
			enemies++
		}
	}
	if victim.Team == killer.Team {
		return 0
	}
	return enemies + 1
}

func alivePlayers(players []*common.Player) []alivePlayer {
	out := make([]alivePlayer, 0, len(players))
	for _, p := range players {
		if p != nil && p.IsAlive() {
			out = append(out, alivePlayer{SteamID64: p.SteamID64, Team: p.Team})
		}
	}
	return out
}

// BuildRawKill converts a demoinfocs Kill event into a RawKill, applying the
// target-match and team-kill checks. The second return value reports whether
// the event should be recorded; weapon/headshot/round filters are applied
// later by the Collector.
func BuildRawKill(e events.Kill, gi gameInfo, targetID uint64, excludeTeamKills bool) (RawKill, bool) {
	if e.Killer == nil || e.Killer.SteamID64 != targetID {
		return RawKill{}, false
	}
	if e.Victim == nil {
		return RawKill{}, false
	}
	// A suicide is a POV death boundary, never a target frag. Keep this
	// invariant independent of the caller's team-kill preference.
	if e.Killer.SteamID64 == e.Victim.SteamID64 {
		return RawKill{}, false
	}
	if excludeTeamKills && e.Killer.Team == e.Victim.Team {
		return RawKill{}, false
	}
	return RawKill{
		Tick:     gi.Tick,
		Round:    gi.Round,
		Weapon:   weaponName(e.Weapon),
		Headshot: e.IsHeadshot,
		Wallbang: e.PenetratedObjects > 0,
		ClutchVs: gi.ClutchVs,
		Killer: killplan.Player{
			SteamID64:  strconv.FormatUint(e.Killer.SteamID64, 10),
			NameInDemo: e.Killer.Name,
			TeamAtKill: teamLabel(e.Killer.Team),
		},
		Victim: killplan.Player{
			SteamID64:  strconv.FormatUint(e.Victim.SteamID64, 10),
			NameInDemo: e.Victim.Name,
			TeamAtKill: teamLabel(e.Victim.Team),
		},
		KillerPos: playerPosition(e.Killer),
		VictimPos: playerPosition(e.Victim),
	}, true
}

// targetDeath adds who killed the target, and with what, to a live death.
func targetDeath(e events.Kill, death TargetDeath) TargetDeath {
	death.Victim = demoPlayer(e.Victim)
	death.Killer = demoPlayer(e.Killer)
	death.Weapon = weaponName(e.Weapon)
	death.Headshot = e.IsHeadshot
	return death
}

func demoPlayer(p *common.Player) killplan.Player {
	if p == nil {
		return killplan.Player{}
	}
	return killplan.Player{SteamID64: strconv.FormatUint(p.SteamID64, 10), NameInDemo: p.Name, TeamAtKill: teamLabel(p.Team)}
}

func weaponName(w *common.Equipment) string {
	if w == nil {
		return ""
	}
	// CS:GO demos populate Equipment.OriginalString with the in-game entity
	// name (e.g. "weapon_m4a1_silencer"); strip the prefix and use it.
	if w.OriginalString != "" {
		return strings.TrimPrefix(w.OriginalString, "weapon_")
	}
	// CS2 demos leave OriginalString empty. Fall back to a hand-curated
	// EquipmentType → canonical entity-name map. The strings match the
	// keys of demoinfocs' internal eqNameToWeapon table so the rest of
	// the pipeline (rules JSON, filters) uses a single vocabulary.
	if n, ok := equipmentNames[w.Type]; ok {
		return n
	}
	return strings.ToLower(w.Type.String())
}

// equipmentNames maps EquipmentType to the canonical lowercase entity name
// expected by zackvideo rules (matching the game's "weapon_X" entity names
// used in CS:GO demos and the demoinfocs reverse lookup).
var equipmentNames = map[common.EquipmentType]string{
	// Pistols
	common.EqP2000:        "hkp2000",
	common.EqGlock:        "glock",
	common.EqP250:         "p250",
	common.EqDeagle:       "deagle",
	common.EqFiveSeven:    "fiveseven",
	common.EqDualBerettas: "elite",
	common.EqTec9:         "tec9",
	common.EqCZ:           "cz75a",
	common.EqUSP:          "usp_silencer",
	common.EqRevolver:     "revolver",
	// SMGs
	common.EqMP7:   "mp7",
	common.EqMP9:   "mp9",
	common.EqBizon: "bizon",
	common.EqMac10: "mac10",
	common.EqUMP:   "ump45",
	common.EqP90:   "p90",
	common.EqMP5:   "mp5sd",
	// Heavy
	common.EqSawedOff: "sawedoff",
	common.EqNova:     "nova",
	common.EqSwag7:    "mag7",
	common.EqXM1014:   "xm1014",
	common.EqM249:     "m249",
	common.EqNegev:    "negev",
	// Rifles
	common.EqGalil:  "galilar",
	common.EqFamas:  "famas",
	common.EqAK47:   "ak47",
	common.EqM4A4:   "m4a1", // weapon_m4a1 = M4A4
	common.EqM4A1:   "m4a1_silencer",
	common.EqScout:  "ssg08",
	common.EqSG556:  "sg556",
	common.EqAUG:    "aug",
	common.EqAWP:    "awp",
	common.EqScar20: "scar20",
	common.EqG3SG1:  "g3sg1",
	// Grenades
	common.EqDecoy:      "decoy",
	common.EqMolotov:    "molotov",
	common.EqIncendiary: "incgrenade",
	common.EqFlash:      "flashbang",
	common.EqSmoke:      "smokegrenade",
	common.EqHE:         "hegrenade",
	// Equipment
	common.EqZeus:  "taser",
	common.EqKnife: "knife",
	common.EqBomb:  "c4",
	common.EqWorld: "world",
}

func teamLabel(t common.Team) string {
	switch t {
	case common.TeamCounterTerrorists:
		return "CT"
	case common.TeamTerrorists:
		return "T"
	case common.TeamSpectators:
		return "SPEC"
	default:
		return ""
	}
}

func playerPosition(p *common.Player) [3]float64 {
	if p == nil {
		return [3]float64{}
	}
	v := p.Position()
	return [3]float64{v.X, v.Y, v.Z}
}

// runKillEvents parses one plan in mode: kills, recap or deaths.
func runKillEvents(p demoinfocs.Parser, target string, r rules.Rules, m PlanMeta, mode SegmentMode) (killplan.Plan, error) {
	dual, err := collectKills(p, target, r, m, mode, false)
	return dual.Kills, err
}

// collectKills returns the plan for mode as Kills. With alsoRecap it also
// fills Recap, Deaths and Facts from the same pass.
func collectKills(p demoinfocs.Parser, target string, r rules.Rules, m PlanMeta, mode SegmentMode, alsoRecap bool) (DualPlan, error) {
	targetID, err := strconv.ParseUint(target, 10, 64)
	if err != nil {
		return DualPlan{}, fmt.Errorf("invalid target steamid %q: %w", target, err)
	}

	c := NewCollector(target, r)
	var mapName string
	var maxTick int
	var activeRound int
	var roundLive bool
	var freezeDeaths []TargetDeath
	var sawMatchStart bool
	var factsLastTick int
	var crosshairs crosshairTimeline
	var watch *utilityWatch
	if mode == SegmentModeRecap || alsoRecap {
		watch = newUtilityWatch(targetID, target, &maxTick, c.RecordTargetIdentity)
	}

	p.RegisterNetMessageHandler(func(info *msg.CSVCMsg_ServerInfo) {
		if name := info.GetMapName(); name != "" {
			mapName = name
		}
	})
	p.RegisterEventHandler(func(events.MatchStart) {
		c.resetForMatchStart()
		freezeDeaths = nil
		crosshairs = crosshairTimeline{}
		sawMatchStart = true
		activeRound = 0
		roundLive = false
	})

	p.RegisterEventHandler(func(e events.Kill) {
		gs := p.GameState()
		tick := gs.IngameTick()
		if tick > maxTick {
			maxTick = tick
		}
		round := gs.TotalRoundsPlayed() + 1
		if activeRound > 0 {
			round = activeRound
		}
		gi := gameInfo{Tick: tick, Round: round}
		if e.Killer != nil && e.Killer.SteamID64 == targetID && e.Victim != nil && gs.Participants() != nil {
			gi.ClutchVs = clutchVs(alivePlayers(gs.Participants().Playing()),
				alivePlayer{SteamID64: e.Killer.SteamID64, Team: e.Killer.Team}, alivePlayer{SteamID64: e.Victim.SteamID64, Team: e.Victim.Team})
		}

		targetDied := e.Victim != nil && e.Victim.SteamID64 == targetID
		if targetDied {
			c.RecordTargetIdentity(e.Victim.Name, teamLabel(e.Victim.Team))
			// CS2 can emit cleanup deaths after RoundEnd, when the game-state
			// counter already points at the next round. Only a live round owns a
			// POV boundary; post-round events must not clip the following round.
			if activeRound > 0 && roundLive {
				c.RecordTargetDeath(targetDeath(e, TargetDeath{Round: activeRound, Tick: gi.Tick}))
			} else if activeRound > 0 {
				freezeDeaths = append(freezeDeaths, TargetDeath{Round: activeRound, Tick: gi.Tick})
			}
		}
		if e.Killer != nil && e.Killer.SteamID64 == targetID {
			c.RecordTargetIdentity(e.Killer.Name, teamLabel(e.Killer.Team))
		} else if targetDied {
			return
		}

		rk, ok := BuildRawKill(e, gi, targetID, r.ExcludeTeamKills)
		if !ok {
			return
		}
		c.RecordKill(rk)
	})

	if watch != nil {
		watch.bind(p)
	}

	p.RegisterEventHandler(func(events.RoundStart) {
		gs := p.GameState()
		tick := gs.IngameTick()
		if tick > maxTick {
			maxTick = tick
		}
		if tick < 1 {
			tick = 1
		}
		activeRound = gs.TotalRoundsPlayed() + 1
		roundLive = false
		c.RecordRoundStart(RoundStart{Round: activeRound, Tick: tick})
	})

	p.RegisterEventHandler(func(events.RoundFreezetimeEnd) {
		gs := p.GameState()
		tick := gs.IngameTick()
		if tick > maxTick {
			maxTick = tick
		}
		if tick < 1 {
			tick = 1
		}
		if activeRound == 0 {
			activeRound = gs.TotalRoundsPlayed() + 1
		}
		roundLive = true
		c.RecordRoundLiveStart(RoundLiveStart{Round: activeRound, Tick: tick})
	})

	p.RegisterEventHandler(func(events.RoundEnd) {
		gs := p.GameState()
		tick := gs.IngameTick()
		if tick > maxTick {
			maxTick = tick
		}
		round := gs.TotalRoundsPlayed() + 1
		if activeRound > 0 {
			round = activeRound
		}
		c.RecordRoundEnd(RoundEnd{Round: round, Tick: tick})
		activeRound = 0
		roundLive = false
	})

	if alsoRecap {
		p.RegisterEventHandler(func(events.FrameDone) {
			tick := p.GameState().IngameTick()
			if tick > factsLastTick {
				factsLastTick = tick
			}
			code := ""
			participants := p.GameState().Participants()
			if participants != nil {
				for _, player := range participants.Playing() {
					if player == nil || player.Entity == nil || player.SteamID64 != targetID {
						continue
					}
					code = player.CrosshairCode()
					break
				}
			}
			crosshairs.record(tick, code)
		})
	}
	if err := parseCaptureDemo(p); err != nil {
		return DualPlan{}, fmt.Errorf("parsing demo: %w", err)
	}

	if m.Tickrate <= 0 {
		m.Tickrate = int(p.TickRate())
	}
	if m.Map == "" {
		m.Map = mapName
	}
	if m.DurationTicks <= 0 {
		m.DurationTicks = maxTick
	}

	if watch != nil {
		for _, u := range watch.flush() {
			annotateUtilityDestination(u, m.Map, watch.placeGeometry())
			c.RecordUtility(*u)
		}
	}

	plan, err := c.BuildPlan(m, mode)
	if err != nil {
		// Translate the collector's "target not seen" error into the sentinel
		// so the CLI can map it to its exit code.
		if errors.Is(err, ErrTargetNotFound) {
			return DualPlan{}, ErrTargetNotFound
		}
		return DualPlan{}, err
	}
	dual := DualPlan{Kills: plan}
	if !alsoRecap || mode == SegmentModeRecap {
		return dual, nil
	}
	factsMeta := m
	factsMeta.DurationTicks = max(m.DurationTicks, factsLastTick)
	dual.Facts = c.fullDemoFacts(factsMeta, freezeDeaths, sawMatchStart)
	dual.Facts.Crosshairs = crosshairs.samples
	if crosshairs.overflow {
		dual.Facts.Complete = false
		dual.Facts.Warnings = append(dual.Facts.Warnings, recapplan.Notice{Code: "crosshair_evidence_limit", Message: "Observed crosshair exceeded its bounded source timeline"})
	}
	if dual.Recap, err = c.BuildPlan(m, SegmentModeRecap); err != nil {
		return DualPlan{Kills: plan}, err
	}
	if dual.Deaths, err = c.BuildPlan(m, SegmentModeDeaths); err != nil {
		return DualPlan{Kills: plan}, err
	}
	return dual, nil
}
