package customhud

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"
	st "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/sendtables"
)

const MaxTelemetryBytes = 128 << 20

// Extract consumes the whole immutable demo, checking its digest as it goes.
// It does not launch CS2 and never substitutes example values for missing data.
func Extract(ctx context.Context, input io.Reader, demoSHA, target string, tickRate int) (Timeline, error) {
	d := Timeline{Version: Version, DemoSHA256: demoSHA, TargetSteamID: target, TickRate: tickRate, Snapshots: []Snapshot{}}
	if len(demoSHA) != 64 || len(target) != 17 || tickRate < 1 || tickRate > 1024 {
		return d, fmt.Errorf("invalid HUD extraction identity")
	}
	digest := sha256.New()
	reader := io.TeeReader(input, digest)
	p := demoinfocs.NewParser(reader)
	defer p.Close()
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		select {
		case <-ctx.Done():
			p.Cancel()
		case <-stop:
		}
	})
	defer func() { close(stop); wg.Wait() }()
	var currentRound int
	phase := "unknown"
	var serverTick uint32
	var plantedBomb st.Entity
	lastTick := -1
	identities := map[string]Player{}
	var failure error
	p.RegisterNetMessageHandler(func(tick *msg.CNETMsg_Tick) { serverTick = tick.GetTick() })
	p.RegisterEventHandler(func(events.DataTablesParsed) {
		if class := p.ServerClasses().FindByName("CPlantedC4"); class != nil {
			class.OnEntityCreated(func(entity st.Entity) {
				plantedBomb = entity
				entity.OnDestroy(func() {
					if plantedBomb == entity {
						plantedBomb = nil
					}
				})
			})
		}
	})
	p.RegisterNetMessageHandler(func(info *msg.CSVCMsg_ServerInfo) {
		if name := info.GetMapName(); name != "" {
			d.Map = name
		}
	})
	p.RegisterEventHandler(func(events.MatchStart) {
		d.Snapshots = []Snapshot{}
		currentRound = 0
		phase = "unknown"
		lastTick = -1
	})
	p.RegisterEventHandler(func(events.RoundStart) {
		gs := p.GameState()
		if gs.IsWarmupPeriod() {
			return
		}
		currentRound = gs.TotalRoundsPlayed() + 1
		phase = "freeze"
	})
	p.RegisterEventHandler(func(events.RoundFreezetimeEnd) {
		gs := p.GameState()
		if gs.IsWarmupPeriod() {
			return
		}
		if currentRound == 0 {
			currentRound = gs.TotalRoundsPlayed() + 1
		}
		phase = "live"
	})
	p.RegisterEventHandler(func(events.BombPlanted) {
		if phase != "ended" {
			phase = "planted"
		}
	})
	p.RegisterEventHandler(func(events.BombDefuseStart) {
		if phase == "planted" {
			phase = "defusing"
		}
	})
	p.RegisterEventHandler(func(events.BombDefuseAborted) {
		if phase == "defusing" {
			phase = "planted"
		}
	})
	p.RegisterEventHandler(func(events.RoundEnd) { phase = "ended" })
	p.RegisterEventHandler(func(events.FrameDone) {
		if failure != nil {
			return
		}
		gs := p.GameState()
		tick := gs.IngameTick()
		d.EndTick = max(d.EndTick, tick)
		if currentRound == 0 || gs.IsWarmupPeriod() || tick <= lastTick {
			return
		}
		lastTick = tick
		s := Snapshot{Tick: tick, Round: currentRound, Phase: phase, TimeRemaining: -1, Players: []Player{}}
		s.TimeRemaining = sourceClock(gs.Rules().Entity(), plantedBomb, serverTick, tickRate, phase)
		if team := gs.TeamCounterTerrorists(); team != nil {
			s.CTScore = team.Score()
			s.CTName = cleanText(team.ClanName(), 64)
		}
		if team := gs.TeamTerrorists(); team != nil {
			s.TScore = team.Score()
			s.TName = cleanText(team.ClanName(), 64)
		}
		seen := map[string]bool{}
		for _, pl := range gs.Participants().All() {
			if pl == nil || pl.SteamID64 == 0 || (pl.Team != common.TeamCounterTerrorists && pl.Team != common.TeamTerrorists) {
				continue
			}
			player := Player{SteamID: strconv.FormatUint(pl.SteamID64, 10), Name: cleanText(pl.Name, 100), Side: "CT", Ammo: -1, Reserve: -1}
			if pl.Team == common.TeamTerrorists {
				player.Side = "T"
			}
			if seen[player.SteamID] {
				continue
			}
			seen[player.SteamID] = true
			if pl.Entity != nil && pl.PlayerPawnEntity() != nil {
				player.Known = true
				player.Alive = pl.IsAlive()
				player.Health = max(0, pl.Health())
				player.Armor = max(0, pl.Armor())
				player.Money = pl.Money()
				player.Kills = pl.Kills()
				player.Deaths = pl.Deaths()
				player.Assists = pl.Assists()
				if weapon := pl.ActiveWeapon(); weapon != nil {
					player.Weapon = cleanText(strings.ToUpper(weapon.Type.String()), 32)
					class := weapon.Class()
					if class == common.EqClassPistols || class == common.EqClassSMG || class == common.EqClassHeavy || class == common.EqClassRifle {
						player.Ammo = sourceMagazine(weapon.Entity)
						player.Reserve = weapon.AmmoReserve()
					}
				}
			}
			identities[player.SteamID] = Player{SteamID: player.SteamID, Name: player.Name, Side: player.Side, Ammo: -1, Reserve: -1}
			s.Players = append(s.Players, player)
		}
		// Preserve identity, never stale HP/ammo, if a controller disappears.
		for id, identity := range identities {
			if !seen[id] {
				s.Players = append(s.Players, identity)
			}
		}
		sort.Slice(s.Players, func(i, j int) bool { return s.Players[i].SteamID < s.Players[j].SteamID })
		if n := len(d.Snapshots); n > 0 {
			previous := d.Snapshots[n-1]
			previous.Tick = s.Tick
			if reflect.DeepEqual(previous, s) {
				return
			}
		}
		if len(d.Snapshots) >= 2_000_000 {
			failure = fmt.Errorf("custom HUD telemetry exceeds snapshot limit")
			p.Cancel()
			return
		}
		d.Snapshots = append(d.Snapshots, s)
	})
	if err := p.ParseToEnd(); err != nil {
		if ctx.Err() != nil {
			return d, ctx.Err()
		}
		if failure != nil {
			return d, failure
		}
		return d, fmt.Errorf("parse custom HUD: %w", err)
	}
	if ctx.Err() != nil {
		return d, ctx.Err()
	}
	if failure != nil {
		return d, failure
	}
	// The demo parser may stop at its terminator before EOF. Include any tail
	// in the digest, so its identity is exactly the file approved by the user.
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return d, err
	}
	if hex.EncodeToString(digest.Sum(nil)) != demoSHA {
		return d, fmt.Errorf("custom HUD demo digest differs from approval")
	}
	if err := d.Validate(); err != nil {
		return d, err
	}
	found := false
	for _, s := range d.Snapshots {
		if hasTarget(s, target) {
			found = true
			break
		}
	}
	if !found {
		return d, fmt.Errorf("custom HUD target not found in source demo")
	}
	return d, nil
}

