// Package hudthemes holds the user's own Full Demo HUD themes: a catalog
// design duplicated with other colours, corner radius and name.
package hudthemes

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	MaxThemes    = 10
	MaxNameRunes = 40
	MaxRadius    = 15
)

var (
	ErrNotFound = errors.New("hud theme not found")
	ErrLimit    = fmt.Errorf("at most %d HUD themes can be saved", MaxThemes)
)

// Style is what a theme changes on its catalog base. A nil Radius keeps the
// base design's shape; a value draws every plate with that corner radius.
type Style struct {
	Name       string `json:"name"`
	CT         string `json:"ct"`
	T          string `json:"t"`
	Background string `json:"background"`
	Radius     *int   `json:"radius,omitempty"`
}

// Theme is a stored user theme. Base is the catalog design id it copies.
type Theme struct {
	ID   uuid.UUID `json:"id"`
	Base string    `json:"base"`
	Style
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Clean trims the name and upper-cases the colours, then validates them.
func (s Style) Clean() (Style, error) {
	s.Name = strings.TrimSpace(s.Name)
	s.CT, s.T, s.Background = strings.ToUpper(strings.TrimPrefix(s.CT, "#")), strings.ToUpper(strings.TrimPrefix(s.T, "#")), strings.ToUpper(strings.TrimPrefix(s.Background, "#"))
	if s.Radius != nil {
		radius := *s.Radius
		s.Radius = &radius
	}
	return s, s.Validate()
}

func (s Style) Validate() error {
	if s.Name == "" || utf8.RuneCountInString(s.Name) > MaxNameRunes || strings.IndexFunc(s.Name, unicode.IsControl) >= 0 {
		return fmt.Errorf("name must hold 1 to %d characters", MaxNameRunes)
	}
	for _, c := range []struct{ field, value string }{{"ct", s.CT}, {"t", s.T}, {"background", s.Background}} {
		if !ValidColor(c.value) {
			return fmt.Errorf("%s must be a colour like 42C9E8", c.field)
		}
	}
	if s.Radius != nil && (*s.Radius < 0 || *s.Radius > MaxRadius) {
		return fmt.Errorf("radius must be between 0 and %d px", MaxRadius)
	}
	return nil
}

// ValidColor accepts six upper-case hex digits, the catalog's colour form.
func ValidColor(value string) bool {
	if len(value) != 6 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9') && !(c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

// Copy gives the caller its own radius pointer.
func (t Theme) Copy() Theme {
	if t.Radius != nil {
		radius := *t.Radius
		t.Radius = &radius
	}
	return t
}
