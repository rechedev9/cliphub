package customhud

import (
	"encoding/base64"
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"
)

const Width, Height = 1920, 1080

// Node is the common display list for SVG previews and ASS exports. Geometry
// and text fitting are performed once here, rather than reimplemented in CSS.
type Node struct {
	ID         string
	Layer      int
	Path       string
	Text       string
	X, Y, Size int
	W, H       int
	Align      string
	Color      string
	Opacity    float64
	Font       string
	Weight     int
}

type Renderer struct {
	Theme Theme
}

type scene struct {
	r     *Renderer
	nodes []Node
}

func (s *scene) path(id, path, color string, layer int) {
	s.nodes = append(s.nodes, Node{ID: id, Path: path, Color: color, Opacity: 1, Layer: layer})
}
func (s *scene) rect(id string, x, y, w, h int, color string, layer int) {
	s.path(id, polygon(x, y, x+w, y, x+w, y+h, x, y+h), color, layer)
	n := &s.nodes[len(s.nodes)-1]
	n.X, n.Y, n.W, n.H = x, y, w, h
}
func (s *scene) text(id, value string, x, y, size, width int, color, align string) {
	s.typeText(id, value, x, y, size, width, 600, color, align)
}
func (s *scene) typeText(id, value string, x, y, size, width, weight int, color, align string) {
	fitted, face := fitText(value, size, width, weight)
	s.nodes = append(s.nodes, Node{ID: id, Text: fitted, X: x, Y: y, W: width, Size: size, Align: align, Color: color, Opacity: 1, Layer: 10, Font: face.Family, Weight: face.Weight})
}
func (s *scene) fade(opacity float64) {
	s.nodes[len(s.nodes)-1].Opacity = opacity
}

func polygon(xy ...int) string {
	var out strings.Builder
	for i := 0; i < len(xy); i += 2 {
		if i == 0 {
			out.WriteString("M ")
		} else {
			out.WriteString(" L ")
		}
		fmt.Fprintf(&out, "%d %d", xy[i], xy[i+1])
	}
	out.WriteString(" Z")
	return out.String()
}

func shapePath(kind string, x, y, w, h int) string {
	b := min(8, h/4)
	switch kind {
	case "slant":
		return polygon(x+b, y, x+w, y, x+w-b, y+h, x, y+h)
	case "bevel":
		return polygon(x+b, y, x+w-b, y, x+w, y+b, x+w, y+h-b, x+w-b, y+h, x+b, y+h, x, y+h-b, x, y+b)
	case "step":
		return polygon(x, y, x+w-18, y, x+w-18, y+10, x+w, y+10, x+w, y+h, x+18, y+h, x+18, y+h-10, x, y+h-10)
	case "notch":
		return polygon(x+12, y, x+w, y, x+w, y+h-12, x+w-12, y+h-12, x+w-12, y+h, x, y+h, x, y+12, x+12, y+12)
	case "round":
		r := min(8, h/2)
		k := float64(r) * .55228475
		return fmt.Sprintf("M %d %d L %d %d C %.2f %d %d %.2f %d %d L %d %d C %d %.2f %.2f %d %d %d L %d %d C %.2f %d %d %.2f %d %d L %d %d C %d %.2f %.2f %d %d %d Z",
			x+r, y, x+w-r, y, float64(x+w-r)+k, y, x+w, float64(y+r)-k, x+w, y+r, x+w, y+h-r, x+w, float64(y+h-r)+k, float64(x+w-r)+k, y+h, x+w-r, y+h, x+r, y+h, float64(x+r)-k, y+h, x, float64(y+h-r)+k, x, y+h-r, x, y+r, x, float64(y+r)-k, float64(x+r)-k, y, x+r, y)
	default:
		return polygon(x, y, x+w, y, x+w, y+h, x, y+h)
	}
}

func (s *scene) panel(id string, x, y, w, h int, accent string) {
	t := s.r.Theme
	s.path(id+"/shadow", shapePath(t.Shape, x, y+3, w, h), "000000", 0)
	s.fade(.18)
	s.path(id+"/plate", shapePath(t.Shape, x, y, w, h), t.Background, 1)
	s.fade(t.Opacity)
	if strings.HasPrefix(id, "score") {
		// Keep clock and score luminance stable through flashes and camera
		// transitions; the larger roster and focus plates remain translucent.
		s.fade(1)
	}
	// Small structural accents retain each design's character without filling
	// the screen with team colors. All data stays above the translucent plates.
	switch t.Accent {
	case "outline":
		s.rect(id+"/edge-top", x+12, y, w-24, 1, accent, 3)
		s.fade(.55)
		s.rect(id+"/edge-bottom", x+12, y+h-1, w-24, 1, accent, 3)
		s.fade(.25)
	case "stripe":
		s.path(id+"/accent", polygon(x+8, y+8, x+11, y+8, x+7, y+h-8, x+4, y+h-8), accent, 3)
	case "edge":
		s.rect(id+"/accent", x+12, y+h-2, w-24, 2, accent, 3)
	default:
		s.rect(id+"/accent", x+12, y, 28, 2, accent, 3)
	}
}

