package library

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// Play limits. A play is a handful of grenades and five positions; the
// examples are the rounds it was read from.
const (
	MaxPlayTitleRunes = 120
	MaxPlayNotesRunes = 4000
	MaxPlayGrenades   = tacticalplan.PlayMaxGrenades
	MaxPlayPositions  = 10
	MaxPlayExamples   = 50
	// MaxPlayList caps one list read and one stratbook file.
	MaxPlayList = 1000
)

// Where a play came from.
const (
	PlayOriginRound    = "round"    // saved from one round of the replay
	PlayOriginDetected = "detected" // saved from a repeated play the detection found
	PlayOriginImport   = "import"   // read from a stratbook file
)

var playMapPattern = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

// Play is a saved strat: the map and side it is played on, its kind in the
// round classifier's vocabulary, its grenades and positions, the coach's notes
// and the rounds it was read from. Team is the team that runs it, own or rival.
type Play struct {
	ID               uuid.UUID                   `json:"id"`
	Title            string                      `json:"title"`
	TeamID           *uuid.UUID                  `json:"team_id"`
	TeamName         string                      `json:"team_name,omitempty"`
	Map              string                      `json:"map"`
	Side             string                      `json:"side"`
	Kind             string                      `json:"kind"`
	Site             string                      `json:"site"`
	Origin           string                      `json:"origin"`
	PositionsSeconds float64                     `json:"positions_seconds"`
	Grenades         []tacticalplan.PlayGrenade  `json:"grenades"`
	Positions        []tacticalplan.PlayPosition `json:"positions"`
	Notes            string                      `json:"notes"`
	Examples         []tacticalplan.Evidence     `json:"examples"`
	CreatedAt        time.Time                   `json:"created_at"`
	UpdatedAt        time.Time                   `json:"updated_at"`
}

// PlayFilter selects plays for a list read. Zero fields do not filter.
type PlayFilter struct {
	TeamID *uuid.UUID
	Map    string
	Side   string
	Kind   string
	Limit  int
	Offset int
}

// PlayPatch is a user edit of a play. Nil fields are left unchanged; ClearTeam
// detaches the play from its team.
type PlayPatch struct {
	Title     *string
	TeamID    *uuid.UUID
	ClearTeam bool
	Kind      *string
	Site      *string
	Notes     *string
	Grenades  *[]tacticalplan.PlayGrenade
	Positions *[]tacticalplan.PlayPosition
	Examples  *[]tacticalplan.Evidence
}

// ValidPlayKind reports whether kind is a round pattern of side.
func ValidPlayKind(side, kind string) bool {
	switch side {
	case string(tacticalplan.SideT):
		for _, p := range tacticalplan.TPatterns {
			if string(p) == kind {
				return true
			}
		}
	case string(tacticalplan.SideCT):
		for _, p := range tacticalplan.CTPatterns {
			if string(p) == kind {
				return true
			}
		}
	}
	return false
}

func validPlaySite(site string) bool {
	switch tacticalplan.Site(site) {
	case tacticalplan.SiteA, tacticalplan.SiteB, tacticalplan.SiteMid, tacticalplan.SiteNone:
		return true
	default:
		return false
	}
}

func validPlayGrenade(kind tacticalplan.EventKind) bool {
	switch kind {
	case tacticalplan.EventSmoke, tacticalplan.EventFlash, tacticalplan.EventHE, tacticalplan.EventMolotov:
		return true
	default:
		return false
	}
}

func validPoint(p [3]float64) bool {
	for _, v := range p {
		if !validWorldCoordinate(v) {
			return false
		}
	}
	return true
}

func cleanShare(v float64) bool {
	return !math.IsNaN(v) && v >= 0 && v <= 1
}

// CleanPlay trims the texts, gives nil lists empty ones, and checks every
// field against the play vocabulary and limits.
func CleanPlay(p Play) (Play, error) {
	p.Title = CleanName(p.Title)
	p.Notes = strings.TrimSpace(p.Notes)
	p.Map = strings.ToLower(strings.TrimSpace(p.Map))
	p.Side = strings.ToUpper(strings.TrimSpace(p.Side))
	if p.Site == "" {
		p.Site = string(tacticalplan.SiteNone)
	}
	if p.Grenades == nil {
		p.Grenades = []tacticalplan.PlayGrenade{}
	}
	if p.Positions == nil {
		p.Positions = []tacticalplan.PlayPosition{}
	}
	if p.Examples == nil {
		p.Examples = []tacticalplan.Evidence{}
	}
	switch {
	case p.Title == "":
		return p, errors.New("title must not be empty")
	case len([]rune(p.Title)) > MaxPlayTitleRunes:
		return p, fmt.Errorf("title must be at most %d characters", MaxPlayTitleRunes)
	case len([]rune(p.Notes)) > MaxPlayNotesRunes:
		return p, fmt.Errorf("notes must be at most %d characters", MaxPlayNotesRunes)
	case !playMapPattern.MatchString(p.Map):
		return p, errors.New("map must match ^[a-z0-9_]+$")
	case p.Side != string(tacticalplan.SideCT) && p.Side != string(tacticalplan.SideT):
		return p, errors.New("side must be CT or T")
	case !ValidPlayKind(p.Side, p.Kind):
		return p, fmt.Errorf("kind %q is not a %s round pattern", p.Kind, p.Side)
	case !validPlaySite(p.Site):
		return p, errors.New("site must be A, B, mid or none")
	case p.Origin != PlayOriginRound && p.Origin != PlayOriginDetected && p.Origin != PlayOriginImport:
		return p, errors.New("origin must be round, detected or import")
	case math.IsNaN(p.PositionsSeconds) || p.PositionsSeconds < 0 || p.PositionsSeconds > 300:
		return p, errors.New("positions_seconds must be from 0 to 300")
	case len(p.Grenades) > MaxPlayGrenades:
		return p, fmt.Errorf("a play has at most %d grenades", MaxPlayGrenades)
	case len(p.Positions) > MaxPlayPositions:
		return p, fmt.Errorf("a play has at most %d positions", MaxPlayPositions)
	case len(p.Examples) > MaxPlayExamples:
		return p, fmt.Errorf("a play has at most %d example rounds", MaxPlayExamples)
	}
	for i, g := range p.Grenades {
		if !validPlayGrenade(g.Kind) {
			return p, fmt.Errorf("grenade %d: kind must be smoke, flash, he or molotov", i)
		}
		if !validPoint(g.Pos) || math.IsNaN(g.Seconds) || g.Seconds < 0 || g.Seconds > 300 || !cleanShare(g.Share) {
			return p, fmt.Errorf("grenade %d: position, seconds or share out of range", i)
		}
		p.Grenades[i].Callout = cleanPlayLabel(g.Callout)
		p.Grenades[i].Player = cleanPlayLabel(g.Player)
		p.Grenades[i].LineupID = cleanPlayLabel(g.LineupID)
		p.Grenades[i].Lineup = cleanPlayLabel(g.Lineup)
	}
	for i, pos := range p.Positions {
		if !validPoint(pos.Pos) || !cleanShare(pos.Share) {
			return p, fmt.Errorf("position %d: position or share out of range", i)
		}
		if pos.SteamID64 != "" && !numericSteamID(pos.SteamID64) {
			return p, fmt.Errorf("position %d: steamid64 must be numeric", i)
		}
		p.Positions[i].Callout = cleanPlayLabel(pos.Callout)
		p.Positions[i].Player = cleanPlayLabel(pos.Player)
	}
	for i, e := range p.Examples {
		if e.MatchID == uuid.Nil || e.Round < 1 || e.Tick < 0 {
			return p, fmt.Errorf("example %d: needs a match, a positive round and a tick", i)
		}
	}
	return p, nil
}

// cleanPlayLabel bounds a short label such as a callout or a nick.
func cleanPlayLabel(raw string) string {
	text := CleanName(raw)
	if runes := []rune(text); len(runes) > 64 {
		text = string(runes[:64])
	}
	return text
}

func numericSteamID(raw string) bool {
	if raw == "" || len(raw) > 20 {
		return false
	}
	for _, r := range raw {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
