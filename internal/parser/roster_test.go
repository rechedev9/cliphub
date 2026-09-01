package parser

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"
	dp "github.com/markus-wa/godispatch"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

// loadRosterDemo reads the demo the roster scan runs against, mirroring the
// parser/worker convention: TEST_DEMO_PATH overrides the testdata fixture, and
// an unavailable demo skips the test rather than failing the gate.
func loadRosterDemo(t *testing.T) []byte {
	t.Helper()
	path := os.Getenv("TEST_DEMO_PATH")
	if path == "" {
		path = filepath.Join("..", "..", "testdata", "lavked-vs-tnc-m2-nuke.dem")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("no test demo at %s: %v", path, err)
	}
	return b
}

func TestRosterScansRealDemo(t *testing.T) {
	demo := loadRosterDemo(t)

	p := demoinfocs.NewParser(bytes.NewReader(demo))
	defer p.Close()

	result, err := RosterScan(p)
	if err != nil {
		t.Fatalf("RosterScan error = %v", err)
	}
	roster := result.Players
	if len(roster) == 0 {
		t.Fatal("roster is empty, want at least one player")
	}
	if result.Match.Map == "" {
		t.Error("Match.Map is empty, want the demo header's map name")
	}
	if result.Match.Rounds <= 0 {
		t.Errorf("Match.Rounds = %d, want > 0", result.Match.Rounds)
	}
	if result.Match.ScoreCT < 0 || result.Match.ScoreT < 0 {
		t.Errorf("Match scores = {CT:%d T:%d}, want non-negative", result.Match.ScoreCT, result.Match.ScoreT)
	}
	if result.Match.ScoreCT+result.Match.ScoreT > result.Match.Rounds {
		t.Errorf("Match scores {CT:%d T:%d} sum to more than Rounds=%d",
			result.Match.ScoreCT, result.Match.ScoreT, result.Match.Rounds)
	}

	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "sorted by kills desc then name asc",
			check: func(t *testing.T) {
				for i := 1; i < len(roster); i++ {
					prev, cur := roster[i-1], roster[i]
					if prev.Kills < cur.Kills {
						t.Fatalf("roster[%d].Kills=%d < roster[%d].Kills=%d, want desc", i-1, prev.Kills, i, cur.Kills)
					}
					if prev.Kills == cur.Kills && prev.Name > cur.Name {
						t.Fatalf("roster[%d].Name=%q > roster[%d].Name=%q at equal kills, want asc", i-1, prev.Name, i, cur.Name)
					}
				}
			},
		},
		{
			name: "skips bots and zero steamids",
			check: func(t *testing.T) {
				for _, pl := range roster {
					if pl.SteamID64 == "" || pl.SteamID64 == "0" {
						t.Fatalf("roster includes a bot / zero steamid: %#v", pl)
					}
				}
			},
		},
		{
			name: "tallies are non-negative and at least one kill exists",
			check: func(t *testing.T) {
				totalKills := 0
				for _, pl := range roster {
					if pl.Kills < 0 || pl.Deaths < 0 || pl.Assists < 0 {
						t.Fatalf("negative tally: %#v", pl)
					}
					totalKills += pl.Kills
				}
				if totalKills == 0 {
					t.Fatal("roster has zero total kills, expected > 0 (regression)")
				}
			},
		},
	}
	if os.Getenv("TEST_DEMO_PATH") == "" {
		const maaryySteamID = "76561198148986856"
		tests = append(tests, struct {
			name  string
			check func(t *testing.T)
		}{
			name: "contains the default fixture target steamid",
			check: func(t *testing.T) {
				if findRosterPlayer(roster, maaryySteamID) == nil {
					t.Fatalf("default fixture roster missing steamid %s", maaryySteamID)
				}
			},
		})
	}
	for _, tc := range tests {
		t.Run(tc.name, tc.check)
	}
}

