// Package recording defines the local recording contract consumed by
// zv-recorder. It intentionally does not know about HTTP, Asynq, or Postgres.
package recording

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/killplan"
)

const steamID64AccountIDBase uint64 = 76561197960265728

// CaptureContractVersion identifies the recorder behavior required before a
// persisted capture may be reused. V2 binds the verified capture to the exact
// demo, timeline, target, and profile through CaptureInputFingerprint.
const CaptureContractVersion = "observer-steamid-input-v2"

// LegacyCaptureContractVersion is the durable V1 contract emitted before
// capture-input fingerprints existed. It remains readable so an upgrade does
// not strand already verified local captures whose source is no longer
// available. New recordings must never emit this version.
const LegacyCaptureContractVersion = "observer-steamid-v1"

// Capture console markers are written by the HLAE runtime and consumed by the
// recorder. A plan names the requested behavior; only the verified marker
// attests that the runtime completed every protected capture window.
const (
	CaptureFailedMarker   = "ZACKVIDEO_CAPTURE_FAILED_OBSERVER_STEAMID_V1"
	CaptureVerifiedMarker = "ZACKVIDEO_CAPTURE_VERIFIED_OBSERVER_STEAMID_V1"
)

type CaptureMode string

const (
	CaptureModeReal   CaptureMode = "real"
	CaptureModeFake   CaptureMode = "fake"
	CaptureModeDryRun CaptureMode = "dry_run"
)

func CaptureFailedAttestation(token string) string {
	return CaptureFailedMarker + ":" + token
}

func CaptureVerifiedAttestation(token string) string {
	return CaptureVerifiedMarker + ":" + token
}

const (
	defaultDeathnoticeSafeZoneX       = 0.28
	defaultDeathnoticeSafeZoneY       = 0.82
	defaultDeathnoticeLifetimeSeconds = 1.6
)

// StreamMode names the HLAE output strategy.
type StreamMode string

const (
	// StreamModeFFmpegDirect uses the candidate direct ffmpeg mirv_streams
	// command being validated by the HLAE prototype.
	StreamModeFFmpegDirect StreamMode = "ffmpeg_direct"

	// StreamModeTGASequence is the fallback raw-frame path.
	StreamModeTGASequence StreamMode = "tga_sequence"
)

// HUDMode controls whether HLAE records a clean stream or the in-game HUD.
type HUDMode string

const (
	// HUDModeGameplay keeps the CS2 HUD, crosshair, killfeed, weapon, ammo,
	// and health visible for a normal gameplay-looking recording.
	HUDModeGameplay HUDMode = "gameplay"

	// HUDModeClean hides the HUD for cinematic/effects-friendly captures.
	HUDModeClean HUDMode = "clean"

	// HUDModeDeathnotices hides the gameplay HUD but keeps kill notices visible.
	HUDModeDeathnotices HUDMode = "deathnotices"
)

// Valid reports whether the value is one of the recording-stage HUD modes.
// Keep this boundary here so CLI plans and orchestrator startup cannot drift.
func (m HUDMode) Valid() bool {
	return m == HUDModeGameplay || m == HUDModeClean || m == HUDModeDeathnotices
}

// StreamConfig describes how HLAE should emit raw recordings.
// PortraitSafeKillfeed requests target-filtered notices for portrait delivery:
// deathnotice-only capture moves them into the safe area, while gameplay
// capture preserves the native HUD layout for editor overlay.
type StreamConfig struct {
	Mode                 StreamMode `json:"mode"`
	HUDMode              HUDMode    `json:"hud_mode,omitempty"`
	FPS                  int        `json:"fps"`
	Width                int        `json:"width"`
	Height               int        `json:"height"`
	CRF                  int        `json:"crf,omitempty"`
	PortraitSafeKillfeed bool       `json:"portrait_safe_killfeed,omitempty"`
	DeathnoticeSafeZoneX float64    `json:"deathnotice_safe_zone_x,omitempty"`
	DeathnoticeSafeZoneY float64    `json:"deathnotice_safe_zone_y,omitempty"`
	DeathnoticeLifetime  float64    `json:"deathnotice_lifetime_seconds,omitempty"`
}

