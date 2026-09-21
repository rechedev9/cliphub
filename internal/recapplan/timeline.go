package recapplan

import (
	"fmt"
	"math"
	"slices"
)

// RebuildTimeline quantizes each round once and inserts sponsor content without
// consuming gameplay time. It preserves requested options on placement failure.
// An intro bumper opens the program before the first captured frame and an
// outro bumper closes it after the last one; both are plain appends around the
// gameplay span, so sponsor candidates and windows stay in program frames.
func (d *Document) RebuildTimeline() error {
	d.Timeline = []TimelineItem{}
	d.SponsorPlacement.Candidates = []Boundary{}
	d.Blockers = slices.DeleteFunc(d.Blockers, func(n Notice) bool { return n.Code == ErrSponsorPlacement })
	var cursor int64
	if intro, ok := d.bumperItem(d.Options.IntroBumper, BumperRoleIntro, cursor); ok {
		d.Timeline = append(d.Timeline, intro)
		cursor = intro.EndFrame
	}
	gameplayStart := cursor
	for _, r := range d.Rounds {
		frames, err := TickFrames(r.EffectiveEndTick-r.RequestedStartTick, d.Clock.TickRate)
		if err != nil {
			return err
		}
		if frames == 0 {
			continue
		}
		d.Timeline = append(d.Timeline, TimelineItem{Role: "round", SourceRef: r.ID, SourceStartTick: r.RequestedStartTick, SourceEndTick: r.EffectiveEndTick, StartFrame: cursor, EndFrame: cursor + frames, Reason: r.StartReason + "/" + r.EndReason})
		cursor += frames
		d.SponsorPlacement.Candidates = append(d.SponsorPlacement.Candidates, Boundary{r.ID, cursor})
	}
	if d.Options.Sponsor.Enabled && d.SponsorPlacement.DurationFrames > 0 {
		frame, boundary, found := resolveSponsor(d.Options.Sponsor, d.SponsorPlacement.Candidates, gameplayStart, cursor)
		if !found {
			d.block(ErrSponsorPlacement, "No approved sponsor position fits the current round timeline")
		} else {
			d.SponsorPlacement.StartFrame, d.SponsorPlacement.Boundary = frame, boundary
			d.insertSponsor(frame)
		}
	}
	if len(d.Timeline) > 0 {
		if outro, ok := d.bumperItem(d.Options.OutroBumper, BumperRoleOutro, d.Timeline[len(d.Timeline)-1].EndFrame); ok {
			d.Timeline = append(d.Timeline, outro)
		}
	}
	for i := range d.Timeline {
		d.Timeline[i].StartSample = d.Timeline[i].StartFrame * SamplesPerFrame
		d.Timeline[i].EndSample = d.Timeline[i].EndFrame * SamplesPerFrame
	}
	return ValidateTimeline(d.Timeline)
}

// bumperItem builds the timeline item for an enabled bumper whose asset the
// plan already verified. A requested bumper without usable evidence is a
// planner blocker, not a placement decision, so it is skipped here.
func (d *Document) bumperItem(slot func() (BumperSlot, bool), reason string, start int64) (TimelineItem, bool) {
	bumper, ok := slot()
	if !ok || bumper.Video == nil {
		return TimelineItem{}, false
	}
	a, ok := findAsset(d.Assets, *bumper.Video)
	if !ok || !a.HasVideo || a.DurationFrames <= 0 {
		return TimelineItem{}, false
	}
	return TimelineItem{Role: "bumper", SourceRef: bumper.Video.ID, StartFrame: start, EndFrame: start + a.DurationFrames, Reason: reason}, true
}

// resolveSponsor picks the sponsor frame inside [start, total]: candidates are
// round ends and a manual frame may split a round, never a bumper.
func resolveSponsor(o SponsorOptions, candidates []Boundary, start, total int64) (int64, string, bool) {
	switch o.PlacementPolicy {
	case "first-two-rounds":
		low, high := int64(math.Ceil(o.WindowStartSeconds*OutputFPS)), int64(math.Floor(o.WindowEndSeconds*OutputFPS))
		for _, i := range []int{1, 0} {
			if i < len(candidates) {
				c := candidates[i]
				if c.Frame >= low && c.Frame <= high && c.Frame <= total {
					return c.Frame, c.AfterRoundID, true
				}
			}
		}
	case "round-boundary":
		for _, c := range candidates {
			if c.AfterRoundID == o.AfterRoundID && c.Frame <= total {
				return c.Frame, c.AfterRoundID, true
			}
		}
	case "manual-frame":
		if o.ManualStartFrame == nil || *o.ManualStartFrame < start || *o.ManualStartFrame > total {
			return 0, "", false
		}
		for _, c := range candidates {
			if c.Frame == *o.ManualStartFrame {
				return c.Frame, c.AfterRoundID, true
			}
		}
		if o.AllowSplitRound && *o.ManualStartFrame < total {
			return *o.ManualStartFrame, "manual-inside-round", true
		}
	}
	return 0, "", false
}

