// Package streamclips defines local streamer-MP4 clip jobs and render plans.
package streamclips

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/keydropbanner"
)

// ErrNotFound is returned when no stream job has the requested id.
var ErrNotFound = errors.New("stream job not found")

const (
	VariantStreamerVerticalStack = "streamer-vertical-stack"

	StatusAcquiring Status = "acquiring"
	StatusUploaded  Status = "uploaded"
	StatusReady     Status = "ready"
	StatusRendering Status = "rendering"
	StatusRendered  Status = "rendered"
	StatusFailed    Status = "failed"
)

// RenderErrorCodeSuperseded marks an admitted render whose immutable plan
// changed before it could commit. The job remains editable and can be
// rendered again.
const RenderErrorCodeSuperseded = "render_superseded"

var clipIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// Twitch-compatible streamer names keep the value safe to embed directly in
// FFmpeg's drawtext filter while covering the handles the banner is designed
// for. Twitch usernames are at most 25 ASCII letters, digits, or underscores.
var streamerNickPattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,25}$`)

type Status string

func ParseStatus(value string) (Status, error) {
	switch Status(value) {
	case StatusAcquiring, StatusUploaded, StatusReady, StatusRendering, StatusRendered, StatusFailed:
		return Status(value), nil
	default:
		return "", fmt.Errorf("unknown stream job status %q", value)
	}
}

type Job struct {
	ID            uuid.UUID `json:"id"`
	Status        Status    `json:"status"`
	FailureReason string    `json:"failure_reason,omitempty"`
	// FailureCode is the stable obs class for a terminal failure. FailureReason
	// stays the user-facing text (including Spanish acquire reasons).
	FailureCode  string `json:"failure_code,omitempty"`
	SourcePath   string `json:"source_path"`
	SourceSHA256 string `json:"source_sha256"`
	// SourceURL is the private, short-lived acquisition URL used by the worker.
	// It may contain provider query material and is never serialized. Durable
	// repositories clear it on acquisition success or terminal failure.
	SourceURL string `json:"-"`
	// PublicSourceURL is the credential-free provider URL returned by APIs and
	// retained after the private acquisition URL has been cleared.
	PublicSourceURL string          `json:"source_url,omitempty"`
	Title           string          `json:"title,omitempty"`
	Probe           SourceProbe     `json:"probe"`
	EditPlan        json.RawMessage `json:"edit_plan,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type SourceProbe struct {
	Width           int     `json:"width,omitempty"`
	Height          int     `json:"height,omitempty"`
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
	VideoCodec      string  `json:"video_codec,omitempty"`
	AudioCodec      string  `json:"audio_codec,omitempty"`
	FrameRate       string  `json:"frame_rate,omitempty"`
	// VideoTimeBase is the selected video stream's ffprobe time_base (for
	// example "1/30000"). StartTimeSeconds is format.start_time: the container
	// timestamp represented by source/UI time zero. Frame-aware analyzers map a
	// video PTS to that timeline as PTS*time_base-StartTimeSeconds.
	//
	// VideoStartTimeSeconds is the selected video stream's stream.start_time.
	// It can be later than StartTimeSeconds when, for example, audio begins
	// before the first video frame; it is retained as source metadata and must
	// not replace the container timeline origin.
	VideoTimeBase         string   `json:"video_time_base,omitempty"`
	StartTimeSeconds      float64  `json:"start_time_seconds,omitempty"`
	VideoStartTimeSeconds float64  `json:"video_start_time_seconds,omitempty"`
	Warnings              []string `json:"warnings,omitempty"`
}

type CropRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type ClipRange struct {
	ID           string    `json:"id"`
	StartSeconds float64   `json:"start_seconds"`
	EndSeconds   float64   `json:"end_seconds"`
	Title        string    `json:"title,omitempty"`
	Edit         *ClipEdit `json:"edit,omitempty"`
}

// Clip edit limits. Speed stays within what chained atempo filters reproduce
// faithfully; fades and overlay text are bounded so a plan can never produce a
// degenerate render.
const (
	minClipSpeed           = 0.25
	maxClipSpeed           = 3
	maxSourceVolume        = 2
	maxClipFadeSeconds     = 5
	maxTextOverlaysPerClip = 4
	maxTextOverlayRunes    = 120
	minOverlayFontSize     = 24
	maxOverlayFontSize     = 120
	defaultOverlayFontSize = 64
	// Vertical center bounds shared by the streamer banner and text overlays:
	// the drag-handle margin that keeps either strip fully inside the frame.
	minVerticalPositionY = 0.025
	maxVerticalPositionY = 0.975
)

// ClipEdit carries the optional per-clip edit options: playback speed, the
// original-audio gain, fades, and burned-in text overlays. A nil or zero value
// renders the clip exactly as before the edit options existed.
type ClipEdit struct {
	// Speed is the playback rate in [0.25, 3]; 0 means unchanged (1x).
	Speed float64 `json:"speed,omitempty"`
	// SourceVolume scales the clip's original audio in [0, 2]; nil means
	// unchanged and 0 mutes the source (music, if any, still plays).
	SourceVolume *float64 `json:"source_volume,omitempty"`
	// FadeInSeconds / FadeOutSeconds fade video and audio at the clip
	// boundaries, measured in output (post-speed) seconds.
	FadeInSeconds  float64       `json:"fade_in_seconds,omitempty"`
	FadeOutSeconds float64       `json:"fade_out_seconds,omitempty"`
	TextOverlays   []TextOverlay `json:"text_overlays,omitempty"`
}

// TextOverlay burns a centered text line into the rendered clip. Times are
// relative to the clip start in source seconds (the same timeline the web
// preview scrubs); nil bounds extend to the clip edge.
type TextOverlay struct {
	Text string `json:"text"`
	// PositionY is the normalized vertical center in [0.025, 0.975].
	PositionY    float64  `json:"position_y"`
	StartSeconds *float64 `json:"start_seconds,omitempty"`
	EndSeconds   *float64 `json:"end_seconds,omitempty"`
	// FontSize in output pixels, [24, 120]; 0 means the default 64.
	FontSize int `json:"font_size,omitempty"`
}

type EditPlan struct {
	SchemaVersion    string             `json:"schema_version"`
	Variant          string             `json:"variant"`
	FaceCrop         CropRect           `json:"face_crop"`
	FaceCropReviewed bool               `json:"face_crop_reviewed,omitempty"`
	GameplayCrop     CropRect           `json:"gameplay_crop"`
	Clips            []ClipRange        `json:"clips"`
	StreamerBanner   StreamerBannerPlan `json:"streamer_banner,omitzero"`
	KeyDropBanner    KeyDropBannerPlan  `json:"keydrop_banner,omitzero"`
	Music            MusicPlan          `json:"music,omitzero"`
	Effects          EffectsPlan        `json:"effects,omitzero"`
	UpdatedAt        time.Time          `json:"updated_at"`
}

const EditPlanSchemaVersion = "1.1"

const (
	StreamerBannerPlatformTwitch = "twitch"
	StreamerBannerPlatformKick   = "kick"
)

// StreamerBannerPlan adds an optional branded separator to the rendered
// vertical clip. An empty Nick keeps the render visually unchanged.
// Platform selects Twitch or Kick chrome; empty means Twitch.
type StreamerBannerPlan struct {
	Nick         string   `json:"nick,omitempty"`
	Platform     string   `json:"platform,omitempty"`
	PositionY    *float64 `json:"position_y,omitempty"`
	SlideEnabled bool     `json:"slide_enabled,omitempty"`
}

func (p StreamerBannerPlan) ResolvedPlatform() string {
	if strings.EqualFold(strings.TrimSpace(p.Platform), StreamerBannerPlatformKick) {
		return StreamerBannerPlatformKick
	}
	return StreamerBannerPlatformTwitch
}

// KeyDropBannerPlan overlays the optional affiliate sponsor plate. An empty
// Style keeps the render visually unchanged; Code defaults to ZACKCSGO when
// the style is set and the code is blank. Family selects the plate catalog
// (KEYDROP, CSGOSKINS). Empty family with a style still means KEYDROP.
//
// StartSeconds / EndSeconds are relative to each clip's start on the source
// timeline (same origin as text overlays). Nil means the clip edge: unset
// start = 0, unset end = full clip duration. A short window (e.g. 0–4s) is
// the intended product default so the plate does not sit on the whole clip.
type KeyDropBannerPlan struct {
	// Family is a keydropbanner family id (KEYDROP, CSGOSKINS). Empty with a
	// style still means KEYDROP so older plans keep rendering.
	Family string `json:"family,omitempty"`
	// Style is a plate id in Family (operator, classic, tigerr, jcorko); empty disables the banner.
	Style        string   `json:"style,omitempty"`
	Code         string   `json:"code,omitempty"`
	PositionY    *float64 `json:"position_y,omitempty"`
	SlideEnabled bool     `json:"slide_enabled,omitempty"`
	// StartSeconds is when the plate becomes visible within each clip.
	StartSeconds *float64 `json:"start_seconds,omitempty"`
	// EndSeconds is when the plate disappears within each clip.
	EndSeconds *float64 `json:"end_seconds,omitempty"`
}

// Enabled reports whether the plan requests a KeyDrop plate.
func (p KeyDropBannerPlan) Enabled() bool {
	return strings.TrimSpace(p.Style) != ""
}

// defaultMusicVolume is the music gain mixed under the clip's original audio
// when the plan selects a track without an explicit volume: loud enough to
// carry the edit, quiet enough that the streamer stays intelligible.
const defaultMusicVolume = 0.25

// musicKeyPattern matches a music catalog track id (same shape the songs API
// serves); it doubles as path-traversal defence since a valid key can never
// contain a separator or "..".
var musicKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// MusicPlan mixes a catalog track from the orchestrator's music dir under the
// clip's original audio. Key is the track id ("concrete-teeth"); empty means
// no music. Volume is the music gain in (0,1]; 0 means the default.
type MusicPlan struct {
	Key    string  `json:"key,omitempty"`
	Volume float64 `json:"volume,omitempty"`
}

// EffectsPlan opts a render into light, deterministic post effects. Grade
// applies the mild contrast/saturation lift used across ClipHub's viral
// presets; heavier looks are deliberately not offered.
type EffectsPlan struct {
	Grade bool `json:"grade,omitempty"`
}

type RenderState struct {
	JobID   uuid.UUID `json:"job_id"`
	Variant string    `json:"variant"`
	// AttemptID owns the mutable attempt status. It prevents an older queued
	// task from completing or failing a newer attempt for the same variant.
	AttemptID uuid.UUID `json:"attempt_id,omitempty"`
	Status    Status    `json:"status"`
	// Published means ResultKey, GalleryKey, ArtifactDir, and Videos identify
	// the last fully committed render. Those pointers remain valid while a
	// newer attempt is rendering or has failed.
	Published   bool            `json:"published,omitempty"`
	ResultKey   string          `json:"result_key"`
	GalleryKey  string          `json:"gallery_key"`
	ArtifactDir string          `json:"artifact_dir"`
	Warnings    []string        `json:"warnings,omitempty"`
	Error       string          `json:"error,omitempty"`
	ErrorCode   string          `json:"error_code,omitempty"`
	UpdatedAt   time.Time       `json:"updated_at"`
	Videos      []VideoEntry    `json:"videos,omitempty"`
	Delivery    []DeliveryEntry `json:"delivery,omitempty"`
}

// DeliveryEntry is one upload-ready asset inside shortslistosparasubir.
type DeliveryEntry struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Key  string `json:"key"`
}

type RenderResult struct {
	SchemaVersion string       `json:"schema_version"`
	JobID         uuid.UUID    `json:"job_id"`
	Variant       string       `json:"variant"`
	Clips         []VideoEntry `json:"clips"`
	Warnings      []string     `json:"warnings,omitempty"`
	Error         string       `json:"error,omitempty"`
	RenderedAt    time.Time    `json:"rendered_at"`
}

// VideoPerformance is a content-free local measurement of one stream encode.
// It is persisted with the render result so real jobs can be compared without
// collecting host identity, media names, or source URLs.
type VideoPerformance struct {
	RenderMS                    int64   `json:"render_ms,omitempty"`
	OutputBytes                 int64   `json:"output_bytes,omitempty"`
	MediaDurationSeconds        float64 `json:"media_duration_seconds,omitempty"`
	RenderSecondsPerMediaSecond float64 `json:"render_seconds_per_media_second,omitempty"`
}

type VideoEntry struct {
	ClipID          string            `json:"clip_id"`
	Title           string            `json:"title,omitempty"`
	Key             string            `json:"key"`
	DurationSeconds float64           `json:"duration_seconds,omitempty"`
	Performance     *VideoPerformance `json:"performance,omitempty"`
}

func NewVideoEntry(clip ClipRange, key string) VideoEntry {
	return VideoEntry{
		ClipID:          clip.ID,
		Title:           clip.Title,
		Key:             key,
		DurationSeconds: clip.OutputDurationSeconds(),
	}
}

func NewVideoPerformance(elapsed time.Duration, mediaSeconds float64, outputBytes int64) *VideoPerformance {
	performance := &VideoPerformance{
		RenderMS:             elapsed.Milliseconds(),
		OutputBytes:          outputBytes,
		MediaDurationSeconds: mediaSeconds,
	}
	if elapsed > 0 && mediaSeconds > 0 {
		performance.RenderSecondsPerMediaSecond = elapsed.Seconds() / mediaSeconds
	}
	return performance
}

func NewRenderResult(id uuid.UUID, variant string, videos []VideoEntry, renderedAt time.Time) (RenderResult, error) {
	if _, err := RenderPrefix(id, variant); err != nil {
		return RenderResult{}, err
	}
	if renderedAt.IsZero() {
		renderedAt = time.Now()
	}
	clips := append([]VideoEntry(nil), videos...)
	for i := range clips {
		if clips[i].Performance == nil {
			continue
		}
		performance := *clips[i].Performance
		clips[i].Performance = &performance
	}
	return RenderResult{
		SchemaVersion: "1.0",
		JobID:         id,
		Variant:       variant,
		Clips:         clips,
		RenderedAt:    renderedAt.UTC(),
	}, nil
}

func NewRenderState(id uuid.UUID, variant string, status Status, warnings []string, errMsg string, videos []VideoEntry) (RenderState, error) {
	resultKey, err := RenderResultKey(id, variant)
	if err != nil {
		return RenderState{}, err
	}
	galleryKey, err := RenderGalleryKey(id, variant)
	if err != nil {
		return RenderState{}, err
	}
	prefix, err := RenderPrefix(id, variant)
	if err != nil {
		return RenderState{}, err
	}
	return RenderState{
		JobID:       id,
		Variant:     variant,
		Status:      status,
		Published:   status == StatusRendered,
		ResultKey:   resultKey,
		GalleryKey:  galleryKey,
		ArtifactDir: prefix,
		Warnings:    append([]string(nil), warnings...),
		Error:       errMsg,
		Videos:      append([]VideoEntry(nil), videos...),
		UpdatedAt:   time.Now().UTC(),
	}, nil
}

// HasPublishedRender reports whether the state carries an active completed
// render. StatusRendered is accepted for compatibility with states written
// before Published was added.
func (s RenderState) HasPublishedRender() bool {
	return s.Published || s.Status == StatusRendered
}

// PreservePublishedRender copies only the immutable active-revision pointer
// from previous. Attempt status, warnings, and errors stay owned by the new
// state.
func (s *RenderState) PreservePublishedRender(previous RenderState) {
	if s == nil || !previous.HasPublishedRender() {
		return
	}
	s.Published = true
	s.ResultKey = previous.ResultKey
	s.GalleryKey = previous.GalleryKey
	s.ArtifactDir = previous.ArtifactDir
	s.Videos = append([]VideoEntry(nil), previous.Videos...)
	s.Delivery = append([]DeliveryEntry(nil), previous.Delivery...)
}