// CS2's network clock has a different origin from the demo's seek ticks.
// Read its actual deadlines rather than applying assumed match cvars. This
// also follows freeze-time extensions and nonstandard bomb/round durations.
func sourceClock(rules, bomb st.Entity, serverTick uint32, tickRate int, phase string) int {
	if serverTick == 0 || tickRate < 1 {
		return -1
	}
	var deadline float64
	switch phase {
	case "freeze", "live":
		if rules == nil {
			return -1
		}
		start, ok := rules.PropertyValue("m_pGameRules.m_fRoundStartTime")
		if !ok || start.Any == nil {
			return -1
		}
		deadline = float64(start.Float())
		if phase == "live" {
			length, ok := rules.PropertyValue("m_pGameRules.m_iRoundTime")
			if !ok || length.Any == nil || length.Int() <= 0 {
				return -1
			}
			deadline += float64(length.Int())
		}
	case "planted", "defusing":
		if bomb == nil {
			return -1
		}
		blow, ok := bomb.PropertyValue("m_flC4Blow")
		if !ok || blow.Any == nil {
			return -1
		}
		deadline = float64(blow.Float())
	default:
		return -1
	}
	if math.IsNaN(deadline) || math.IsInf(deadline, 0) || deadline <= 0 {
		return -1
	}
	return max(0, int(math.Ceil(deadline-float64(serverTick)/float64(tickRate))))
}

// The pinned parser's AmmoInMagazine still subtracts the Source 1 encoding
// bias. CS2 m_iClip1 is the actual magazine count (verified against capture).
func sourceMagazine(entity st.Entity) int {
	if entity == nil {
		return -1
	}
	value, ok := entity.PropertyValue("m_iClip1")
	if !ok || value.Any == nil || value.UInt32() > 10000 {
		return -1
	}
	return int(value.UInt32())
}

func Decode(input io.Reader) (Timeline, error) {
	var d Timeline
	body, err := io.ReadAll(io.LimitReader(input, MaxTelemetryBytes+1))
	if err != nil {
		return d, err
	}
	if len(body) > MaxTelemetryBytes {
		return d, fmt.Errorf("custom HUD telemetry exceeds byte limit")
	}
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return d, err
	}
	if dec.Decode(new(any)) != io.EOF {
		return d, fmt.Errorf("custom HUD telemetry must contain one document")
	}
	return d, d.Validate()
}

func Load(path string) (Timeline, error) {
	file, err := os.Open(path)
	if err != nil {
		return Timeline{}, err
	}
	defer file.Close()
	return Decode(file)
}
