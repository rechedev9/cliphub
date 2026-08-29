// Package timelineplan is the canonical multitrack editor document.
package timelineplan

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when no editor project has the requested id.
var ErrNotFound = errors.New("editor project not found")

const (
	SchemaVersion = "1.0"

	KindVideo TrackKind = "video"
	KindAudio TrackKind = "audio"

	FilterNone  Filter = ""
	FilterGrade Filter = "grade"

	TransitionCut       TransitionKind = "cut"
	TransitionCrossfade TransitionKind = "crossfade"

	StatusDraft     Status = "draft"
	StatusRendering Status = "rendering"
	StatusRendered  Status = "rendered"
	StatusFailed    Status = "failed"

	CanvasPortraitWidth   = 1080
	CanvasPortraitHeight  = 1920
	CanvasLandscapeWidth  = 1920
	CanvasLandscapeHeight = 1080
	DefaultFPS            = 60

	minSpeed           = 0.25
	maxSpeed           = 3
	maxVolume          = 2
	maxFadeSeconds     = 5
	maxTextRunes       = 120
	minFontSize        = 24
	maxFontSize        = 120
	defaultFontSize    = 64
	maxOverlays        = 8
	maxTracks          = 8
	maxItemsPerTrack   = 64
	minVerticalCenterY = 0.025
	maxVerticalCenterY = 0.975
)

// Status is the lifecycle of an editor project.
type Status string

// TrackKind is video (composited, z-order = slice order) or audio-only.
type TrackKind string

// Filter is a closed set of color treatments.
type Filter string

// TransitionKind is how two adjacent items on one track meet.
type TransitionKind string

// Canvas is the output frame.
type Canvas struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	FPS    int `json:"fps"`
}

// Transform places an item as a picture-in-picture rectangle. Coordinates are
// normalized to the canvas. A nil transform means the item fills the canvas.
type Transform struct {
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Width   float64 `json:"width"`
	Height  float64 `json:"height"`
	Opacity float64 `json:"opacity,omitempty"`
}

// Item is one asset occurrence on a track.
type Item struct {
	ID            string     `json:"id"`
	AssetID       string     `json:"asset_id"`
	TimelineStart float64    `json:"timeline_start"`
	SourceIn      float64    `json:"source_in"`
	SourceOut     float64    `json:"source_out"`
	Speed         float64    `json:"speed,omitempty"`
	Volume        *float64   `json:"volume,omitempty"`
	FadeIn        float64    `json:"fade_in,omitempty"`
	FadeOut       float64    `json:"fade_out,omitempty"`
	Transform     *Transform `json:"transform,omitempty"`
	Filter        Filter     `json:"filter,omitempty"`
}

// Track is an ordered stack of items. Video tracks later in the document
// overlay earlier ones.
type Track struct {
	ID    string    `json:"id"`
	Kind  TrackKind `json:"kind"`
	Items []Item    `json:"items"`
}

// TextOverlay burns a centered line onto the output timeline.
type TextOverlay struct {
	ID           string   `json:"id"`
	Text         string   `json:"text"`
	PositionY    float64  `json:"position_y"`
	StartSeconds float64  `json:"start_seconds"`
	EndSeconds   *float64 `json:"end_seconds,omitempty"`
	FontSize     int      `json:"font_size,omitempty"`
}

// Transition describes a join after one item on the same track.
type Transition struct {
	ID        string         `json:"id"`
	Kind      TransitionKind `json:"kind"`
	AfterItem string         `json:"after_item"`
	Duration  float64        `json:"duration,omitempty"`
}

// MusicPlan mixes a catalog track under the timeline audio.
type MusicPlan struct {
	Key    string  `json:"key,omitempty"`
	Volume float64 `json:"volume,omitempty"`
}

// Document is the durable editor contract later stages must honor.
type Document struct {
	SchemaVersion string        `json:"schema_version"`
	Canvas        Canvas        `json:"canvas"`
	Tracks        []Track       `json:"tracks"`
	Overlays      []TextOverlay `json:"overlays,omitempty"`
	Transitions   []Transition  `json:"transitions,omitempty"`
	Music         MusicPlan     `json:"music,omitzero"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// Project is one editor session: a persisted document plus render status.
type Project struct {
	ID            uuid.UUID `json:"id"`
	Title         string    `json:"title"`
	Status        Status    `json:"status"`
	FailureReason string    `json:"failure_reason,omitempty"`
	Plan          []byte    `json:"plan,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// RenderState is the mutable pointer to the latest editor render attempt.
type RenderState struct {
	ProjectID   uuid.UUID `json:"project_id"`
	AttemptID   uuid.UUID `json:"attempt_id,omitempty"`
	Status      Status    `json:"status"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	VideoKey    string    `json:"video_key,omitempty"`
	CoverKey    string    `json:"cover_key,omitempty"`
	ResultKey   string    `json:"result_key,omitempty"`
	Warnings    []string  `json:"warnings,omitempty"`
	Error       string    `json:"error,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// RenderPerformance is a content-free measurement of one timeline render.
type RenderPerformance struct {
	RenderMS                    int64   `json:"render_ms,omitempty"`
	CoverMS                     int64   `json:"cover_ms,omitempty"`
	QualityCheckMS              int64   `json:"quality_check_ms,omitempty"`
	OutputBytes                 int64   `json:"output_bytes,omitempty"`
	MediaDurationSeconds        float64 `json:"media_duration_seconds,omitempty"`
	RenderSecondsPerMediaSecond float64 `json:"render_seconds_per_media_second,omitempty"`
}

// RenderResult is the immutable record of one finished render.
type RenderResult struct {
	ProjectID   uuid.UUID         `json:"project_id"`
	AttemptID   uuid.UUID         `json:"attempt_id"`
	Fingerprint string            `json:"fingerprint"`
	VideoKey    string            `json:"video_key"`
	CoverKey    string            `json:"cover_key"`
	Duration    float64           `json:"duration_seconds"`
	Width       int               `json:"width"`
	Height      int               `json:"height"`
	Warnings    []string          `json:"warnings,omitempty"`
	Performance RenderPerformance `json:"performance,omitzero"`
	CreatedAt   time.Time         `json:"created_at"`
}