func DefaultEditPlan() EditPlan {
	variant := DefaultVariant()
	return EditPlan{
		SchemaVersion: EditPlanSchemaVersion,
		Variant:       variant.Name,
		FaceCrop:      variant.DefaultFaceCrop,
		GameplayCrop:  variant.DefaultGameplayCrop,
		Clips:         []ClipRange{},
		UpdatedAt:     time.Now().UTC(),
	}
}

func (p EditPlan) Validate() error {
	if p.SchemaVersion != "" && p.SchemaVersion != EditPlanSchemaVersion {
		return fmt.Errorf("schema_version must be %q", EditPlanSchemaVersion)
	}
	if p.Variant == "" {
		return fmt.Errorf("variant is required")
	}
	layout, ok := VariantByName(p.Variant)
	if !ok {
		return unknownVariantError(p.Variant)
	}
	if !layout.FullFrame {
		if err := p.FaceCrop.Validate("face_crop"); err != nil {
			return err
		}
	}
	if err := p.GameplayCrop.Validate("gameplay_crop"); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, clip := range p.Clips {
		if err := clip.Validate(); err != nil {
			return err
		}
		if seen[clip.ID] {
			return fmt.Errorf("duplicate clip id %q", clip.ID)
		}
		seen[clip.ID] = true
	}
	if p.Music.Key != "" && !musicKeyPattern.MatchString(p.Music.Key) {
		return fmt.Errorf("invalid music key %q", p.Music.Key)
	}
	if p.Music.Volume < 0 || p.Music.Volume > 1 {
		return fmt.Errorf("music volume must be between 0 and 1")
	}
	if p.StreamerBanner.Nick != "" && !streamerNickPattern.MatchString(p.StreamerBanner.Nick) {
		return fmt.Errorf("streamer banner nick must use 1-25 letters, numbers, or underscores")
	}
	switch strings.ToLower(strings.TrimSpace(p.StreamerBanner.Platform)) {
	case "", StreamerBannerPlatformTwitch, StreamerBannerPlatformKick:
	default:
		return fmt.Errorf("streamer banner platform must be twitch or kick")
	}
	if positionY := p.StreamerBanner.PositionY; positionY != nil {
		if math.IsNaN(*positionY) || math.IsInf(*positionY, 0) || *positionY < minVerticalPositionY || *positionY > maxVerticalPositionY {
			return fmt.Errorf("streamer banner position_y must be finite and between 0.025 and 0.975")
		}
	}
	if err := validateKeyDropBanner(p.KeyDropBanner); err != nil {
		return err
	}
	return nil
}

func validateKeyDropBanner(banner KeyDropBannerPlan) error {
	if err := keydropbanner.ValidateFamily(banner.Family); err != nil {
		return err
	}
	if err := keydropbanner.ValidateStyle(banner.Family, banner.Style); err != nil {
		return err
	}
	if err := keydropbanner.ValidateCode(banner.Code); err != nil {
		return err
	}
	if positionY := banner.PositionY; positionY != nil {
		if math.IsNaN(*positionY) || math.IsInf(*positionY, 0) || *positionY < minVerticalPositionY || *positionY > maxVerticalPositionY {
			return fmt.Errorf("keydrop banner position_y must be finite and between 0.025 and 0.975")
		}
	}
	if s := banner.StartSeconds; s != nil {
		if math.IsNaN(*s) || math.IsInf(*s, 0) || *s < 0 {
			return fmt.Errorf("keydrop banner start_seconds must be finite and >= 0")
		}
	}
	if e := banner.EndSeconds; e != nil {
		if math.IsNaN(*e) || math.IsInf(*e, 0) || *e <= 0 {
			return fmt.Errorf("keydrop banner end_seconds must be finite and > 0")
		}
	}
	if banner.StartSeconds != nil && banner.EndSeconds != nil && *banner.EndSeconds <= *banner.StartSeconds {
		return fmt.Errorf("keydrop banner end_seconds must be greater than start_seconds")
	}
	return nil
}