func (r *Renderer) Scene(state Snapshot, target string) []Node {
	s := &scene{r: r}
	t := r.Theme
	var ct, tr []Player
	for _, p := range state.Players {
		if p.Inactive {
			continue
		}
		if p.Side == "CT" {
			ct = append(ct, p)
		} else {
			tr = append(tr, p)
		}
	}
	// Deaths never reorder the roster. A spectator can keep following the
	// same slot while its health, equipment and elimination state change.
	sort.SliceStable(ct, func(i, j int) bool { return ct[i].SteamID < ct[j].SteamID })
	sort.SliceStable(tr, func(i, j int) bool { return tr[i].SteamID < tr[j].SteamID })
	s.scoreboard(state, ct, tr)
	ct, tr = ct[:min(5, len(ct))], tr[:min(5, len(tr))]
	for side, players := range [][]Player{ct, tr} {
		accent := t.CT
		if side == 1 {
			accent = t.T
		}
		for i, p := range players {
			switch t.Layout {
			case "sides":
				x := 32
				if side == 1 {
					x = Width - 280
				}
				s.player(p, x, 698+i*70, 248, 64, accent, target, false)
			case "split":
				x := 368 + i*112
				if side == 1 {
					x = 1000 + i*112
				}
				s.player(p, x, 124, 104, 82, accent, target, true)
			case "ribbon":
				x := 394 + (side*5+i)*114
				s.player(p, x, 124, 106, 72, accent, target, true)
			case "dock":
				x := 32 + i*128
				if side == 1 {
					x = 1252 + i*128
				}
				s.player(p, x, 962, 124, 82, accent, target, true)
			default:
				x := 390 + i*112
				if side == 1 {
					x = 978 + i*112
				}
				s.player(p, x, 122, 104, 84, accent, target, true)
			}
		}
	}
	for _, p := range state.Players {
		if p.SteamID == target {
			s.focus(p)
			break
		}
	}
	sort.SliceStable(s.nodes, func(i, j int) bool { return s.nodes[i].Layer < s.nodes[j].Layer })
	return s.nodes
}

func teamName(value, fallback string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(value), "team_") {
		value = value[5:]
	}
	if value == "" {
		return fallback
	}
	return value
}

func aliveCount(players []Player) string {
	n := 0
	for _, p := range players {
		if !p.Known {
			return "—"
		}
		if p.Alive {
			n++
		}
	}
	return strconv.Itoa(n)
}

func (s *scene) scoreboard(state Snapshot, ct, tr []Player) {
	t := s.r.Theme
	w, y, center := t.ScoreWidth, 32, Width/2
	x := (Width - w) / 2
	if t.Layout == "split" {
		s.panel("score/left", x, y, w/2-80, 78, t.CT)
		s.panel("score/right", center+80, y, w/2-80, 78, t.T)
		s.panel("score", center-72, y+4, 144, 74, t.Muted)
	} else {
		s.panel("score", x, y, w, 78, t.Muted)
		s.path("score/center", shapePath(t.Shape, center-74, y+4, 148, 70), t.Surface, 2)
		s.fade(.72)
	}
	nameWidth := w/2 - 174
	ctName, trName := teamName(state.CTName, "COUNTER-TERRORISTS"), teamName(state.TName, "TERRORISTS")
	s.typeText("score/ct-label", "CT", x+20, y+16, 11, 30, 700, t.CT, "left")
	s.typeText("score/t-label", "T", x+w-20, y+16, 11, 30, 700, t.T, "right")
	s.text("score/ct-team", ctName, x+20, y+39, 25, nameWidth, t.Text, "left")
	s.text("score/t-team", trName, x+w-20, y+39, 25, nameWidth, t.Text, "right")
	s.typeText("score/ct-points", strconv.Itoa(state.CTScore), center-112, y+38, 47, 62, 700, t.CT, "center")
	s.typeText("score/t-points", strconv.Itoa(state.TScore), center+112, y+38, 47, 62, 700, t.T, "center")
	s.text("alive/ct", aliveCount(ct), x+20, y+62, 14, 20, t.CT, "left")
	s.text("alive/t", aliveCount(tr), x+w-20, y+62, 14, 20, t.T, "right")
	s.typeText("alive/ct-label", "ALIVE", x+36, y+62, 11, 60, 500, t.Muted, "left")
	s.typeText("alive/t-label", "ALIVE", x+w-36, y+62, 11, 60, 500, t.Muted, "right")
	for side, players := range [][]Player{ct, tr} {
		for i := 0; i < min(5, len(players)); i++ {
			px, color := x+78+i*10, t.CT
			if side == 1 {
				px, color = x+w-84-i*10, t.T
			}
			s.rect(fmt.Sprintf("score/alive/%d/%d", side, i), px, y+60, 5, 5, color, 3)
			if !players[i].Known || !players[i].Alive {
				s.fade(.2)
			}
		}
	}
	timeText, color := "--:--", t.Text
	if state.TimeRemaining >= 0 {
		timeText = fmt.Sprintf("%d:%02d", state.TimeRemaining/60, state.TimeRemaining%60)
	}
	phase := fmt.Sprintf("ROUND %02d", state.Round)
	switch state.Phase {
	case "freeze":
		phase = "FREEZE TIME"
	case "planted":
		phase, color = "BOMB PLANTED", t.T
	case "defusing":
		phase, color = "DEFUSING", t.CT
	case "ended":
		phase, timeText = "ROUND END", "—"
	}
	s.typeText("score/time", timeText, center, y+31, 36, 128, 700, color, "center")
	s.typeText("score/round", phase, center, y+59, 12, 126, 600, color, "center")
	if state.Phase == "planted" || state.Phase == "defusing" {
		// This attaches to the clock and never occupies the reticle area.
		s.rect("score/bomb-phase", center-62, y+74, 124, 3, color, 4)
		s.icon("score/bomb", "status/icon_bomb_default", center-61, y+23, 13, 16, color, false)
	}
}

