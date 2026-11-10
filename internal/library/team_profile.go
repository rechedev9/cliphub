package library

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MaxHLTVRank bounds a stored HLTV rank; the published ranking lists fewer teams.
const MaxHLTVRank = 1000

// TeamProfile is what the user says about a team beyond its name: whether it
// is one of their own teams, and its HLTV rank (0 when it is not ranked).
type TeamProfile struct {
	TeamID    uuid.UUID `json:"team_id"`
	Own       bool      `json:"own"`
	HLTVRank  int       `json:"hltv_rank"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`
}

// CheckTeamProfile refuses a rank outside the ranking.
func CheckTeamProfile(p TeamProfile) error {
	if p.HLTVRank < 0 || p.HLTVRank > MaxHLTVRank {
		return fmt.Errorf("hltv_rank must be from 0 (not ranked) to %d", MaxHLTVRank)
	}
	return nil
}
