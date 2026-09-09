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
	s.scoreboard(state)
	ct, tr = ct[:min(5, len(ct))], tr[:min(5, len(tr))]
	for side, players := range [][]Player{ct, tr} {
		accent := t.CT
		if side == 1 {
			accent = t.T
		}
		for i, p := range players {
			x := 452 + i*68
			if side == 1 {
				x = 1132 + i*68
			}
			s.player(p, x, 28, accent, target)
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

func (s *scene) scoreboard(state Snapshot) {
	t := s.r.Theme
	const y, center = 28, Width / 2
	s.panel("score", center-80, y, 160, 64, t.Muted)
	for _, side := range []struct {
		id, label, color string
		x, points        int
	}{{"ct", "CT", t.CT, center - 160, state.CTScore}, {"t", "T", t.T, center + 86, state.TScore}} {
		s.path("score/"+side.id+"-plate", shapePath(t.Shape, side.x, y, 74, 64), side.color, 2)
		s.typeText("score/"+side.id+"-points", strconv.Itoa(side.points), side.x+37, y+25, 43, 62, 700, t.Background, "center")
		s.typeText("score/"+side.id+"-label", side.label, side.x+37, y+51, 12, 62, 600, t.Background, "center")
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
	s.typeText("score/time", timeText, center, y+25, 36, 132, 700, color, "center")
	s.typeText("score/round", phase, center, y+51, 12, 138, 600, color, "center")
	if state.Phase == "planted" || state.Phase == "defusing" {
		// This attaches to the clock and never occupies the reticle area.
		s.rect("score/bomb-phase", center-66, y+60, 132, 3, color, 4)
		s.icon("score/bomb", "status/icon_bomb_default", center-65, y+17, 13, 16, color, false)
	}
}

func (s *scene) player(p Player, x, y int, accent, target string) {
	const w, h = 64, 64
	id, t := "player/"+p.SteamID, s.r.Theme
	dead := p.Known && !p.Alive
	nameColor, detailColor := t.Text, t.Text
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
		s.rect(id+"/observed", x+8, y-4, w-16, 2, accent, 4)
		nameColor = accent
	}
	if dead {
		s.icon(id+"/skull", "status/icon_skull_default", x+21, y+11, 22, 24, detailColor, false)
	} else if !p.Known || !s.icon(id+"/weapon-icon", weaponIcon(p.Weapon), x+8, y+12, w-16, 24, detailColor, false) {
		s.typeText(id+"/weapon", knownWeapon(p), x+w/2, y+24, 13, w-12, 500, detailColor, "center")
	}
	s.text(id+"/name", p.Name, x+w/2, y+47, 14, w-10, nameColor, "center")
	s.rect(id+"/track", x+6, y+h-6, w-12, 3, t.Surface, 4)
	if p.Known && p.Alive && p.Health > 0 {
		s.rect(id+"/bar", x+6, y+h-6, (w-12)*min(100, p.Health)/100, 3, accent, 5)
	} else {
		// Unknown is distinct from an eliminated slot: no invented health bar.
		if !p.Known {
			s.typeText(id+"/unknown", "?", x+w-8, y+10, 12, 10, 500, t.Muted, "center")
		}
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
	x, y, w, h := t.FocusX, t.FocusY, t.FocusWidth, 102
	accent := t.CT
	if p.Side == "T" {
		accent = t.T
	}
	if p.Known && !p.Alive {
		accent = t.Muted
	}
	s.panel("focus", x, y, w, h, accent)
	s.path("focus/header", shapePath(t.Shape, x, y, w, 38), accent, 2)
	s.text("focus/name", p.Name, x+w/2, y+19, 27, w-30, t.Background, "center")
	hp, armor := "—", "—"
	if p.Known {
		hp, armor = strconv.Itoa(p.Health), strconv.Itoa(p.Armor)
	}
	healthIcon := "status/icon_health_default"
	if p.Known && !p.Alive {
		healthIcon = "status/icon_skull_default"
	}
	s.icon("focus/health-icon", healthIcon, x+16, y+55, 23, 26, accent, false)
	s.typeText("focus/health", hp, x+50, y+68, 45, 82, 700, t.Text, "left")
	s.icon("focus/armor-icon", "status/icon_armor_full_default", x+151, y+55, 23, 26, accent, false)
	s.typeText("focus/armor", armor, x+185, y+68, 45, w-201, 700, t.Text, "left")
	s.rect("focus/track", x+16, y+h-7, w-32, 3, t.Surface, 4)
	if p.Known && p.Alive && p.Health > 0 {
		s.rect("focus/bar", x+16, y+h-7, (w-32)*min(100, p.Health)/100, 3, accent, 5)
	}
	s.loadout(p, accent)
}

func (s *scene) loadout(p Player, accent string) {
	t := s.r.Theme
	x, y, w := t.LoadoutX, t.LoadoutY, t.LoadoutWidth
	s.panel("loadout", x, y, w, 90, accent)
	s.path("loadout/header", shapePath(t.Shape, x+3, y+3, w-6, 28), t.Surface, 2)
	s.fade(.7)
	kda, ammo := "K/D/A  —/—/—", "—"
	if p.Known {
		kda = fmt.Sprintf("K/D/A  %d/%d/%d", p.Kills, p.Deaths, p.Assists)
		if p.Alive && p.Ammo >= 0 {
			reserve := "—"
			if p.Reserve >= 0 {
				reserve = strconv.Itoa(p.Reserve)
			}
			ammo = fmt.Sprintf("%d/%s", p.Ammo, reserve)
		}
	}
	s.typeText("focus/kd", kda, x+w-15, y+17, 14, 156, 500, t.Muted, "right")
	weapon := knownWeapon(p)
	if p.Known && !p.Alive {
		weapon = "ELIMINATED"
	}
	if p.Known && p.Alive {
		s.icon("focus/weapon-icon", weaponIcon(p.Weapon), x+18, y+42, 118, 33, t.Text, false)
	}
	s.typeText("focus/weapon", weapon, x+15, y+17, 14, w-180, 600, t.Muted, "left")
	s.typeText("focus/ammo", ammo, x+w-47, y+57, 44, w-196, 700, t.Text, "right")
	s.icon("loadout/bullets", "status/icon_bullets_default", x+w-34, y+44, 19, 30, t.Muted, false)
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
