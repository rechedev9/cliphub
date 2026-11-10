package botreplay

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// Console keys of the session. The keypad digits throw the lineups.
const (
	PrepareKey = "KP_INS"
	RestartKey = "KP_ENTER"
	NextKey    = "KP_PLUS"
	StartKey   = "KP_MINUS"
	BaseKey    = "KP_DEL"
)

// Console aliases the user can also type.
const (
	PrepareAlias = "cliphub_preparar"
	RestartAlias = "cliphub_rr"
	NextAlias    = "cliphub_paso"
	StartAlias   = "cliphub_otra"
	BaseAlias    = "cliphub_base_mas"
)

// DefaultBase is the setpos_player number of the first bot when the listen
// server host is 1. CS2 does not document whether the number is an entity
// index, a user id or a slot, so KP_DEL cycles the base through 1 to MaxBase.
const (
	DefaultBase = 2
	MaxBase     = 8
)

// maxBody keeps an alias body well under the 512-byte console command limit.
const maxBody = 400

// Limits says, in the user's words, what plain CS2 cannot do here.
var Limits = []string{
	"Los bots no repiten el recorrido: CS2 sin un plugin de servidor no deja que un bot siga una ruta grabada. Saltan de un momento al siguiente cuando pulsas la tecla.",
	"Los bots no miran hacia donde miraba el rival: CS2 no tiene un comando para girar a un bot.",
	"Los bots están quietos y no disparan. El bot de un rival que ya ha muerto espera en su spawn.",
	"Los bots no lanzan granadas: CS2 no deja lanzar una granada por orden. Tú lanzas las del rival desde su sitio; puede que tengas que sacar la granada con su tecla.",
	"CS2 numera a los bots de una forma que Studio no puede comprobar: si no se colocan, pulsa KP_DEL y vuelve a pulsar KP_ENTER.",
	"Las teclas del teclado numérico sustituyen a lo que tuvieras asignado en ellas.",
	"Los comandos de chat .rr y .setup necesitan un plugin; aquí son teclas del teclado numérico.",
}

var serverSetup = []string{
	"sv_cheats 1",
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
	"bot_dont_shoot 1",
}

const (
	settingsAlias = "cliphub_ajustes"
	prepareOne    = "cliphub_preparar_1"
	prepareTwo    = "cliphub_preparar_2"
	placeAlias    = "cliphub_colocar"
	helpAlias     = "cliphub_ayuda"
)

func momentAlias(k int) string { return "cliphub_momento_" + strconv.Itoa(k) }
func infoAlias(k int) string   { return fmt.Sprintf("cliphub_m%d_info", k) }
func userAlias(k int) string   { return "cliphub_yo_m" + strconv.Itoa(k) }
func startMomentAlias(p, k int) string {
	return fmt.Sprintf("cliphub_s%d_m%d", p, k)
}
func botAlias(k, i int) string        { return fmt.Sprintf("cliphub_m%d_b%d", k, i) }
func basePosAlias(b, k, i int) string { return fmt.Sprintf("cliphub_b%d_m%d_%d", b, k, i) }
func baseAlias(b int) string          { return "cliphub_base_" + strconv.Itoa(b) }
func startAlias(p int) string         { return "cliphub_salida_" + strconv.Itoa(p) }
func cycleAlias(p int) string         { return "cliphub_otra_" + strconv.Itoa(p) }

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

// aliasLines defines name to run cmds. A body over maxBody is split into
// name_p1, name_p2... and name runs those in order.
func aliasLines(name string, cmds []string) []string {
	if body := strings.Join(cmds, "; "); len(body) <= maxBody {
		return []string{fmt.Sprintf("alias %s \"%s\"", name, body)}
	}
	var lines, parts, current []string
	size := 0
	flush := func() {
		part := fmt.Sprintf("%s_p%d", name, len(parts)+1)
		lines = append(lines, fmt.Sprintf("alias %s \"%s\"", part, strings.Join(current, "; ")))
		parts, current, size = append(parts, part), nil, 0
	}
	for _, c := range cmds {
		if len(current) > 0 && size+len(c)+2 > maxBody {
			flush()
		}
		current = append(current, c)
		size += len(c) + 2
	}
	flush()
	return append(lines, fmt.Sprintf("alias %s \"%s\"", name, strings.Join(parts, "; ")))
}

