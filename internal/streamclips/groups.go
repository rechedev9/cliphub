package streamclips

import (
	"fmt"
	"strings"
)

// ClipGroup joins two or more moments into one rendered Short. ClipIDs is the
// order the moments play in; Title empty means the member titles joined.
type ClipGroup struct {
	ID      string   `json:"id"`
	ClipIDs []string `json:"clip_ids"`
	Title   string   `json:"title,omitempty"`
}

// ShortMaxSeconds is the YouTube Shorts length limit a joined Short is warned against.
const ShortMaxSeconds = 60

// PlanOutput is one video a render produces: a lone moment or a group.
type PlanOutput struct {
	ID    string
	Title string
	Clips []ClipRange
}

// DurationSeconds is the output length: the members play back to back.
func (o PlanOutput) DurationSeconds() float64 {
	total := 0.0
	for _, clip := range o.Clips {
		total += clip.OutputDurationSeconds()
	}
	return total
}

// Outputs lists the videos the plan renders. A group takes the place of its
// earliest member in plan order, so outputs follow the VOD like the moments.
func (p EditPlan) Outputs() []PlanOutput {
	byID := make(map[string]ClipRange, len(p.Clips))
	for _, clip := range p.Clips {
		byID[clip.ID] = clip
	}
	groupOf := map[string]int{}
	for i, group := range p.Groups {
		for _, id := range group.ClipIDs {
			groupOf[id] = i
		}
	}
	emitted := map[int]bool{}
	outputs := make([]PlanOutput, 0, len(p.Clips))
	for _, clip := range p.Clips {
		gi, grouped := groupOf[clip.ID]
		if !grouped {
			outputs = append(outputs, PlanOutput{ID: clip.ID, Title: clip.Title, Clips: []ClipRange{clip}})
			continue
		}
		if emitted[gi] {
			continue
		}
		emitted[gi] = true
		group := p.Groups[gi]
		out := PlanOutput{ID: group.ID, Title: group.Title}
		titles := make([]string, 0, len(group.ClipIDs))
		for _, id := range group.ClipIDs {
			member := byID[id]
			out.Clips = append(out.Clips, member)
			if t := strings.TrimSpace(member.Title); t != "" {
				titles = append(titles, t)
			}
		}
		if out.Title == "" {
			out.Title = strings.Join(titles, " + ")
		}
		outputs = append(outputs, out)
	}
	return outputs
}

// MusicOffsetSeconds is where the music track already is when a clip starts:
// inside a group the track carries on across the cut instead of restarting.
func (p EditPlan) MusicOffsetSeconds(clipID string) float64 {
	for _, out := range p.Outputs() {
		offset := 0.0
		for _, clip := range out.Clips {
			if clip.ID == clipID {
				return offset
			}
			offset += clip.OutputDurationSeconds()
		}
	}
	return 0
}

func (p EditPlan) validateGroups() error {
	clipIDs := make(map[string]bool, len(p.Clips))
	for _, clip := range p.Clips {
		clipIDs[clip.ID] = true
	}
	groupIDs := map[string]bool{}
	member := map[string]string{}
	for _, group := range p.Groups {
		if !clipIDPattern.MatchString(group.ID) {
			return fmt.Errorf("invalid group id %q", group.ID)
		}
		if groupIDs[group.ID] || clipIDs[group.ID] {
			return fmt.Errorf("group id %q is already used by a clip or another group", group.ID)
		}
		groupIDs[group.ID] = true
		if len(group.ClipIDs) < 2 {
			return fmt.Errorf("group %s must join at least 2 clips", group.ID)
		}
		if len([]rune(group.Title)) > maxTextOverlayRunes {
			return fmt.Errorf("group %s title must be at most 120 characters", group.ID)
		}
		for _, id := range group.ClipIDs {
			if !clipIDs[id] {
				return fmt.Errorf("group %s names unknown clip %q", group.ID, id)
			}
			if other, taken := member[id]; taken {
				return fmt.Errorf("clip %s is in groups %s and %s", id, other, group.ID)
			}
			member[id] = group.ID
		}
	}
	return nil
}

// normalizeGroups drops members whose moment was removed and any group left
// with fewer than two, so deleting a moment never strands a saved plan.
func normalizeGroups(plan EditPlan) []ClipGroup {
	if len(plan.Groups) == 0 {
		return nil
	}
	clipIDs := make(map[string]bool, len(plan.Clips))
	for _, clip := range plan.Clips {
		clipIDs[strings.TrimSpace(clip.ID)] = true
	}
	groups := make([]ClipGroup, 0, len(plan.Groups))
	for _, group := range plan.Groups {
		next := ClipGroup{ID: strings.TrimSpace(group.ID), Title: strings.TrimSpace(group.Title)}
		for _, id := range group.ClipIDs {
			if id = strings.TrimSpace(id); clipIDs[id] {
				next.ClipIDs = append(next.ClipIDs, id)
			}
		}
		if len(next.ClipIDs) >= 2 {
			groups = append(groups, next)
		}
	}
	if len(groups) == 0 {
		return nil
	}
	return groups
}
