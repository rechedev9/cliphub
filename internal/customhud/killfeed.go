package customhud

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"golang.org/x/image/font/sfnt"
)

// KillNotice is one death notice drawn in place of CS2's own, so its position
// and width never depend on the game's right-anchored layout.
type KillNotice struct {
	AtSeconds    float64
	Attacker     string
	AttackerSide string
	Victim       string
	VictimSide   string
	Weapon       string
	Headshot     bool
}

// KillfeedLayout is the output frame the stack is centered on. Top is the
// upper edge of the first row; DurationSeconds clips every event to the video.
type KillfeedLayout struct {
	Width, Height   int
	Top             int
	DurationSeconds float64
}

// Metrics at a 1080 px wide frame; other widths scale proportionally.
const (
	killfeedRowHeight  = 58
	killfeedRowGap     = 8
	killfeedPadX       = 18
	killfeedItemGap    = 12
	killfeedNameSize   = 36
	killfeedNameWidth  = 330
	killfeedLabelSize  = 26
	killfeedIconHeight = 30
	killfeedIconWidth  = 112
	killfeedHSSize     = 28
	killfeedRadius     = 6
	killfeedBorder     = 3
	killfeedLifetime   = 3.4
	killfeedFadeInMS   = 120
	killfeedFadeOutMS  = 300
	killfeedSlideMS    = 150
	killfeedMaxRows    = 5
)

const (
	killfeedCTColor      = "9CC8FF"
	killfeedTColor       = "F2C14E"
	killfeedNeutralColor = "F4F6F8"
	killfeedIconColor    = "FFFFFF"
	killfeedBackground   = "0B0D10"
	killfeedBackOpacity  = 0.64
	killfeedBorderColor  = "E0313B"
)

// killfeedSpan is a stretch of one notice's lifetime spent in a single row.
// shift is how many rows below that row it starts, so it can slide up.
type killfeedSpan struct {
	start, end  float64
	slot, shift int
}

// killfeedEvent is a span ready to write. Its offset and fade continue from the
// previous span, so a row never pops when the stack changes quickly.
type killfeedEvent struct {
	killfeedSpan
	offset int
	fade   [7]int
}

// killfeedLabel is a name or weapon label fitted to its width budget.
type killfeedLabel struct {
	text  string
	face  measuredFace
	width int
}

type killfeedFit struct {
	value                string
	size, budget, weight int
}

type killfeedPart struct {
	path   string
	text   string
	font   string
	weight int
	size   int
	x, y   int
	color  string
	alpha  uint8
}

type killfeedRowBox struct {
	centerX, top int
	scale        float64
}

type roundedRect struct {
	x, y, w, h, r int
}