// RenderCfg builds the cfg text. On a fresh launch it runs before the map's
// server exists, so its top level only defines aliases and binds: the first
// KP_ENTER sets the server up and kicks, the second adds the bots, later ones
// place them. Names are sanitized by Build, so none can close a quote.
func RenderCfg(plan Plan) string {
	lines := []string{fmt.Sprintf("// ClipHub Studio: contra la ronda %d en %s. Studio rewrites this file on every launch.", plan.Round, plan.Map)}
	add := func(more ...string) { lines = append(lines, more...) }
	join := fmt.Sprintf("jointeam %d", teamNumber(plan.UserSide))

	add(aliasLines(settingsAlias, serverSetup)...)
	add(aliasLines(prepareOne, []string{settingsAlias, "bot_kick", "bot_quota 0", join,
		"echo ClipHub: pulsa " + RestartKey + " otra vez para crear los bots", "alias " + RestartAlias + " " + prepareTwo})...)
	two := []string{}
	for range plan.Bots {
		two = append(two, botAdd(plan.BotSide))
	}
	two = append(two, "mp_warmup_end", "alias "+RestartAlias+" "+placeAlias, helpAlias)
	add(aliasLines(prepareTwo, two)...)
	add(aliasLines(PrepareAlias, []string{"alias " + RestartAlias + " " + prepareOne, prepareOne})...)
	add(aliasLines(placeAlias, []string{momentAlias(1)})...)

	add(baseLines(plan)...)
	add(momentLines(plan)...)
	add(startLines(plan)...)
	for i, l := range plan.Lineups {
		add(aliasLines(LineupAlias(i+1), []string{
			"setpos " + coords(l.Pose.Pos), "setang " + angles(l.Pose) + " 0", "give " + l.Weapon, "slot4",
			fmt.Sprintf("echo Granada %d: %s de %s a %s s", i+1, grenadeLabel(l.Kind), l.Player, secondsText(l.Seconds)),
		})...)
	}
	add(aliasLines(helpAlias, helpLines(plan))...)

	// The starting state: the default base, the chosen spawn and setup on KP_ENTER.
	for k := range plan.Moments {
		for i := range plan.Bots {
			add(fmt.Sprintf("alias %s %s", botAlias(k+1, i+1), basePosAlias(DefaultBase, k+1, i+1)))
		}
		if len(plan.Starts) > 0 {
			add(fmt.Sprintf("alias %s %s", userAlias(k+1), startMomentAlias(plan.Current+1, k+1)))
		}
	}
	add(fmt.Sprintf("alias %s %s", BaseAlias, baseAlias(DefaultBase%MaxBase+1)))
	add(fmt.Sprintf("alias %s %s", NextAlias, momentAlias(1)))
	if len(plan.Starts) > 0 {
		add(fmt.Sprintf("alias %s %s", StartAlias, cycleAlias((plan.Current+1)%len(plan.Starts)+1)))
	} else {
		add(fmt.Sprintf("alias %s \"echo No hay otra salida en esta ronda\"", StartAlias))
	}
	add(fmt.Sprintf("alias %s %s", RestartAlias, prepareOne))

	add(
		"bind "+PrepareKey+" "+PrepareAlias,
		"bind "+RestartKey+" "+RestartAlias,
		"bind "+NextKey+" "+NextAlias,
		"bind "+StartKey+" "+StartAlias,
		"bind "+BaseKey+" "+BaseAlias,
	)
	for i, l := range plan.Lineups {
		add("bind " + l.Key + " " + LineupAlias(i+1))
	}
	return strings.Join(lines, "\n") + "\n"
}

// baseLines writes every bot position once per base, and the alias that
// points the moments at the next base.
func baseLines(plan Plan) []string {
	var out []string
	for b := 1; b <= MaxBase; b++ {
		for k, m := range plan.Moments {
			for i := range plan.Bots {
				out = append(out, fmt.Sprintf("alias %s \"setpos_player %d %s\"", basePosAlias(b, k+1, i+1), b+i, coords(m.Bots[i].Pos)))
			}
		}
	}
	for b := 1; b <= MaxBase; b++ {
		cmds := []string{}
		for k := range plan.Moments {
			for i := range plan.Bots {
				cmds = append(cmds, fmt.Sprintf("alias %s %s", botAlias(k+1, i+1), basePosAlias(b, k+1, i+1)))
			}
		}
		cmds = append(cmds, fmt.Sprintf("alias %s %s", BaseAlias, baseAlias(b%MaxBase+1)),
			fmt.Sprintf("echo ClipHub: primer bot con el numero %d. Pulsa %s para colocarlos", b, RestartKey))
		out = append(out, aliasLines(baseAlias(b), cmds)...)
	}
	return out
}

func momentLines(plan Plan) []string {
	var out []string
	count := len(plan.Moments)
	for k, m := range plan.Moments {
		n := k + 1
		for p, st := range plan.Starts {
			if pose := m.Users[p]; pose != nil {
				out = append(out, fmt.Sprintf("alias %s \"setpos %s; setang %s 0\"", startMomentAlias(p+1, n), coords(pose.Pos), angles(*pose)))
			} else {
				out = append(out, fmt.Sprintf("alias %s \"echo %s ya ha muerto en este momento\"", startMomentAlias(p+1, n), st.Name))
			}
		}
		info := []string{fmt.Sprintf("echo ClipHub %d/%d: %s a %s s", n, count, m.Label, secondsText(m.Seconds))}
		for _, note := range m.Notes {
			info = append(info, "echo - "+note)
		}
		out = append(out, aliasLines(infoAlias(n), info)...)
		calls := []string{}
		for i := range plan.Bots {
			calls = append(calls, botAlias(n, i+1))
		}
		if len(plan.Starts) > 0 {
			calls = append(calls, userAlias(n))
		}
		calls = append(calls, infoAlias(n), fmt.Sprintf("alias %s %s", NextAlias, momentAlias(n%count+1)))
		out = append(out, aliasLines(momentAlias(n), calls)...)
	}
	return out
}

func startLines(plan Plan) []string {
	var out []string
	for p, st := range plan.Starts {
		n := p + 1
		cmds := make([]string, 0, len(plan.Moments)+2)
		for k := range plan.Moments {
			cmds = append(cmds, fmt.Sprintf("alias %s %s", userAlias(k+1), startMomentAlias(n, k+1)))
		}
		cmds = append(cmds, "echo Sales como "+st.Name, placeAlias)
		out = append(out, aliasLines(startAlias(n), cmds)...)
	}
	for p := range plan.Starts {
		n := p + 1
		out = append(out, fmt.Sprintf("alias %s \"%s; alias %s %s\"", cycleAlias(n), startAlias(n), StartAlias, cycleAlias(n%len(plan.Starts)+1)))
	}
	return out
}

func helpLines(plan Plan) []string {
	names := make([]string, len(plan.Bots))
	for i, bot := range plan.Bots {
		names[i] = bot.Name
	}
	out := []string{
		fmt.Sprintf("echo ClipHub: contra la ronda %d en %s", plan.Round, plan.Map),
		fmt.Sprintf("echo Bots %s, por orden: %s", plan.BotSide, strings.Join(names, ", ")),
	}
	if len(plan.Starts) > 0 {
		out = append(out, "echo Sales como "+plan.Starts[plan.Current].Name)
	}
	out = append(out,
		"echo "+RestartKey+": coloca a los bots en el momento 1",
		"echo "+NextKey+": siguiente momento de la ronda",
		"echo "+StartKey+": otra salida",
		"echo "+BaseKey+": cambia el numero del primer bot",
		"echo "+PrepareKey+": prepara el servidor otra vez",
	)
	for _, l := range plan.Lineups {
		out = append(out, fmt.Sprintf("echo %s: %s de %s", l.Key, grenadeLabel(l.Kind), l.Player))
	}
	return append(out,
		"echo Si los bots no se colocan, pulsa "+BaseKey+" y vuelve a pulsar "+RestartKey,
		"echo Los bots estan quietos y no disparan ni lanzan granadas",
	)
}

func coords(p [3]float64) string {
	return num(p[0]) + " " + num(p[1]) + " " + num(p[2])
}

func angles(p Pose) string { return num(p.Pitch) + " " + num(p.Yaw) }

func num(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

func secondsText(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }
