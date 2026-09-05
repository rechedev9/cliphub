package recapplan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/sharecode"
)

const (
	ErrPlanStale         = "full_demo_plan_stale"
	ErrAssetMissing      = "full_demo_asset_missing"
	ErrVoiceUnavailable  = "voice_unavailable"
	ErrVoiceDecode       = "voice_decode_failed"
	ErrPOVContract       = "pov_contract_failed"
	ErrSponsorPlacement  = "sponsor_placement_conflict"
	ErrAudioMaster       = "audio_master_validation_failed"
	ErrFactsInsufficient = "full_demo_facts_insufficient"
)

type Error struct {
	Code   string
	Detail string
}

func (e *Error) Error() string { return e.Code + ": " + e.Detail }

var hashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var steamIDPattern = regexp.MustCompile(`^[0-9]{17}$`)
var crosshairPattern = regexp.MustCompile(`^CSGO(?:-[ABCDEFGHJKLMNOPQRSTUVWXYZabcdefhijkmnopqrstuvwxyz23456789]{5}){5}$`)

func ValidCrosshairCode(code string) bool {
	if !crosshairPattern.MatchString(code) {
		return false
	}
	_, err := sharecode.CrosshairCvars(code)
	return err == nil
}

func ValidHash(s string) bool { return hashPattern.MatchString(s) }

// UnmarshalJSON requires explicit decisions, including false, zero and nullable
// asset fields. Unknown fields, duplicates and partial nested objects fail.
func (o *Options) UnmarshalJSON(data []byte) error {
	type plain Options
	var decoded plain
	if err := decodeStrict(data, &decoded); err != nil {
		return err
	}
	*o = Options(decoded)
	return o.Validate()
}

// Snapshot decoding enforces the complete approval wire contract. Semantic
// freshness remains an admission check so stale plans retain their typed 409.
func (s *Snapshot) UnmarshalJSON(data []byte) error {
	type plain Snapshot
	var decoded plain
	if err := decodeStrict(data, &decoded); err != nil {
		return err
	}
	*s = Snapshot(decoded)
	return nil
}

func decodeStrict(data []byte, target any) error {
	if len(data) > 4<<20 {
		return fmt.Errorf("full demo document exceeds 4 MiB")
	}
	if err := requireFields(data, reflect.TypeOf(target).Elem(), "$", 0); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return fmt.Errorf("decode full demo document: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("full demo document must contain one JSON value")
	}
	return nil
}

