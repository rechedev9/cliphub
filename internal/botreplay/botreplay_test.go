package botreplay

import (
	"errors"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func slot(n uint8) *uint8 { return &n }

func sample(s uint8, x, y float64, alive bool) tacticalplan.Sample {
	flags := tacticalplan.SampleFlags(0)
	if alive {
		flags |= tacticalplan.FlagAlive
	}
	return tacticalplan.Sample{Slot: s, X: x, Y: y, Z: -160, Yaw: 90, Pitch: 350, Flags: flags}
}

// fixture is round 3 on Mirage: two CT (slots 0, 1) and two T (slots 2, 3).
// The T smoke at 1600 and flash at 1650 fold into one moment; slot 3 dies at 2300.
func fixture() Options {
	doc := tacticalplan.Document{
		Demo: tacticalplan.Demo{Map: "de_mirage", Tickrate: 64},
		Players: []tacticalplan.Player{
			{Slot: 0, SteamID64: "1", Name: "ana"},
			{Slot: 1, SteamID64: "2", Name: "bea\"; quit"},
			{Slot: 2, SteamID64: "3", Name: "carl"},
			{Slot: 3, SteamID64: "4", Name: "dan"},
		},
	}
	round := tacticalplan.Round{
		Number: 3, TickStart: 500, TickFreezeEnd: 1000, TickEnd: 5000,
		Players: []tacticalplan.PlayerRound{
			{Slot: 3, Side: tacticalplan.SideT}, {Slot: 0, Side: tacticalplan.SideCT},
			{Slot: 2, Side: tacticalplan.SideT}, {Slot: 1, Side: tacticalplan.SideCT},
		},
		Class: tacticalplan.Class{FirstContactTick: 2300},
		Bomb:  &tacticalplan.Bomb{PlantTick: 3000},
		Events: []tacticalplan.Event{
			{Tick: 1700, Kind: tacticalplan.EventSmoke, Side: tacticalplan.SideT, ActorSlot: slot(2), Pos: [3]float64{1, 2, 3},
				Throw: &tacticalplan.Throw{Tick: 1600, Pos: [3]float64{-300, -1200, -167.5}, Yaw: 45, Pitch: 340}},
			{Tick: 1650, Kind: tacticalplan.EventFlash, Side: tacticalplan.SideT, ActorSlot: slot(3), Pos: [3]float64{4, 5, 6}},
			{Tick: 1650, Kind: tacticalplan.EventFlash, Side: tacticalplan.SideT, ActorSlot: slot(3), Pos: [3]float64{4, 5, 6}},
			{Tick: 1800, Kind: tacticalplan.EventMolotov, Side: tacticalplan.SideCT, ActorSlot: slot(0), Pos: [3]float64{7, 8, 9}},
		},
	}
	frames := []tacticalplan.Frame{
		{Tick: 1000, Samples: []tacticalplan.Sample{sample(0, 100, 100, true), sample(1, 110, 100, true), sample(2, -100, -100, true), sample(3, -110, -100, true)}},
		{Tick: 1600, Samples: []tacticalplan.Sample{sample(0, 200, 100, true), sample(1, 210, 100, true), sample(2, -300, -1200, true), sample(3, -210, -100, true)}},
		{Tick: 2304, Samples: []tacticalplan.Sample{sample(0, 300, 100, true), sample(1, 310, 100, false), sample(2, -400, -100, true), sample(3, -310, -100, false)}},
		{Tick: 3000, Samples: []tacticalplan.Sample{sample(0, 400, 100, true), sample(2, -500, -100, true)}},
	}
	return Options{Map: "de_mirage", Doc: doc, Round: round, Frames: frames, Side: tacticalplan.SideCT, ReplaceSteamID: "2"}
}

const fixtureCfg = `// ClipHub Studio: contra la ronda 3 en de_mirage. Studio rewrites this file on every launch.
sv_cheats 1
bot_kick
bot_quota 0
bot_quota_mode normal
mp_autoteambalance 0
mp_limitteams 0
mp_freezetime 0
mp_roundtime 60
mp_roundtime_defuse 60
mp_buy_anywhere 1
mp_buytime 9999
mp_maxmoney 60000
mp_startmoney 60000
mp_respawn_on_death_ct 1
mp_respawn_on_death_t 1
mp_ignore_round_win_conditions 1
sv_infinite_ammo 1
ammo_grenade_limit_total 5
sv_grenade_trajectory_prac_pipreview 1
bot_stop 1
jointeam 3 1
bot_add_t
bot_add_t
mp_warmup_end

alias cliphub_m1_b2 "setpos_player 2 -100.00 -100.00 -160.00; setang_player 2 -10.00 90.00 0"
alias cliphub_m1_b3 "setpos_player 3 -110.00 -100.00 -160.00; setang_player 3 -10.00 90.00 0"
alias cliphub_s1_m1 "setpos 100.00 100.00 -160.00; setang -10.00 90.00 0"
alias cliphub_s2_m1 "setpos 110.00 100.00 -160.00; setang -10.00 90.00 0"
alias cliphub_momento_1 "cliphub_m1_b2; cliphub_m1_b3; cliphub_yo_m1; echo ClipHub 1/4: Salida a 0.0 s; alias cliphub_paso cliphub_momento_2"
alias cliphub_m2_b2 "setpos_player 2 -300.00 -1200.00 -160.00; setang_player 2 -10.00 90.00 0"
alias cliphub_m2_b3 "setpos_player 3 -210.00 -100.00 -160.00; setang_player 3 -10.00 90.00 0"
alias cliphub_s1_m2 "setpos 200.00 100.00 -160.00; setang -10.00 90.00 0"
alias cliphub_s2_m2 "setpos 210.00 100.00 -160.00; setang -10.00 90.00 0"
alias cliphub_momento_2 "cliphub_m2_b2; cliphub_m2_b3; cliphub_yo_m2; echo ClipHub 2/4: Granadas a 9.4 s; echo - Humo de carl; echo - Flash de dan; alias cliphub_paso cliphub_momento_3"
alias cliphub_m3_b2 "setpos_player 2 -400.00 -100.00 -160.00; setang_player 2 -10.00 90.00 0"
alias cliphub_m3_b3 "setpos_player 3 -110.00 -100.00 -160.00; setang_player 3 -10.00 90.00 0"
alias cliphub_s1_m3 "setpos 300.00 100.00 -160.00; setang -10.00 90.00 0"
alias cliphub_s2_m3 "echo bea quit ya ha muerto en este momento"
alias cliphub_momento_3 "cliphub_m3_b2; cliphub_m3_b3; cliphub_yo_m3; echo ClipHub 3/4: Primer contacto a 20.3 s; alias cliphub_paso cliphub_momento_4"
alias cliphub_m4_b2 "setpos_player 2 -500.00 -100.00 -160.00; setang_player 2 -10.00 90.00 0"
alias cliphub_m4_b3 "setpos_player 3 -110.00 -100.00 -160.00; setang_player 3 -10.00 90.00 0"
alias cliphub_s1_m4 "setpos 400.00 100.00 -160.00; setang -10.00 90.00 0"
alias cliphub_s2_m4 "echo bea quit ya ha muerto en este momento"
alias cliphub_momento_4 "cliphub_m4_b2; cliphub_m4_b3; cliphub_yo_m4; echo ClipHub 4/4: Plantada a 31.3 s; alias cliphub_paso cliphub_momento_1"

alias cliphub_salida_1 "alias cliphub_yo_m1 cliphub_s1_m1; alias cliphub_yo_m2 cliphub_s1_m2; alias cliphub_yo_m3 cliphub_s1_m3; alias cliphub_yo_m4 cliphub_s1_m4; echo Sales como ana; cliphub_rr"
alias cliphub_salida_2 "alias cliphub_yo_m1 cliphub_s2_m1; alias cliphub_yo_m2 cliphub_s2_m2; alias cliphub_yo_m3 cliphub_s2_m3; alias cliphub_yo_m4 cliphub_s2_m4; echo Sales como bea quit; cliphub_rr"
alias cliphub_otra_1 "cliphub_salida_1; alias cliphub_otra cliphub_otra_2"
alias cliphub_otra_2 "cliphub_salida_2; alias cliphub_otra cliphub_otra_1"
alias cliphub_rr "jointeam 3 1; cliphub_momento_1"
alias cliphub_granada_1 "setpos -300.00 -1200.00 -167.50; setang -20.00 45.00 0; give weapon_smokegrenade; echo Granada 1: Humo de carl a 9.4 s"

alias cliphub_yo_m1 cliphub_s2_m1
alias cliphub_yo_m2 cliphub_s2_m2
alias cliphub_yo_m3 cliphub_s2_m3
alias cliphub_yo_m4 cliphub_s2_m4
alias cliphub_paso cliphub_momento_1
alias cliphub_otra cliphub_otra_1

bind KP_ENTER cliphub_rr
bind KP_PLUS cliphub_paso
bind KP_MINUS cliphub_otra
bind KP_END cliphub_granada_1

echo "ClipHub: contra la ronda 3 en de_mirage"
echo "Bots T, por orden: carl, dan"
echo "Sales como bea quit"
echo "KP_ENTER o cliphub_rr: colocar a los bots y reiniciar"
echo "KP_PLUS o cliphub_paso: siguiente momento de la ronda"
echo "KP_MINUS o cliphub_otra: otra salida"
echo "KP_END o cliphub_granada_1: Humo de carl"
echo "Los bots no se mueven ni lanzan granadas: CS2 sin plugin no lo permite"
`

func TestRenderCfgFixture(t *testing.T) {
	plan, err := Build(fixture())
	if err != nil {
		t.Fatal(err)
	}
	got := RenderCfg(plan)
	if got != fixtureCfg {
		t.Fatalf("cfg mismatch:\n%s", got)
	}
	for i, line := range strings.Split(got, "\n") {
		if len(line) > maxCfgLine {
			t.Errorf("line %d has %d bytes, over %d: %s", i+1, len(line), maxCfgLine, line)
		}
	}
}

// maxCfgLine stays under the 512-byte console line Source engines read.
const maxCfgLine = 400

func TestBuildPicksSidesAndMoments(t *testing.T) {
	plan, err := Build(fixture())
	if err != nil {
		t.Fatal(err)
	}
	if plan.BotSide != tacticalplan.SideT || len(plan.Bots) != 2 || plan.Bots[0].Name != "carl" || plan.Bots[1].Index != 3 {
		t.Fatalf("bots = %+v", plan.Bots)
	}
	if plan.Current != 1 || plan.Starts[1].SteamID64 != "2" {
		t.Fatalf("current start = %d of %+v", plan.Current, plan.Starts)
	}
	if len(plan.Moments) != 4 || plan.Moments[1].Tick != 1600 {
		t.Fatalf("moments = %+v", plan.Moments)
	}
	if plan.Moments[2].Bots[1].Alive {
		t.Fatal("a dead player's bot must wait at its spawn")
	}
	if len(plan.Lineups) != 1 || plan.Lineups[0].Kind != "smoke" {
		t.Fatalf("lineups = %+v; a grenade without a throw has no lineup and a CT molotov is not the bots'", plan.Lineups)
	}
}

func TestBuildCTMolotovIsIncendiary(t *testing.T) {
	opts := fixture()
	opts.Side, opts.ReplaceSteamID = tacticalplan.SideT, ""
	plan, err := Build(opts)
	if err != nil {
		t.Fatal(err)
	}
	notes := strings.Join(plan.Moments[1].Notes, "|")
	if !strings.Contains(notes, "Incendiaria de ana") {
		t.Fatalf("notes = %q", notes)
	}
	if plan.Current != 0 || len(plan.Lineups) != 0 {
		t.Fatalf("current %d lineups %+v", plan.Current, plan.Lineups)
	}
}

func TestBuildErrors(t *testing.T) {
	cases := map[string]func(*Options){
		"side":           func(o *Options) { o.Side = "" },
		"map":            func(o *Options) { o.Map = "mirage" },
		"replace":        func(o *Options) { o.ReplaceSteamID = "3" },
		"steamid length": func(o *Options) { o.ReplaceSteamID = strings.Repeat("1", 21) },
		"no bots":        func(o *Options) { o.Round.Players = o.Round.Players[1:2] },
	}
	for name, mutate := range cases {
		opts := fixture()
		mutate(&opts)
		if _, err := Build(opts); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
	opts := fixture()
	opts.Frames = nil
	if _, err := Build(opts); !errors.Is(err, ErrNoPositions) {
		t.Fatalf("no frames: err = %v", err)
	}
}

func TestMomentsAreThinnedKeepingFirstAndLast(t *testing.T) {
	opts := fixture()
	opts.Round.Events = nil
	opts.Round.Bomb = nil
	opts.Round.Class.FirstContactTick = 0
	opts.Round.TickEnd = 8000
	opts.Frames = append(opts.Frames, tacticalplan.Frame{Tick: 7000, Samples: []tacticalplan.Sample{sample(2, 0, 0, true), sample(3, 0, 0, true)}})
	for i := 1; i <= 30; i++ {
		opts.Round.Events = append(opts.Round.Events, tacticalplan.Event{Tick: 1000 + i*200, Kind: tacticalplan.EventHE, Side: tacticalplan.SideT, ActorSlot: slot(2)})
	}
	plan, err := Build(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Moments) != MaxMoments || plan.Moments[0].Label != "Salida" || plan.Moments[MaxMoments-1].Tick != 7000 {
		t.Fatalf("moments: %d, first %q, last tick %d", len(plan.Moments), plan.Moments[0].Label, plan.Moments[len(plan.Moments)-1].Tick)
	}
	for _, m := range plan.Moments {
		if len(m.Notes) > MaxNotes+1 {
			t.Fatalf("moment %d has %d notes", m.Tick, len(m.Notes))
		}
	}
}
