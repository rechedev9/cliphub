package recapplan

import (
	"time"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/killplan"
)

const (
	DocumentVersion      = "1.0"
	ProfileChill         = "full-demo-pov-chill-v1"
	PlannerVersion       = "full-demo-editorial-v2"
	LegacyPlannerVersion = "full-demo-editorial-v1"
	// Reserve unrecorded freeze after respawn for asynchronous POV acquisition.
	POVAcquireSeconds  = 2
	FixedFreezeSeconds = 2
	CaptureContract    = "full-demo-observer-v1"
	// NativeHUDProfile keeps the observed player's own CS2 HUD and hides only
	// spectator-only panels. It is the capture used without a custom HUD.
	NativeHUDProfile = "native-clean-spectator"
	ClockIngame      = "ingame_tick"
	OutputFPS        = 60
	SampleRate       = 48000
	SamplesPerFrame  = SampleRate / OutputFPS
)

// The outro scoreboard waits ScoreboardAfterLastKillSeconds after the last
// kill so it never covers the final play, then stays ScoreboardSeconds. The
// planner keeps enough final-round tail for both.
const (
	ScoreboardAfterLastKillSeconds = 1.0
	ScoreboardSeconds              = 8
	// Matches the parser's demo-end safety margin.
	scoreboardEOFMarginSeconds = 2
)

// Options contains creative decisions only. Facts and resolved media properties
// are supplied by the server, and approval binds the resulting Document.
type Options struct {
	ProfileID   string             `json:"profile_id"`
	SourceKind  string             `json:"source_kind"`
	Capture     CaptureOptions     `json:"capture"`
	Editorial   EditorialOptions   `json:"editorial"`
	Audio       AudioOptions       `json:"audio"`
	Bumpers     *BumperOptions     `json:"bumpers,omitempty"`
	Overlays    OverlayOptions     `json:"overlays"`
	Outputs     OutputOptions      `json:"outputs"`
	Transitions *TransitionOptions `json:"transitions,omitempty"`
}

// BumperOptions are the optional channel clips of the program: a pre-roll
// before the first captured frame, a sponsor right after the second gameplay
// round and an outro after the last one. Their placement is fixed, so they
// carry no policy. The pointers are omitted from the wire when absent so
// documents approved before bumpers, or the sponsor slot, existed keep their
// hash.
type BumperOptions struct {
	Intro   BumperSlot  `json:"intro"`
	Sponsor *BumperSlot `json:"sponsor,omitempty"`
	Outro   BumperSlot  `json:"outro"`
}

// SponsorAfterRounds is how many gameplay rounds play before the sponsor.
const SponsorAfterRounds = 2

// BumperSlot is one bumper. Its audio is always the clip's own track; a clip
// without audio plays silent, which is an ordinary outro on YouTube.
type BumperSlot struct {
	Enabled bool      `json:"enabled"`
	Video   *AssetRef `json:"video"`
}

// BumperRoleIntro, BumperRoleSponsor and BumperRoleOutro are the Reason values
// of "bumper" timeline items, so the renderer never has to guess which slot an
// item is.
const (
	BumperRoleIntro   = "intro-bumper"
	BumperRoleSponsor = "sponsor-bumper"
	BumperRoleOutro   = "outro-bumper"
)

// NamedBumper is one requested bumper slot with its label and timeline reason.
type NamedBumper struct {
	Name string
	Role string
	Slot BumperSlot
}

// BumperSlots lists the requested bumper slots in program order.
func (o Options) BumperSlots() []NamedBumper {
	var slots []NamedBumper
	for _, s := range []struct {
		name, role string
		slot       func() (BumperSlot, bool)
	}{{"intro", BumperRoleIntro, o.IntroBumper}, {"sponsor", BumperRoleSponsor, o.SponsorBumper}, {"outro", BumperRoleOutro, o.OutroBumper}} {
		if slot, ok := s.slot(); ok {
			slots = append(slots, NamedBumper{s.name, s.role, slot})
		}
	}
	return slots
}

// BumperFor returns the requested slot a bumper timeline reason plays.
func (o Options) BumperFor(role string) (BumperSlot, bool) {
	for _, named := range o.BumperSlots() {
		if named.Role == role {
			return named.Slot, true
		}
	}
	return BumperSlot{}, false
}

// IntroBumper returns the enabled intro slot, or false when none is requested.
func (o Options) IntroBumper() (BumperSlot, bool) {
	if o.Bumpers == nil || !o.Bumpers.Intro.Enabled {
		return BumperSlot{}, false
	}
	return o.Bumpers.Intro, true
}

