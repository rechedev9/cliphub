package library

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// MaxCalloutNameRunes bounds the team's own name for a CS2 place.
const MaxCalloutNameRunes = 40

// CalloutName is the team's own name for one CS2 place of a map.
type CalloutName struct {
	Map       string    `json:"map"`
	Token     string    `json:"token"`
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`
}

// CleanCalloutName trims the fields and checks the map, the token and the name.
func CleanCalloutName(c CalloutName) (CalloutName, error) {
	c.Map, c.Token, c.Name = strings.ToLower(strings.TrimSpace(c.Map)), strings.TrimSpace(c.Token), CleanName(c.Name)
	switch {
	case !playMapPattern.MatchString(c.Map):
		return c, errors.New("map must match ^[a-z0-9_]+$")
	case !ValidCalloutToken(c.Token):
		return c, errors.New("token must be a CS2 place name of at most 64 characters")
	case c.Name == "" || len([]rune(c.Name)) > MaxCalloutNameRunes:
		return c, fmt.Errorf("name takes 1 to %d characters", MaxCalloutNameRunes)
	}
	return c, nil
}

// ValidCalloutToken reports whether token can be a CS2 place name.
func ValidCalloutToken(token string) bool {
	return token != "" && len([]rune(token)) <= 64 && !strings.ContainsFunc(token, unicode.IsControl)
}
