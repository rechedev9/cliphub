package customhud

import (
	"encoding/base64"
	"fmt"
	"html"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/rechedev9/cliphub/internal/mediafont"
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
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
	Align      string
	Color      string
	Opacity    float64
}

type Renderer struct {
	Theme Theme
	font  *sfnt.Font
}

var fontOnce sync.Once
var hudFont *sfnt.Font
var hudFontData []byte
var hudFontError error

func NewRenderer(id string) (*Renderer, error) {
	theme, ok := Lookup(id)
	if !ok {
		return nil, fmt.Errorf("unknown custom HUD %q", id)
	}
	fontOnce.Do(func() {
		path, err := mediafont.Materialize()
		if err != nil {
			hudFontError = err
			return
		}
		hudFontData, hudFontError = os.ReadFile(path)
		if hudFontError == nil {
			hudFont, hudFontError = sfnt.Parse(hudFontData)
		}
	})
	if hudFontError != nil {
		return nil, hudFontError
	}
	return &Renderer{Theme: theme, font: hudFont}, nil
}

func (r *Renderer) fit(text string, size, width int) string {
	text = cleanText(text, 100)
	measure := func(value string) int {
		var buf sfnt.Buffer
		var advance fixed.Int26_6
		var previous sfnt.GlyphIndex
		for i, c := range value {
			glyph, err := r.font.GlyphIndex(&buf, c)
			if err != nil {
				return width + 1
			}
			if i > 0 {
				kern, _ := r.font.Kern(&buf, previous, glyph, fixed.I(size), font.HintingNone)
				advance += kern
			}
			step, err := r.font.GlyphAdvance(&buf, glyph, fixed.I(size), font.HintingNone)
			if err != nil {
				return width + 1
			}
			advance += step
			previous = glyph
		}
		return advance.Ceil()
	}
	if measure(text) <= width {
		return text
	}
	runes := []rune(text)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		candidate := string(runes) + "…"
		if measure(candidate) <= width {
			return candidate
		}
	}
	return ""
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
}
func (s *scene) text(id, value string, x, y, size, width int, color, align string) {
	s.nodes = append(s.nodes, Node{ID: id, Text: s.r.fit(value, size, width), X: x, Y: y, Size: size, Align: align, Color: color, Opacity: 1, Layer: 10})
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
	b := min(14, h/3)
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
		r := min(20, h/2)
		k := float64(r) * .55228475
		return fmt.Sprintf("M %d %d L %d %d C %.2f %d %d %.2f %d %d L %d %d C %d %.2f %.2f %d %d %d L %d %d C %.2f %d %d %.2f %d %d L %d %d C %d %.2f %.2f %d %d %d Z",
			x+r, y, x+w-r, y, float64(x+w-r)+k, y, x+w, float64(y+r)-k, x+w, y+r, x+w, y+h-r, x+w, float64(y+h-r)+k, float64(x+w-r)+k, y+h, x+w-r, y+h, x+r, y+h, float64(x+r)-k, y+h, x, float64(y+h-r)+k, x, y+h-r, x, y+r, x, float64(y+r)-k, float64(x+r)-k, y, x+r, y)
	default:
		return polygon(x, y, x+w, y, x+w, y+h, x, y+h)
	}
}

