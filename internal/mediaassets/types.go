// Package mediaassets owns reusable editor media files and their probes.
package mediaassets

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when no asset has the requested id.
var ErrNotFound = errors.New("editor asset not found")

const (
	OriginUpload       Origin = "upload"
	OriginDemoRender   Origin = "demo_render"
	OriginStreamRender Origin = "stream_render"
)

var fileNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._ -]{0,127}$`)

// Origin is how an asset entered the library.
type Origin string

// Probe is the ffprobe summary persisted with an asset.
type Probe struct {
	Width           int     `json:"width,omitempty"`
	Height          int     `json:"height,omitempty"`
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
	VideoCodec      string  `json:"video_codec,omitempty"`
	AudioCodec      string  `json:"audio_codec,omitempty"`
	FrameRate       string  `json:"frame_rate,omitempty"`
	HasAudio        bool    `json:"has_audio,omitempty"`
}

// Asset is one reusable media file the editor can place on a timeline.
type Asset struct {
	ID            uuid.UUID  `json:"id"`
	SHA256        string     `json:"sha256"`
	FileName      string     `json:"file_name"`
	Origin        Origin     `json:"origin"`
	OriginJobID   *uuid.UUID `json:"origin_job_id,omitempty"`
	OriginVariant string     `json:"origin_variant,omitempty"`
	OriginName    string     `json:"origin_name,omitempty"`
	Probe         Probe      `json:"probe"`
	MediaKey      string     `json:"media_key"`
	CreatedAt     time.Time  `json:"created_at"`
}

func (o Origin) Validate() error {
	switch o {
	case OriginUpload, OriginDemoRender, OriginStreamRender:
		return nil
	default:
		return fmt.Errorf("unknown asset origin %q", o)
	}
}

func SanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-', r == ' ':
			b.WriteRune(r)
		}
	}
	cleaned := strings.Trim(strings.TrimSpace(b.String()), ".")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" || !fileNamePattern.MatchString(cleaned) {
		return "clip.mp4"
	}
	return cleaned
}

func (a Asset) Validate() error {
	if a.ID == uuid.Nil {
		return fmt.Errorf("asset id is required")
	}
	if !sha256HexPattern.MatchString(a.SHA256) {
		return fmt.Errorf("asset sha256 must be a hex digest")
	}
	if !fileNamePattern.MatchString(strings.TrimSpace(a.FileName)) {
		return fmt.Errorf("invalid asset file name %q", a.FileName)
	}
	if err := a.Origin.Validate(); err != nil {
		return err
	}
	if a.MediaKey == "" {
		return fmt.Errorf("asset media key is required")
	}
	if a.Probe.DurationSeconds < 0 {
		return fmt.Errorf("asset duration must be >= 0")
	}
	return nil
}

var sha256HexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// AssetPrefix is the storage directory that holds everything for one asset.
func AssetPrefix(id uuid.UUID) string {
	return path.Join("editor-assets", id.String())
}

func MediaKey(id uuid.UUID) string {
	return path.Join(AssetPrefix(id), "media.mp4")
}