// KillfeedASS renders the notices as a libass script. Rows stack downward with
// the newest at the bottom; when an older row expires the rest slide up.
func KillfeedASS(notices []KillNotice, layout KillfeedLayout) (string, error) {
	if layout.Width < 320 || layout.Height < 320 || layout.DurationSeconds <= 0 || layout.Top < 0 {
		return "", fmt.Errorf("invalid killfeed layout")
	}
	if err := loadAssets(); err != nil {
		return "", err
	}
	ordered := append([]KillNotice(nil), notices...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].AtSeconds < ordered[j].AtSeconds })
	scale := float64(layout.Width) / 1080
	step := int(math.Round(float64(killfeedRowHeight+killfeedRowGap) * scale))
	var b strings.Builder
	fmt.Fprintf(&b, "[Script Info]\nScriptType: v4.00+\nPlayResX: %d\nPlayResY: %d\nScaledBorderAndShadow: yes\nWrapStyle: 2\n\n", layout.Width, layout.Height)
	b.WriteString("[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	b.WriteString("Style: Killfeed,Barlow Semi Condensed Bold,32,&H00FFFFFF,&H00FFFFFF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,0,0,7,0,0,0,1\n\n")
	b.WriteString("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	stack := killfeedStack{notices: ordered, duration: layout.DurationSeconds}
	for i, notice := range ordered {
		for _, event := range stack.events(i, step) {
			box := killfeedRowBox{centerX: layout.Width / 2, top: layout.Top + event.slot*step, scale: scale}
			event.write(&b, killfeedRow(notice, box))
		}
	}
	return b.String(), nil
}

// killfeedStack holds the notices in time order, clipped to the video.
type killfeedStack struct {
	notices  []KillNotice
	duration float64
}

func (k killfeedStack) end(notice KillNotice) float64 {
	return min(k.duration, notice.AtSeconds+killfeedLifetime)
}

// spans splits one notice's lifetime wherever its row changes.
func (k killfeedStack) spans(index int) []killfeedSpan {
	start, end := max(0, k.notices[index].AtSeconds), k.end(k.notices[index])
	if end <= start {
		return nil
	}
	cuts := []float64{start, end}
	for j, other := range k.notices {
		for _, t := range []float64{other.AtSeconds, k.end(other)} {
			if j != index && t > start && t < end {
				cuts = append(cuts, t)
			}
		}
	}
	sort.Float64s(cuts)
	var spans []killfeedSpan
	for i := 0; i+1 < len(cuts); i++ {
		a, z := cuts[i], cuts[i+1]
		slot, ok := k.slot(index, a)
		if z-a < 0.005 || !ok {
			continue
		}
		if n := len(spans); n > 0 && spans[n-1].end == a {
			if spans[n-1].slot == slot {
				spans[n-1].end = z
				continue
			}
			spans = append(spans, killfeedSpan{start: a, end: z, slot: slot, shift: spans[n-1].slot - slot})
			continue
		}
		spans = append(spans, killfeedSpan{start: a, end: z, slot: slot})
	}
	return spans
}

// events turns the spans into rows step px apart. A slide still under way when
// the row changes again carries over, and the fade follows the notice, not the span.
func (k killfeedStack) events(index, step int) []killfeedEvent {
	notice := k.notices[index]
	born, dies := notice.AtSeconds, k.end(notice)
	var out []killfeedEvent
	carry := 0.0
	for i, span := range k.spans(index) {
		if i > 0 && out[i-1].end != span.start {
			carry = 0
		}
		offset := carry + float64(span.shift*step)
		out = append(out, killfeedEvent{killfeedSpan: span, offset: int(math.Round(offset)), fade: killfeedFade(born, dies, span)})
		carry = offset * max(0, 1-(span.end-span.start)*1000/killfeedSlideMS)
	}
	return out
}

// killfeedFade is the 7-argument fade tag for span: alphas (255 is transparent)
// and times relative to the span's start.
func killfeedFade(born, dies float64, span killfeedSpan) [7]int {
	alpha := func(t float64) int {
		opacity := min(1, (t-born)*1000/killfeedFadeInMS, (dies-t)*1000/killfeedFadeOutMS)
		return int(math.Round(255 * (1 - max(0, opacity))))
	}
	ms := func(t float64) int { return int(math.Round((t - span.start) * 1000)) }
	fadedIn, fadingOut := born+killfeedFadeInMS/1000.0, dies-killfeedFadeOutMS/1000.0
	peak := max(span.start, min(fadedIn, fadingOut))
	out := max(peak, fadingOut)
	return [7]int{alpha(span.start), alpha(peak), 255, 0, ms(peak), ms(out), ms(dies)}
}

// slot is the notice's row at t among the notices on screen, keeping only
// the newest rows so a long multi-kill never grows down the frame.
func (k killfeedStack) slot(index int, t float64) (int, bool) {
	var visible []int
	for j, n := range k.notices {
		if n.AtSeconds <= t && k.end(n) > t {
			visible = append(visible, j)
		}
	}
	if len(visible) > killfeedMaxRows {
		visible = visible[len(visible)-killfeedMaxRows:]
	}
	for slot, j := range visible {
		if j == index {
			return slot, true
		}
	}
	return 0, false
}

// killfeedRow lays out one notice centered on box.centerX in frame
// coordinates, which is what the drawings' \pos(0,0) expects.
func killfeedRow(notice KillNotice, box killfeedRowBox) []killfeedPart {
	px := func(v int) int { return int(math.Round(float64(v) * box.scale)) }
	height, pad, gap := px(killfeedRowHeight), px(killfeedPadX), px(killfeedItemGap)
	nameSize, nameWidth := px(killfeedNameSize), px(killfeedNameWidth)
	attacker := killfeedFit{value: notice.Attacker, size: nameSize, budget: nameWidth, weight: 700}.label()
	victim := killfeedFit{value: notice.Victim, size: nameSize, budget: nameWidth, weight: 700}.label()

	weapon := iconBox{key: weaponIcon(notice.Weapon), h: px(killfeedIconHeight), maxW: px(killfeedIconWidth)}
	weaponW := weapon.width()
	var label killfeedLabel
	if weaponW == 0 {
		// Unknown equipment keeps its source label instead of a lookalike icon.
		label = killfeedFit{value: notice.Weapon, size: px(killfeedLabelSize), budget: px(killfeedIconWidth), weight: 600}.label()
		weaponW = label.width
	}
	skull := iconBox{key: "status/icon_skull_default", h: px(killfeedHSSize), maxW: px(killfeedHSSize)}
	hsW := 0
	if notice.Headshot {
		hsW = gap + skull.width()
	}
	width := pad + attacker.width + gap + weaponW + hsW + gap + victim.width + pad
	left, mid := box.centerX-width/2, box.top+height/2
	border := px(killfeedBorder)
	outer := roundedRect{x: left, y: box.top, w: width, h: height, r: px(killfeedRadius)}
	inner := roundedRect{x: left + border, y: box.top + border, w: width - 2*border, h: height - 2*border, r: max(0, outer.r-border)}
	parts := []killfeedPart{
		{path: outer.path(false), color: killfeedBackground, alpha: uint8(math.Round(255 * (1 - killfeedBackOpacity)))},
		{path: outer.path(false) + " " + inner.path(true), color: killfeedBorderColor},
	}
	x := left + pad
	parts = append(parts, killfeedPart{text: attacker.text, font: attacker.face.Family, weight: attacker.face.Weight, size: nameSize, x: x, y: mid, color: sideColor(notice.AttackerSide)})
	x += attacker.width + gap
	if label.text == "" {
		weapon.x, weapon.y = x, mid-weapon.h/2
		parts = append(parts, killfeedPart{path: weapon.path(), color: killfeedIconColor})
	} else {
		parts = append(parts, killfeedPart{text: label.text, font: label.face.Family, weight: label.face.Weight, size: px(killfeedLabelSize), x: x, y: mid, color: killfeedIconColor})
	}
	x += weaponW
	if notice.Headshot {
		skull.x, skull.y = x+gap, mid-skull.h/2
		parts = append(parts, killfeedPart{path: skull.path(), color: killfeedIconColor})
		x += hsW
	}
	parts = append(parts, killfeedPart{text: victim.text, font: victim.face.Family, weight: victim.face.Weight, size: nameSize, x: x + gap, y: mid, color: sideColor(notice.VictimSide)})
	return parts
}

// iconBox fits an icon's silhouette, not its padded view box, into height h
// (or maxW for long rifles), so icon gaps match the text gaps.
type iconBox struct {
	key        string
	x, y, h    int
	maxW, w    int
	scale      float64
	minX, minY float64
	bodyH      float64
}

func (b *iconBox) measure() bool {
	icon, ok := icons[b.key]
	if !ok || b.scale > 0 {
		return ok
	}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	axis := 0
	for _, token := range icon.tokens {
		if token.command != "" {
			axis = 0
			continue
		}
		if axis%2 == 0 {
			minX, maxX = min(minX, token.value), max(maxX, token.value)
		} else {
			minY, maxY = min(minY, token.value), max(maxY, token.value)
		}
		axis++
	}
	if maxX <= minX || maxY <= minY {
		return false
	}
	b.scale = min(float64(b.h)/(maxY-minY), float64(b.maxW)/(maxX-minX))
	b.minX, b.minY, b.bodyH = minX, minY, (maxY-minY)*b.scale
	b.w = int(math.Round((maxX - minX) * b.scale))
	return true
}

func (b *iconBox) width() int {
	if !b.measure() {
		return 0
	}
	return b.w
}

func (b *iconBox) path() string {
	if !b.measure() {
		return ""
	}
	dy := float64(b.y) + (float64(b.h)-b.bodyH)/2
	var out strings.Builder
	axis := 0
	for _, token := range icons[b.key].tokens {
		if token.command != "" {
			out.WriteString(token.command + " ")
			axis = 0
			continue
		}
		v := (token.value-b.minY)*b.scale + dy
		if axis%2 == 0 {
			v = (token.value-b.minX)*b.scale + float64(b.x)
		}
		fmt.Fprintf(&out, "%.2f ", v)
		axis++
	}
	return out.String()
}

// label fits the escaped text, as drawn, to the budget at the size libass renders.
func (f killfeedFit) label() killfeedLabel {
	value := assText(cleanText(f.value, 100))
	face := textFace(value, f.weight)
	if width := f.width(value, face); width <= f.budget {
		return killfeedLabel{text: value, face: face, width: width}
	}
	runes := []rune(value)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		candidate := string(runes) + "…"
		if width := f.width(candidate, face); width <= f.budget {
			return killfeedLabel{text: candidate, face: face, width: width}
		}
	}
	return killfeedLabel{face: face}
}