// ValidateForRender applies source-duration validation and requires at least
// one clip. Empty plans remain valid editing drafts, but are never renderable.
func (p EditPlan) ValidateForRender(durationSeconds float64) error {
	if err := p.ValidateForSourceDuration(durationSeconds); err != nil {
		return err
	}
	if len(p.Clips) == 0 {
		return fmt.Errorf("edit plan has no clips")
	}
	return nil
}

// ValidateForSourceDuration validates the edit plan and also proves every clip
// range fits inside the probed source media. A zero duration means the source
// has not been probed and preserves the structural-only validation behavior.
func (p EditPlan) ValidateForSourceDuration(durationSeconds float64) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if math.IsNaN(durationSeconds) || math.IsInf(durationSeconds, 0) || durationSeconds < 0 {
		return fmt.Errorf("source duration must be finite and >= 0")
	}
	if durationSeconds == 0 {
		return nil
	}
	const durationToleranceSeconds = 0.001
	for _, clip := range p.Clips {
		if clip.EndSeconds > durationSeconds+durationToleranceSeconds {
			return fmt.Errorf(
				"clip %s end_seconds %.3f exceeds source duration %.3f",
				clip.ID, clip.EndSeconds, durationSeconds,
			)
		}
	}
	return nil
}

const legacyInitialClipEndSeconds = 20.0

// MigrateLegacySourceDuration fits only the historical fixed 20-second clip
// endpoint to a shorter probed source. Older Studio versions persisted that
// default before media duration was loaded, so rejecting it during render
// would strand otherwise valid jobs after an upgrade. Other overruns remain
// untouched and fail ValidateForSourceDuration, preserving strict validation
// for newly submitted or genuinely invalid plans.
func MigrateLegacySourceDuration(plan EditPlan, durationSeconds float64) (EditPlan, bool) {
	const tolerance = 0.001
	if durationSeconds <= 0 || durationSeconds >= legacyInitialClipEndSeconds-tolerance ||
		math.IsNaN(durationSeconds) || math.IsInf(durationSeconds, 0) {
		return plan, false
	}

	plan = NormalizeEditPlan(plan)
	clips := make([]ClipRange, 0, len(plan.Clips))
	changed := false
	for _, clip := range plan.Clips {
		if math.Abs(clip.EndSeconds-legacyInitialClipEndSeconds) > tolerance || clip.EndSeconds <= durationSeconds+tolerance {
			clips = append(clips, clip)
			continue
		}
		changed = true
		if clip.StartSeconds >= durationSeconds {
			continue
		}
		clip.EndSeconds = durationSeconds
		clips = append(clips, clip)
	}
	if !changed {
		return plan, false
	}
	plan.Clips = clips
	return NormalizeEditPlan(plan), true
}

func (c CropRect) Validate(label string) error {
	if math.IsNaN(c.X) || math.IsInf(c.X, 0) ||
		math.IsNaN(c.Y) || math.IsInf(c.Y, 0) ||
		math.IsNaN(c.Width) || math.IsInf(c.Width, 0) ||
		math.IsNaN(c.Height) || math.IsInf(c.Height, 0) {
		return fmt.Errorf("%s must use finite normalized coordinates", label)
	}
	if c.X < 0 || c.Y < 0 || c.Width <= 0 || c.Height <= 0 {
		return fmt.Errorf("%s must use positive normalized coordinates", label)
	}
	if c.X+c.Width > 1 || c.Y+c.Height > 1 {
		return fmt.Errorf("%s must stay within the source frame", label)
	}
	return nil
}

func (c ClipRange) Validate() error {
	if !clipIDPattern.MatchString(c.ID) {
		return fmt.Errorf("invalid clip id %q", c.ID)
	}
	if math.IsNaN(c.StartSeconds) || math.IsInf(c.StartSeconds, 0) {
		return fmt.Errorf("clip %s start_seconds must be finite", c.ID)
	}
	if c.StartSeconds < 0 {
		return fmt.Errorf("clip %s start_seconds must be >= 0", c.ID)
	}
	if math.IsNaN(c.EndSeconds) || math.IsInf(c.EndSeconds, 0) {
		return fmt.Errorf("clip %s end_seconds must be finite", c.ID)
	}
	if c.EndSeconds <= c.StartSeconds {
		return fmt.Errorf("clip %s end_seconds must be greater than start_seconds", c.ID)
	}
	if err := c.Edit.validate(c.ID, c.EndSeconds-c.StartSeconds); err != nil {
		return err
	}
	return nil
}