// RuntimeConfig captures HLAE runtime toggles that affect timing.
type RuntimeConfig struct {
	HostTimescale float64 `json:"host_timescale,omitempty"`
	QuitTickPad   int     `json:"quit_tick_pad,omitempty"`
}

// RecordingPlan is the lowest-level input to script generation.
type RecordingPlan struct {
	CaptureContract       string             `json:"capture_contract"`
	KillPlanSchemaVersion string             `json:"killplan_schema_version"`
	DemoPath              string             `json:"demo_path"`
	DemoSHA256            string             `json:"demo_sha256"`
	DemoMap               string             `json:"demo_map,omitempty"`
	DemoDurationTicks     int                `json:"demo_duration_ticks,omitempty"`
	OutputDir             string             `json:"output_dir"`
	TargetSteamID64       string             `json:"target_steamid64"`
	TargetNameInDemo      string             `json:"target_name_in_demo,omitempty"`
	TargetAccountID       uint32             `json:"target_account_id"`
	Tickrate              int                `json:"tickrate"`
	Segments              []RecordingSegment `json:"segments"`
	EditorialSegmentIDs   []string           `json:"editorial_segment_ids,omitempty"`
	Stream                StreamConfig       `json:"stream"`
	Runtime               RuntimeConfig      `json:"runtime"`
}

// RecordingSegment is one HLAE record window.
type RecordingSegment struct {
	ID        string                  `json:"id"`
	Round     int                     `json:"round,omitempty"`
	TickStart int                     `json:"tick_start"`
	TickEnd   int                     `json:"tick_end"`
	Kills     []killplan.Kill         `json:"kills,omitempty"`
	Utility   []killplan.UtilityThrow `json:"utility,omitempty"`
}

// RecordingArtifact is one file discovered after recording.
type RecordingArtifact struct {
	SegmentID       string  `json:"segment_id,omitempty"`
	TakeID          string  `json:"take_id,omitempty"`
	Type            string  `json:"type,omitempty"`
	Role            string  `json:"role,omitempty"`
	Path            string  `json:"path"`
	SizeBytes       int64   `json:"size_bytes"`
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
	FrameCount      int64   `json:"frame_count,omitempty"`
	FrameRate       string  `json:"frame_rate,omitempty"`
	Codec           string  `json:"codec,omitempty"`
	Width           int     `json:"width,omitempty"`
	Height          int     `json:"height,omitempty"`
	SampleRate      int     `json:"sample_rate,omitempty"`
	Channels        int     `json:"channels,omitempty"`
	ProbeError      string  `json:"probe_error,omitempty"`
}

// RecordingResult is the file emitted by zv-recorder after a run.
type RecordingResult struct {
	Plan                    RecordingPlan         `json:"plan"`
	Script                  string                `json:"script"`
	Artifacts               []RecordingArtifact   `json:"artifacts"`
	Performance             *RecordingPerformance `json:"performance,omitempty"`
	CaptureMode             CaptureMode           `json:"capture_mode"`
	CaptureInputFingerprint string                `json:"capture_input_fingerprint"`
	CaptureVerified         bool                  `json:"capture_verified,omitempty"`
	CaptureRevision         string                `json:"capture_revision,omitempty"`
	PublicationPending      bool                  `json:"publication_pending,omitempty"`
	Warnings                []string              `json:"warnings,omitempty"`
	Error                   string                `json:"error,omitempty"`
}

// RecordingPerformance contains monotonic timings for one or more physical
// recorder runs represented by a recording result.
type RecordingPerformance struct {
	Version int                       `json:"version"`
	Runs    []RecordingRunPerformance `json:"runs"`
}