func (s *scene) panel(id string, x, y, w, h int, accent string) {
	t := s.r.Theme
	if t.Accent == "outline" {
		s.path(id+"/edge", shapePath(t.Shape, x, y, w, h), accent, 0)
		s.path(id+"/plate", shapePath(t.Shape, x+2, y+2, w-4, h-4), t.Background, 1)
	} else {
		s.path(id+"/plate", shapePath(t.Shape, x, y, w, h), t.Background, 0)
		if t.Accent == "stripe" {
			s.path(id+"/accent", polygon(x+14, y, x+24, y, x+14, y+h, x+4, y+h), accent, 2)
		} else if t.Accent == "edge" {
			s.rect(id+"/accent", x+14, y+h-4, w-28, 4, accent, 2)
		} else {
			s.rect(id+"/accent", x, y, w, 2, accent, 2)
		}
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
	sort.SliceStable(ct, func(i, j int) bool { return ct[i].SteamID < ct[j].SteamID })
	sort.SliceStable(tr, func(i, j int) bool { return tr[i].SteamID < tr[j].SteamID })
	s.scoreboard(state, ct, tr)
	ct = ct[:min(5, len(ct))]
	tr = tr[:min(5, len(tr))]
	for side, players := range [][]Player{ct, tr} {
		accent := t.CT
		if side == 1 {
			accent = t.T
		}
		for i, p := range players {
			switch t.Layout {
			case "sides":
				x := 24
				if side == 1 {
					x = 1604
				}
				s.player(p, x, 464+i*78, 292, 70, accent, target, false)
			case "split":
				x := 366 + i*113
				if side == 1 {
					x = 1012 + i*113
				}
				s.player(p, x, 140, 105, 96, accent, target, true)
			case "ribbon":
				x := 394 + (side*5+i)*113
				s.player(p, x, 118, 105, 72, accent, target, true)
			case "dock":
				x := 26 + (side*5+i)*188
				s.player(p, x, 986, 178, 70, accent, target, false)
			default:
				x := 398 + (side*5+i)*113
				s.player(p, x, 134, 105, 100, accent, target, true)
			}
		}
	}
	for _, p := range state.Players {
		if p.SteamID == target {
			s.focus(p)
			break
		}
	}
	// Tiny team labels keep even monochrome themes unambiguous. Native radar
	// and killfeed deliberately receive no mask or replacement artwork.
	sort.SliceStable(s.nodes, func(i, j int) bool { return s.nodes[i].Layer < s.nodes[j].Layer })
	return s.nodes
}

func (s *scene) scoreboard(state Snapshot, ct, tr []Player) {
	t := s.r.Theme
	w := t.ScoreWidth
	x := (Width - w) / 2
	y := 26
	s.panel("score", x, y, w, 80, t.CT)
	center := Width / 2
	teamWidth := (w - 160) / 2
	s.path("score/ct", shapePath(t.Shape, x+4, y+4, teamWidth-3, 72), t.Surface, 2)
	s.path("score/t", shapePath(t.Shape, center+80, y+4, teamWidth-4, 72), t.Surface, 2)
	s.text("score/ct-label", "CT", x+28, y+21, 15, 40, t.CT, "left")
	s.text("score/t-label", "T", x+w-28, y+21, 15, 40, t.T, "right")
	ctName, trName := state.CTName, state.TName
	if ctName == "" {
		ctName = "COUNTER-TERRORISTS"
	}
	if trName == "" {
		trName = "TERRORISTS"
	}
	s.text("score/ct-team", ctName, x+28, y+48, 21, teamWidth-101, t.Text, "left")
	s.text("score/t-team", trName, x+w-28, y+48, 21, teamWidth-101, t.Text, "right")
	s.text("score/ct-points", strconv.Itoa(state.CTScore), center-112, y+42, 45, 68, t.CT, "center")
	s.text("score/t-points", strconv.Itoa(state.TScore), center+112, y+42, 45, 68, t.T, "center")
	timeText := "--:--"
	if state.TimeRemaining >= 0 {
		timeText = fmt.Sprintf("%d:%02d", state.TimeRemaining/60, state.TimeRemaining%60)
	}
	phase := fmt.Sprintf("ROUND %02d", state.Round)
	color := t.Text
	switch state.Phase {
	case "freeze":
		phase = "FREEZE"
	case "planted":
		phase = "BOMB PLANTED"
		color = t.T
	case "defusing":
		phase = "DEFUSING"
		color = t.CT
	case "ended":
		phase = "ROUND END"
		timeText = "—"
	}
	s.text("score/time", timeText, center, y+31, 38, 144, color, "center")
	s.text("score/round", phase, center, y+62, 14, 144, t.Muted, "center")
	alive := func(players []Player) string {
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
	s.panel("alive", center-72, 254, 144, 30, t.T)
	s.text("alive/ct", alive(ct), center-44, 269, 21, 24, t.CT, "center")
	s.text("alive/copy", "ALIVE", center, 269, 11, 45, t.Muted, "center")
	s.text("alive/t", alive(tr), center+44, 269, 21, 24, t.T, "center")
}

func (s *scene) player(p Player, x, y, w, h int, accent, target string, compact bool) {
	id := "player/" + p.SteamID
	t := s.r.Theme
	if !p.Alive {
		accent = t.Muted
	}
	s.panel(id, x, y, w, h, accent)
	if p.SteamID == target {
		s.rect(id+"/observed", x+12, y+6, w-24, 3, accent, 3)
	}
	nameColor := t.Text
	if !p.Alive {
		nameColor = t.Muted
	}
	if compact {
		s.text(id+"/name", p.Name, x+w/2, y+22, 17, w-18, nameColor, "center")
		hp := "—"
		if p.Known {
			hp = strconv.Itoa(p.Health)
		}
		if !p.Alive && p.Known {
			hp = "OUT"
		}
		s.text(id+"/health", hp, x+w/2, y+46, 24, w-20, accent, "center")
		if h > 85 {
			s.text(id+"/weapon", p.Weapon, x+w/2, y+70, 12, w-20, t.Muted, "center")
		}
	} else {
		nameWidth := w - 86
		size := 22
		if w < 200 {
			size = 17
			nameWidth = w - 59
		}
		s.text(id+"/name", p.Name, x+22, y+23, size, nameWidth, nameColor, "left")
		hp := "—"
		if p.Known {
			hp = strconv.Itoa(p.Health)
		}
		if !p.Alive && p.Known {
			hp = "OUT"
		}
		s.text(id+"/health", hp, x+w-17, y+23, size, w/4, accent, "right")
		weapon := p.Weapon
		if weapon == "" {
			weapon = "—"
		}
		s.text(id+"/weapon", weapon, x+22, y+47, 14, w/2-20, t.Muted, "left")
		money := "—"
		if p.Known {
			money = fmt.Sprintf("$%d", p.Money)
		}
		s.text(id+"/money", money, x+w-17, y+47, 14, w/2-20, t.Muted, "right")
	}
	s.rect(id+"/track", x+14, y+h-10, w-28, 3, t.Surface, 3)
	if p.Known && p.Health > 0 {
		s.rect(id+"/bar", x+14, y+h-10, (w-28)*min(100, p.Health)/100, 3, accent, 4)
	}
}

func (s *scene) focus(p Player) {
	t := s.r.Theme
	x, y, w, h := t.FocusX, t.FocusY, t.FocusWidth, 144
	accent := t.CT
	if p.Side == "T" {
		accent = t.T
	}
	s.panel("focus", x, y, w, h, accent)
	s.text("focus/name", p.Name, x+26, y+28, 34, w-176, t.Text, "left")
	kda := "— / — / —"
	if p.Known {
		kda = fmt.Sprintf("%d / %d / %d", p.Kills, p.Deaths, p.Assists)
	}
	s.text("focus/kd", kda, x+w-24, y+28, 17, 126, t.Muted, "right")
	s.rect("focus/rule", x+24, y+50, w-48, 1, t.Surface, 2)
	hp, armor, ammo := "—", "—", "—"
	if p.Known {
		hp = strconv.Itoa(p.Health)
		armor = strconv.Itoa(p.Armor)
		if p.Ammo >= 0 {
			ammo = fmt.Sprintf("%d / %d", p.Ammo, p.Reserve)
		}
	}
	s.text("focus/health", hp, x+26, y+85, 48, 109, accent, "left")
	s.text("focus/armor", armor, x+164, y+85, 34, 89, t.Text, "left")
	s.text("focus/ammo", ammo, x+w-24, y+85, 38, w-278, t.Text, "right")
	s.text("focus/hp-label", "HEALTH", x+28, y+117, 12, 96, t.Muted, "left")
	s.text("focus/armor-label", "ARMOR", x+166, y+117, 12, 82, t.Muted, "left")
	weapon := p.Weapon
	if weapon == "" {
		weapon = "NO WEAPON"
	}
	if p.Known && !p.Alive {
		weapon = "ELIMINATED"
	}
	s.text("focus/weapon", weapon, x+w-24, y+117, 14, w-276, t.Muted, "right")
	s.rect("focus/track", x+24, y+h-9, w-48, 4, t.Surface, 3)
	if p.Known && p.Health > 0 {
		s.rect("focus/bar", x+24, y+h-9, (w-48)*min(100, p.Health)/100, 4, accent, 4)
	}
}

// SVG embeds the already bundled font so a thumbnail and the export do not
// silently depend on fonts installed on the viewer's computer.
func (r *Renderer) SVG(state Snapshot, target string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="1920" height="1080" viewBox="0 0 1920 1080" role="img" aria-label="%s HUD"><defs><style>@font-face{font-family:HUD;src:url(data:font/ttf;base64,%s)}text{font-family:HUD,sans-serif}</style></defs>`, html.EscapeString(r.Theme.Name), base64.StdEncoding.EncodeToString(hudFontData))
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
		fmt.Fprintf(&b, `<text x="%d" y="%d" dominant-baseline="central" text-anchor="%s" font-size="%d" fill="#%s">%s</text>`, n.X, n.Y, anchor, n.Size, n.Color, html.EscapeString(n.Text))
	}
	b.WriteString("</svg>")
	return b.String()
}