// speed returns the effective playback rate, treating nil and 0 as 1x.
func (e *ClipEdit) speed() float64 {
	if e == nil || e.Speed == 0 {
		return 1
	}
	return e.Speed
}

// OutputDurationSeconds is the rendered clip length after the speed edit.
func (c ClipRange) OutputDurationSeconds() float64 {
	return (c.EndSeconds - c.StartSeconds) / c.Edit.speed()
}

// EffectiveSpeed is the clip's playback rate with the unset default applied,
// so callers can map source-time positions onto the rendered output timeline.
func (c ClipRange) EffectiveSpeed() float64 {
	return c.Edit.speed()
}

// SourceAudioMuted reports whether the clip edit silences the original audio,
// so the source speech is not present in the rendered output.
func (c ClipRange) SourceAudioMuted() bool {
	return c.Edit != nil && c.Edit.SourceVolume != nil && *c.Edit.SourceVolume == 0
}

// HasTextOverlays reports whether any clip burns text overlays, which decides
// whether the render worker must resolve a font file up front.
func (p EditPlan) HasTextOverlays() bool {
	for _, clip := range p.Clips {
		if clip.Edit != nil && len(clip.Edit.TextOverlays) > 0 {
			return true
		}
	}
	return false
}

func (e *ClipEdit) validate(clipID string, sourceDuration float64) error {
	if e == nil {
		return nil
	}
	if e.Speed != 0 && (math.IsNaN(e.Speed) || e.Speed < minClipSpeed || e.Speed > maxClipSpeed) {
		return fmt.Errorf("clip %s speed must be between 0.25 and 3", clipID)
	}
	if v := e.SourceVolume; v != nil && (math.IsNaN(*v) || *v < 0 || *v > maxSourceVolume) {
		return fmt.Errorf("clip %s source_volume must be between 0 and 2", clipID)
	}
	if math.IsNaN(e.FadeInSeconds) || e.FadeInSeconds < 0 || e.FadeInSeconds > maxClipFadeSeconds {
		return fmt.Errorf("clip %s fade_in_seconds must be between 0 and 5", clipID)
	}
	if math.IsNaN(e.FadeOutSeconds) || e.FadeOutSeconds < 0 || e.FadeOutSeconds > maxClipFadeSeconds {
		return fmt.Errorf("clip %s fade_out_seconds must be between 0 and 5", clipID)
	}
	// Fades run in output time, so they must fit the sped-up duration.
	if e.FadeInSeconds+e.FadeOutSeconds > sourceDuration/e.speed() {
		return fmt.Errorf("clip %s fades must fit within the clip's output duration", clipID)
	}
	if len(e.TextOverlays) > maxTextOverlaysPerClip {
		return fmt.Errorf("clip %s has at most 4 text overlays", clipID)
	}
	for _, overlay := range e.TextOverlays {
		if err := overlay.validate(clipID, sourceDuration); err != nil {
			return err
		}
	}
	return nil
}

func (o TextOverlay) validate(clipID string, clipDuration float64) error {
	text := strings.TrimSpace(o.Text)
	if text == "" {
		return fmt.Errorf("clip %s text overlay text is required", clipID)
	}
	if len([]rune(text)) > maxTextOverlayRunes {
		return fmt.Errorf("clip %s text overlay text must be at most 120 characters", clipID)
	}
	for _, r := range text {
		// The render reads the text from a file with expansion=none, so every
		// printable character is safe; only control characters break layout.
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("clip %s text overlay text must not contain control characters", clipID)
		}
	}
	if math.IsNaN(o.PositionY) || math.IsInf(o.PositionY, 0) || o.PositionY < minVerticalPositionY || o.PositionY > maxVerticalPositionY {
		return fmt.Errorf("clip %s text overlay position_y must be finite and between 0.025 and 0.975", clipID)
	}
	if o.FontSize != 0 && (o.FontSize < minOverlayFontSize || o.FontSize > maxOverlayFontSize) {
		return fmt.Errorf("clip %s text overlay font_size must be between 24 and 120", clipID)
	}
	if s := o.StartSeconds; s != nil && (math.IsNaN(*s) || *s < 0 || *s >= clipDuration) {
		return fmt.Errorf("clip %s text overlay start_seconds must be inside the clip", clipID)
	}
	if e := o.EndSeconds; e != nil && (math.IsNaN(*e) || *e <= 0 || *e > clipDuration) {
		return fmt.Errorf("clip %s text overlay end_seconds must be inside the clip", clipID)
	}
	if o.StartSeconds != nil && o.EndSeconds != nil && *o.EndSeconds <= *o.StartSeconds {
		return fmt.Errorf("clip %s text overlay end_seconds must be greater than start_seconds", clipID)
	}
	return nil
}