// RecordingRunPerformance contains elapsed timings for one recorder process.
// IncrementalMuxMS can overlap LaunchAndCaptureMS and must not be added to
// BeforeResultWriteMS.
type RecordingRunPerformance struct {
	CaptureSegmentIDs   []string                      `json:"capture_segment_ids"`
	Stream              StreamConfig                  `json:"stream"`
	PrepareMS           int64                         `json:"prepare_ms,omitempty"`
	LaunchAndCaptureMS  int64                         `json:"launch_and_capture_ms,omitempty"`
	IncrementalMuxMS    int64                         `json:"incremental_mux_ms,omitempty"`
	ArtifactProbeMS     int64                         `json:"artifact_probe_ms,omitempty"`
	FinalMuxMS          int64                         `json:"final_mux_ms,omitempty"`
	ValidationMS        int64                         `json:"validation_ms,omitempty"`
	BeforeResultWriteMS int64                         `json:"before_result_write_ms,omitempty"`
	Events              []RecordingPerformanceEvent   `json:"events,omitempty"`
	Segments            []RecordingSegmentPerformance `json:"segments,omitempty"`
}

// RecordingSegmentPerformance combines observed recorder markers with probed
// video metadata. Marker timings describe when zv-recorder observed the
// requested HLAE commands, not exact encoder boundaries.
type RecordingSegmentPerformance struct {
	SegmentID               string  `json:"segment_id"`
	RecordStartObservedMS   int64   `json:"record_start_observed_ms,omitempty"`
	RecordEndObservedMS     int64   `json:"record_end_observed_ms,omitempty"`
	RequestedActiveMS       int64   `json:"requested_active_ms,omitempty"`
	VideoFrameCount         int64   `json:"video_frame_count,omitempty"`
	VideoDurationSeconds    float64 `json:"video_duration_seconds,omitempty"`
	ObservedFramesPerSecond float64 `json:"observed_frames_per_second,omitempty"`
}

// RecordingPerformanceEvent is a recorder marker observed in the CS2 console.
// ElapsedMS is measured when the recorder observes the marker, not when an
// encoded frame reaches disk.
type RecordingPerformanceEvent struct {
	Kind         string `json:"kind"`
	SegmentID    string `json:"segment_id,omitempty"`
	TargetTick   int    `json:"target_tick,omitempty"`
	ObservedTick int    `json:"observed_tick,omitempty"`
	ElapsedMS    int64  `json:"elapsed_ms"`
}

// DefaultStreamConfig returns the current V1 target recording format.
func DefaultStreamConfig() StreamConfig {
	return StreamConfig{
		Mode:    StreamModeFFmpegDirect,
		HUDMode: HUDModeGameplay,
		FPS:     60,
		Width:   1920,
		Height:  1080,
		CRF:     18,
	}
}

// NewPlanFromKillPlan converts parser output into the local recorder contract.
func NewPlanFromKillPlan(plan killplan.Plan, demoPath, outputDir string, stream StreamConfig) (RecordingPlan, error) {
	accountID, err := AccountIDFromSteamID64(plan.Target.SteamID64)
	if err != nil {
		return RecordingPlan{}, err
	}
	stream = normalizeStreamConfig(stream)
	out := RecordingPlan{
		CaptureContract:       CaptureContractVersion,
		KillPlanSchemaVersion: plan.SchemaVersion,
		DemoPath:              demoPath,
		DemoSHA256:            plan.Demo.SHA256,
		DemoMap:               plan.Demo.Map,
		DemoDurationTicks:     plan.Demo.DurationTicks,
		OutputDir:             outputDir,
		TargetSteamID64:       plan.Target.SteamID64,
		TargetNameInDemo:      plan.Target.NameInDemo,
		TargetAccountID:       accountID,
		Tickrate:              plan.Demo.Tickrate,
		Stream:                stream,
		Runtime: RuntimeConfig{
			QuitTickPad: 200,
		},
	}
	for _, s := range plan.Segments {
		out.EditorialSegmentIDs = append(out.EditorialSegmentIDs, s.ID)
		out.Segments = append(out.Segments, RecordingSegment{
			ID:        s.ID,
			Round:     s.Round,
			TickStart: s.TickStart,
			TickEnd:   s.TickEnd,
			Kills:     s.Kills,
			Utility:   s.Utility,
		})
	}
	// Kill-plan order is editorial: a top-moments selection is deliberately
	// best-first. Capture order is a different contract because one demo
	// playback can only advance safely through non-overlapping windows.
	sort.SliceStable(out.Segments, func(i, j int) bool {
		if out.Segments[i].TickStart != out.Segments[j].TickStart {
			return out.Segments[i].TickStart < out.Segments[j].TickStart
		}
		if out.Segments[i].TickEnd != out.Segments[j].TickEnd {
			return out.Segments[i].TickEnd < out.Segments[j].TickEnd
		}
		return out.Segments[i].ID < out.Segments[j].ID
	})
	if err := out.Validate(); err != nil {
		return RecordingPlan{}, err
	}
	return out, nil
}

