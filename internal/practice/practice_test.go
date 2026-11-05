package practice

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func twoSpots() Input {
	return Input{Map: "de_mirage", Spots: []SpotInput{
		{Label: "Ventana desde T spawn", Pos: []float64{-1185.5, -1253, -167.97}, Ang: []float64{-12.5, 45.25}, Grenade: "smoke"},
		{Label: "Palacio", Pos: []float64{100, 200.125, -50}, Ang: []float64{0, -90}},
	}}
}

const twoSpotsCfg = `// ClipHub Studio: practica en de_mirage. Studio rewrites this file on every launch.
sv_cheats 1
bot_kick
bot_quota 0
mp_warmup_end
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

alias cliphub_spot_1 "setpos -1185.50 -1253.00 -167.97; setang -12.50 45.25 0; give weapon_smokegrenade; echo ClipHub 1/2: Ventana desde T spawn"
alias cliphub_spot_2 "setpos 100.00 200.12 -50.00; setang 0.00 -90.00 0; echo ClipHub 2/2: Palacio"
alias cliphub_next_1 "cliphub_spot_1; alias cliphub_next cliphub_next_2"
alias cliphub_next_2 "cliphub_spot_2; alias cliphub_next cliphub_next_1"
alias cliphub_next cliphub_next_1

bind KP_END cliphub_spot_1
bind KP_DOWNARROW cliphub_spot_2
bind KP_PLUS cliphub_next

echo "ClipHub: practica en de_mirage"
echo "cliphub_spot_1: Ventana desde T spawn (smoke) [KP_END]"
echo "cliphub_spot_2: Palacio [KP_DOWNARROW]"
echo "KP_PLUS o cliphub_next: siguiente posicion"
`

func TestRenderCfgTwoSpots(t *testing.T) {
	plan, err := Validate(twoSpots())
	if err != nil {
		t.Fatal(err)
	}
	if got := RenderCfg(plan); got != twoSpotsCfg {
		t.Fatalf("cfg mismatch:\n%s", got)
	}
}

func TestRenderCfgBindsOnlyNineKeypadKeys(t *testing.T) {
	in := Input{Map: "de_inferno"}
	for range 12 {
		in.Spots = append(in.Spots, SpotInput{Pos: []float64{1, 2, 3}, Ang: []float64{0, 0}})
	}
	plan, err := Validate(in)
	if err != nil {
		t.Fatal(err)
	}
	cfg := RenderCfg(plan)
	if got := strings.Count(cfg, "\nbind KP_") - 1; got != 9 {
		t.Fatalf("keypad binds = %d, want 9", got)
	}
	if !strings.Contains(cfg, `alias cliphub_next_12 "cliphub_spot_12; alias cliphub_next cliphub_next_1"`) {
		t.Fatal("the last spot must cycle back to the first")
	}
	if !strings.Contains(cfg, "echo ClipHub 12/12: Spot 12") {
		t.Fatal("an empty label falls back to its spot number")
	}
}

