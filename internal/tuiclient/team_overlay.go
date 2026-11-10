package tuiclient

import (
	"context"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// TeamOverlay is GET /api/library/teams/{id}/overlay: a team's rounds on one map laid on the
// clock from freeze end, with the paths of its players and the radar they are placed on.
type TeamOverlay struct {
	Map      string                    `json:"map"`
	Geometry *tacticalplan.MapGeometry `json:"geometry"`
	Overlay  *tacticalplan.Overlay     `json:"overlay"`
}

// LibraryTeamOverlay reads the overlay of the team's rounds that q keeps; the orchestrator caps the rounds drawn.
func (c *Client) LibraryTeamOverlay(ctx context.Context, teamID string, q AnalysisQuery) (TeamOverlay, error) {
	q.Limit = 0
	var out TeamOverlay
	err := c.getJSON(ctx, teamPath(teamID, "overlay")+encodeValues(q.values()), &out)
	return out, err
}
