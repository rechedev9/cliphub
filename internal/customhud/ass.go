package customhud

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
)

// Window is a slice of the source demo placed at time zero of one prepared
// video item. SourceOffsetFrames includes editorial trims and sponsor splits.
type Window struct {
	StartTick          int
	SourceOffsetFrames int64
	Frames             int64
}

func assTime(frame int64) string {
	cs := frame * 100 / 60
	return fmt.Sprintf("%d:%02d:%02d.%02d", cs/360000, cs/6000%60, cs/100%60, cs%100)
}

func assColor(hex string) string { return hex[4:6] + hex[2:4] + hex[0:2] }

func assText(value string) string {
	// ASS has no general escape mechanism for tag braces. Full-width literal
	// forms preserve user names without ever interpreting them as overrides.
	return strings.NewReplacer("\\", "＼", "{", "｛", "}", "｝", "\n", " ", "\r", " ").Replace(value)
}

func assNode(n Node) string {
	alpha := uint8(math.Round(255 * (1 - max(0, min(1, n.Opacity)))))
	if n.Path != "" {
		path := strings.NewReplacer("M", "m", "L", "l", "C", "b", "Z", "").Replace(n.Path)
		return fmt.Sprintf("{\\an7\\pos(0,0)\\bord0\\shad0\\1c&H%s&\\1a&H%02X&\\p1}%s{\\p0}", assColor(n.Color), alpha, path)
	}
	align := 4
	if n.Align == "center" {
		align = 5
	}
	if n.Align == "right" {
		align = 6
	}
	return fmt.Sprintf("{\\an%d\\pos(%d,%d)\\fn%s\\b%d\\fs%d\\bord0\\shad0\\1c&H%s&\\1a&H%02X&}%s", align, n.X, n.Y, n.Font, n.Weight, n.Size, assColor(n.Color), alpha, assText(n.Text))
}

// ASS emits each component only when its value changes. It preserves the
// input frame clock, including repeated freezes and split round segments.
func (r *Renderer) ASS(d Timeline, window Window) (string, error) {
	if err := d.Validate(); err != nil {
		return "", err
	}
	if window.Frames < 1 || window.Frames > 60*43200 || window.StartTick < 0 || window.SourceOffsetFrames < 0 {
		return "", fmt.Errorf("invalid HUD render window")
	}
	startTick := window.StartTick + int(window.SourceOffsetFrames*int64(d.TickRate)/60)
	endTick := window.StartTick + int((window.SourceOffsetFrames+window.Frames-1)*int64(d.TickRate)/60)
	initial, ok := d.At(startTick)
	if !ok || endTick > d.EndTick {
		return "", fmt.Errorf("HUD telemetry does not cover the render window")
	}
	if !hasTarget(initial, d.TargetSteamID) {
		return "", fmt.Errorf("HUD telemetry has no observed player at tick %d", startTick)
	}
	var b strings.Builder
	b.WriteString("[Script Info]\nScriptType: v4.00+\nPlayResX: 1920\nPlayResY: 1080\nScaledBorderAndShadow: yes\nWrapStyle: 2\n\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\nStyle: HUD,Montserrat ExtraBold,20,&H00FFFFFF,&H00FFFFFF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,0,0,7,0,0,0,1\n\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	type activeNode struct {
		node  Node
		start int64
	}
	active := map[string]activeNode{}
	emit := func(a activeNode, end int64) {
		if end <= a.start {
			return
		}
		fmt.Fprintf(&b, "Dialogue: %d,%s,%s,HUD,,0,0,0,,%s\n", a.node.Layer, assTime(a.start), assTime(end), assNode(a.node))
	}
	apply := func(state Snapshot, frame int64) {
		next := map[string]Node{}
		for _, node := range r.Scene(state, d.TargetSteamID) {
			next[node.ID] = node
		}
		keys := make([]string, 0, len(active))
		for key := range active {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			a := active[key]
			if n, ok := next[key]; !ok || n != a.node {
				emit(a, frame)
				delete(active, key)
			}
		}
		for key, node := range next {
			if _, ok := active[key]; !ok {
				active[key] = activeNode{node, frame}
			}
		}
	}
	apply(initial, 0)
	for _, state := range d.Snapshots {
		if state.Tick <= startTick {
			continue
		}
		if state.Tick > endTick {
			break
		}
		if !hasTarget(state, d.TargetSteamID) {
			return "", fmt.Errorf("HUD telemetry lost observed player at tick %d", state.Tick)
		}
		frame := sourceFrame(state.Tick, d.TickRate, window)
		if frame >= window.Frames {
			break
		}
		if frame < 0 {
			continue
		}
		apply(state, frame)
	}
	keys := make([]string, 0, len(active))
	for key := range active {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		emit(active[key], window.Frames)
	}
	r.writeDamageEvents(&b, d, window)
	return b.String(), nil
}

func hasTarget(s Snapshot, id string) bool {
	for _, p := range s.Players {
		if p.SteamID == id {
			return true
		}
	}
	return false
}

func ASSFilter(path, fontDir string, alpha bool) string {
	quote := func(value string) string {
		value = strings.ReplaceAll(filepath.ToSlash(value), ":", "\\:")
		return "'" + strings.ReplaceAll(value, "'", "'\\\\\\''") + "'"
	}
	filter := "ass=filename=" + quote(path) + ":fontsdir=" + quote(fontDir)
	if alpha {
		filter += ":alpha=1"
	}
	return filter
}
