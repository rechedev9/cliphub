package recapplan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"

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

// FixedFreezeOptions migrates old drafts without letting voice activity or
// persisted controls override the fixed two-second editorial policy. Keep the
// wire fields readable for historical evidence; canonical new plans ignore them.
func FixedFreezeOptions(options Options) Options {
	options.Editorial.FreezeSeconds = FixedFreezeSeconds
	options.Editorial.KeepFreezeVoice = false
	options.Editorial.VoiceContextSeconds = 0
	options.Editorial.MaxFreezeSeconds = FixedFreezeSeconds
	return options
}

// CanonicalNewOptions retires controls that no longer describe a Full Demo.
// Historical documents retain their original wire shape and hashes; callers
// use this only before creating a replacement plan.
func CanonicalNewOptions(options Options) (Options, error) {
	if options.Audio.Music.Enabled || len(options.Audio.Music.Assets) != 0 {
		return Options{}, fmt.Errorf("background music is not available for Full Demo")
	}
	if options.Capture.Crosshair.Mode != "observed" || options.Capture.Crosshair.Code != "" || options.Capture.Crosshair.AllowCaptureDefault {
		return Options{}, fmt.Errorf("Full Demo uses the observed player crosshair; manual codes and capture defaults are retired")
	}

	canonical := DefaultOptions()
	// Keep the selected factual origin. OverlaySource resolves it independently
	// from cosmetics, so a local demo never acquires FACEIT claims by style.
	canonical.SourceKind = options.SourceKind
	canonical.Audio.Voice.Enabled = options.Audio.Voice.Enabled
	// Game and voice levels are the user's mix; Validate bounds them to [0, 2].
	// Calibration, team policy, and the voice fallback stay automatic.
	canonical.Audio.Voice.Gain = options.Audio.Voice.Gain
	canonical.Audio.Game.Gain = options.Audio.Game.Gain
	canonical.Sponsor = options.Sponsor
	if options.Bumpers != nil {
		bumpers := *options.Bumpers
		canonical.Bumpers = &bumpers
	}

	// The custom HUD is optional. A native capture without a theme keeps the
	// player's own CS2 HUD; plain "native" also retires its spectator panels.
	// Any other absent HUD becomes the current broadcast default.
	if options.Overlays.HUDTheme == "" && (options.Capture.HUDProfile == NativeHUDProfile || options.Capture.HUDProfile == "native") {
		canonical.Capture.HUDProfile = NativeHUDProfile
		canonical.Overlays.HUDTheme = ""
	} else if options.Overlays.HUDTheme != "" {
		canonical.Overlays.HUDTheme = options.Overlays.HUDTheme
	}
	canonical.Capture.TrueView = options.Capture.TrueView
	if canonical.Overlays.HUDTheme == "focus" {
		canonical.Overlays.HUDPortrait = options.Overlays.HUDPortrait
	}

	// Roster and scoreboard are generated from factual demo data. The overlay
	// source is deliberately demo: SourceKind above is the only origin signal.
	canonical.Overlays.Theme = "neon-violet"
	canonical.Overlays.Source = "demo"
	canonical.Overlays.Mode = "generated"
	canonical.Overlays.Roster = true
	canonical.Overlays.Scoreboard = true

	// The compact control can only enable or disable the established Dinamico
	// preset; do not let stale granular draft values change a new approval.
	if options.Transitions != nil {
		canonical.Transitions.Enabled = options.Transitions.Enabled
	}
	return FixedFreezeOptions(canonical), nil
}

// ValidateCurrentFullDemoPolicy is an execution gate. Historical documents can
// still be inspected byte-for-byte, but a newly admitted execution must use
// the current automatic capture and overlay policy.
func (o Options) ValidateCurrentFullDemoPolicy() error {
	canonical, err := CanonicalNewOptions(o)
	if err != nil {
		return err
	}
	if o.Capture != canonical.Capture {
		return fmt.Errorf("observed player crosshair and a current HUD capture profile are required")
	}
	if o.Overlays != canonical.Overlays {
		return fmt.Errorf("automatic neon-violet roster and scoreboard overlays are required")
	}
	if o.Transitions == nil || canonical.Transitions == nil || *o.Transitions != *canonical.Transitions {
		return fmt.Errorf("transitions must use the Dinamico preset")
	}
	// Empty historical slices and current empty arrays have the same policy.
	// Work on the value copy: approved documents and their hashes stay intact.
	if len(o.Editorial.ManualRanges) == 0 {
		o.Editorial.ManualRanges = canonical.Editorial.ManualRanges
	}
	o.Audio.Music.Assets = canonical.Audio.Music.Assets // Nonempty music was rejected above.
	if !reflect.DeepEqual(o.Editorial, canonical.Editorial) {
		return fmt.Errorf("automatic round timing is required")
	}
	if !reflect.DeepEqual(o.Audio, canonical.Audio) {
		return fmt.Errorf("automatic audio calibration and voice fallback policy are required")
	}
	if o.Outputs != canonical.Outputs {
		return fmt.Errorf("automatic output settings are required")
	}
	return nil
}

// UsesFixedFreeze is an execution gate, not a historical document decoder.
func (d Document) UsesFixedFreeze() bool {
	o := d.Options.Editorial
	if o.FreezeSeconds != FixedFreezeSeconds || o.KeepFreezeVoice || o.VoiceContextSeconds != 0 || o.MaxFreezeSeconds != FixedFreezeSeconds {
		return false
	}
	for _, r := range d.Rounds {
		if r.LiveStartTick-r.RequestedStartTick != FixedFreezeSeconds*d.Clock.TickRate {
			return false
		}
	}
	return true
}

// Plan derives editorial windows from independent facts and verified assets.
// Missing media yields actionable blockers while retaining enabled decisions.
func Plan(f Facts, options Options, voice VoiceEvidence, assets []AssetEvidence, factsRef string) (Document, error) {
	if err := f.Validate(); err != nil {
		return Document{}, err
	}
	if err := options.Validate(); err != nil {
		return Document{}, err
	}
	options = FixedFreezeOptions(options)
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
		r, notices, err := planRound(f, fact, options.Editorial)
		if err != nil {
			return Document{}, err
		}
		for _, notice := range notices {
			if notice.Code == ErrPOVContract {
				d.Blockers = append(d.Blockers, notice)
			} else {
				d.Warnings = append(d.Warnings, notice)
			}
		}
		if r.RequestedEndTick > r.RequestedStartTick {
			d.Rounds = append(d.Rounds, r)
		}
	}
	if len(d.Rounds) == 0 {
		d.block(ErrFactsInsufficient, "No publishable player round remains")
	}
	if options.Capture.Crosshair.Mode == "observed" {
		missing := []int{}
		for _, round := range d.Rounds {
			if observedCrosshairCovers(f.Crosshairs, round, options.Editorial.AllowSafeTailTrim) {
				continue
			}
			if options.Capture.Crosshair.AllowCaptureDefault {
				d.Warnings = append(d.Warnings, Notice{Code: "crosshair_capture_default_approved", Message: "Capture default explicitly permitted where the demo crosshair is unavailable", RoundID: round.ID})
			} else {
				missing = append(missing, round.Number)
			}
		}
		if len(missing) > 0 {
			d.block(ErrPOVContract, observedCrosshairBlocker(missing, len(d.Rounds)))
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
	for _, bumper := range []struct {
		name string
		slot func() (BumperSlot, bool)
	}{{"intro", options.IntroBumper}, {"outro", options.OutroBumper}} {
		slot, ok := bumper.slot()
		if !ok {
			continue
		}
		if slot.Video == nil {
			d.block(ErrAssetMissing, "Select or import an "+bumper.name+" video, or explicitly disable the "+bumper.name)
		} else if a, ok := findAsset(assets, *slot.Video); !ok || !a.HasVideo || a.DurationFrames <= 0 {
			d.block(ErrAssetMissing, "The "+bumper.name+" video is missing or invalid")
		}
	}
	if len(options.Overlays.ImageSlots()) > 0 {
		for _, slot := range options.Overlays.ImageSlots() {
			if slot.Ref == nil {
				d.block(ErrAssetMissing, "Sube la captura de "+slot.Label)
			} else if a, ok := findAsset(assets, *slot.Ref); !ok || !a.HasImage {
				d.block(ErrAssetMissing, "La captura de "+slot.Label+" no está disponible o no es una imagen verificada")
			}
		}
	}
	for _, a := range assets {
		if !a.HasImage && (a.Permission == "" || a.Creator == "" || a.Title == "" || a.SourceURL == "") {
			d.block(ErrAssetMissing, "Asset provenance and permission are required: "+a.Ref.ID)
		}
	}
	if err := d.RebuildTimeline(); err != nil {
		return Document{}, err
	}
	d.PlanHash, err = d.Hash()
	return d, err
}

func planRound(f Facts, source RoundFacts, opts EditorialOptions) (Round, []Notice, error) {
	notices := []Notice{}
	r := Round{ID: source.ID, Number: source.Number, LiveStartTick: source.FreezeEndTick, RoundEndTick: source.RoundEndTick, DeathTick: source.DeathTick, BoundsEvidence: source.Evidence, ExcludedIntervals: []TickRange{}, Kills: []killplan.Kill{}, Utility: []killplan.UtilityThrow{}}
	if source.FreezeEndTick < source.StartTick || source.FreezeEndTick == 0 || source.RoundEndTick < source.FreezeEndTick || source.Evidence != "round-events" {
		return r, append(notices, Notice{Code: ErrFactsInsufficient, Message: "Round excluded: complete independent boundaries are unavailable", RoundID: source.ID}), nil
	}
	limit := f.EndTick
	if source.NextStartTick > source.StartTick {
		limit = min(limit, source.NextStartTick)
	}
	start := source.FreezeEndTick - FixedFreezeSeconds*f.TickRate
	r.StartReason = "fixed-freeze-2s"
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
	// POV acquisition is unrecorded. Never shorten/extend the fixed two
	// seconds to hide a source that cannot provide both acquisition and freeze.
	safeStart := source.StartTick + POVAcquireSeconds*f.TickRate
	if start < safeStart {
		return r, append(notices, Notice{Code: ErrPOVContract, Message: "La ronda no permite conservar exactamente 2 segundos de freeze y preparar antes el POV; no se ha cambiado la duración", RoundID: source.ID}), nil
	}
	for _, manual := range opts.ManualRanges {
		if manual.RoundID != source.ID {
			continue
		}
		if manual.StartTick != start || manual.EndTick > end || manual.EndTick <= source.FreezeEndTick {
			return Round{}, nil, fmt.Errorf("manual range for %s must retain exactly 2 seconds of freeze and stay within safe source boundaries", source.ID)
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

// observedCrosshairCovers reports whether the demo carries the target's own
// crosshair on every frame the capture must show from the target's POV: the
// fixed freeze lead-in and live play through LiveEndTick. CS2 draws that code
// itself (cl_show_observer_crosshair 2) and ClipHub never decodes it, so any
// code the demo networks for the player is evidence. Requiring the version-1
// layout that sharecode decodes blocks every round of a demo whose codes use
// another layout. With safe tail trim, later frames are not certified: the
// capture ends the round where the target stops being observed, which is where
// the parser records an empty code (the player left after dying or at the end
// of the match).
func observedCrosshairCovers(samples []CrosshairSample, r Round, tailTrim bool) bool {
	last := r.RequestedEndTick - 1
	if tailTrim {
		last = min(r.LiveEndTick, last)
	}
	code := ""
	for _, sample := range samples {
		if sample.Tick > last {
			break
		}
		if sample.Tick <= r.RequestedStartTick {
			code = sample.Code
		} else if sample.Code == "" {
			return false
		}
	}
	return code != ""
}

// observedCrosshairBlocker names the rounds without the player's crosshair and
// what the user can change. The capture never substitutes another crosshair.
func observedCrosshairBlocker(missing []int, rounds int) string {
	where := ""
	if len(missing) < rounds {
		numbers := make([]string, len(missing))
		for i, number := range missing {
			numbers[i] = strconv.Itoa(number)
		}
		where = " en la ronda " + numbers[0]
		if len(numbers) > 1 {
			where = " en las rondas " + strings.Join(numbers[:len(numbers)-1], ", ") + " y " + numbers[len(numbers)-1]
		}
	}
	return "La demo no incluye la mira del jugador" + where + ". ClipHub no graba con otra mira: elige otro jugador o importa otra demo de la partida."
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
		// Legacy duration can stop at the last tracked event, before the
		// post-round/death tail proven by Full Demo facts. Preserve existing
		// bounds when sufficient; otherwise include the approved coverage.
		base.Demo.DurationTicks = max(base.Demo.DurationTicks, r.CaptureEndTick)
		base.Segments = append(base.Segments, killplan.Segment{ID: r.ID, Round: r.Number, TickStart: r.CaptureStartTick, TickEnd: r.CaptureEndTick, LiveEndTick: r.LiveEndTick, Kills: slices.Clone(r.Kills), Utility: slices.Clone(r.Utility)})
	}
	base.Stats.SegmentsCreated = len(base.Segments)
	base.Stats.DurationSecondsTotal = 0
	for _, r := range d.Rounds {
		base.Stats.DurationSecondsTotal += float64(r.EffectiveEndTick-r.RequestedStartTick) / float64(d.Clock.TickRate)
	}
	return base
}
