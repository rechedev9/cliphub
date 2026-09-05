package recapplan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"slices"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/killplan"
)

func HashValue(value any) (string, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("hash full demo content: %w", err)
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

// Hash ignores storage locations, revision identifiers and the hash itself.
func (d Document) Hash() (string, error) {
	d.PlanID = ""
	d.Revision = 0
	d.PlanHash = ""
	d.Input.FactsRef = ""
	d.Voice.IndexRef = ""
	return HashValue(d)
}

// CaptureHash covers capture decisions and requested coverage, independently
// of music, sponsor, overlays and export settings.
func (d Document) CaptureHash() (string, error) {
	type coverage struct {
		ID         string
		Start, End int
	}
	windows := make([]coverage, 0, len(d.Rounds))
	for _, r := range d.Rounds {
		windows = append(windows, coverage{r.ID, r.CaptureStartTick, r.CaptureEndTick})
	}
	return HashValue(struct {
		Demo, Target string
		Clock        Clock
		Capture      CaptureOptions
		Windows      []coverage
		Crosshairs   []CrosshairSample
	}{d.Input.DemoSHA256, d.Input.TargetSteamID64, d.Clock, d.Options.Capture, windows, d.Crosshairs})
}

// CaptureCovers permits narrower edits inside compatible certified coverage.
func CaptureCovers(captured, requested Document) bool {
	if captured.Input.DemoSHA256 != requested.Input.DemoSHA256 || captured.Input.TargetSteamID64 != requested.Input.TargetSteamID64 || captured.Clock != requested.Clock || captured.Options.Capture != requested.Options.Capture || !slices.Equal(captured.Crosshairs, requested.Crosshairs) {
		return false
	}
	for _, want := range requested.Rounds {
		found := false
		for _, have := range captured.Rounds {
			if have.ID == want.ID && have.CaptureStartTick <= want.RequestedStartTick && have.EffectiveEndTick >= want.RequestedEndTick {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// TickFrames rounds a duration exactly once using integer arithmetic.
func TickFrames(ticks, tickRate int) (int64, error) {
	if tickRate < 1 || tickRate > 1024 || ticks < 0 || int64(ticks) > int64(tickRate)*43200 {
		return 0, fmt.Errorf("tick duration outside supported clock")
	}
	return (int64(ticks)*OutputFPS + int64(tickRate)/2) / int64(tickRate), nil
}

func secondsTicks(seconds float64, rate int) int { return int(math.Round(seconds * float64(rate))) }

// Plan derives editorial windows from independent facts and verified assets.
// Missing media yields actionable blockers while retaining enabled decisions.
func Plan(f Facts, options Options, voice VoiceEvidence, assets []AssetEvidence, factsRef string) (Document, error) {
	if err := f.Validate(); err != nil {
		return Document{}, err
	}
	if err := options.Validate(); err != nil {
		return Document{}, err
	}
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return Document{}, err
	}
	if err := json.Unmarshal(optionsJSON, &options); err != nil {
		return Document{}, err
	}
	factsHash, err := HashValue(f)
	if err != nil {
		return Document{}, err
	}
	if voice.Activity == nil {
		voice.Activity = []TickRange{}
	}
	d := Document{
		Crosshairs:    append([]CrosshairSample{}, f.Crosshairs...),
		SchemaVersion: DocumentVersion, PlanID: uuid.NewString(), Revision: 1, PlannerVersion: PlannerVersion,
		Input:   Input{DemoSHA256: f.DemoSHA256, TargetSteamID64: f.TargetSteamID64, FactsRef: factsRef, FactsHash: factsHash},
		Clock:   Clock{SourceKind: ClockIngame, TickRate: f.TickRate, FPS: OutputFPS, SampleRate: SampleRate},
		Options: options, Voice: voice, Assets: append([]AssetEvidence{}, assets...),
		Rounds: []Round{}, Timeline: []TimelineItem{}, Warnings: append([]Notice{}, f.Warnings...), Blockers: []Notice{},
		SponsorPlacement: SponsorPlacement{Candidates: []Boundary{}},
	}
	if !f.Complete {
		d.block(ErrFactsInsufficient, "Source ended without complete round evidence")
	}
	for _, fact := range f.Rounds {
		r, notices, err := planRound(f, fact, options.Editorial, voice)
		if err != nil {
			return Document{}, err
		}
		d.Warnings = append(d.Warnings, notices...)
		if r.RequestedEndTick > r.RequestedStartTick {
			d.Rounds = append(d.Rounds, r)
		}
	}
	if len(d.Rounds) == 0 {
		d.block(ErrFactsInsufficient, "No publishable player round remains")
	}
	if options.Capture.Crosshair.Mode == "observed" {
		for _, round := range d.Rounds {
			code := ""
			missing := false
			for _, sample := range f.Crosshairs {
				if sample.Tick <= round.RequestedStartTick {
					code = sample.Code
				}
				if sample.Tick > round.RequestedStartTick && sample.Tick < round.RequestedEndTick && !ValidCrosshairCode(sample.Code) {
					missing = true
				}
			}
			if !ValidCrosshairCode(code) || missing {
				if !options.Capture.Crosshair.AllowCaptureDefault {
					d.block(ErrPOVContract, "Observed crosshair is unavailable in "+round.ID+"; provide a code or explicitly approve the capture default")
				} else {
					d.Warnings = append(d.Warnings, Notice{Code: "crosshair_capture_default_approved", Message: "Capture default explicitly permitted where the demo crosshair is unavailable", RoundID: round.ID})
				}
			}
		}
	}
	for _, manual := range options.Editorial.ManualRanges {
		if !slices.ContainsFunc(d.Rounds, func(r Round) bool { return r.ID == manual.RoundID }) {
			return Document{}, fmt.Errorf("manual range references unavailable round %q", manual.RoundID)
		}
	}
	if options.Audio.Voice.Enabled && voice.Availability != "available" {
		if voice.Availability == "failed" || voice.Availability == "invalid_timeline" || voice.Availability == "unsupported_codec" {
			d.block(ErrVoiceDecode, "Team voice extraction is incompatible or failed: "+voice.Availability)
		} else if options.Audio.Voice.ApprovedFallback != "without-voice" {
			d.block(ErrVoiceUnavailable, "Team voice is unavailable: "+voice.Availability)
		} else {
			d.Warnings = append(d.Warnings, Notice{Code: ErrVoiceUnavailable, Message: "Approved export without team voice: " + voice.Availability})
		}
	}
	if voice.Availability == "available" && (voice.ClockKind != ClockIngame || !ValidHash(voice.IndexHash) || voice.IndexRef == "") {
		d.block(ErrVoiceDecode, "Team voice index lacks a verified clock or content reference")
	}
	if options.Audio.Music.Enabled {
		if len(options.Audio.Music.Assets) == 0 {
			d.block(ErrAssetMissing, "Select or import a music track, or explicitly disable music")
		}
		for _, ref := range options.Audio.Music.Assets {
			a, ok := findAsset(assets, ref)
			if !ok || !a.HasAudio || a.DurationFrames <= 0 {
				d.block(ErrAssetMissing, "Selected music has no verified audio: "+ref.ID)
			}
		}
	}
	if options.Sponsor.Enabled {
		if options.Sponsor.Video == nil {
			d.block(ErrAssetMissing, "Select or import a sponsor video, or explicitly disable the sponsor")
		} else {
			a, ok := findAsset(assets, *options.Sponsor.Video)
			if !ok || !a.HasVideo || a.DurationFrames <= 0 {
				d.block(ErrAssetMissing, "Sponsor video is missing or invalid")
			} else {
				d.SponsorPlacement.DurationFrames = a.DurationFrames
				if options.Sponsor.AudioPolicy == "embedded" && !a.HasAudio {
					d.block(ErrAssetMissing, "Sponsor video has no embedded narration; select replacement narration")
				}
			}
		}
		if options.Sponsor.AudioPolicy == "replace-narration" {
			if options.Sponsor.Narration == nil {
				d.block(ErrAssetMissing, "Replacement narration is required")
			} else {
				a, ok := findAsset(assets, *options.Sponsor.Narration)
				if !ok || !a.HasAudio {
					d.block(ErrAssetMissing, "Replacement narration has no verified audio")
				} else if a.DurationFrames < d.SponsorPlacement.DurationFrames && options.Sponsor.ShortNarrationPolicy == "block" {
					d.block(ErrAssetMissing, "Narration is shorter than sponsor; explicitly approve silence padding or replace it")
				}
			}
		}
	}
	for _, a := range assets {
		if a.Permission == "" || a.Creator == "" || a.Title == "" || a.SourceURL == "" {
			d.block(ErrAssetMissing, "Asset provenance and permission are required: "+a.Ref.ID)
		}
	}
	if err := d.RebuildTimeline(); err != nil {
		return Document{}, err
	}
	d.PlanHash, err = d.Hash()
	return d, err
}

func planRound(f Facts, source RoundFacts, opts EditorialOptions, voice VoiceEvidence) (Round, []Notice, error) {
	notices := []Notice{}
	r := Round{ID: source.ID, Number: source.Number, LiveStartTick: source.FreezeEndTick, RoundEndTick: source.RoundEndTick, DeathTick: source.DeathTick, BoundsEvidence: source.Evidence, ExcludedIntervals: []TickRange{}, Kills: []killplan.Kill{}, Utility: []killplan.UtilityThrow{}}
	if source.FreezeEndTick < source.StartTick || source.FreezeEndTick == 0 || source.RoundEndTick < source.FreezeEndTick || source.Evidence != "round-events" {
		return r, append(notices, Notice{Code: ErrFactsInsufficient, Message: "Round excluded: complete independent boundaries are unavailable", RoundID: source.ID}), nil
	}
	limit := f.EndTick
	if source.NextStartTick > source.StartTick {
		limit = min(limit, source.NextStartTick)
	}
	start := max(source.StartTick, source.FreezeEndTick-secondsTicks(opts.FreezeSeconds, f.TickRate))
	r.StartReason = "freeze-context"
	if opts.KeepFreezeVoice && voice.Availability == "available" {
		for _, activity := range voice.Activity {
			if activity.End <= source.StartTick || activity.Start >= source.FreezeEndTick {
				continue
			}
			candidate := max(source.StartTick, source.FreezeEndTick-secondsTicks(opts.MaxFreezeSeconds, f.TickRate), activity.Start-secondsTicks(opts.VoiceContextSeconds, f.TickRate))
			if candidate < start {
				start = candidate
				r.StartReason = "team-voice-activity"
			}
		}
	}
	liveEnd := source.RoundEndTick
	end := min(limit, source.RoundEndTick+max(1, secondsTicks(opts.RoundTailSeconds, f.TickRate)))
	r.EndReason = "round-tail"
	if source.DeathTick != nil && *source.DeathTick <= source.RoundEndTick {
		liveEnd = *source.DeathTick
		end = min(limit, *source.DeathTick+max(1, secondsTicks(opts.DeathTailSeconds, f.TickRate)))
		r.EndReason = "death-tail-requires-certified-pov"
		if *source.DeathTick < source.FreezeEndTick {
			return r, append(notices, Notice{Code: "pov_dead_in_freeze", Message: "Round excluded: player died before live play", RoundID: source.ID}), nil
		}
	}
	for _, manual := range opts.ManualRanges {
		if manual.RoundID != source.ID {
			continue
		}
		if manual.StartTick < source.StartTick || manual.EndTick > end || manual.StartTick > liveEnd {
			return Round{}, nil, fmt.Errorf("manual range for %s exceeds safe source boundaries", source.ID)
		}
		start, end = manual.StartTick, manual.EndTick
		r.StartReason, r.EndReason = "manual-approved-range", "manual-approved-range"
	}
	if end <= start {
		return r, append(notices, Notice{Code: ErrFactsInsufficient, Message: "Round excluded: empty source interval", RoundID: source.ID}), nil
	}
	r.RequestedStartTick, r.RequestedEndTick = start, end
	r.CaptureStartTick, r.CaptureEndTick, r.EffectiveEndTick = start, end, end
	r.LiveEndTick = min(liveEnd, end)
	for _, k := range source.Kills {
		if k.Tick >= start && k.Tick < end {
			r.Kills = append(r.Kills, k)
		}
	}
	for _, u := range source.Utility {
		if u.ThrowTick >= start && u.ThrowTick < end {
			r.Utility = append(r.Utility, u)
		}
	}
	if source.StartTick < start {
		r.ExcludedIntervals = append(r.ExcludedIntervals, TickRange{source.StartTick, start})
	}
	if end < limit {
		r.ExcludedIntervals = append(r.ExcludedIntervals, TickRange{end, limit})
	}
	return r, notices, nil
}

func (d *Document) block(code, message string) {
	d.Blockers = append(d.Blockers, Notice{Code: code, Message: message})
}

func findAsset(assets []AssetEvidence, ref AssetRef) (AssetEvidence, bool) {
	for _, a := range assets {
		if a.Ref == ref {
			return a, true
		}
	}
	return AssetEvidence{}, false
}

// KillPlan adapts editorial coverage to the existing recorder/editor contract.
// The legacy plan and the approved document remain untouched.
func (d Document) KillPlan(base killplan.Plan) killplan.Plan {
	base.Segments = make([]killplan.Segment, 0, len(d.Rounds))
	for _, r := range d.Rounds {
		base.Segments = append(base.Segments, killplan.Segment{ID: r.ID, Round: r.Number, TickStart: r.CaptureStartTick, TickEnd: r.CaptureEndTick, LiveEndTick: r.LiveEndTick, Kills: slices.Clone(r.Kills), Utility: slices.Clone(r.Utility)})
	}
	base.Stats.SegmentsCreated = len(base.Segments)
	base.Stats.DurationSecondsTotal = 0
	for _, r := range d.Rounds {
		base.Stats.DurationSecondsTotal += float64(r.EffectiveEndTick-r.RequestedStartTick) / float64(d.Clock.TickRate)
	}
	return base
}