func NormalizeEditPlan(plan EditPlan) EditPlan {
	if plan.SchemaVersion == "" || plan.SchemaVersion == "1.0" {
		plan.SchemaVersion = EditPlanSchemaVersion
	}
	if plan.Variant == "" {
		plan.Variant = DefaultVariant().Name
	}
	if plan.UpdatedAt.IsZero() {
		plan.UpdatedAt = time.Now().UTC()
	}
	if len(plan.Clips) > 0 {
		plan.Clips = append([]ClipRange(nil), plan.Clips...)
	}
	for i := range plan.Clips {
		plan.Clips[i] = normalizeClipRange(plan.Clips[i])
		plan.Clips[i].ID = strings.TrimSpace(plan.Clips[i].ID)
	}
	plan.StreamerBanner.Nick = strings.TrimSpace(plan.StreamerBanner.Nick)
	plan.StreamerBanner.Platform = strings.ToLower(strings.TrimSpace(plan.StreamerBanner.Platform))
	plan.KeyDropBanner.Family = keydropbanner.NormalizeFamily(plan.KeyDropBanner.Family)
	plan.KeyDropBanner.Style = strings.ToLower(strings.TrimSpace(plan.KeyDropBanner.Style))
	plan.KeyDropBanner.Code = strings.ToUpper(strings.TrimSpace(plan.KeyDropBanner.Code))
	if plan.KeyDropBanner.Style != "" && plan.KeyDropBanner.Family == "" {
		plan.KeyDropBanner.Family = keydropbanner.FamilyKeyDrop
	}
	plan.Music.Key = strings.TrimSpace(plan.Music.Key)
	if plan.Music.Key == "" {
		plan.Music.Volume = 0
	} else if plan.Music.Volume == 0 {
		plan.Music.Volume = defaultMusicVolume
	}
	return plan
}

func normalizeClipRange(clip ClipRange) ClipRange {
	clip.Edit = normalizeClipEdit(clip.Edit)
	return clip
}

// normalizeClipEdit trims overlay text and collapses an all-defaults edit back
// to nil so untouched clips keep their pre-edit plan shape. It deep-copies so
// the caller's overlays are never mutated.
func normalizeClipEdit(edit *ClipEdit) *ClipEdit {
	if edit == nil {
		return nil
	}
	normalized := *edit
	if len(edit.TextOverlays) > 0 {
		normalized.TextOverlays = make([]TextOverlay, len(edit.TextOverlays))
		for i, overlay := range edit.TextOverlays {
			overlay.Text = strings.TrimSpace(overlay.Text)
			normalized.TextOverlays[i] = overlay
		}
	}
	// Identity values render exactly like unset ones, so collapse them too;
	// this keeps plans saved through any surface shape-identical.
	if normalized.Speed == 1 {
		normalized.Speed = 0
	}
	if normalized.SourceVolume != nil && *normalized.SourceVolume == 1 {
		normalized.SourceVolume = nil
	}
	if normalized.Speed == 0 && normalized.SourceVolume == nil &&
		normalized.FadeInSeconds == 0 && normalized.FadeOutSeconds == 0 &&
		len(normalized.TextOverlays) == 0 {
		return nil
	}
	return &normalized
}