// width counts a full font size for each character the HUD fonts lack, since
// libass draws those with a system fallback font the embedded faces cannot measure.
func (f killfeedFit) width(text string, face measuredFace) int {
	var buf sfnt.Buffer
	var covered []rune
	missing, total := 0, 0.0
	flush := func() {
		total += float64(measureText(string(covered), f.size, face)) * face.assScale
		covered = covered[:0]
	}
	for _, c := range text {
		if glyph, err := face.font.GlyphIndex(&buf, c); err != nil || glyph == 0 {
			flush()
			missing++
			continue
		}
		covered = append(covered, c)
	}
	flush()
	return int(math.Ceil(total)) + missing*f.size
}

func sideColor(side string) string {
	switch strings.ToUpper(strings.TrimSpace(side)) {
	case "CT":
		return killfeedCTColor
	case "T":
		return killfeedTColor
	}
	return killfeedNeutralColor
}

// path uses the icon grammar (M/L/C/Z). A reversed contour punches a hole
// under libass's nonzero fill, which is how the border ring is drawn.
func (r roundedRect) path(reverse bool) string {
	rad := float64(min(r.r, r.w/2, r.h/2))
	k := rad * 0.5523
	x0, y0, x1, y1 := float64(r.x), float64(r.y), float64(r.x+r.w), float64(r.y+r.h)
	type pt struct{ x, y float64 }
	// Clockwise: each corner is its straight-edge start then a quarter curve.
	corners := [][4]pt{
		{{x1 - rad, y0}, {x1 - rad + k, y0}, {x1, y0 + rad - k}, {x1, y0 + rad}},
		{{x1, y1 - rad}, {x1, y1 - rad + k}, {x1 - rad + k, y1}, {x1 - rad, y1}},
		{{x0 + rad, y1}, {x0 + rad - k, y1}, {x0, y1 - rad + k}, {x0, y1 - rad}},
		{{x0, y0 + rad}, {x0, y0 + rad - k}, {x0 + rad - k, y0}, {x0 + rad, y0}},
	}
	if reverse {
		reversed := make([][4]pt, len(corners))
		for i, c := range corners {
			reversed[len(corners)-1-i] = [4]pt{c[3], c[2], c[1], c[0]}
		}
		corners = reversed
	}
	var out strings.Builder
	last := corners[len(corners)-1][3]
	fmt.Fprintf(&out, "M %.2f %.2f", last.x, last.y)
	for _, c := range corners {
		fmt.Fprintf(&out, " L %.2f %.2f C %.2f %.2f %.2f %.2f %.2f %.2f", c[0].x, c[0].y, c[1].x, c[1].y, c[2].x, c[2].y, c[3].x, c[3].y)
	}
	return out.String() + " Z"
}

