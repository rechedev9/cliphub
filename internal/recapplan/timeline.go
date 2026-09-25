package recapplan

import (
	"fmt"
	"slices"
)

// WarnSponsorAfterLastRound marks a plan whose sponsor could not follow the
// second round because the program has fewer rounds.
const WarnSponsorAfterLastRound = "sponsor_after_last_round"

// RebuildTimeline quantizes each round once and places the bumpers around it
// without consuming gameplay time: the intro opens the program before the
// first captured frame, the sponsor plays right after the second gameplay
// round (after the last one when there are fewer) and the outro closes the
// program after the last round.
func (d *Document) RebuildTimeline() error {
	d.Timeline = []TimelineItem{}
	d.Warnings = slices.DeleteFunc(slices.Clone(d.Warnings), func(n Notice) bool { return n.Code == WarnSponsorAfterLastRound })
	var cursor int64
	appendBumper := func(role string) {
		if item, ok := d.bumperItem(role, cursor); ok {
			d.Timeline = append(d.Timeline, item)
			cursor = item.EndFrame
		}
	}
	appendBumper(BumperRoleIntro)
	rounds := 0
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
		if rounds++; rounds == SponsorAfterRounds {
			appendBumper(BumperRoleSponsor)
		}
	}
	if _, ok := d.Options.SponsorBumper(); ok && rounds > 0 && rounds < SponsorAfterRounds {
		d.Warnings = append(d.Warnings, Notice{Code: WarnSponsorAfterLastRound, Message: "The program has one round, so the sponsor plays after it"})
		appendBumper(BumperRoleSponsor)
	}
	if len(d.Timeline) > 0 {
		appendBumper(BumperRoleOutro)
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
func (d *Document) bumperItem(role string, start int64) (TimelineItem, bool) {
	bumper, ok := d.Options.BumperFor(role)
	if !ok || bumper.Video == nil {
		return TimelineItem{}, false
	}
	a, ok := findAsset(d.Assets, *bumper.Video)
	if !ok || !a.HasVideo || a.DurationFrames <= 0 {
		return TimelineItem{}, false
	}
	return TimelineItem{Role: "bumper", SourceRef: bumper.Video.ID, StartFrame: start, EndFrame: start + a.DurationFrames, Reason: role}, true
}

func ValidateTimeline(items []TimelineItem) error {
	var cursor int64
	if len(items) > 1000 {
		return fmt.Errorf("timeline exceeds item limit")
	}
	for _, item := range items {
		if item.Role != "round" && item.Role != "bumper" && item.Role != "bookend" {
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