func (s *scene) player(p Player, x, y, w, h int, accent, target string, compact bool) {
	id, t := "player/"+p.SteamID, s.r.Theme
	dead := p.Known && !p.Alive
	nameColor, detailColor := t.Text, t.Muted
	if dead {
		accent, nameColor = t.Muted, t.Muted
	}
	start := len(s.nodes)
	s.panel(id, x, y, w, h, accent)
	if dead {
		for i := start; i < len(s.nodes); i++ {
			if s.nodes[i].ID == id+"/plate" {
				s.nodes[i].Opacity = max(.90, t.Opacity-.02)
			} else {
				s.nodes[i].Opacity *= .65
			}
		}
	}
	if p.SteamID == target {
		s.path(id+"/observed", polygon(x-7, y+h/2-5, x-2, y+h/2, x-7, y+h/2+5), accent, 4)
		nameColor = accent
	}
	hp, money, kd := "—", "—", "— / —"
	if p.Known {
		hp, money, kd = strconv.Itoa(p.Health), fmt.Sprintf("$%d", p.Money), fmt.Sprintf("%d / %d", p.Kills, p.Deaths)
	}
	if compact {
		s.text(id+"/name", p.Name, x+w/2, y+16, 18, w-18, nameColor, "center")
		if dead {
			s.icon(id+"/skull", "status/icon_skull_default", x+w/2-9, y+29, 18, 18, detailColor, false)
		} else if !p.Known || !s.icon(id+"/weapon-icon", weaponIcon(p.Weapon), x+20, y+29, w-40, 22, detailColor, false) {
			s.typeText(id+"/weapon", knownWeapon(p), x+w/2, y+39, 12, w-16, 500, detailColor, "center")
		}
		s.typeText(id+"/health", hp, x+12, y+h-18, 21, 35, 700, accent, "left")
		if dead {
			money = kd
		}
		s.typeText(id+"/money", money, x+w-12, y+h-18, 13, w-52, 500, detailColor, "right")
	} else {
		s.text(id+"/name", p.Name, x+17, y+17, 22, w-80, nameColor, "left")
		s.typeText(id+"/health", hp, x+w-16, y+17, 25, 55, 700, accent, "right")
		if dead {
			s.icon(id+"/skull", "status/icon_skull_default", x+17, y+33, 17, 18, detailColor, false)
			s.typeText(id+"/eliminated", "ELIMINATED", x+45, y+42, 12, 110, 500, detailColor, "left")
		} else {
			if !p.Known || !s.icon(id+"/weapon-icon", weaponIcon(p.Weapon), x+15, y+31, 65, 24, detailColor, p.Side == "T") {
				s.typeText(id+"/weapon", knownWeapon(p), x+17, y+42, 12, 66, 500, detailColor, "left")
			}
			s.typeText(id+"/money", money, x+96, y+42, 15, 70, 500, detailColor, "left")
		}
		s.typeText(id+"/kd", kd, x+w-16, y+42, 14, 55, 500, detailColor, "right")
	}
	s.rect(id+"/track", x+12, y+h-6, w-24, 2, t.Surface, 4)
	if p.Known && p.Alive && p.Health > 0 {
		s.rect(id+"/bar", x+12, y+h-6, (w-24)*min(100, p.Health)/100, 2, accent, 5)
	}
}