func (e killfeedEvent) write(b *strings.Builder, parts []killfeedPart) {
	start, end := killfeedTime(e.start), killfeedTime(e.end)
	f := e.fade
	fade := fmt.Sprintf("\\fade(%d,%d,%d,%d,%d,%d,%d)", f[0], f[1], f[2], f[3], f[4], f[5], f[6])
	for layer, part := range parts {
		if part.path != "" {
			path := strings.NewReplacer("M", "m", "L", "l", "C", "b", "Z", "").Replace(part.path)
			fmt.Fprintf(b, "Dialogue: %d,%s,%s,Killfeed,,0,0,0,,{\\an7\\move(0,%d,0,0,0,%d)\\bord0\\shad0\\1c&H%s&\\1a&H%02X&%s\\p1}%s{\\p0}\n",
				20+layer, start, end, e.offset, killfeedSlideMS, assColor(part.color), part.alpha, fade, path)
			continue
		}
		fmt.Fprintf(b, "Dialogue: %d,%s,%s,Killfeed,,0,0,0,,{\\an4\\move(%d,%d,%d,%d,0,%d)\\fn%s\\b%d\\fs%d\\bord0\\shad0\\1c&H%s&%s}%s\n",
			20+layer, start, end, part.x, part.y+e.offset, part.x, part.y, killfeedSlideMS, part.font, part.weight, part.size, assColor(part.color), fade, assText(part.text))
	}
}

func killfeedTime(seconds float64) string {
	cs := int64(math.Round(seconds * 100))
	return fmt.Sprintf("%d:%02d:%02d.%02d", cs/360000, cs/6000%60, cs/100%60, cs%100)
}
