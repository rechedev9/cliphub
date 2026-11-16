// Package brandkit holds the creator's saved brand kit: the channel logo drawn
// as a watermark in one corner, and an own sponsor banner shown for a window.
package brandkit

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	CornerTopLeft     = "top-left"
	CornerTopRight    = "top-right"
	CornerBottomLeft  = "bottom-left"
	CornerBottomRight = "bottom-right"

	DefaultCorner  = CornerTopRight
	DefaultOpacity = 0.8
	MinOpacity     = 0.1
	// MaxBannerSeconds bounds a banner window; a Full Demo program stays under 12 hours.
	MaxBannerSeconds = 43200
)

// Corners are the watermark positions, in the order the UI offers them.
var Corners = []string{CornerTopLeft, CornerTopRight, CornerBottomLeft, CornerBottomRight}

var ErrMissing = errors.New("brand kit image is missing")

// ImageRef names an immutable uploaded overlay image (internal/overlayassets).
type ImageRef struct {
	ID     string `json:"id"`
	SHA256 string `json:"sha256"`
}

// Watermark is the logo drawn in Corner for the whole video.
type Watermark struct {
	Image   ImageRef `json:"image"`
	Corner  string   `json:"corner"`
	Opacity float64  `json:"opacity"`
}

// Banner is the sponsor image shown from StartSeconds; EndSeconds 0 keeps it to the end.
type Banner struct {
	Image        ImageRef `json:"image"`
	StartSeconds float64  `json:"start_seconds"`
	EndSeconds   float64  `json:"end_seconds,omitempty"`
}

// Kit is the one saved brand kit of this Studio. A nil image is not set.
type Kit struct {
	Logo               *ImageRef `json:"logo,omitempty"`
	WatermarkCorner    string    `json:"watermark_corner"`
	WatermarkOpacity   float64   `json:"watermark_opacity"`
	Banner             *ImageRef `json:"banner,omitempty"`
	BannerStartSeconds float64   `json:"banner_start_seconds"`
	BannerEndSeconds   float64   `json:"banner_end_seconds,omitempty"`
	UpdatedAt          time.Time `json:"updated_at,omitzero"`
}

// Default is the kit before the user saves one.
func Default() Kit {
	return Kit{WatermarkCorner: DefaultCorner, WatermarkOpacity: DefaultOpacity}
}

// Copy returns a kit that shares no pointers with k.
func (k Kit) Copy() Kit {
	if k.Logo != nil {
		logo := *k.Logo
		k.Logo = &logo
	}
	if k.Banner != nil {
		banner := *k.Banner
		k.Banner = &banner
	}
	return k
}

// Watermark is the kit's watermark, or ErrMissing without a logo.
func (k Kit) Watermark() (Watermark, error) {
	if k.Logo == nil {
		return Watermark{}, fmt.Errorf("%w: upload a channel logo first", ErrMissing)
	}
	return Watermark{Image: *k.Logo, Corner: k.WatermarkCorner, Opacity: k.WatermarkOpacity}, nil
}

// SponsorBanner is the kit's banner, or ErrMissing without a banner image.
func (k Kit) SponsorBanner() (Banner, error) {
	if k.Banner == nil {
		return Banner{}, fmt.Errorf("%w: upload a sponsor banner first", ErrMissing)
	}
	return Banner{Image: *k.Banner, StartSeconds: k.BannerStartSeconds, EndSeconds: k.BannerEndSeconds}, nil
}

func (k Kit) Validate() error {
	if k.Logo != nil {
		if err := k.Logo.Validate(); err != nil {
			return fmt.Errorf("logo: %w", err)
		}
	}
	if k.Banner != nil {
		if err := k.Banner.Validate(); err != nil {
			return fmt.Errorf("banner: %w", err)
		}
	}
	if err := ValidateCorner(k.WatermarkCorner); err != nil {
		return err
	}
	if err := ValidateOpacity(k.WatermarkOpacity); err != nil {
		return err
	}
	return ValidateWindow(k.BannerStartSeconds, k.BannerEndSeconds)
}

func (r ImageRef) Validate() error {
	id, err := uuid.Parse(r.ID)
	if err != nil || id == uuid.Nil || id.String() != r.ID {
		return fmt.Errorf("image id must be a lowercase uuid")
	}
	if len(r.SHA256) != 64 || strings.Trim(r.SHA256, "0123456789abcdef") != "" {
		return fmt.Errorf("image sha256 must be 64 lowercase hex characters")
	}
	return nil
}

func (w Watermark) Validate() error {
	if err := w.Image.Validate(); err != nil {
		return fmt.Errorf("watermark %w", err)
	}
	if err := ValidateCorner(w.Corner); err != nil {
		return err
	}
	return ValidateOpacity(w.Opacity)
}

func (b Banner) Validate() error {
	if err := b.Image.Validate(); err != nil {
		return fmt.Errorf("sponsor banner %w", err)
	}
	return ValidateWindow(b.StartSeconds, b.EndSeconds)
}

// Window is the banner's visible span clamped to a video of duration seconds; ok is false when nothing shows.
func (b Banner) Window(duration float64) (float64, float64, bool) {
	end := b.EndSeconds
	if end <= 0 || end > duration {
		end = duration
	}
	return b.StartSeconds, end, b.StartSeconds < end
}

func ValidateCorner(corner string) error {
	for _, c := range Corners {
		if corner == c {
			return nil
		}
	}
	return fmt.Errorf("watermark corner must be one of %s", strings.Join(Corners, ", "))
}

func ValidateOpacity(opacity float64) error {
	if math.IsNaN(opacity) || opacity < MinOpacity || opacity > 1 {
		return fmt.Errorf("watermark opacity must be between %g and 1", MinOpacity)
	}
	return nil
}

func ValidateWindow(start, end float64) error {
	for _, v := range []float64{start, end} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > MaxBannerSeconds {
			return fmt.Errorf("sponsor banner seconds must be between 0 and %d", MaxBannerSeconds)
		}
	}
	if end != 0 && end <= start {
		return fmt.Errorf("sponsor banner end must be after its start")
	}
	return nil
}
