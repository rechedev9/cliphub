package practice

import (
	"fmt"
	"strconv"
	"strings"
)

// CfgName is the one file Studio owns in the CS2 cfg folder. It is always
// overwritten whole; no other config of the user is read or written.
const CfgName = "cliphub_practice.cfg"

// ExecCommand loads the practice cfg from the CS2 console.
const ExecCommand = "exec cliphub_practice"

// NextKey cycles through the spots; the first nine also get a keypad key.
const NextKey = "KP_PLUS"

var keypadKeys = []string{
	"KP_END", "KP_DOWNARROW", "KP_PGDN",
	"KP_LEFTARROW", "KP_5", "KP_RIGHTARROW",
	"KP_HOME", "KP_UPARROW", "KP_PGUP",
}

var serverSetup = []string{
	"sv_cheats 1",
	"bot_kick",
	"bot_quota 0",
	"mp_warmup_end",
	"mp_freezetime 0",
	"mp_roundtime 60",
	"mp_roundtime_defuse 60",
	"mp_buy_anywhere 1",
	"mp_buytime 9999",
	"mp_maxmoney 60000",
	"mp_startmoney 60000",
	"mp_respawn_on_death_ct 1",
	"mp_respawn_on_death_t 1",
	"mp_ignore_round_win_conditions 1",
	"sv_infinite_ammo 1",
	"ammo_grenade_limit_total 5",
	"sv_grenade_trajectory_prac_pipreview 1",
}

// SpotAlias names the console alias that teleports to spot n (1-based).
func SpotAlias(n int) string {
	return "cliphub_spot_" + strconv.Itoa(n)
}

// SpotKey is the keypad key bound to spot n, or "" past the ninth spot.
func SpotKey(n int) string {
	if n < 1 || n > len(keypadKeys) {
		return ""
	}
	return keypadKeys[n-1]
}

// RenderCfg builds the cfg text. Labels are already sanitized by Validate,
// so nothing a demo or a user typed can close a quote or chain a command.
func RenderCfg(plan Plan) string {
	var b strings.Builder
	b.WriteString("// ClipHub Studio: practica en " + plan.Map + ". Studio rewrites this file on every launch.\n")
	for _, line := range serverSetup {
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")
	count := len(plan.Spots)
	for i, spot := range plan.Spots {
		n := i + 1
		fmt.Fprintf(&b, "alias %s \"%s\"\n", SpotAlias(n), spotCommands(spot, n, count))
	}
	for i := range plan.Spots {
		n := i + 1
		next := n%count + 1
		fmt.Fprintf(&b, "alias cliphub_next_%d \"%s; alias cliphub_next cliphub_next_%d\"\n", n, SpotAlias(n), next)
	}
	b.WriteString("alias cliphub_next cliphub_next_1\n\n")
	for i := range plan.Spots {
		if key := SpotKey(i + 1); key != "" {
			fmt.Fprintf(&b, "bind %s %s\n", key, SpotAlias(i+1))
		}
	}
	fmt.Fprintf(&b, "bind %s cliphub_next\n\n", NextKey)
	b.WriteString("echo \"ClipHub: practica en " + plan.Map + "\"\n")
	for i, spot := range plan.Spots {
		fmt.Fprintf(&b, "echo \"%s\"\n", spotLine(spot, i+1))
	}
	b.WriteString("echo \"" + NextKey + " o cliphub_next: siguiente posicion\"\n")
	return b.String()
}

func spotCommands(spot Spot, n, count int) string {
	parts := []string{
		"setpos " + coord(spot.Pos[0]) + " " + coord(spot.Pos[1]) + " " + coord(spot.Pos[2]),
		"setang " + coord(spot.Ang[0]) + " " + coord(spot.Ang[1]) + " 0",
	}
	if spot.Grenade != "" {
		parts = append(parts, "give "+grenadeWeapons[spot.Grenade])
	}
	parts = append(parts, fmt.Sprintf("echo ClipHub %d/%d: %s", n, count, spot.Label))
	return strings.Join(parts, "; ")
}

func spotLine(spot Spot, n int) string {
	line := fmt.Sprintf("%s: %s", SpotAlias(n), spot.Label)
	if spot.Grenade != "" {
		line += " (" + spot.Grenade + ")"
	}
	if key := SpotKey(n); key != "" {
		line += " [" + key + "]"
	}
	return line
}

func coord(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}
