package library

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Role limits: a side has five players, and a role is a short name and task.
const (
	MaxRoleSlots     = 5
	MaxRoleNameRunes = 40
	MaxRoleTaskRunes = 200
)

// RoleSlot is one player's job in a play or on a team's role sheet.
type RoleSlot struct {
	Name      string `json:"name"`
	SteamID64 string `json:"steamid64,omitempty"`
	Callout   string `json:"callout,omitempty"`
	Task      string `json:"task,omitempty"`
}

// TeamRoleSheet is who does what for a team on one map, per side.
type TeamRoleSheet struct {
	TeamID    uuid.UUID  `json:"team_id"`
	Map       string     `json:"map"`
	T         []RoleSlot `json:"t"`
	CT        []RoleSlot `json:"ct"`
	UpdatedAt time.Time  `json:"updated_at,omitzero"`
}

// CleanRoleSlots trims every slot and checks the names, players and limits.
func CleanRoleSlots(slots []RoleSlot) ([]RoleSlot, error) {
	if len(slots) > MaxRoleSlots {
		return nil, fmt.Errorf("at most %d roles", MaxRoleSlots)
	}
	out := make([]RoleSlot, 0, len(slots))
	for i, s := range slots {
		s.Name, s.Task = CleanName(s.Name), strings.TrimSpace(s.Task)
		s.SteamID64, s.Callout = strings.TrimSpace(s.SteamID64), cleanPlayLabel(s.Callout)
		switch {
		case s.Name == "" || len([]rune(s.Name)) > MaxRoleNameRunes:
			return nil, fmt.Errorf("role %d: name takes 1 to %d characters", i+1, MaxRoleNameRunes)
		case len([]rune(s.Task)) > MaxRoleTaskRunes:
			return nil, fmt.Errorf("role %d: task takes at most %d characters", i+1, MaxRoleTaskRunes)
		case s.SteamID64 != "" && !numericSteamID(s.SteamID64):
			return nil, fmt.Errorf("role %d: steamid64 must be numeric", i+1)
		}
		out = append(out, s)
	}
	return out, nil
}

// CleanTeamRoleSheet checks the map and both sides of a sheet.
func CleanTeamRoleSheet(sheet TeamRoleSheet) (TeamRoleSheet, error) {
	sheet.Map = strings.ToLower(strings.TrimSpace(sheet.Map))
	if !playMapPattern.MatchString(sheet.Map) {
		return sheet, errors.New("map must match ^[a-z0-9_]+$")
	}
	var err error
	if sheet.T, err = CleanRoleSlots(sheet.T); err != nil {
		return sheet, fmt.Errorf("t: %w", err)
	}
	if sheet.CT, err = CleanRoleSlots(sheet.CT); err != nil {
		return sheet, fmt.Errorf("ct: %w", err)
	}
	return sheet, nil
}