// ToKillPlan reconstructs the factual plan embedded in a recording result.
// RecordingPlan intentionally carries every identity and event field needed by
// downstream scoring and rhythm analysis, so resumed render workflows do not
// need to guess or locate a separate parser artifact.
func (p RecordingPlan) ToKillPlan() killplan.Plan {
	out := killplan.NewPlan()
	out.Demo = killplan.Demo{
		Path:          p.DemoPath,
		SHA256:        p.DemoSHA256,
		Map:           p.DemoMap,
		Tickrate:      p.Tickrate,
		DurationTicks: p.DemoDurationTicks,
	}
	out.Target = killplan.Target{
		SteamID64:  p.TargetSteamID64,
		NameInDemo: p.TargetNameInDemo,
	}
	orderedSegments := p.SegmentsInEditorialOrder()
	out.Segments = make([]killplan.Segment, 0, len(orderedSegments))
	for _, segment := range orderedSegments {
		converted := killplan.Segment{
			ID:        segment.ID,
			Round:     segment.Round,
			TickStart: segment.TickStart,
			TickEnd:   segment.TickEnd,
			Kills:     append([]killplan.Kill(nil), segment.Kills...),
			Utility:   append([]killplan.UtilityThrow(nil), segment.Utility...),
		}
		out.Segments = append(out.Segments, converted)
		out.Stats.TotalKillsTarget += len(converted.Kills)
		out.Stats.TotalUtilityTarget += len(converted.Utility)
		for _, utility := range converted.Utility {
			if strings.EqualFold(utility.Type, "smoke") || strings.EqualFold(utility.Type, "smokegrenade") {
				out.Stats.TotalSmokesTarget++
			}
		}
		if converted.TickEnd > converted.TickStart && p.Tickrate > 0 {
			out.Stats.DurationSecondsTotal += float64(converted.TickEnd-converted.TickStart) / float64(p.Tickrate)
		}
	}
	out.Stats.KillsAfterFilters = out.Stats.TotalKillsTarget
	out.Stats.UtilityAfterFilters = out.Stats.TotalUtilityTarget
	out.Stats.SmokesAfterFilters = out.Stats.TotalSmokesTarget
	out.Stats.SegmentsCreated = len(out.Segments)
	return out
}

// SegmentsInEditorialOrder returns a copy ordered for rendering. Capture keeps
// RecordingPlan.Segments chronological so HLAE never seeks backwards.
func (p RecordingPlan) SegmentsInEditorialOrder() []RecordingSegment {
	orderedSegments := append([]RecordingSegment(nil), p.Segments...)
	if len(p.EditorialSegmentIDs) == len(p.Segments) {
		byID := make(map[string]RecordingSegment, len(p.Segments))
		for _, segment := range p.Segments {
			byID[segment.ID] = segment
		}
		orderedSegments = make([]RecordingSegment, 0, len(p.Segments))
		for _, id := range p.EditorialSegmentIDs {
			if segment, ok := byID[id]; ok {
				orderedSegments = append(orderedSegments, segment)
			}
		}
		if len(orderedSegments) != len(p.Segments) {
			orderedSegments = append(orderedSegments[:0], p.Segments...)
		}
	}
	return orderedSegments
}