func requireFields(data []byte, typ reflect.Type, field string, depth int) error {
	if depth > 24 {
		return fmt.Errorf("%s: JSON nesting exceeds limit", field)
	}
	if typ.Kind() == reflect.Pointer {
		if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
			return nil
		}
		return requireFields(data, typ.Elem(), field, depth+1)
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return fmt.Errorf("%s cannot be null", field)
	}
	switch typ.Kind() {
	case reflect.Struct:
		if typ.PkgPath() == "time" {
			return nil
		}
		dec := json.NewDecoder(bytes.NewReader(data))
		tok, err := dec.Token()
		if err != nil || tok != json.Delim('{') {
			return fmt.Errorf("%s must be an object", field)
		}
		fields := make(map[string]reflect.StructField)
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if f.IsExported() && name != "-" {
				fields[name] = f
			}
		}
		seen := map[string]bool{}
		for dec.More() {
			tok, err := dec.Token()
			if err != nil {
				return err
			}
			name, ok := tok.(string)
			if !ok {
				return fmt.Errorf("%s: invalid key", field)
			}
			f, ok := fields[name]
			if !ok || seen[name] {
				return fmt.Errorf("%s.%s: unknown or duplicate field", field, name)
			}
			seen[name] = true
			var raw json.RawMessage
			if err := dec.Decode(&raw); err != nil {
				return err
			}
			if err := requireFields(raw, f.Type, field+"."+name, depth+1); err != nil {
				return err
			}
		}
		if _, err := dec.Token(); err != nil {
			return err
		}
		for name, f := range fields {
			if !seen[name] && !strings.Contains(f.Tag.Get("json"), ",omitempty") {
				return fmt.Errorf("%s.%s is required", field, name)
			}
		}
	case reflect.Slice, reflect.Array:
		var entries []json.RawMessage
		if err := json.Unmarshal(data, &entries); err != nil {
			return err
		}
		if len(entries) > 10000 {
			return fmt.Errorf("%s exceeds item limit", field)
		}
		for i, entry := range entries {
			if err := requireFields(entry, typ.Elem(), fmt.Sprintf("%s[%d]", field, i), depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func (o Options) Validate() error {
	for _, choice := range []struct {
		name, value string
		allowed     []string
	}{
		{"profile_id", o.ProfileID, []string{ProfileChill}},
		{"source_kind", o.SourceKind, []string{"demo", "premier", "professional", "faceit"}},
		{"hud_profile", o.Capture.HUDProfile, []string{"native-clean-spectator", "native"}},
		{"camera_policy", o.Capture.CameraPolicy, []string{"strict-first-person"}},
		{"contract_version", o.Capture.ContractVersion, []string{CaptureContract}},
		{"crosshair.mode", o.Capture.Crosshair.Mode, []string{"observed", "provided-code"}},
		{"voice.team_policy", o.Audio.Voice.TeamPolicy, []string{"same-side-at-packet"}},
		{"voice.normalization", o.Audio.Voice.Normalization, []string{"bounded-activity-v1", "none"}},
		{"voice.approved_fallback", o.Audio.Voice.ApprovedFallback, []string{"block", "without-voice"}},
		{"music.reference_level", o.Audio.Music.ReferenceLevel, []string{"track-lufs-minus-16-v1"}},
		{"music.loop_policy", o.Audio.Music.LoopPolicy, []string{"ordered-loop", "once-pad-silence"}},
		{"loudness.policy_version", o.Audio.Loudness.PolicyVersion, []string{"program-aac-v1"}},
		{"sponsor.audio_policy", o.Sponsor.AudioPolicy, []string{"embedded", "replace-narration"}},
		{"sponsor.short_narration_policy", o.Sponsor.ShortNarrationPolicy, []string{"block", "pad-silence"}},
		{"sponsor.placement_policy", o.Sponsor.PlacementPolicy, []string{"first-two-rounds", "round-boundary", "manual-frame"}},
		{"sponsor.music_policy", o.Sponsor.MusicPolicy, []string{"pause-resume"}},
		{"overlays.theme", o.Overlays.Theme, []string{"faceit-orange", "neon-violet"}},
		{"overlays.source", o.Overlays.Source, []string{"demo", "faceit"}},
		{"outputs.media_profile", o.Outputs.MediaProfile, []string{"h264-1080p60-aac48-stereo"}},
		{"outputs.cover_policy", o.Outputs.CoverPolicy, []string{"no-cover", "generated-gameplay"}},
		{"outputs.metadata_policy", o.Outputs.MetadataPolicy, []string{"factual-v1"}},
	} {
		found := false
		for _, value := range choice.allowed {
			if value == choice.value {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid %s %q", choice.name, choice.value)
		}
	}
	if o.Capture.XRay {
		return fmt.Errorf("full demo profile requires xray disabled")
	}
	if o.Capture.Crosshair.Mode == "provided-code" {
		if !ValidCrosshairCode(o.Capture.Crosshair.Code) {
			return fmt.Errorf("invalid crosshair code")
		}
	} else if o.Capture.Crosshair.Code != "" {
		return fmt.Errorf("observed crosshair must not carry a provided code")
	}
	for _, n := range []struct {
		name             string
		value, low, high float64
	}{
		{"freeze_seconds", o.Editorial.FreezeSeconds, 0, 20},
		{"voice_context_seconds", o.Editorial.VoiceContextSeconds, 0, 3},
		{"max_freeze_seconds", o.Editorial.MaxFreezeSeconds, 0, 60},
		{"death_tail_seconds", o.Editorial.DeathTailSeconds, 0, 3},
		{"round_tail_seconds", o.Editorial.RoundTailSeconds, 0, 2},
		{"voice.gain", o.Audio.Voice.Gain, 0, 2}, {"game.gain", o.Audio.Game.Gain, 0, 2},
		{"music.bed_gain_db", o.Audio.Music.BedGainDB, -24, -18},
		{"ducking.game_contribution", o.Audio.Music.Ducking.GameContribution, 0, 1},
		{"ducking.attack_ms", o.Audio.Music.Ducking.AttackMS, 1, 2000},
		{"ducking.release_ms", o.Audio.Music.Ducking.ReleaseMS, 20, 5000},
		{"ducking.threshold", o.Audio.Music.Ducking.Threshold, 0.001, 1},
		{"ducking.ratio", o.Audio.Music.Ducking.Ratio, 1, 20},
		{"loudness.target_i_lufs", o.Audio.Loudness.TargetILUFS, -14, -14},
		{"loudness.target_tp_dbtp", o.Audio.Loudness.TargetTPDBTP, -1.5, -1.5},
		{"loudness.target_lra", o.Audio.Loudness.TargetLRA, 11, 11},
		{"sponsor.window_start_seconds", o.Sponsor.WindowStartSeconds, 0, 43200},
		{"sponsor.window_end_seconds", o.Sponsor.WindowEndSeconds, 0, 43200},
	} {
		if math.IsNaN(n.value) || math.IsInf(n.value, 0) || n.value < n.low || n.value > n.high {
			return fmt.Errorf("%s must be finite and between %g and %g", n.name, n.low, n.high)
		}
	}
	if o.Editorial.MaxFreezeSeconds < o.Editorial.FreezeSeconds {
		return fmt.Errorf("max freeze must cover base freeze")
	}
	if o.Sponsor.WindowEndSeconds < o.Sponsor.WindowStartSeconds {
		return fmt.Errorf("sponsor window is reversed")
	}
	if o.Sponsor.PlacementPolicy == "manual-frame" && (o.Sponsor.ManualStartFrame == nil || *o.Sponsor.ManualStartFrame < 0) {
		return fmt.Errorf("manual sponsor frame is required and non-negative")
	}
	if o.Sponsor.PlacementPolicy == "round-boundary" && o.Sponsor.AfterRoundID == "" {
		return fmt.Errorf("sponsor round boundary is required")
	}
	if len(o.Audio.Music.Assets) > 20 || len(o.Editorial.ManualRanges) > 200 {
		return fmt.Errorf("full demo selection exceeds item limit")
	}
	for _, ref := range o.Audio.Music.Assets {
		if err := ref.Validate(); err != nil {
			return err
		}
	}
	for _, ref := range []*AssetRef{o.Sponsor.Video, o.Sponsor.Narration} {
		if ref != nil {
			if err := ref.Validate(); err != nil {
				return err
			}
		}
	}
	seen := map[string]bool{}
	for _, r := range o.Editorial.ManualRanges {
		if r.RoundID == "" || seen[r.RoundID] || r.StartTick < 0 || r.EndTick <= r.StartTick {
			return fmt.Errorf("invalid or duplicate manual round range %q", r.RoundID)
		}
		seen[r.RoundID] = true
	}
	return nil
}

func (a AssetRef) Validate() error {
	id, err := uuid.Parse(a.ID)
	if err != nil || id == uuid.Nil || id.String() != a.ID || !ValidHash(a.SHA256) {
		return fmt.Errorf("asset requires a canonical UUID and content SHA-256")
	}
	return nil
}

func (f Facts) Validate() error {
	if f.SchemaVersion != DocumentVersion || f.ClockKind != ClockIngame || f.TickRate < 1 || f.TickRate > 1024 || f.EndTick <= 0 || int64(f.EndTick) > int64(f.TickRate)*43200 {
		return &Error{ErrFactsInsufficient, "unsupported facts version, clock or duration"}
	}
	if !ValidHash(f.DemoSHA256) || !steamIDPattern.MatchString(f.TargetSteamID64) {
		return &Error{ErrFactsInsufficient, "demo hash and exact player identity are required"}
	}
	if len(f.Rounds) == 0 || len(f.Rounds) > 200 {
		return &Error{ErrFactsInsufficient, "no bounded round evidence"}
	}
	if err := validateCrosshairSamples(f.Crosshairs, f.EndTick); err != nil {
		return err
	}
	seen := map[string]bool{}
	previous := -1
	for _, r := range f.Rounds {
		if r.ID == "" || seen[r.ID] || r.Number <= 0 || r.StartTick < 0 || r.StartTick < previous || r.StartTick >= f.EndTick {
			return &Error{ErrFactsInsufficient, "invalid round identity or source clock order"}
		}
		seen[r.ID] = true
		previous = r.StartTick
		if len(r.ID) > 96 || r.Number > 1000 || r.FreezeEndTick < 0 || r.FreezeEndTick > f.EndTick || r.RoundEndTick < 0 || r.RoundEndTick > f.EndTick || r.NextStartTick < 0 || r.NextStartTick > f.EndTick || len(r.Kills) > 1000 || len(r.Utility) > 1000 {
			return &Error{ErrFactsInsufficient, "round evidence exceeds source/resource bounds"}
		}
		if r.DeathTick != nil && (*r.DeathTick < r.StartTick || *r.DeathTick >= f.EndTick) {
			return &Error{ErrFactsInsufficient, "death lies outside source bounds"}
		}
	}
	return nil
}

func (s Snapshot) Validate() error {
	if err := s.Document.Validate(); err != nil {
		return err
	}
	if len(s.Document.Blockers) != 0 {
		b := s.Document.Blockers[0]
		return &Error{b.Code, b.Message}
	}
	if len(s.Document.Rounds) == 0 || len(s.Document.Timeline) == 0 {
		return &Error{ErrFactsInsufficient, "an approved Full Demo requires publishable rounds"}
	}
	if s.Approval.PlanHash != s.Document.PlanHash || s.Approval.Timestamp.IsZero() || s.Approval.AllowSafeTailTrim != s.Document.Options.Editorial.AllowSafeTailTrim {
		return &Error{ErrPlanStale, "approval does not match the document and safety rule"}
	}
	return nil
}

func (d Document) Validate() error {
	if d.SchemaVersion != DocumentVersion || d.PlannerVersion != PlannerVersion || d.Revision < 1 {
		return fmt.Errorf("unsupported full demo document version")
	}
	if _, err := uuid.Parse(d.PlanID); err != nil {
		return fmt.Errorf("invalid plan id: %w", err)
	}
	if err := d.Options.Validate(); err != nil {
		return err
	}
	if d.Clock.SourceKind != ClockIngame || d.Clock.TickRate < 1 || d.Clock.TickRate > 1024 || d.Clock.FPS != OutputFPS || d.Clock.SampleRate != SampleRate {
		return fmt.Errorf("unsupported full demo clock")
	}
	if !ValidHash(d.Input.DemoSHA256) || !ValidHash(d.Input.FactsHash) || !steamIDPattern.MatchString(d.Input.TargetSteamID64) {
		return fmt.Errorf("invalid full demo input identity")
	}
	if len(d.Rounds) > 200 || len(d.Assets) > 100 || len(d.Voice.Activity) > 10000 || len(d.Warnings) > 1000 || len(d.Blockers) > 1000 {
		return fmt.Errorf("full demo document exceeds resource bounds")
	}
	if err := validateCrosshairSamples(d.Crosshairs, d.Clock.TickRate*43200); err != nil {
		return err
	}
	seen := map[string]bool{}
	previousEnd := 0
	for _, round := range d.Rounds {
		if round.ID == "" || len(round.ID) > 96 || seen[round.ID] || round.Number < 1 || round.Number > 1000 || round.CaptureStartTick < 0 || round.RequestedStartTick < round.CaptureStartTick || round.RequestedStartTick < previousEnd || round.RequestedEndTick <= round.RequestedStartTick || round.RequestedEndTick > round.CaptureEndTick || round.EffectiveEndTick > round.RequestedEndTick || round.EffectiveEndTick <= round.RequestedStartTick || int64(round.CaptureEndTick) > int64(d.Clock.TickRate)*43200 || round.LiveEndTick < round.RequestedStartTick || round.LiveEndTick > round.RequestedEndTick || round.EffectiveEndTick < min(round.LiveEndTick+1, round.RequestedEndTick) || len(round.Kills) > 1000 || len(round.Utility) > 1000 {
			return fmt.Errorf("invalid Full Demo round coverage: %s", round.ID)
		}
		seen[round.ID] = true
		previousEnd = round.EffectiveEndTick
	}
	hash, err := d.Hash()
	if err != nil {
		return err
	}
	if !ValidHash(d.PlanHash) || hash != d.PlanHash {
		return &Error{ErrPlanStale, "document content hash differs"}
	}
	if err := ValidateTimeline(d.Timeline); err != nil {
		return err
	}
	expected := d
	expected.Blockers = append([]Notice{}, d.Blockers...)
	if err := expected.RebuildTimeline(); err != nil {
		return err
	}
	gotTimeline, err := HashValue(struct {
		Items   []TimelineItem
		Sponsor SponsorPlacement
	}{d.Timeline, d.SponsorPlacement})
	if err != nil {
		return err
	}
	wantTimeline, err := HashValue(struct {
		Items   []TimelineItem
		Sponsor SponsorPlacement
	}{expected.Timeline, expected.SponsorPlacement})
	if err != nil {
		return err
	}
	if gotTimeline != wantTimeline {
		return fmt.Errorf("full demo timeline does not derive from its round and sponsor document")
	}
	return nil
}

func validateCrosshairSamples(samples []CrosshairSample, endTick int) error {
	if len(samples) > 4096 {
		return fmt.Errorf("crosshair evidence exceeds resource bounds")
	}
	previous := -1
	for _, sample := range samples {
		if sample.Tick < 0 || sample.Tick < previous || sample.Tick > endTick || len(sample.Code) > 64 {
			return fmt.Errorf("invalid crosshair evidence timeline")
		}
		previous = sample.Tick
	}
	return nil
}
