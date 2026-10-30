package library

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Annotation limits. A note is a coach's short remark; a drawing is a handful
// of strokes on the radar, kept in world units so it lands on the same spot at
// any radar size.
const (
	MaxAnnotationTextRunes   = 2000
	MaxAnnotationStrokes     = 64
	MaxAnnotationStrokePoint = 2000
	MaxAnnotationPoints      = 20000
	// MaxAnnotationList caps one list read, ordered by match, round and tick.
	MaxAnnotationList = 2000
	// maxWorldCoordinate bounds a point well past the largest CS2 map.
	maxWorldCoordinate = 65536
	minStrokeWidth     = 1
	maxStrokeWidth     = 12
)

// StrokeColors are the colour tokens a stroke may name. The web resolves each
// to the theme, so a drawing follows light and dark mode.
var StrokeColors = []string{"accent", "ct", "t", "mark"}

// Stroke is one line of a drawing: a colour token, a width in radar pixels at
// the 1024 reference size, and its points as world x and y.
type Stroke struct {
	Color  string       `json:"color"`
	Width  int          `json:"width"`
	Arrow  bool         `json:"arrow"`
	Points [][2]float64 `json:"points"`
}

// Annotation is a note and an optional drawing anchored to one tick of one
// round of a match. Deleting the match deletes its annotations.
type Annotation struct {
	ID        uuid.UUID `json:"id"`
	MatchID   uuid.UUID `json:"match_id"`
	Round     int       `json:"round"`
	Tick      int       `json:"tick"`
	Text      string    `json:"text"`
	Strokes   []Stroke  `json:"strokes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AnnotationFilter selects annotations by match and, optionally, round.
type AnnotationFilter struct {
	MatchIDs []uuid.UUID
	Round    int
}

// AnnotationPatch is a user edit of an annotation. Nil fields are left unchanged.
type AnnotationPatch struct {
	Tick    *int
	Text    *string
	Strokes *[]Stroke
}

// CleanAnnotation trims the text, gives a nil drawing an empty one, and checks
// every limit. A note needs text or at least one stroke.
func CleanAnnotation(a Annotation) (Annotation, error) {
	a.Text = strings.TrimSpace(a.Text)
	if a.Strokes == nil {
		a.Strokes = []Stroke{}
	}
	if a.Round < 1 {
		return a, errors.New("round must be a positive integer")
	}
	if a.Tick < 0 {
		return a, errors.New("tick must not be negative")
	}
	if len([]rune(a.Text)) > MaxAnnotationTextRunes {
		return a, fmt.Errorf("text must be at most %d characters", MaxAnnotationTextRunes)
	}
	if a.Text == "" && len(a.Strokes) == 0 {
		return a, errors.New("a note needs text or a drawing")
	}
	if len(a.Strokes) > MaxAnnotationStrokes {
		return a, fmt.Errorf("a drawing has at most %d strokes", MaxAnnotationStrokes)
	}
	total := 0
	for i, s := range a.Strokes {
		if !validStrokeColor(s.Color) {
			return a, fmt.Errorf("stroke %d: color must be one of %s", i, strings.Join(StrokeColors, ", "))
		}
		if s.Width < minStrokeWidth || s.Width > maxStrokeWidth {
			return a, fmt.Errorf("stroke %d: width must be from %d to %d", i, minStrokeWidth, maxStrokeWidth)
		}
		if len(s.Points) < 2 || len(s.Points) > MaxAnnotationStrokePoint {
			return a, fmt.Errorf("stroke %d: a stroke has from 2 to %d points", i, MaxAnnotationStrokePoint)
		}
		for _, p := range s.Points {
			if !validWorldCoordinate(p[0]) || !validWorldCoordinate(p[1]) {
				return a, fmt.Errorf("stroke %d: points must be finite world coordinates", i)
			}
		}
		total += len(s.Points)
	}
	if total > MaxAnnotationPoints {
		return a, fmt.Errorf("a drawing has at most %d points", MaxAnnotationPoints)
	}
	return a, nil
}

func validStrokeColor(color string) bool {
	for _, c := range StrokeColors {
		if c == color {
			return true
		}
	}
	return false
}

func validWorldCoordinate(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && math.Abs(v) <= maxWorldCoordinate
}