// SponsorBumper returns the enabled sponsor slot, or false when none is requested.
func (o Options) SponsorBumper() (BumperSlot, bool) {
	if o.Bumpers == nil || o.Bumpers.Sponsor == nil || !o.Bumpers.Sponsor.Enabled {
		return BumperSlot{}, false
	}
	return *o.Bumpers.Sponsor, true
}

// OutroBumper returns the enabled outro slot, or false when none is requested.
func (o Options) OutroBumper() (BumperSlot, bool) {
	if o.Bumpers == nil || !o.Bumpers.Outro.Enabled {
		return BumperSlot{}, false
	}
	return o.Bumpers.Outro, true
}

// HasBumpers reports whether any bumper is requested.
func (o Options) HasBumpers() bool {
	return len(o.BumperSlots()) > 0
}

// OverlaySource resolves the intro/outro overlay layout for the plan. The demo
// origin (source_kind) owns the format: a FACEIT, Premier or professional
// demo keeps its layout regardless of any later cosmetic option such as a
// custom HUD. overlays.source only adds FACEIT enrichment to a plain demo and
// is kept for documents approved before source_kind drove the layout. The
// empty result means demo-facts-only overlays.
func (o Options) OverlaySource() string {
	switch o.SourceKind {
	case "faceit", "premier", "professional":
		return o.SourceKind
	}
	if o.Overlays.Source == "faceit" {
		return "faceit"
	}
	return ""
}

type CaptureOptions struct {
	HUDProfile      string           `json:"hud_profile"`
	XRay            bool             `json:"xray"`
	CameraPolicy    string           `json:"camera_policy"`
	Crosshair       CrosshairOptions `json:"crosshair"`
	ContractVersion string           `json:"contract_version"`
	// TrueView replays the player's recorded client prediction (CS2
	// cl_demo_predict 1) instead of the interpolated server view. Omitted when
	// off so existing documents keep their wire format and capture hash.
	TrueView bool `json:"trueview,omitempty"`
}

type CrosshairOptions struct {
	Mode                string `json:"mode"`
	Code                string `json:"code"`
	AllowCaptureDefault bool   `json:"allow_capture_default"`
}

type EditorialOptions struct {
	FreezeSeconds       float64       `json:"freeze_seconds"`
	KeepFreezeVoice     bool          `json:"keep_freeze_voice"`
	VoiceContextSeconds float64       `json:"voice_context_seconds"`
	MaxFreezeSeconds    float64       `json:"max_freeze_seconds"`
	DeathTailSeconds    float64       `json:"death_tail_seconds"`
	RoundTailSeconds    float64       `json:"round_tail_seconds"`
	AllowSafeTailTrim   bool          `json:"allow_safe_tail_trim"`
	ManualRanges        []ManualRange `json:"manual_ranges"`
}

type ManualRange struct {
	RoundID   string `json:"round_id"`
	StartTick int    `json:"start_tick"`
	EndTick   int    `json:"end_tick"`
}

type AudioOptions struct {
	Voice    VoiceOptions    `json:"voice"`
	Game     GameOptions     `json:"game"`
	Music    MusicOptions    `json:"music"`
	Loudness LoudnessOptions `json:"loudness"`
}

type VoiceOptions struct {
	Enabled          bool    `json:"enabled"`
	Gain             float64 `json:"gain"`
	TeamPolicy       string  `json:"team_policy"`
	Normalization    string  `json:"normalization"`
	ApprovedFallback string  `json:"approved_fallback"`
}

type GameOptions struct {
	Gain          float64 `json:"gain"`
	VoicePriority bool    `json:"voice_priority"`
}

// AssetRef is an immutable content reference, never an FFmpeg path or URL.
type AssetRef struct {
	ID     string `json:"id"`
	SHA256 string `json:"sha256"`
}

type MusicOptions struct {
	Enabled        bool           `json:"enabled"`
	Assets         []AssetRef     `json:"assets"`
	ReferenceLevel string         `json:"reference_level"`
	BedGainDB      float64        `json:"bed_gain_db"`
	LoopPolicy     string         `json:"loop_policy"`
	Ducking        DuckingOptions `json:"ducking"`
}

type DuckingOptions struct {
	Enabled          bool    `json:"enabled"`
	GameContribution float64 `json:"game_contribution"`
	AttackMS         float64 `json:"attack_ms"`
	ReleaseMS        float64 `json:"release_ms"`
	Threshold        float64 `json:"threshold"`
	Ratio            float64 `json:"ratio"`
}

type LoudnessOptions struct {
	TargetILUFS   float64 `json:"target_i_lufs"`
	TargetTPDBTP  float64 `json:"target_tp_dbtp"`
	TargetLRA     float64 `json:"target_lra"`
	PolicyVersion string  `json:"policy_version"`
}

type OverlayOptions struct {
	HUDTheme        string    `json:"hud_theme,omitempty"`
	HUDPortrait     *AssetRef `json:"hud_portrait,omitempty"`
	Roster          bool      `json:"roster"`
	Scoreboard      bool      `json:"scoreboard"`
	Theme           string    `json:"theme"`
	Source          string    `json:"source"`
	Mode            string    `json:"mode,omitempty"`
	Team1Image      *AssetRef `json:"team1_image,omitempty"`
	Team2Image      *AssetRef `json:"team2_image,omitempty"`
	ScoreboardImage *AssetRef `json:"scoreboard_image,omitempty"`
}

type OutputOptions struct {
	MediaProfile   string `json:"media_profile"`
	CoverPolicy    string `json:"cover_policy"`
	MetadataPolicy string `json:"metadata_policy"`
}

// Facts is immutable demo evidence. Round numbers retain their source identity.
type Facts struct {
	Crosshairs      []CrosshairSample `json:"crosshairs,omitempty"`
	SchemaVersion   string            `json:"schema_version"`
	DemoSHA256      string            `json:"demo_sha256"`
	TargetSteamID64 string            `json:"target_steamid64"`
	ClockKind       string            `json:"clock_kind"`
	TickRate        int               `json:"tick_rate"`
	EndTick         int               `json:"end_tick"`
	Complete        bool              `json:"complete"`
	Rounds          []RoundFacts      `json:"rounds"`
	Warnings        []Notice          `json:"warnings"`
}

type CrosshairSample struct {
	Tick int    `json:"tick"`
	Code string `json:"code"`
}

type RoundFacts struct {
	ID            string                  `json:"id"`
	Number        int                     `json:"number"`
	StartTick     int                     `json:"start_tick"`
	FreezeEndTick int                     `json:"freeze_end_tick"`
	RoundEndTick  int                     `json:"round_end_tick"`
	NextStartTick int                     `json:"next_start_tick"`
	DeathTick     *int                    `json:"death_tick"`
	Kills         []killplan.Kill         `json:"kills"`
	Utility       []killplan.UtilityThrow `json:"utility"`
	Evidence      string                  `json:"evidence"`
}

type Notice struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	RoundID string `json:"round_id,omitempty"`
}

// VoiceEvidence describes actual extraction, including unavailable content.
type VoiceEvidence struct {
	Availability     string      `json:"availability"`
	IndexRef         string      `json:"index_ref"`
	IndexHash        string      `json:"index_hash"`
	ExtractorVersion string      `json:"extractor_version"`
	ClockKind        string      `json:"clock_kind"`
	Activity         []TickRange `json:"activity"`
	SelectedPackets  int         `json:"selected_packets"`
	ExcludedPackets  int         `json:"excluded_packets"`
}

type TickRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// AssetEvidence records server-probed properties and declared provenance.
type AssetEvidence struct {
	Ref            AssetRef `json:"ref"`
	DurationFrames int64    `json:"duration_frames"`
	HasVideo       bool     `json:"has_video"`
	HasAudio       bool     `json:"has_audio"`
	HasImage       bool     `json:"has_image,omitempty"`
	Title          string   `json:"title"`
	Creator        string   `json:"creator"`
	SourceURL      string   `json:"source_url"`
	Permission     string   `json:"permission"`
	Attribution    string   `json:"attribution"`
}

type Round struct {
	Kills              []killplan.Kill         `json:"kills"`
	Utility            []killplan.UtilityThrow `json:"utility"`
	ID                 string                  `json:"round_id"`
	Number             int                     `json:"source_round_number"`
	LiveStartTick      int                     `json:"live_start_tick"`
	RoundEndTick       int                     `json:"round_end_tick"`
	DeathTick          *int                    `json:"death_tick"`
	RequestedStartTick int                     `json:"requested_start_tick"`
	RequestedEndTick   int                     `json:"requested_end_tick"`
	LiveEndTick        int                     `json:"live_end_tick"`
	CaptureStartTick   int                     `json:"capture_start_tick"`
	CaptureEndTick     int                     `json:"capture_end_tick"`
	EffectiveEndTick   int                     `json:"effective_end_tick"`
	StartReason        string                  `json:"start_reason"`
	EndReason          string                  `json:"end_reason"`
	BoundsEvidence     string                  `json:"bounds_evidence"`
	ExcludedIntervals  []TickRange             `json:"excluded_intervals"`
}

type Input struct {
	DemoSHA256      string `json:"demo_sha256"`
	TargetSteamID64 string `json:"target_steamid64"`
	FactsRef        string `json:"facts_ref"`
	FactsHash       string `json:"facts_hash"`
}

type Clock struct {
	SourceKind string `json:"source_clock_kind"`
	TickRate   int    `json:"tick_rate"`
	FPS        int    `json:"output_fps"`
	SampleRate int    `json:"audio_sample_rate"`
}

// TimelineItem uses half-open frame/sample intervals. SourceOffsetFrames is
// relative to the editorial round, so split rounds never requantize ticks.
type TimelineItem struct {
	Role               string `json:"role"`
	SourceRef          string `json:"source_ref"`
	SourceStartTick    int    `json:"source_start_tick"`
	SourceEndTick      int    `json:"source_end_tick"`
	SourceOffsetFrames int64  `json:"source_offset_frames"`
	StartFrame         int64  `json:"start_frame"`
	EndFrame           int64  `json:"end_frame"`
	StartSample        int64  `json:"start_sample"`
	EndSample          int64  `json:"end_sample"`
	Reason             string `json:"reason"`
}

// Document is a planned or effective revision. An approved document is never
// overwritten by the effective copy produced from runtime evidence.
type Document struct {
	Crosshairs     []CrosshairSample `json:"crosshairs"`
	SchemaVersion  string            `json:"schema_version"`
	PlanID         string            `json:"plan_id"`
	Revision       int               `json:"revision"`
	PlanHash       string            `json:"plan_hash"`
	PlannerVersion string            `json:"planner_version"`
	Input          Input             `json:"input"`
	Clock          Clock             `json:"clock"`
	Options        Options           `json:"options"`
	Rounds         []Round           `json:"rounds"`
	Voice          VoiceEvidence     `json:"voice"`
	Assets         []AssetEvidence   `json:"assets"`
	Timeline       []TimelineItem    `json:"timeline"`
	Warnings       []Notice          `json:"warnings"`
	Blockers       []Notice          `json:"blockers"`
}

type Approval struct {
	PlanHash          string    `json:"approved_plan_hash"`
	AllowSafeTailTrim bool      `json:"allow_safe_tail_trim"`
	Timestamp         time.Time `json:"timestamp"`
}

type Snapshot struct {
	Document Document `json:"document"`
	Approval Approval `json:"approval"`
}

func DefaultOptions() Options {
	// Dinamico is the existing complete transition preset. New plans expose
	// only its on/off decision, but persist every resolved setting for approval.
	transitions := DynamicTransitions()
	return Options{
		ProfileID: ProfileChill, SourceKind: "demo",
		Capture:   CaptureOptions{HUDProfile: customhud.CaptureProfile, CameraPolicy: "strict-first-person", Crosshair: CrosshairOptions{Mode: "observed", AllowCaptureDefault: false}, ContractVersion: CaptureContract},
		Editorial: EditorialOptions{FreezeSeconds: FixedFreezeSeconds, KeepFreezeVoice: false, VoiceContextSeconds: 0, MaxFreezeSeconds: FixedFreezeSeconds, DeathTailSeconds: 3, RoundTailSeconds: 2, AllowSafeTailTrim: true, ManualRanges: []ManualRange{}},
		Audio: AudioOptions{
			Voice:    VoiceOptions{Enabled: true, Gain: 1.1, TeamPolicy: "same-side-at-packet", Normalization: "bounded-activity-v1", ApprovedFallback: "block"},
			Game:     GameOptions{Gain: 1},
			Music:    MusicOptions{Enabled: false, Assets: []AssetRef{}, ReferenceLevel: "track-lufs-minus-16-v1", BedGainDB: -21, LoopPolicy: "ordered-loop", Ducking: DuckingOptions{Enabled: true, AttackMS: 20, ReleaseMS: 800, Threshold: 0.025, Ratio: 8}},
			Loudness: LoudnessOptions{TargetILUFS: -14, TargetTPDBTP: -1.5, TargetLRA: 11, PolicyVersion: "program-aac-v1"},
		},
		Overlays:    OverlayOptions{HUDTheme: "arena", Roster: true, Scoreboard: true, Theme: "neon-violet", Source: "demo", Mode: "generated"},
		Outputs:     OutputOptions{MediaProfile: "h264-1080p60-aac48-stereo", CoverPolicy: "no-cover", MetadataPolicy: "factual-v1"},
		Transitions: &transitions,
	}
}