func TestValidateRejects(t *testing.T) {
	cases := map[string]func(*Input){
		"map without de_":   func(in *Input) { in.Map = "cs_office" },
		"map with command":  func(in *Input) { in.Map = "de_dust2; quit" },
		"map uppercase":     func(in *Input) { in.Map = "de_Mirage" },
		"no spots":          func(in *Input) { in.Spots = nil },
		"too many spots":    func(in *Input) { in.Spots = make([]SpotInput, MaxSpots+1) },
		"short pos":         func(in *Input) { in.Spots[0].Pos = []float64{1, 2} },
		"long ang":          func(in *Input) { in.Spots[0].Ang = []float64{1, 2, 3} },
		"NaN coordinate":    func(in *Input) { in.Spots[0].Pos[1] = math.NaN() },
		"infinite yaw":      func(in *Input) { in.Spots[0].Ang[1] = math.Inf(1) },
		"far coordinate":    func(in *Input) { in.Spots[0].Pos[0] = 1e9 },
		"pitch past 90":     func(in *Input) { in.Spots[0].Ang[0] = 91 },
		"unknown grenade":   func(in *Input) { in.Spots[0].Grenade = "weapon_awp" },
		"grenade injection": func(in *Input) { in.Spots[0].Grenade = "smoke; quit" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := twoSpots()
			mutate(&in)
			if _, err := Validate(in); !errors.Is(err, ErrInvalid) {
				t.Fatalf("Validate error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestSanitizeLabelBlocksConsoleInjection(t *testing.T) {
	cases := map[string]string{
		`Ventana"; quit; echo "`:     "Ventana quit echo",
		"A // comentario":            "A comentario",
		"Línea\n\tmolotov  $(x)":     "Línea molotov (x)",
		"Smoke 'CT' + flash #2!":     "Smoke CT + flash #2!",
		"   ":                        "",
		strings.Repeat("ab", 40):     strings.Repeat("ab", 24),
		"bind kp_end \"disconnect\"": "bind kp_end disconnect",
	}
	for in, want := range cases {
		got := SanitizeLabel(in)
		if got != want {
			t.Errorf("SanitizeLabel(%q) = %q, want %q", in, got, want)
		}
		if strings.ContainsAny(got, "\";/\n\r'`") {
			t.Errorf("SanitizeLabel(%q) kept a console metacharacter: %q", in, got)
		}
	}
}

type fakeGame struct {
	dir      string
	running  bool
	launched []string
}

func (g *fakeGame) CfgDir() (string, error) { return g.dir, nil }
func (g *fakeGame) Running() (bool, error)  { return g.running, nil }
func (g *fakeGame) Launch(m string) error {
	g.launched = append(g.launched, m)
	return nil
}

func TestStartWritesCfgAndLaunchesOnlyWhenClosed(t *testing.T) {
	plan, err := Validate(twoSpots())
	if err != nil {
		t.Fatal(err)
	}
	game := &fakeGame{dir: t.TempDir()}
	other := filepath.Join(game.dir, "autoexec.cfg")
	if err := os.WriteFile(other, []byte("bind x jump"), 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := Start(game, plan)
	if err != nil || !res.Launched || res.AlreadyRunning || len(game.launched) != 1 || game.launched[0] != "de_mirage" {
		t.Fatalf("closed CS2: result %+v, err %v, launches %v", res, err, game.launched)
	}
	written, err := os.ReadFile(res.CfgPath)
	if err != nil || string(written) != twoSpotsCfg || filepath.Base(res.CfgPath) != CfgName {
		t.Fatalf("cfg at %s not written as rendered: %v", res.CfgPath, err)
	}

	game.running = true
	plan.Spots = plan.Spots[:1]
	res, err = Start(game, plan)
	if err != nil || res.Launched || !res.AlreadyRunning || len(game.launched) != 1 {
		t.Fatalf("running CS2: result %+v, err %v, launches %v", res, err, game.launched)
	}
	if written, _ := os.ReadFile(res.CfgPath); strings.Contains(string(written), "cliphub_spot_2") {
		t.Fatal("a second start must overwrite the cfg whole")
	}
	if kept, _ := os.ReadFile(other); string(kept) != "bind x jump" {
		t.Fatal("the user's other configs must stay untouched")
	}
	entries, _ := os.ReadDir(game.dir)
	if len(entries) != 2 {
		t.Fatalf("cfg dir holds %d files, want autoexec and the practice cfg", len(entries))
	}
}

func TestSteamGameCfgDirNeedsTheInstallTree(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "game", "bin", "win64", "cs2.exe")
	if _, err := (SteamGame{CS2Path: exe}).CfgDir(); !errors.Is(err, ErrCS2NotFound) {
		t.Fatalf("missing cfg dir: err = %v, want ErrCS2NotFound", err)
	}
	if _, err := (SteamGame{}).CfgDir(); !errors.Is(err, ErrCS2NotFound) {
		t.Fatalf("empty path: err = %v, want ErrCS2NotFound", err)
	}
	want := filepath.Join(root, "game", "csgo", "cfg")
	if err := os.MkdirAll(want, 0o750); err != nil {
		t.Fatal(err)
	}
	if got, err := (SteamGame{CS2Path: exe}).CfgDir(); err != nil || got != want {
		t.Fatalf("CfgDir = %q, %v; want %q", got, err, want)
	}
}

func TestLaunchURL(t *testing.T) {
	want := "steam://rungameid/730//+map%20de_nuke%20+exec%20cliphub_practice"
	if got := LaunchURL("de_nuke"); got != want {
		t.Fatalf("LaunchURL = %q, want %q", got, want)
	}
}