// AccountIDFromSteamID64 converts a SteamID64 to the 32-bit account id used
// by CS2's spec_player_by_accountid command.
func AccountIDFromSteamID64(raw string) (uint32, error) {
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse steamid64: %w", err)
	}
	if id < steamID64AccountIDBase || id-steamID64AccountIDBase > uint64(^uint32(0)) {
		return 0, fmt.Errorf("steamid64 %q is outside account-id range", raw)
	}
	return uint32(id - steamID64AccountIDBase), nil
}

// Validate rejects plans that would generate ambiguous or unsafe scripts.
func (p RecordingPlan) Validate() error {
	if p.CaptureContract != CaptureContractVersion {
		return fmt.Errorf("capture_contract must be %q", CaptureContractVersion)
	}
	if p.KillPlanSchemaVersion != killplan.SchemaVersion {
		return fmt.Errorf("killplan_schema_version must be %q", killplan.SchemaVersion)
	}
	if p.DemoPath == "" {
		return fmt.Errorf("demo_path is required")
	}
	if len(p.DemoSHA256) != 64 {
		return fmt.Errorf("demo_sha256 must be a 64-character SHA-256")
	}
	for _, c := range p.DemoSHA256 {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return fmt.Errorf("demo_sha256 must be lowercase hexadecimal")
		}
	}
	if p.DemoDurationTicks <= 0 {
		return fmt.Errorf("demo_duration_ticks must be positive")
	}
	if p.OutputDir == "" {
		return fmt.Errorf("output_dir is required")
	}
	if p.TargetAccountID == 0 {
		return fmt.Errorf("target_account_id is required")
	}
	accountID, err := AccountIDFromSteamID64(p.TargetSteamID64)
	if err != nil {
		return fmt.Errorf("target_steamid64: %w", err)
	}
	if p.TargetAccountID != accountID {
		return fmt.Errorf(
			"target_account_id %d does not match target_steamid64 %q (want %d)",
			p.TargetAccountID,
			p.TargetSteamID64,
			accountID,
		)
	}
	if p.Tickrate <= 0 {
		return fmt.Errorf("tickrate must be positive")
	}
	if len(p.Segments) == 0 {
		return fmt.Errorf("at least one segment is required")
	}
	if p.Stream.Mode == "" {
		return fmt.Errorf("stream mode is required")
	}
	if p.Stream.HUDMode != "" && !p.Stream.HUDMode.Valid() {
		return fmt.Errorf("stream hud_mode must be %q, %q, or %q", HUDModeGameplay, HUDModeClean, HUDModeDeathnotices)
	}
	if p.Stream.PortraitSafeKillfeed && p.Stream.HUDMode != HUDModeGameplay && p.Stream.HUDMode != HUDModeDeathnotices {
		return fmt.Errorf("stream portrait_safe_killfeed requires hud_mode %q or %q", HUDModeGameplay, HUDModeDeathnotices)
	}
	if p.Stream.FPS <= 0 || p.Stream.Width <= 0 || p.Stream.Height <= 0 {
		return fmt.Errorf("stream fps, width, and height must be positive")
	}
	if p.Stream.CRF < 1 || p.Stream.CRF > 51 {
		return fmt.Errorf("stream crf must be between 1 and 51")
	}
	if p.Stream.DeathnoticeSafeZoneX < 0 || p.Stream.DeathnoticeSafeZoneX > 1 {
		return fmt.Errorf("stream deathnotice_safe_zone_x must be between 0 and 1")
	}
	if p.Stream.DeathnoticeSafeZoneY < 0 || p.Stream.DeathnoticeSafeZoneY > 1 {
		return fmt.Errorf("stream deathnotice_safe_zone_y must be between 0 and 1")
	}
	if p.Stream.DeathnoticeLifetime < 0 || p.Stream.DeathnoticeLifetime > 10 {
		return fmt.Errorf("stream deathnotice_lifetime_seconds must be between 0 and 10")
	}
	seen := map[string]bool{}
	previousEnd := 0
	for i, s := range p.Segments {
		if s.ID == "" {
			return fmt.Errorf("segments[%d].id is required", i)
		}
		if err := artifacts.ValidateArtifactToken(fmt.Sprintf("segments[%d].id", i), s.ID); err != nil {
			return err
		}
		if seen[s.ID] {
			return fmt.Errorf("duplicate segment id %q", s.ID)
		}
		seen[s.ID] = true
		if s.TickStart <= 0 {
			return fmt.Errorf("segment %s tick_start must be positive", s.ID)
		}
		if s.TickEnd <= s.TickStart {
			return fmt.Errorf("segment %s tick_end must be greater than tick_start", s.ID)
		}
		recordStart := EffectiveRecordStartTick(s, p.Tickrate)
		if i > 0 && recordStart < previousEnd {
			return fmt.Errorf("segment %s capture window overlaps or is out of order", s.ID)
		}
		if s.TickEnd > p.DemoDurationTicks {
			return fmt.Errorf("segment %s tick_end %d exceeds demo duration %d", s.ID, s.TickEnd, p.DemoDurationTicks)
		}
		for j, kill := range s.Kills {
			if kill.Tick < s.TickStart || kill.Tick > s.TickEnd {
				return fmt.Errorf("segment %s kills[%d] tick %d is outside segment bounds", s.ID, j, kill.Tick)
			}
		}
		for j, utility := range s.Utility {
			if utility.ThrowTick < s.TickStart || utility.ThrowTick > s.TickEnd {
				return fmt.Errorf("segment %s utility[%d] throw_tick %d is outside segment bounds", s.ID, j, utility.ThrowTick)
			}
		}
		previousEnd = s.TickEnd
	}
	if len(p.EditorialSegmentIDs) > 0 {
		if len(p.EditorialSegmentIDs) != len(p.Segments) {
			return fmt.Errorf("editorial_segment_ids must name every capture segment exactly once")
		}
		editorialSeen := make(map[string]bool, len(p.EditorialSegmentIDs))
		for i, id := range p.EditorialSegmentIDs {
			if !seen[id] {
				return fmt.Errorf("editorial_segment_ids[%d] names unknown segment %q", i, id)
			}
			if editorialSeen[id] {
				return fmt.Errorf("editorial_segment_ids contains duplicate segment %q", id)
			}
			editorialSeen[id] = true
		}
	}
	return nil
}

