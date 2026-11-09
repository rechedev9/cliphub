package botreplay

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// Console keys of the session. The keypad digits throw the lineups.
const (
	RestartKey = "KP_ENTER"
	NextKey    = "KP_PLUS"
	StartKey   = "KP_MINUS"
)

// Console aliases the user can also type.
const (
	RestartAlias = "cliphub_rr"
	NextAlias    = "cliphub_paso"
	StartAlias   = "cliphub_otra"
)

// Limits says, in the user's words, what plain CS2 cannot do here.
var Limits = []string{
	"Los bots no repiten el recorrido: CS2 sin un plugin de servidor no deja que un bot siga una ruta grabada.",
	"Los bots no lanzan granadas: CS2 no deja lanzar una granada por orden. Tú lanzas las del rival desde su posición.",
	"Los comandos de chat .rr y .setup necesitan un plugin; aquí son teclas del teclado numérico.",
}

var serverSetup = []string{
	"sv_cheats 1",
	"bot_kick",
	"bot_quota 0",
	"bot_quota_mode normal",
	"mp_autoteambalance 0",
	"mp_limitteams 0",
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
	"bot_stop 1",
}

func momentAlias(k int) string { return "cliphub_momento_" + strconv.Itoa(k) }
func userAlias(k int) string   { return "cliphub_yo_m" + strconv.Itoa(k) }
func startMomentAlias(p, k int) string {
	return fmt.Sprintf("cliphub_s%d_m%d", p, k)
}
func botAlias(k, index int) string { return fmt.Sprintf("cliphub_m%d_b%d", k, index) }
func startAlias(p int) string      { return "cliphub_salida_" + strconv.Itoa(p) }
func cycleAlias(p int) string      { return "cliphub_otra_" + strconv.Itoa(p) }

// LineupAlias names the console alias of lineup n (1-based).
func LineupAlias(n int) string { return "cliphub_granada_" + strconv.Itoa(n) }

func teamNumber(side tacticalplan.Side) int {
	if side == tacticalplan.SideCT {
		return 3
	}
	return 2
}

func botAdd(side tacticalplan.Side) string {
	if side == tacticalplan.SideCT {
		return "bot_add_ct"
	}
	return "bot_add_t"
}

// RenderCfg builds the cfg text. Names are sanitized by Build, so nothing a
// demo carried can close a quote or chain a command.
func RenderCfg(plan Plan) string {
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	line("// ClipHub Studio: contra la ronda %d en %s. Studio rewrites this file on every launch.", plan.Round, plan.Map)
	for _, s := range serverSetup {
		line("%s", s)
	}
	join := fmt.Sprintf("jointeam %d 1", teamNumber(plan.UserSide))
	line("%s", join)
	for range plan.Bots {
		line("%s", botAdd(plan.BotSide))
	}
	line("mp_warmup_end")
	b.WriteString("\n")
	count := len(plan.Moments)
	for k, m := range plan.Moments {
		n := k + 1
		calls := []string{}
		for i, bot := range plan.Bots {
			pose := m.Bots[i]
			line("alias %s \"setpos_player %d %s; setang_player %d %s 0\"", botAlias(n, bot.Index), bot.Index, coords(pose.Pos), bot.Index, angles(pose.Pose))
			calls = append(calls, botAlias(n, bot.Index))
		}
		for p, st := range plan.Starts {
			if pose := m.Users[p]; pose != nil {
				line("alias %s \"setpos %s; setang %s 0\"", startMomentAlias(p+1, n), coords(pose.Pos), angles(*pose))
			} else {
				line("alias %s \"echo %s ya ha muerto en este momento\"", startMomentAlias(p+1, n), st.Name)
			}
		}
		if len(plan.Starts) > 0 {
			calls = append(calls, userAlias(n))
		}
		calls = append(calls, fmt.Sprintf("echo ClipHub %d/%d: %s a %s s", n, count, m.Label, secondsText(m.Seconds)))
		for _, note := range m.Notes {
			calls = append(calls, "echo - "+note)
		}
		calls = append(calls, fmt.Sprintf("alias %s %s", NextAlias, momentAlias(n%count+1)))
		line("alias %s \"%s\"", momentAlias(n), strings.Join(calls, "; "))
	}
	b.WriteString("\n")
	for p, st := range plan.Starts {
		n := p + 1
		parts := make([]string, 0, count+2)
		for k := range plan.Moments {
			parts = append(parts, fmt.Sprintf("alias %s %s", userAlias(k+1), startMomentAlias(n, k+1)))
		}
		parts = append(parts, "echo Sales como "+st.Name, RestartAlias)
		line("alias %s \"%s\"", startAlias(n), strings.Join(parts, "; "))
	}
	for p := range plan.Starts {
		n := p + 1
		line("alias %s \"%s; alias %s %s\"", cycleAlias(n), startAlias(n), StartAlias, cycleAlias(n%len(plan.Starts)+1))
	}
	line("alias %s \"%s; %s\"", RestartAlias, join, momentAlias(1))
	for i, l := range plan.Lineups {
		line("alias %s \"setpos %s; setang %s 0; give %s; echo Granada %d: %s de %s a %s s\"",
			LineupAlias(i+1), coords(l.Pose.Pos), angles(l.Pose), l.Weapon, i+1, grenadeLabel(l.Kind), l.Player, secondsText(l.Seconds))
	}
	b.WriteString("\n")
	// The first run only arms the aliases: the bots spawn after the warmup ends.
	for k := range plan.Moments {
		if len(plan.Starts) > 0 {
			line("alias %s %s", userAlias(k+1), startMomentAlias(plan.Current+1, k+1))
		}
	}
	line("alias %s %s", NextAlias, momentAlias(1))
	if len(plan.Starts) > 0 {
		line("alias %s %s", StartAlias, cycleAlias((plan.Current+1)%len(plan.Starts)+1))
	} else {
		line("alias %s \"echo No hay otra salida en esta ronda\"", StartAlias)
	}
	b.WriteString("\n")
	line("bind %s %s", RestartKey, RestartAlias)
	line("bind %s %s", NextKey, NextAlias)
	line("bind %s %s", StartKey, StartAlias)
	for i, l := range plan.Lineups {
		line("bind %s %s", l.Key, LineupAlias(i+1))
	}
	b.WriteString("\n")
	line("echo \"ClipHub: contra la ronda %d en %s\"", plan.Round, plan.Map)
	names := make([]string, len(plan.Bots))
	for i, bot := range plan.Bots {
		names[i] = bot.Name
	}
	line("echo \"Bots %s, por orden: %s\"", plan.BotSide, strings.Join(names, ", "))
	if len(plan.Starts) > 0 {
		line("echo \"Sales como %s\"", plan.Starts[plan.Current].Name)
	}
	line("echo \"%s o %s: colocar a los bots y reiniciar\"", RestartKey, RestartAlias)
	line("echo \"%s o %s: siguiente momento de la ronda\"", NextKey, NextAlias)
	line("echo \"%s o %s: otra salida\"", StartKey, StartAlias)
	for i, l := range plan.Lineups {
		line("echo \"%s o %s: %s de %s\"", l.Key, LineupAlias(i+1), grenadeLabel(l.Kind), l.Player)
	}
	line("echo \"Los bots no se mueven ni lanzan granadas: CS2 sin plugin no lo permite\"")
	return b.String()
}

func coords(p [3]float64) string {
	return num(p[0]) + " " + num(p[1]) + " " + num(p[2])
}

func angles(p Pose) string { return num(p.Pitch) + " " + num(p.Yaw) }

func num(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

func secondsText(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }
