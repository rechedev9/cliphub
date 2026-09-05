package recapplan

import (
	"time"

	"github.com/rechedev9/cliphub/internal/killplan"
)

const (
	DocumentVersion = "1.0"
	ProfileChill    = "full-demo-pov-chill-v1"
	PlannerVersion  = "full-demo-editorial-v1"
	CaptureContract = "full-demo-observer-v1"
	ClockIngame     = "ingame_tick"
	OutputFPS       = 60
	SampleRate      = 48000
	SamplesPerFrame = SampleRate / OutputFPS
)

// Options contains creative decisions only. Facts and resolved media properties
// are supplied by the server, and approval binds the resulting Document.
type Options struct {
	ProfileID  string           `json:"profile_id"`
	SourceKind string           `json:"source_kind"`
	Capture    CaptureOptions   `json:"capture"`
	Editorial  EditorialOptions `json:"editorial"`
	Audio      AudioOptions     `json:"audio"`
	Sponsor    SponsorOptions   `json:"sponsor"`
	Overlays   OverlayOptions   `json:"overlays"`
	Outputs    OutputOptions    `json:"outputs"`
}

type CaptureOptions struct {
	HUDProfile      string           `json:"hud_profile"`
	XRay            bool             `json:"xray"`
	CameraPolicy    string           `json:"camera_policy"`
	Crosshair       CrosshairOptions `json:"crosshair"`
	ContractVersion string           `json:"contract_version"`
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

type SponsorOptions struct {
	Enabled              bool      `json:"enabled"`
	Video                *AssetRef `json:"video"`
	Narration            *AssetRef `json:"narration"`
	AudioPolicy          string    `json:"audio_policy"`
	ShortNarrationPolicy string    `json:"short_narration_policy"`
	PlacementPolicy      string    `json:"placement_policy"`
	WindowStartSeconds   float64   `json:"window_start_seconds"`
	WindowEndSeconds     float64   `json:"window_end_seconds"`
	AfterRoundID         string    `json:"after_round_id"`
	ManualStartFrame     *int64    `json:"manual_start_frame"`
	AllowSplitRound      bool      `json:"allow_split_round"`
	MusicPolicy          string    `json:"music_policy"`
}

type OverlayOptions struct {
	Roster     bool   `json:"roster"`
	Scoreboard bool   `json:"scoreboard"`
	Theme      string `json:"theme"`
	Source     string `json:"source"`
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
// relative to the editorial round, so sponsor splits never requantize ticks.
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

type SponsorPlacement struct {
	Boundary       string     `json:"boundary"`
	StartFrame     int64      `json:"start_frame"`
	DurationFrames int64      `json:"duration_frames"`
	Candidates     []Boundary `json:"candidates"`
}

type Boundary struct {
	AfterRoundID string `json:"after_round_id"`
	Frame        int64  `json:"frame"`
}

// Document is a planned or effective revision. An approved document is never
// overwritten by the effective copy produced from runtime evidence.
type Document struct {
	Crosshairs       []CrosshairSample `json:"crosshairs"`
	SchemaVersion    string            `json:"schema_version"`
	PlanID           string            `json:"plan_id"`
	Revision         int               `json:"revision"`
	PlanHash         string            `json:"plan_hash"`
	PlannerVersion   string            `json:"planner_version"`
	Input            Input             `json:"input"`
	Clock            Clock             `json:"clock"`
	Options          Options           `json:"options"`
	Rounds           []Round           `json:"rounds"`
	Voice            VoiceEvidence     `json:"voice"`
	Assets           []AssetEvidence   `json:"assets"`
	SponsorPlacement SponsorPlacement  `json:"sponsor_placement"`
	Timeline         []TimelineItem    `json:"timeline"`
	Warnings         []Notice          `json:"warnings"`
	Blockers         []Notice          `json:"blockers"`
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
	return Options{
		ProfileID: ProfileChill, SourceKind: "demo",
		Capture:   CaptureOptions{HUDProfile: "native-clean-spectator", CameraPolicy: "strict-first-person", Crosshair: CrosshairOptions{Mode: "observed"}, ContractVersion: CaptureContract},
		Editorial: EditorialOptions{FreezeSeconds: 5, KeepFreezeVoice: true, VoiceContextSeconds: 0.5, MaxFreezeSeconds: 20, DeathTailSeconds: 3, RoundTailSeconds: 2, AllowSafeTailTrim: true, ManualRanges: []ManualRange{}},
		Audio: AudioOptions{
			Voice:    VoiceOptions{Enabled: true, Gain: 0.85, TeamPolicy: "same-side-at-packet", Normalization: "bounded-activity-v1", ApprovedFallback: "block"},
			Game:     GameOptions{Gain: 1},
			Music:    MusicOptions{Enabled: true, Assets: []AssetRef{}, ReferenceLevel: "track-lufs-minus-16-v1", BedGainDB: -21, LoopPolicy: "ordered-loop", Ducking: DuckingOptions{Enabled: true, AttackMS: 20, ReleaseMS: 800, Threshold: 0.025, Ratio: 8}},
			Loudness: LoudnessOptions{TargetILUFS: -14, TargetTPDBTP: -1.5, TargetLRA: 11, PolicyVersion: "program-aac-v1"},
		},
		Sponsor:  SponsorOptions{Enabled: true, AudioPolicy: "embedded", ShortNarrationPolicy: "block", PlacementPolicy: "first-two-rounds", WindowStartSeconds: 90, WindowEndSeconds: 130, MusicPolicy: "pause-resume"},
		Overlays: OverlayOptions{Theme: "faceit-orange", Source: "demo"},
		Outputs:  OutputOptions{MediaProfile: "h264-1080p60-aac48-stereo", CoverPolicy: "no-cover", MetadataPolicy: "factual-v1"},
	}
}