func (d *Document) insertSponsor(frame int64) {
	duration := d.SponsorPlacement.DurationFrames
	sponsor := TimelineItem{Role: "sponsor", SourceRef: d.Options.Sponsor.Video.ID, StartFrame: frame, EndFrame: frame + duration, Reason: "approved-sponsor-placement"}
	out := make([]TimelineItem, 0, len(d.Timeline)+2)
	inserted := false
	for _, item := range d.Timeline {
		if !inserted && item.Role == "round" && frame >= item.StartFrame && frame < item.EndFrame {
			if frame > item.StartFrame {
				prefix := item
				prefix.EndFrame = frame
				out = append(out, prefix)
				item.SourceOffsetFrames += frame - item.StartFrame
				item.StartFrame = frame
			}
			out = append(out, sponsor)
			inserted = true
		}
		if inserted {
			item.StartFrame += duration
			item.EndFrame += duration
		}
		out = append(out, item)
	}
	if !inserted && len(out) > 0 && frame == out[len(out)-1].EndFrame {
		out = append(out, sponsor)
	}
	d.Timeline = out
}

func ValidateTimeline(items []TimelineItem) error {
	var cursor int64
	if len(items) > 1000 {
		return fmt.Errorf("timeline exceeds item limit")
	}
	for _, item := range items {
		if item.Role != "round" && item.Role != "sponsor" && item.Role != "bumper" && item.Role != "bookend" {
			return fmt.Errorf("unknown timeline role %q", item.Role)
		}
		if item.SourceRef == "" || item.SourceOffsetFrames < 0 || item.StartFrame != cursor || item.EndFrame <= cursor || item.EndFrame > 43200*OutputFPS {
			return fmt.Errorf("invalid or discontinuous frame interval")
		}
		if item.StartSample != item.StartFrame*SamplesPerFrame || item.EndSample != item.EndFrame*SamplesPerFrame {
			return fmt.Errorf("timeline sample clock disagrees with frames")
		}
		cursor = item.EndFrame
	}
	return nil
}

// ApplyCertifiedEnds returns an effective copy. Only an approved tail reduction
// is allowed; missing/interior POV coverage fails instead of hiding a jump cut.
func ApplyCertifiedEnds(approved Snapshot, ends map[string]int) (Document, error) {
	if err := approved.Validate(); err != nil {
		return Document{}, err
	}
	d := approved.Document
	d.Rounds = slices.Clone(d.Rounds)
	d.Warnings = slices.Clone(d.Warnings)
	d.Blockers = slices.Clone(d.Blockers)
	for i, r := range d.Rounds {
		end, ok := ends[r.ID]
		if !ok || end < min(r.LiveEndTick+1, r.RequestedEndTick) {
			return Document{}, &Error{ErrPOVContract, "uncertified live footage in " + r.ID}
		}
		if end >= r.RequestedEndTick {
			continue
		}
		if !approved.Approval.AllowSafeTailTrim {
			return Document{}, &Error{ErrPOVContract, "tail reduction was not approved"}
		}
		d.Rounds[i].EffectiveEndTick = end
		d.Rounds[i].EndReason = "certified-pov-tail-trim"
		d.Warnings = append(d.Warnings, Notice{Code: "pov_tail_trimmed", Message: fmt.Sprintf("Tail reduced by %d source ticks", r.RequestedEndTick-end), RoundID: r.ID})
	}
	if err := d.RebuildTimeline(); err != nil {
		return Document{}, err
	}
	if len(d.Blockers) > 0 {
		b := d.Blockers[0]
		return Document{}, &Error{b.Code, b.Message}
	}
	d.PlanHash = ""
	var err error
	d.PlanHash, err = d.Hash()
	return d, err
}
