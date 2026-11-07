package library

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// Call limits: a call is a word or two the team shouts, and what it means.
const (
	MaxCallNameRunes    = 60
	MaxCallMeaningRunes = 1000
	MaxCallList         = 1000
	CallSideBoth        = "both"
)

// Call is one word of the team's vocabulary: what is said and what it means.
type Call struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Side      string    `json:"side"`
	Meaning   string    `json:"meaning"`
	Status    string    `json:"status"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CallFilter selects calls for a list read. Zero fields do not filter; a
// side filter also keeps the calls of both sides.
type CallFilter struct {
	Side            string
	Status          string
	Tag             string
	ExcludeArchived bool
	Limit           int
	Offset          int
}

// CallPatch is a user edit of a call. Nil fields are left unchanged.
type CallPatch struct {
	Name    *string
	Side    *string
	Meaning *string
	Status  *string
	Tags    *[]string
}

// ApplyCallPatch returns c with the patch's fields set.
func ApplyCallPatch(c Call, p CallPatch) Call {
	for _, f := range []struct {
		dst *string
		src *string
	}{{&c.Name, p.Name}, {&c.Side, p.Side}, {&c.Meaning, p.Meaning}, {&c.Status, p.Status}} {
		if f.src != nil {
			*f.dst = *f.src
		}
	}
	if p.Tags != nil {
		c.Tags = append([]string{}, *p.Tags...)
	}
	return c
}

// CleanCall trims the texts, defaults side and status, and checks the limits.
func CleanCall(c Call) (Call, error) {
	c.Name, c.Meaning = CleanName(c.Name), strings.TrimSpace(c.Meaning)
	c.Side = strings.TrimSpace(c.Side)
	if strings.EqualFold(c.Side, CallSideBoth) || c.Side == "" {
		c.Side = CallSideBoth
	} else {
		c.Side = strings.ToUpper(c.Side)
	}
	if c.Status == "" {
		c.Status = StatusActive
	}
	c.Tags = NormalizeTags(c.Tags)
	switch {
	case c.Name == "" || len([]rune(c.Name)) > MaxCallNameRunes:
		return c, fmt.Errorf("name takes 1 to %d characters", MaxCallNameRunes)
	case len([]rune(c.Meaning)) > MaxCallMeaningRunes:
		return c, fmt.Errorf("meaning takes at most %d characters", MaxCallMeaningRunes)
	case !ValidCallSide(c.Side):
		return c, errors.New("side must be T, CT or both")
	case !ValidStatus(c.Status):
		return c, errors.New("status must be active, practice, draft, review or archived")
	case strings.ContainsFunc(c.Meaning, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\t' }):
		return c, errors.New("meaning must not hold control characters")
	}
	return c, nil
}

// ValidCallSide reports whether side is T, CT or both.
func ValidCallSide(side string) bool {
	return side == "T" || side == "CT" || side == CallSideBoth
}