func normalizeStreamConfig(stream StreamConfig) StreamConfig {
	defaults := DefaultStreamConfig()
	if stream.Mode == "" {
		return defaults
	}
	if stream.HUDMode == "" {
		stream.HUDMode = defaults.HUDMode
	}
	if stream.FPS == 0 {
		stream.FPS = defaults.FPS
	}
	if stream.Width == 0 {
		stream.Width = defaults.Width
	}
	if stream.Height == 0 {
		stream.Height = defaults.Height
	}
	if stream.CRF == 0 {
		stream.CRF = defaults.CRF
	}
	if stream.PortraitSafeKillfeed && stream.HUDMode == HUDModeDeathnotices {
		if stream.DeathnoticeSafeZoneX == 0 {
			stream.DeathnoticeSafeZoneX = defaultDeathnoticeSafeZoneX
		}
		if stream.DeathnoticeSafeZoneY == 0 {
			stream.DeathnoticeSafeZoneY = defaultDeathnoticeSafeZoneY
		}
	}
	if stream.HUDMode == HUDModeDeathnotices || stream.PortraitSafeKillfeed {
		if stream.DeathnoticeLifetime == 0 {
			stream.DeathnoticeLifetime = defaultDeathnoticeLifetimeSeconds
		}
	}
	return stream
}

// SegmentOutputPath returns the preferred output file path for a segment.
func (p RecordingPlan) SegmentOutputPath(segmentID string) string {
	ext := ".mp4"
	if p.Stream.Mode == StreamModeTGASequence {
		ext = ""
	}
	return filepath.Join(p.OutputDir, segmentID+ext)
}
