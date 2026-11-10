package library

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// Saved pattern limits: a name like a play's title, and the scope a pattern keeps.
const (
	MaxPatternNameRunes = 80
	MaxPatternList      = 500
	MaxPatternMatches   = 50
)

var patternMapPattern = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

// PatternScope is which rounds a saved pattern reads besides its map and side:
// the most recent Matches (0 is every one) and both teams' buys, half for semi and force.
type PatternScope struct {
	Matches     int                  `json:"matches,omitempty"`
	Buy         tacticalplan.BuyType `json:"buy,omitempty"`
	OpponentBuy tacticalplan.BuyType `json:"opponent_buy,omitempty"`
}

// Pattern is a saved pattern of the finder: blocks on one map and side,
// detected over a team's matches or, without a team, over the whole library.
type Pattern struct {
	ID        uuid.UUID            `json:"id"`
	Name      string               `json:"name"`
	Map       string               `json:"map"`
	TeamID    *uuid.UUID           `json:"team_id,omitempty"`
	Pattern   tacticalplan.Pattern `json:"pattern"`
	Scope     PatternScope         `json:"scope"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
}

// PatternFilter selects saved patterns; zero fields do not filter.
type PatternFilter struct {
	Map    string
	TeamID *uuid.UUID
	Limit  int
	Offset int
}

// PatternPatch is a user edit of a saved pattern. Nil fields are left unchanged;
// ClearTeam detaches it from its team, so it reads the whole library.
type PatternPatch struct {
	Name      *string
	Map       *string
	TeamID    *uuid.UUID
	ClearTeam bool
	Pattern   *tacticalplan.Pattern
	Scope     *PatternScope
}

// ApplyPatternPatch returns p with the patch's fields set.
func ApplyPatternPatch(p Pattern, patch PatternPatch) Pattern {
	if patch.Name != nil {
		p.Name = *patch.Name
	}
	if patch.Map != nil {
		p.Map = *patch.Map
	}
	if patch.ClearTeam {
		p.TeamID = nil
	}
	if patch.TeamID != nil {
		id := *patch.TeamID
		p.TeamID = &id
	}
	if patch.Pattern != nil {
		p.Pattern = *patch.Pattern
	}
	if patch.Scope != nil {
		p.Scope = *patch.Scope
	}
	return PatternCopy(p)
}

// PatternCopy gives a pattern its own blocks and team id, the shape both repositories return.
func PatternCopy(p Pattern) Pattern {
	if p.TeamID != nil {
		id := *p.TeamID
		p.TeamID = &id
	}
	blocks := make([]tacticalplan.PatternBlock, len(p.Pattern.Blocks))
	for i, b := range p.Pattern.Blocks {
		b.Players = append([]string{}, b.Players...)
		if b.Area != nil {
			area := *b.Area
			b.Area = &area
		}
		blocks[i] = b
	}
	p.Pattern.Blocks = blocks
	return p
}

// ScopeBuyHalf is the scope bar's half buy: a semi or a force buy.
const ScopeBuyHalf tacticalplan.BuyType = "half"

func validScopeBuy(b tacticalplan.BuyType) bool {
	return b == "" || b == ScopeBuyHalf || (b != tacticalplan.BuyUnknown && slices.Contains(tacticalplan.BuyTypes, b))
}

// CleanPattern trims the name and checks the map, the blocks and the scope.
func CleanPattern(p Pattern) (Pattern, error) {
	p.Name, p.Map = CleanName(p.Name), strings.ToLower(strings.TrimSpace(p.Map))
	switch {
	case p.Name == "" || len([]rune(p.Name)) > MaxPatternNameRunes:
		return p, fmt.Errorf("name takes 1 to %d characters", MaxPatternNameRunes)
	case !patternMapPattern.MatchString(p.Map):
		return p, errors.New("map must match ^[a-z0-9_]+$")
	case p.Pattern.Side == "":
		return p, errors.New("a saved pattern needs a side, CT or T")
	case p.Scope.Matches < 0 || p.Scope.Matches > MaxPatternMatches:
		return p, fmt.Errorf("matches must be from 0 to %d", MaxPatternMatches)
	case !validScopeBuy(p.Scope.Buy) || !validScopeBuy(p.Scope.OpponentBuy):
		return p, errors.New("buy and opponent_buy must be pistol, eco, semi, force, full or half")
	}
	if err := p.Pattern.Check(); err != nil {
		return p, err
	}
	return PatternCopy(p), nil
}