func knownWeapon(p Player) string {
	if !p.Known || p.Weapon == "" {
		return "—"
	}
	return p.Weapon
}

func (s *scene) focus(p Player) {
	t := s.r.Theme
	x, y, w, h := t.FocusX, t.FocusY, t.FocusWidth, 116
	accent := t.CT
	if p.Side == "T" {
		accent = t.T
	}
	if p.Known && !p.Alive {
		accent = t.Muted
	}
	s.panel("focus", x, y, w, h, accent)
	s.path("focus/header", shapePath(t.Shape, x+4, y+4, w-8, 36), t.Surface, 2)
	s.fade(.6)
	s.text("focus/name", p.Name, x+20, y+22, 30, w-162, t.Text, "left")
	kda := "— / — / —"
	if p.Known {
		kda = fmt.Sprintf("%d / %d / %d", p.Kills, p.Deaths, p.Assists)
	}
	s.typeText("focus/kd", kda, x+w-20, y+22, 16, 118, 500, t.Muted, "right")
	hp, armor, ammo := "—", "—", "—"
	if p.Known {
		hp, armor = strconv.Itoa(p.Health), strconv.Itoa(p.Armor)
		if p.Ammo >= 0 {
			ammo = fmt.Sprintf("%d / %d", p.Ammo, p.Reserve)
		}
	}
	healthIcon := "status/icon_health_default"
	if p.Known && !p.Alive {
		healthIcon = "status/icon_skull_default"
	}
	s.icon("focus/health-icon", healthIcon, x+19, y+59, 18, 21, accent, false)
	s.typeText("focus/health", hp, x+46, y+70, 44, 78, 700, accent, "left")
	s.icon("focus/armor-icon", "status/icon_armor_full_default", x+140, y+63, 16, 18, t.Muted, false)
	s.typeText("focus/armor", armor, x+165, y+72, 27, 56, 500, t.Text, "left")
	s.typeText("focus/ammo", ammo, x+w-20, y+70, 35, w-336, 600, t.Text, "right")
	weapon := knownWeapon(p)
	if p.Known && !p.Alive {
		weapon = "ELIMINATED"
	}
	if p.Known && p.Alive {
		s.icon("focus/weapon-icon", weaponIcon(p.Weapon), x+236, y+52, 82, 32, t.Text, false)
	}
	s.typeText("focus/weapon", weapon, x+277, y+99, 13, 103, 500, t.Muted, "center")
	s.typeText("focus/ammo-label", "AMMO", x+w-20, y+99, 11, 80, 500, t.Muted, "right")
	s.typeText("focus/health-label", "HP", x+20, y+99, 11, 30, 500, t.Muted, "left")
	s.rect("focus/track", x+45, y+98, 164, 3, t.Surface, 4)
	if p.Known && p.Alive && p.Health > 0 {
		s.rect("focus/bar", x+45, y+98, 164*min(100, p.Health)/100, 3, accent, 5)
	}
}

// SVG and ASS use the same face, weight, fitted text and display list.
func (r *Renderer) SVG(state Snapshot, target string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="1920" height="1080" viewBox="0 0 1920 1080" role="img" aria-label="%s HUD"><defs><style>`, html.EscapeString(r.Theme.Name))
	for _, face := range hudFaces {
		fmt.Fprintf(&b, `@font-face{font-family:'%s';font-weight:%d;src:url(data:font/ttf;base64,%s)}`, face.Family, face.Weight, base64.StdEncoding.EncodeToString(face.Data))
	}
	b.WriteString(`</style></defs>`)
	for _, n := range r.Scene(state, target) {
		if n.Path != "" {
			fmt.Fprintf(&b, `<path d="%s" fill="#%s" opacity="%g"/>`, n.Path, n.Color, n.Opacity)
			continue
		}
		anchor := "start"
		if n.Align == "center" {
			anchor = "middle"
		}
		if n.Align == "right" {
			anchor = "end"
		}
		fmt.Fprintf(&b, `<text x="%d" y="%d" dominant-baseline="central" text-anchor="%s" font-family="%s" font-weight="%d" font-size="%d" fill="#%s" opacity="%g">%s</text>`, n.X, n.Y, anchor, n.Font, n.Weight, n.Size, n.Color, n.Opacity, html.EscapeString(n.Text))
	}
	b.WriteString("</svg>")
	return b.String()
}
