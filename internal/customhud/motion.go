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

const cardFadeFrames int64 = 12

// fadeIn eases a round card in from the frame it appears on the source clock.
func fadeIn(dialogue string) string {
	return strings.Replace(dialogue, "{", fmt.Sprintf(`{\fad(%d,0)`, cardFadeFrames*1000/60), 1)
}

// The defuse fill grows on the source clock between the defuse start and the
// bomb's recorded completion, so kit and no-kit defuses fill at their own pace.
func (r *Renderer) writeDefuseEvents(out *strings.Builder, d Timeline, window Window) {
	x, y, w, h := defuseTrack()
	fill := assNode(Node{Layer: 6, Path: polygon(x, y, x+w, y, x+w, y+h, x, y+h), Color: r.Theme.CT, Opacity: 1})
	for i := 0; i < len(d.Snapshots); {
		run := d.Snapshots[i].Defuse
		if run == nil {
			i++
			continue
		}
		first, end := i, 0
		for ; i < len(d.Snapshots) && d.Snapshots[i].Defuse != nil && d.Snapshots[i].Defuse.StartTick == run.StartTick; i++ {
			end = max(end, d.Snapshots[i].Defuse.EndTick)
		}
		stop := d.EndTick
		if i < len(d.Snapshots) {
			stop = d.Snapshots[i].Tick
		}
		from, until := sourceFrame(run.StartTick, d.TickRate, window), sourceFrame(end, d.TickRate, window)
		start, finish := max(0, sourceFrame(d.Snapshots[first].Tick, d.TickRate, window)), min(window.Frames, sourceFrame(stop, d.TickRate, window))
		if end == 0 || until <= from || finish <= start {
			continue
		}
		width := func(frame int64) int64 { return int64(w) * (min(until, max(from, frame)) - from) / (until - from) }
		grow := max(0, min(finish, until)-start)
		clip := fmt.Sprintf(`{\clip(%d,%d,%d,%d)\t(0,%d,\clip(%d,%d,%d,%d))`, x, y, int64(x)+width(start), y+h, grow*1000/60, x, y, int64(x)+width(start+grow), y+h)
		fmt.Fprintf(out, "Dialogue: 6,%s,%s,HUD,defuse,0,0,0,,%s\n", assTime(start), assTime(finish), strings.Replace(fill, "{", clip, 1))
	}
}
