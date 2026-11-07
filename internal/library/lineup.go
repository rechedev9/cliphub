package library

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

const (
	MaxLineupTitleRunes = 120
	MaxLineupNotesRunes = 4000
	// MaxLineupList caps one list read and the lineups of one Libreta file.
	MaxLineupList = 1000
)

// How the thrower moved and clicked, in the words CS2 players use for a lineup.
const (
	MovementStanding  = "standing"
	MovementWalking   = "walking"
	MovementRunning   = "running"
	MovementCrouching = "crouching"

	ClickLeft  = "left"
	ClickRight = "right"
	ClickBoth  = "both"
)

// Lineup is a grenade the team throws the same way every time: where the
// thrower stands and aims (setpos and setang), how it is thrown, and where it
// lands. Source is the round it was read from; a lineup written by hand has none.
type Lineup struct {
	ID      uuid.UUID  `json:"id"`
	Title   string     `json:"title"`
	Map     string     `json:"map"`
	Side    string     `json:"side"`
	Grenade string     `json:"grenade"`
	Status  string     `json:"status"`
	Tags    []string   `json:"tags"`
	Notes   string     `json:"notes"`
	Origin  [3]float64 `json:"origin"`
	// Pitch (-90 up to 90 down) and Yaw (-180 to 180) as setang takes them.
	Pitch          float64     `json:"pitch"`
	Yaw            float64     `json:"yaw"`
	Landing        *[3]float64 `json:"landing"`
	ThrowCallout   string      `json:"throw_callout"`
	LandingCallout string      `json:"landing_callout"`
	Movement       string      `json:"movement"`
	Jump           bool        `json:"jump"`
	Click          string      `json:"click"`
	// TechniqueEstimated marks movement, jump and click read from 8 Hz samples, until the user sets them.
	TechniqueEstimated bool                   `json:"technique_estimated"`
	Source             *tacticalplan.Evidence `json:"source"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
}

// LineupFilter selects lineups for a list read. Zero fields do not filter.
type LineupFilter struct {
	Map             string
	Side            string
	Grenade         string
	Status          string
	Tag             string // matched case-insensitively
	ExcludeArchived bool
	Limit           int
	Offset          int
}

// LineupPatch is a user edit of a lineup. Nil fields are left unchanged.
type LineupPatch struct {
	Title              *string
	Status             *string
	Tags               *[]string
	Notes              *string
	Origin             *[3]float64
	Pitch              *float64
	Yaw                *float64
	ThrowCallout       *string
	LandingCallout     *string
	Movement           *string
	Jump               *bool
	Click              *string
	TechniqueEstimated *bool
}

// ApplyLineupPatch is the edit both repositories apply to a stored lineup.
func ApplyLineupPatch(l Lineup, patch LineupPatch) Lineup {
	setString := func(dst *string, src *string) {
		if src != nil {
			*dst = *src
		}
	}
	setString(&l.Title, patch.Title)
	setString(&l.Status, patch.Status)
	setString(&l.Notes, patch.Notes)
	setString(&l.ThrowCallout, patch.ThrowCallout)
	setString(&l.LandingCallout, patch.LandingCallout)
	setString(&l.Movement, patch.Movement)
	setString(&l.Click, patch.Click)
	if patch.Tags != nil {
		l.Tags = append([]string{}, *patch.Tags...)
	}
	if patch.Origin != nil {
		l.Origin = *patch.Origin
	}
	if patch.Pitch != nil {
		l.Pitch = *patch.Pitch
	}
	if patch.Yaw != nil {
		l.Yaw = *patch.Yaw
	}
	if patch.Jump != nil {
		l.Jump = *patch.Jump
	}
	if patch.TechniqueEstimated != nil {
		l.TechniqueEstimated = *patch.TechniqueEstimated
	}
	return l
}

// ValidGrenade reports whether kind is a grenade a lineup can hold.
func ValidGrenade(kind string) bool {
	switch tacticalplan.EventKind(kind) {
	case tacticalplan.EventSmoke, tacticalplan.EventFlash, tacticalplan.EventHE, tacticalplan.EventMolotov, tacticalplan.EventDecoy:
		return true
	default:
		return false
	}
}

func validMovement(m string) bool {
	switch m {
	case MovementStanding, MovementWalking, MovementRunning, MovementCrouching:
		return true
	default:
		return false
	}
}

func validClick(c string) bool { return c == ClickLeft || c == ClickRight || c == ClickBoth }

// CleanLineup trims the texts, fills the defaults and checks every field
// against the lineup vocabulary. Angles are brought into setang's ranges, so a
// raw demo pitch (270 to 360 when looking up) reads as -90 to 0.
func CleanLineup(l Lineup) (Lineup, error) {
	l.Title = CleanName(l.Title)
	l.Notes = strings.TrimSpace(l.Notes)
	l.Map = strings.ToLower(strings.TrimSpace(l.Map))
	l.Side = strings.ToUpper(strings.TrimSpace(l.Side))
	l.Grenade = strings.ToLower(strings.TrimSpace(l.Grenade))
	if l.Status == "" {
		l.Status = StatusActive
	}
	if l.Movement == "" {
		l.Movement = MovementStanding
	}
	if l.Click == "" {
		l.Click = ClickLeft
	}
	l.Tags = NormalizeTags(l.Tags)
	l.ThrowCallout = cleanPlayLabel(l.ThrowCallout)
	l.LandingCallout = cleanPlayLabel(l.LandingCallout)
	l.Pitch, l.Yaw = tacticalplan.SetangPitch(l.Pitch), tacticalplan.SetangYaw(l.Yaw)
	switch {
	case l.Title == "":
		return l, errors.New("title must not be empty")
	case len([]rune(l.Title)) > MaxLineupTitleRunes:
		return l, fmt.Errorf("title must be at most %d characters", MaxLineupTitleRunes)
	case len([]rune(l.Notes)) > MaxLineupNotesRunes:
		return l, fmt.Errorf("notes must be at most %d characters", MaxLineupNotesRunes)
	case !playMapPattern.MatchString(l.Map):
		return l, errors.New("map must match ^[a-z0-9_]+$")
	case l.Side != string(tacticalplan.SideCT) && l.Side != string(tacticalplan.SideT):
		return l, errors.New("side must be CT or T")
	case !ValidGrenade(l.Grenade):
		return l, errors.New("grenade must be smoke, flash, he, molotov or decoy")
	case !ValidStatus(l.Status):
		return l, errors.New("status must be active, practice, draft, review or archived")
	case !validMovement(l.Movement):
		return l, errors.New("movement must be standing, walking, running or crouching")
	case !validClick(l.Click):
		return l, errors.New("click must be left, right or both")
	case !validPoint(l.Origin):
		return l, errors.New("origin is out of the map")
	case math.IsNaN(l.Pitch) || math.IsNaN(l.Yaw):
		return l, errors.New("pitch and yaw must be numbers")
	case l.Landing != nil && !validPoint(*l.Landing):
		return l, errors.New("landing is out of the map")
	case l.Source != nil && (l.Source.MatchID == uuid.Nil || l.Source.Round < 1 || l.Source.Tick < 0):
		return l, errors.New("source needs a match, a positive round and a tick")
	}
	return l, nil
}
