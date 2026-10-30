package library

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Veto actions. A decider is the map left after the last ban; it belongs to
// neither team.
const (
	VetoBan     = "ban"
	VetoPick    = "pick"
	VetoDecider = "decider"
)

// Series team slots. A veto step names the slot, not the team id, so merging
// a team only has to move the slot.
const (
	SeriesTeamA = "a"
	SeriesTeamB = "b"
)

// Series limits.
const (
	MaxSeriesEventRunes = 120
	MaxSeriesVetoRunes  = 480
	// MaxVetoSteps covers a nine map pool; the active duty pool has seven.
	MaxVetoSteps = 9
)

// VetoStep is one step of a known veto, in the order it happened.
type VetoStep struct {
	Team   string `json:"team"`
	Action string `json:"action"`
	Map    string `json:"map"`
}

// SeriesTeam is one of the two teams of a series, by slot.
type SeriesTeam struct {
	Slot   string    `json:"slot"`
	TeamID uuid.UUID `json:"team_id"`
	Name   string    `json:"name"`
}

// SeriesFilter selects series for a list read. A team matches when it holds
// either slot.
type SeriesFilter struct {
	TeamID *uuid.UUID
	Limit  int
	Offset int
}

// SeriesPatch is a user edit of a series. Nil fields are left unchanged.
type SeriesPatch struct {
	Format  *string
	Event   *string
	Veto    *string
	TeamIDs *[]uuid.UUID
	Vetoes  *[]VetoStep
}

// Slot returns the slot a team holds in the series, or "" when it holds none.
func (s Series) Slot(teamID uuid.UUID) string {
	for _, t := range s.Teams {
		if t.TeamID == teamID {
			return t.Slot
		}
	}
	return ""
}

// CleanSeries trims the texts, gives nil lists empty ones and checks the
// format, the two teams and the veto steps.
func CleanSeries(s Series) (Series, error) {
	s.Event = CleanName(s.Event)
	s.Veto = strings.TrimSpace(s.Veto)
	if s.Vetoes == nil {
		s.Vetoes = []VetoStep{}
	}
	if s.TeamIDs == nil {
		s.TeamIDs = []uuid.UUID{}
	}
	switch {
	case !ValidSeriesFormat(s.Format):
		return s, errors.New("format must be bo1, bo3 or bo5")
	case len([]rune(s.Event)) > MaxSeriesEventRunes:
		return s, fmt.Errorf("event must be at most %d characters", MaxSeriesEventRunes)
	case len([]rune(s.Veto)) > MaxSeriesVetoRunes:
		return s, fmt.Errorf("veto notes must be at most %d characters", MaxSeriesVetoRunes)
	case len(s.TeamIDs) != 0 && len(s.TeamIDs) != 2:
		return s, errors.New("team_ids must hold no team or two teams")
	case len(s.TeamIDs) == 2 && (s.TeamIDs[0] == uuid.Nil || s.TeamIDs[1] == uuid.Nil || s.TeamIDs[0] == s.TeamIDs[1]):
		return s, errors.New("team_ids must be two different teams")
	case len(s.Vetoes) > MaxVetoSteps:
		return s, fmt.Errorf("a veto has at most %d steps", MaxVetoSteps)
	}
	seen := map[string]bool{}
	for i, step := range s.Vetoes {
		step.Map = canonicalMapName(step.Map)
		step.Team = strings.ToLower(strings.TrimSpace(step.Team))
		step.Action = strings.ToLower(strings.TrimSpace(step.Action))
		s.Vetoes[i] = step
		switch {
		case !playMapPattern.MatchString(step.Map):
			return s, fmt.Errorf("veto step %d: map must match ^[a-z0-9_]+$", i+1)
		case seen[step.Map]:
			return s, fmt.Errorf("veto step %d: %s appears twice", i+1, step.Map)
		case step.Action == VetoDecider:
			if step.Team != "" {
				return s, fmt.Errorf("veto step %d: a decider belongs to no team", i+1)
			}
			if i != len(s.Vetoes)-1 {
				return s, fmt.Errorf("veto step %d: the decider is the last step", i+1)
			}
		case step.Action == VetoBan || step.Action == VetoPick:
			if step.Team != SeriesTeamA && step.Team != SeriesTeamB {
				return s, fmt.Errorf("veto step %d: team must be a or b", i+1)
			}
			if len(s.TeamIDs) != 2 {
				return s, fmt.Errorf("veto step %d: set the two teams of the series first", i+1)
			}
		default:
			return s, fmt.Errorf("veto step %d: action must be ban, pick or decider", i+1)
		}
		seen[step.Map] = true
	}
	return s, nil
}

// ApplySeriesPatch returns s with the patch applied, before CleanSeries runs.
func ApplySeriesPatch(s Series, patch SeriesPatch) Series {
	if patch.Format != nil {
		s.Format = *patch.Format
	}
	if patch.Event != nil {
		s.Event = *patch.Event
	}
	if patch.Veto != nil {
		s.Veto = *patch.Veto
	}
	if patch.TeamIDs != nil {
		s.TeamIDs = append([]uuid.UUID{}, (*patch.TeamIDs)...)
	}
	if patch.Vetoes != nil {
		s.Vetoes = append([]VetoStep{}, (*patch.Vetoes)...)
	}
	return s
}
