package botreplay

import (
	"errors"
	"slices"
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

// maxCfgLine stays under the 512-byte command line the CS2 console reads.
const maxCfgLine = 500

// console runs a cfg as the CS2 console runs aliases: it records every
// command that is neither an alias definition nor an alias call.
type console struct {
	aliases map[string]string
	binds   map[string]string
	ran     []string
}

func load(t *testing.T, cfg string) *console {
	t.Helper()
	c := &console{aliases: map[string]string{}, binds: map[string]string{}}
	for i, line := range strings.Split(strings.TrimSuffix(cfg, "\n"), "\n") {
		if len(line) > maxCfgLine {
			t.Errorf("line %d has %d bytes: %s", i+1, len(line), line)
		}
		if strings.HasPrefix(line, "//") {
			continue
		}
		kind, rest, _ := strings.Cut(line, " ")
		name, body, _ := strings.Cut(rest, " ")
		if quotes := strings.Count(body, "\""); quotes != 0 && (quotes != 2 || !strings.HasPrefix(body, "\"") || !strings.HasSuffix(body, "\"")) {
			t.Errorf("line %d has a stray quote: %s", i+1, line)
		}
		body = strings.Trim(body, "\"")
		switch kind {
		case "alias":
			c.aliases[name] = body
		case "bind":
			c.binds[name] = body
		default:
			t.Errorf("line %d runs before the map exists: %s", i+1, line)
		}
	}
	return c
}

func (c *console) run(t *testing.T, cmd string, depth int) {
	t.Helper()
	if depth > 20 {
		t.Fatalf("alias loop at %s", cmd)
	}
	for _, part := range strings.Split(cmd, ";") {
		part = strings.TrimSpace(part)
		name, rest, _ := strings.Cut(part, " ")
		switch {
		case part == "":
		case name == "alias":
			target, body, _ := strings.Cut(rest, " ")
			c.aliases[target] = body
		case c.aliases[name] != "":
			c.run(t, c.aliases[name], depth+1)
		default:
			c.ran = append(c.ran, part)
		}
	}
}

func (c *console) press(t *testing.T, key string) []string {
	t.Helper()
	c.ran = nil
	c.run(t, c.binds[key], 0)
	return c.ran
}

func has(ran []string, cmd string) bool { return slices.Contains(ran, cmd) }

func countOf(ran []string, cmd string) int {
	n := 0
	for _, r := range ran {
		if r == cmd {
			n++
		}
	}
	return n
}

func TestCfgWalkthroughInTheConsole(t *testing.T) {
	plan, err := Build(fixture())
	if err != nil {
		t.Fatal(err)
	}
	cfg := RenderCfg(plan)
	if strings.Contains(cfg, "setang_player") || strings.Contains(cfg, "jointeam 3 1") {
		t.Fatal("the cfg uses a command CS2 does not have")
	}
	c := load(t, cfg)

	first := c.press(t, RestartKey)
	if !has(first, "bot_kick") || !has(first, "jointeam 3") || !has(first, "bot_dont_shoot 1") || !has(first, "bot_stop 1") || has(first, "bot_add_t") {
		t.Fatalf("first KP_ENTER = %v", first)
	}
	second := c.press(t, RestartKey)
	if countOf(second, "bot_add_t") != 2 || !has(second, "mp_warmup_end") || has(second, "bot_kick") {
		t.Fatalf("second KP_ENTER = %v", second)
	}
	if !has(second, "echo Si los bots no se colocan, pulsa KP_DEL y vuelve a pulsar KP_ENTER") {
		t.Fatalf("setup does not say how to fix the bot numbers: %v", second)
	}
	place := c.press(t, RestartKey)
	for _, w := range []string{"setpos_player 2 -100.00 -100.00 -160.00", "setpos_player 3 -110.00 -100.00 -160.00", "setpos 110.00 100.00 -160.00", "setang -10.00 90.00 0"} {
		if !has(place, w) {
			t.Fatalf("placing = %v, want %q", place, w)
		}
	}
	next := c.press(t, NextKey)
	if !has(next, "setpos_player 2 -300.00 -1200.00 -160.00") || !has(next, "echo - Humo de carl") || !has(next, "echo ClipHub 2/4: Granadas a 9.4 s") {
		t.Fatalf("moment 2 = %v", next)
	}
	c.press(t, NextKey)
	if dead := c.press(t, NextKey); !has(dead, "setpos_player 3 -110.00 -100.00 -160.00") || !has(dead, "echo bea quit ya ha muerto en este momento") {
		t.Fatalf("moment 4: a dead rival waits at spawn and a dead user is told = %v", dead)
	}
	c.press(t, BaseKey)
	if shifted := c.press(t, RestartKey); !has(shifted, "setpos_player 3 -100.00 -100.00 -160.00") || !has(shifted, "setpos_player 4 -110.00 -100.00 -160.00") {
		t.Fatalf("after KP_DEL = %v", shifted)
	}
	for range MaxBase - 1 {
		c.press(t, BaseKey)
	}
	if wrapped := c.press(t, RestartKey); !has(wrapped, "setpos_player 2 -100.00 -100.00 -160.00") {
		t.Fatalf("the base does not wrap: %v", wrapped)
	}
	other := c.press(t, StartKey)
	if !has(other, "echo Sales como ana") || !has(other, "setpos 100.00 100.00 -160.00") {
		t.Fatalf("otra salida = %v", other)
	}
	if asAna := c.press(t, NextKey); !has(asAna, "setpos 200.00 100.00 -160.00") {
		t.Fatalf("moment 2 as ana = %v", asAna)
	}
	lineup := c.press(t, "KP_END")
	if !has(lineup, "give weapon_smokegrenade") || !has(lineup, "slot4") || !has(lineup, "setang -20.00 45.00 0") {
		t.Fatalf("lineup = %v", lineup)
	}
	again := c.press(t, PrepareKey)
	if !has(again, "bot_kick") || has(again, "bot_add_t") {
		t.Fatalf("KP_INS = %v", again)
	}
	if after := c.press(t, RestartKey); countOf(after, "bot_add_t") != 2 {
		t.Fatalf("KP_ENTER after KP_INS = %v", after)
	}
}

func TestCfgLinesStayShortWithLongNamesAndManyMoments(t *testing.T) {
	opts := fixture()
	for i := range opts.Doc.Players {
		opts.Doc.Players[i].Name = strings.Repeat("Ñandú", 12)
	}
	opts.Round.TickEnd = 8000
	opts.Frames = append(opts.Frames, tacticalplan.Frame{Tick: 7000, Samples: []tacticalplan.Sample{sample(2, 0, 0, true), sample(3, 0, 0, true)}})
	for i := 1; i <= 30; i++ {
		opts.Round.Events = append(opts.Round.Events, tacticalplan.Event{Tick: 1000 + i*130, Kind: tacticalplan.EventMolotov, Side: tacticalplan.SideT, ActorSlot: slot(2),
			Throw: &tacticalplan.Throw{Tick: 1000 + i*130, Pos: [3]float64{-12345.67, -12345.67, -1234.5}}})
	}
	plan, err := Build(opts)
	if err != nil {
		t.Fatal(err)
	}
	if n := len([]rune(plan.Bots[0].Name)); n > maxNameRunes {
		t.Fatalf("name has %d runes", n)
	}
	c := load(t, RenderCfg(plan))
	c.press(t, RestartKey)
	c.press(t, RestartKey)
	if placed := c.press(t, RestartKey); !has(placed, "setpos_player 2 -100.00 -100.00 -160.00") {
		t.Fatalf("placing after split aliases = %v", placed)
	}
	if help := c.press(t, PrepareKey); len(help) == 0 {
		t.Fatal("KP_INS ran nothing")
	}
}

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