func TestSortRosterOrdersByKillsDescThenNameAsc(t *testing.T) {
	tally := map[uint64]*PlayerStat{
		1: {SteamID64: "1", Name: "charlie", Kills: 10},
		2: {SteamID64: "2", Name: "alice", Kills: 20},
		3: {SteamID64: "3", Name: "bob", Kills: 20},
		4: {SteamID64: "4", Name: "dave", Kills: 5},
	}

	got := sortRoster(tally)

	wantNames := []string{"alice", "bob", "charlie", "dave"}
	if len(got) != len(wantNames) {
		t.Fatalf("len = %d, want %d", len(got), len(wantNames))
	}
	for i, want := range wantNames {
		if got[i].Name != want {
			t.Errorf("got[%d].Name = %q, want %q (full order: %+v)", i, got[i].Name, want, got)
		}
	}
}

func TestRosterTeamKeepsOnlyPlayingTeams(t *testing.T) {
	tests := []struct {
		name string
		team common.Team
		want string
	}{
		{name: "counter-terrorists", team: common.TeamCounterTerrorists, want: "CT"},
		{name: "terrorists", team: common.TeamTerrorists, want: "T"},
		{name: "spectators collapse to empty", team: common.TeamSpectators, want: ""},
		{name: "unassigned collapse to empty", team: common.TeamUnassigned, want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := rosterTeam(tc.team); got != tc.want {
				t.Errorf("rosterTeam(%v) = %q, want %q", tc.team, got, tc.want)
			}
		})
	}
}

func findRosterPlayer(roster []PlayerStat, steamID string) *PlayerStat {
	for i := range roster {
		if roster[i].SteamID64 == steamID {
			return &roster[i]
		}
	}
	return nil
}

// scanParticipants is a minimal demoinfocs.Participants double: RoundStart's
// handler only ever reads Playing().
type scanParticipants struct {
	demoinfocs.Participants
	players []*common.Player
}

func (p scanParticipants) Playing() []*common.Player { return p.players }

// scanGameState is a minimal demoinfocs.GameState double covering what the
// roster accumulator and RosterScan read: the current tick, the round's
// participants, and the two team scores. Team scores are always 0 here since
// TeamState.Score() reads a real sendtables entity that this double cannot
// fabricate; side-swap scoring is exercised against a real demo instead (see
// TestRosterScansRealDemo, skipped without a local fixture).
type scanGameState struct {
	demoinfocs.GameState
	tick         int
	rounds       int
	participants []*common.Player
}

func (gs *scanGameState) IngameTick() int        { return gs.tick }
func (gs *scanGameState) TotalRoundsPlayed() int { return gs.rounds }
func (gs *scanGameState) Participants() demoinfocs.Participants {
	return scanParticipants{players: gs.participants}
}
func (gs *scanGameState) TeamCounterTerrorists() *common.TeamState { return &common.TeamState{} }
func (gs *scanGameState) TeamTerrorists() *common.TeamState        { return &common.TeamState{} }

// fakeScanParser is a demoinfocs.Parser double that captures the handlers
// rosterAccumulator.register wires up and fires them, in order, from a
// caller-supplied script when ParseToEnd is called. This drives RosterScan
// through the same code path a real demo would, without needing a .dem
// fixture.
type fakeScanParser struct {
	demoinfocs.Parser
	gs              *scanGameState
	matchStart      func(events.MatchStart)
	roundStart      func(events.RoundStart)
	kill            func(events.Kill)
	playerHurt      func(events.PlayerHurt)
	weaponFire      func(events.WeaponFire)
	mvp             func(events.RoundMVPAnnouncement)
	roundEnd        func(events.RoundEnd)
	serverInfo      func(*msg.CSVCMsg_ServerInfo)
	clanNameUpdated func(events.TeamClanNameUpdated)
	script          func(*fakeScanParser)
}

func (p *fakeScanParser) RegisterEventHandler(h any) dp.HandlerIdentifier {
	switch fn := h.(type) {
	case func(events.MatchStart):
		p.matchStart = fn
	case func(events.RoundStart):
		p.roundStart = fn
	case func(events.Kill):
		p.kill = fn
	case func(events.PlayerHurt):
		p.playerHurt = fn
	case func(events.WeaponFire):
		p.weaponFire = fn
	case func(events.RoundMVPAnnouncement):
		p.mvp = fn
	case func(events.RoundEnd):
		p.roundEnd = fn
	case func(events.TeamClanNameUpdated):
		p.clanNameUpdated = fn
	}
	return nil
}

func TestDemoCollectorsDiscardEventsBeforeMatchStart(t *testing.T) {
	target := mkPlayer(killerID, "Target", common.TeamCounterTerrorists)
	victim := mkPlayer(victimID, "Victim", common.TeamTerrorists)
	killEvent := events.Kill{
		Killer: target,
		Victim: victim,
		Weapon: mkEquipment(common.EqAK47, "weapon_ak47"),
	}
	utilityEvent := events.WeaponFire{
		Shooter: target,
		Weapon:  mkEquipment(common.EqSmoke, "weapon_smokegrenade"),
	}
	meta := PlanMeta{DemoPath: "match.dem", SHA256: "sha", Tickrate: 64, DurationTicks: 2_000}

	tests := []struct {
		name string
		run  func(*fakeScanParser) (killplan.Plan, error)
	}{
		{
			name: "kills",
			run: func(p *fakeScanParser) (killplan.Plan, error) {
				return runKills(p, strconv.FormatUint(killerID, 10), rules.Default(), meta)
			},
		},
		{
			name: "smokes",
			run: func(p *fakeScanParser) (killplan.Plan, error) {
				return runSmokes(p, strconv.FormatUint(killerID, 10), rules.Default(), meta)
			},
		},
		{
			name: "utility",
			run: func(p *fakeScanParser) (killplan.Plan, error) {
				return runUtility(p, strconv.FormatUint(killerID, 10), rules.Default(), meta)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &fakeScanParser{gs: &scanGameState{}}
			p.script = func(p *fakeScanParser) {
				p.gs.tick = 100
				if tc.name == "kills" {
					p.kill(killEvent)
				} else {
					p.weaponFire(utilityEvent)
				}
				p.matchStart(events.MatchStart{})
				p.gs.tick = 1_000
				if tc.name == "kills" {
					p.kill(killEvent)
				} else {
					p.weaponFire(utilityEvent)
				}
			}

			plan, err := tc.run(p)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Segments) != 1 {
				t.Fatalf("segments = %#v, want one live segment", plan.Segments)
			}
			switch tc.name {
			case "kills":
				if len(plan.Segments[0].Kills) != 1 || plan.Segments[0].Kills[0].Tick != 1_000 {
					t.Fatalf("kills = %#v, want only live kill", plan.Segments[0].Kills)
				}
			default:
				if len(plan.Segments[0].Utility) != 1 || plan.Segments[0].Utility[0].ThrowTick != 1_000 {
					t.Fatalf("utility = %#v, want only live throw", plan.Segments[0].Utility)
				}
			}
		})
	}
}

func TestRosterScanUsesPerPlayerRoundDenominatorsAndKeepsDamageOnlyPlayers(t *testing.T) {
	always := mkPlayer(killerID, "Always", common.TeamCounterTerrorists)
	late := mkPlayer(victimID+1, "Late", common.TeamCounterTerrorists)
	damageOnly := mkPlayer(victimID+2, "DamageOnly", common.TeamCounterTerrorists)
	victim := mkPlayer(victimID, "Victim", common.TeamTerrorists)
	p := &fakeScanParser{gs: &scanGameState{}}
	p.script = func(p *fakeScanParser) {
		p.matchStart(events.MatchStart{})
		p.gs.participants = []*common.Player{always, victim}
		p.roundStart(events.RoundStart{})
		p.roundEnd(events.RoundEnd{})

		p.gs.participants = []*common.Player{always, late, damageOnly, victim}
		p.roundStart(events.RoundStart{})
		p.kill(events.Kill{Killer: always, Victim: victim, Assister: late})
		p.playerHurt(events.PlayerHurt{Attacker: damageOnly, Player: victim, HealthDamageTaken: 25})
		p.roundEnd(events.RoundEnd{})
	}

	result, err := RosterScan(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := findRosterPlayer(result.Players, strconv.FormatUint(killerID, 10)); got == nil || got.Rounds != 2 {
		t.Fatalf("always player = %#v, want two rounds", got)
	}
	if got := findRosterPlayer(result.Players, strconv.FormatUint(late.SteamID64, 10)); got == nil || got.Rounds != 1 || got.Assists != 1 {
		t.Fatalf("late player = %#v, want one-round assister", got)
	}
	if got := findRosterPlayer(result.Players, strconv.FormatUint(damageOnly.SteamID64, 10)); got == nil || got.Rounds != 1 || got.ADR != 25 {
		t.Fatalf("damage-only player = %#v, want one round and 25 ADR", got)
	}
}

func TestRosterScanWithoutRoundStartMergesPlayingRosterAfterCombat(t *testing.T) {
	killer := mkPlayer(killerID, "Killer", common.TeamCounterTerrorists)
	teammate := mkPlayer(killerID+1, "Teammate", common.TeamCounterTerrorists)
	victim := mkPlayer(victimID, "Victim", common.TeamTerrorists)
	bystander := mkPlayer(victimID+1, "Bystander", common.TeamTerrorists)
	p := &fakeScanParser{gs: &scanGameState{}}
	p.script = func(p *fakeScanParser) {
		p.matchStart(events.MatchStart{})
		p.gs.participants = []*common.Player{killer, teammate, victim, bystander}
		p.kill(events.Kill{Killer: killer, Victim: victim})
		p.roundEnd(events.RoundEnd{})
	}

	result, err := RosterScan(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, player := range []*common.Player{killer, teammate, victim, bystander} {
		got := findRosterPlayer(result.Players, strconv.FormatUint(player.SteamID64, 10))
		if got == nil || got.Rounds != 1 {
			t.Fatalf("player %s = %#v, want one round from fallback roster", player.Name, got)
		}
	}
}

func TestRosterScanWithEmptyRoundStartUsesPlayingRosterAtRoundEnd(t *testing.T) {
	player := mkPlayer(killerID, "LateRoster", common.TeamCounterTerrorists)
	p := &fakeScanParser{gs: &scanGameState{}}
	p.script = func(p *fakeScanParser) {
		p.matchStart(events.MatchStart{})
		p.roundStart(events.RoundStart{})
		p.gs.participants = []*common.Player{player}
		p.roundEnd(events.RoundEnd{})
	}

	result, err := RosterScan(p)
	if err != nil {
		t.Fatal(err)
	}
	got := findRosterPlayer(result.Players, strconv.FormatUint(player.SteamID64, 10))
	if got == nil || got.Rounds != 1 {
		t.Fatalf("player = %#v, want one fallback round", got)
	}
}

func TestRosterScanSupplementsPartialRoundStartRosterAndPreservesCapturedSide(t *testing.T) {
	traded := mkPlayer(killerID, "Traded", common.TeamCounterTerrorists)
	avenger := mkPlayer(killerID+1, "Avenger", common.TeamCounterTerrorists)
	quiet := mkPlayer(killerID+2, "Quiet", common.TeamCounterTerrorists)
	enemy := mkPlayer(victimID, "Enemy", common.TeamTerrorists)
	p := &fakeScanParser{gs: &scanGameState{}}
	p.script = func(p *fakeScanParser) {
		p.matchStart(events.MatchStart{})

		p.gs.participants = []*common.Player{traded}
		p.roundStart(events.RoundStart{})
		p.gs.tick = 100
		p.kill(events.Kill{Killer: enemy, Victim: traded})
		p.gs.tick = 110
		p.kill(events.Kill{Killer: avenger, Victim: enemy})

		// RoundEnd must add Quiet from the current Playing roster without
		// replacing Traded's CT side captured at RoundStart.
		traded.Team = common.TeamTerrorists
		p.gs.participants = []*common.Player{traded, avenger, quiet, enemy}
		p.roundEnd(events.RoundEnd{})

		traded.Team = common.TeamCounterTerrorists
		p.gs.participants = []*common.Player{traded, avenger, quiet, enemy}
		p.roundStart(events.RoundStart{})
		p.playerHurt(events.PlayerHurt{Attacker: quiet, Player: enemy, HealthDamageTaken: 40})
		p.roundEnd(events.RoundEnd{})
	}

	result, err := RosterScan(p)
	if err != nil {
		t.Fatal(err)
	}
	gotQuiet := findRosterPlayer(result.Players, strconv.FormatUint(quiet.SteamID64, 10))
	if gotQuiet == nil || gotQuiet.Rounds != 2 || gotQuiet.ADR != 20 {
		t.Fatalf("quiet player = %#v, want two-round denominator and 20 ADR", gotQuiet)
	}
	gotTraded := findRosterPlayer(result.Players, strconv.FormatUint(traded.SteamID64, 10))
	if gotTraded == nil || gotTraded.KAST != 100 {
		t.Fatalf("traded player = %#v, want captured CT side to preserve traded death", gotTraded)
	}
}

func (p *fakeScanParser) RegisterNetMessageHandler(h any) dp.HandlerIdentifier {
	if fn, ok := h.(func(*msg.CSVCMsg_ServerInfo)); ok {
		p.serverInfo = fn
	}
	return nil
}

func (p *fakeScanParser) TickRate() float64               { return 64 }
func (p *fakeScanParser) Close() error                    { return nil }
func (p *fakeScanParser) Cancel()                         {}
func (p *fakeScanParser) GameState() demoinfocs.GameState { return p.gs }
func (p *fakeScanParser) ParseToEnd() error {
	p.script(p)
	return nil
}

func TestRosterScanCapturesClanNamesFromEvents(t *testing.T) {
	ctState := common.NewTeamState(common.TeamCounterTerrorists, func(common.Team) []*common.Player { return nil }, nil)
	tState := common.NewTeamState(common.TeamTerrorists, func(common.Team) []*common.Player { return nil }, nil)
	p := &fakeScanParser{gs: &scanGameState{}}
	p.script = func(p *fakeScanParser) {
		p.matchStart(events.MatchStart{})
		p.clanNameUpdated(events.TeamClanNameUpdated{NewName: "NAVI", TeamState: &ctState})
		p.clanNameUpdated(events.TeamClanNameUpdated{NewName: "FaZe", TeamState: &tState})
	}

	result, err := RosterScan(p)
	if err != nil {
		t.Fatal(err)
	}
	if result.Match.ClanNameCT != "NAVI" || result.Match.ClanNameT != "FaZe" {
		t.Fatalf("clan names = %q / %q, want NAVI / FaZe", result.Match.ClanNameCT, result.Match.ClanNameT)
	}
}

func TestRosterScanTracksMultiKillRoundsAndMatchInfo(t *testing.T) {
	ace := mkPlayer(killerID, "Ace", common.TeamCounterTerrorists)
	victims := []*common.Player{
		mkPlayer(2, "v2", common.TeamTerrorists),
		mkPlayer(3, "v3", common.TeamTerrorists),
		mkPlayer(4, "v4", common.TeamTerrorists),
		mkPlayer(5, "v5", common.TeamTerrorists),
		mkPlayer(6, "v6", common.TeamTerrorists),
	}
	participants := append([]*common.Player{ace}, victims...)
	mapName := "de_mirage"

	p := &fakeScanParser{gs: &scanGameState{participants: participants}}
	p.script = func(p *fakeScanParser) {
		p.serverInfo(&msg.CSVCMsg_ServerInfo{MapName: &mapName})
		p.matchStart(events.MatchStart{})

		// Round 1: ace kills 3 of the 5 opponents (a 3k round).
		p.roundStart(events.RoundStart{})
		for i := 0; i < 3; i++ {
			p.kill(events.Kill{Killer: ace, Victim: victims[i]})
		}
		p.roundEnd(events.RoundEnd{})

		// Round 2: ace kills all 5 opponents (an ace round).
		p.roundStart(events.RoundStart{})
		for i := 0; i < 5; i++ {
			p.kill(events.Kill{Killer: ace, Victim: victims[i]})
		}
		p.roundEnd(events.RoundEnd{})
	}

	result, err := RosterScan(p)
	if err != nil {
		t.Fatalf("RosterScan error = %v", err)
	}

	got := findRosterPlayer(result.Players, "76561198000000000")
	if got == nil {
		t.Fatal("roster missing the ace player")
	}
	if got.Rounds2K != 0 || got.Rounds3K != 1 || got.Rounds4K != 0 || got.Rounds5K != 1 {
		t.Errorf("multi-kill rounds = {2k:%d 3k:%d 4k:%d 5k:%d}, want {0 1 0 1}",
			got.Rounds2K, got.Rounds3K, got.Rounds4K, got.Rounds5K)
	}

	if result.Match.Map != mapName {
		t.Errorf("Match.Map = %q, want %q", result.Match.Map, mapName)
	}
	if result.Match.Rounds != 2 {
		t.Errorf("Match.Rounds = %d, want 2", result.Match.Rounds)
	}
}
