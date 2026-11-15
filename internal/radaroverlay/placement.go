package radaroverlay

import (
	"fmt"
	"slices"
)

// Corners the radar takes. An empty position is the format's default corner.
const (
	PositionTopLeft     = "top-left"
	PositionTopRight    = "top-right"
	PositionBottomLeft  = "bottom-left"
	PositionBottomRight = "bottom-right"
)

// Sizes of the radar's side, as a share of the frame's short side. An empty size is SizeMedium.
const (
	SizeSmall  = "small"
	SizeMedium = "medium"
	SizeLarge  = "large"
)

// MinOpacity is the faintest radar; DefaultOpacity is used when none is chosen.
const (
	MinOpacity     = 0.4
	DefaultOpacity = 0.9
)

// Positions and Sizes are the values a render request takes.
var (
	Positions = []string{PositionTopLeft, PositionTopRight, PositionBottomLeft, PositionBottomRight}
	Sizes     = []string{SizeSmall, SizeMedium, SizeLarge}
)

var sizeShare = map[string]float64{SizeSmall: 0.26, SizeMedium: 0.32, SizeLarge: 0.40}

// ValidatePosition accepts a corner or the empty default.
func ValidatePosition(position string) error {
	if position == "" || slices.Contains(Positions, position) {
		return nil
	}
	return fmt.Errorf("radar position must be top-left, top-right, bottom-left or bottom-right")
}

// ValidateSize accepts a size or the empty default.
func ValidateSize(size string) error {
	if size == "" || slices.Contains(Sizes, size) {
		return nil
	}
	return fmt.Errorf("radar size must be small, medium or large")
}

// ValidateOpacity accepts 0 (the default) or MinOpacity to 1.
func ValidateOpacity(opacity float64) error {
	if opacity == 0 || (opacity >= MinOpacity && opacity <= 1) {
		return nil
	}
	return fmt.Errorf("radar opacity must be from %.1f to 1", MinOpacity)
}

// EffectiveOpacity is the opacity drawn: the default when none was chosen.
func EffectiveOpacity(opacity float64) float64 {
	if opacity == 0 {
		return DefaultOpacity
	}
	return opacity
}

// Rect is a box on the output frame, in pixels.
type Rect struct {
	X, Y, W, H int
}

func (r Rect) overlaps(o Rect) bool {
	return r.X < o.X+o.W && o.X < r.X+r.W && r.Y < o.Y+o.H && o.Y < r.Y+r.H
}

// Placement is where the radar goes on a Width x Height frame. Avoid holds the boxes already on the
// frame, such as the brand watermark or the sponsor banner, which the radar stacks clear of.
type Placement struct {
	Position, Size string
	Width, Height  int
	Avoid          []Rect
}

// Place sizes the radar and puts it in its corner. A portrait frame keeps the top corners under the
// hook title and the vertical HUD (y=300 on 1920) and the bottom corners above the Shorts caption band.
func Place(p Placement) Rect {
	short := min(p.Width, p.Height)
	share, ok := sizeShare[p.Size]
	if !ok {
		share = sizeShare[SizeMedium]
	}
	side := int(float64(short)*share) &^ 1
	margin := short * 4 / 100
	top, bottom := margin, p.Height-margin
	position := p.Position
	if p.Height > p.Width {
		top, bottom = 300*p.Height/1920, p.Height-p.Height*14/100
		if position == "" {
			position = PositionBottomLeft
		}
	} else if position == "" {
		position = PositionTopLeft
	}
	r := Rect{X: margin, Y: top, W: side, H: side}
	if position == PositionTopRight || position == PositionBottomRight {
		r.X = p.Width - side - margin
	}
	atBottom := position == PositionBottomLeft || position == PositionBottomRight
	if atBottom {
		r.Y = bottom - side
	}
	// Each move goes one way (up from a bottom corner, down from a top one), so passes converge.
	for range len(p.Avoid) {
		for _, a := range p.Avoid {
			if !r.overlaps(a) {
				continue
			}
			if atBottom {
				r.Y = a.Y - margin/2 - side
			} else {
				r.Y = a.Y + a.H + margin/2
			}
		}
	}
	r.Y = max(0, min(r.Y, p.Height-side))
	return r
}
