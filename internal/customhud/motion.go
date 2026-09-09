package customhud

import (
	"fmt"
	"math"
	"strings"
)

const damageFrames int64 = 10

// Damage briefly highlights only the health that was lost. Numbers and the
// live bar update on the source frame, with no easing or delayed death state.
// The fade is clipped on the source clock, so trims and sponsor splits retain
// the same portion of the effect as a continuous export.
func (r *Renderer) writeDamageEvents(out *strings.Builder, d Timeline, window Window) {
	for i := 1; i < len(d.Snapshots); i++ {
		before, after := d.Snapshots[i-1], d.Snapshots[i]
		frame := sourceFrame(after.Tick, d.TickRate, window)
		if frame+damageFrames <= 0 {
			continue
		}
		if frame >= window.Frames {
			break
		}
		if before.Round != after.Round {
			continue
		}
		players := make(map[string]Player, len(after.Players))
		for _, p := range after.Players {
			players[p.SteamID] = p
		}
		var ids []string
		for _, old := range before.Players {
			current, ok := players[old.SteamID]
			if !ok || !old.Known || !current.Known || !old.Alive || old.Inactive || current.Inactive || current.Health >= old.Health {
				continue
			}
			ids = append(ids, "player/"+old.SteamID+"/bar")
			if old.SteamID == d.TargetSteamID {
				ids = append(ids, "focus/bar")
			}
		}
		if len(ids) == 0 {
			continue
		}
		oldNodes, newNodes := map[string]Node{}, map[string]Node{}
		for _, n := range r.Scene(before, d.TargetSteamID) {
			oldNodes[n.ID] = n
		}
		for _, n := range r.Scene(after, d.TargetSteamID) {
			newNodes[n.ID] = n
		}
		start, end := max(int64(0), frame), min(window.Frames, frame+damageFrames)
		for _, id := range ids {
			old, ok := oldNodes[id]
			remaining := newNodes[id].W
			if !ok || old.W <= remaining {
				continue
			}
			lost := Node{Layer: 6, Path: polygon(old.X+remaining, old.Y, old.X+old.W, old.Y, old.X+old.W, old.Y+old.H, old.X+remaining, old.Y+old.H), Color: r.Theme.Text,
				Opacity: .8 * (1 - float64(start-frame)/float64(damageFrames))}
			finalAlpha := uint8(math.Round(255 * (1 - .8*(1-float64(end-frame)/float64(damageFrames)))))
			text := strings.Replace(assNode(lost), "}", fmt.Sprintf("\\t(0,%d,\\1a&H%02X&)}", (end-start)*1000/60, finalAlpha), 1)
			fmt.Fprintf(out, "Dialogue: 6,%s,%s,HUD,damage,0,0,0,,%s\n", assTime(start), assTime(end), text)
		}
	}
}

func sourceFrame(tick, rate int, window Window) int64 {
	delta := int64(tick-window.StartTick) * 60
	if delta < 0 {
		return -((-delta + int64(rate)/2) / int64(rate)) - window.SourceOffsetFrames
	}
	return (delta+int64(rate)/2)/int64(rate) - window.SourceOffsetFrames
}
