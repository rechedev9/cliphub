package tuiclient

import (
	"context"
	"strconv"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// TeamFirstDeath is GET /api/library/teams/{id}/first-death.
type TeamFirstDeath struct {
	TeamName string            `json:"team_name"`
	Skipped  []AnalysisSkipped `json:"skipped"`
	tacticalplan.FirstDeathPositions
}

// FirstDeathQuery is the window after freeze end, in seconds (To 0 runs to the
// round end), and the movement: static, slow or fast.
type FirstDeathQuery struct {
	From, To int
	Movement string
}

// TeamFirstDeathPositions returns where the team's players stand on q.Side
// until the round's first death; q.Map and q.Side are required.
func (c *Client) TeamFirstDeathPositions(ctx context.Context, teamID string, q AnalysisQuery, fd FirstDeathQuery) (TeamFirstDeath, error) {
	q.Limit = 0
	values := q.values()
	values.Set("t_from", strconv.Itoa(fd.From))
	if fd.To > 0 {
		values.Set("t_to", strconv.Itoa(fd.To))
	}
	if fd.Movement != "" {
		values.Set("movement", fd.Movement)
	}
	var out TeamFirstDeath
	err := c.getJSON(ctx, teamPath(teamID, "first-death")+encodeValues(values), &out)
	return out, err
}
