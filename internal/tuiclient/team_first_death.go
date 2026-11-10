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

// FirstDeathRequest is the team, its scope (Map and Side required) and the window after
// freeze end in seconds (To 0 runs to the round end) with the movement: static, slow or fast.
type FirstDeathRequest struct {
	TeamID   string
	Query    AnalysisQuery
	From, To int
	Movement string
}

// TeamFirstDeathPositions returns where the team's players stand on the side until the round's first death.
func (c *Client) TeamFirstDeathPositions(ctx context.Context, req FirstDeathRequest) (TeamFirstDeath, error) {
	q := req.Query
	q.Limit = 0
	values := q.values()
	values.Set("t_from", strconv.Itoa(req.From))
	if req.To > 0 {
		values.Set("t_to", strconv.Itoa(req.To))
	}
	if req.Movement != "" {
		values.Set("movement", req.Movement)
	}
	var out TeamFirstDeath
	err := c.getJSON(ctx, teamPath(req.TeamID, "first-death")+encodeValues(values), &out)
	return out, err
}

// TeamWarningsRequest is the team and the scope whose data warnings to read.
type TeamWarningsRequest struct {
	TeamID string
	Query  AnalysisQuery
}

// TeamDataWarnings returns GET /api/library/teams/{id}/warnings: the line on top of Preparar.
func (c *Client) TeamDataWarnings(ctx context.Context, req TeamWarningsRequest) ([]tacticalplan.DataWarning, error) {
	q := req.Query
	q.Limit = 0
	var out struct {
		Warnings []tacticalplan.DataWarning `json:"warnings"`
	}
	err := c.getJSON(ctx, teamPath(req.TeamID, "warnings")+q.encode(), &out)
	return out.Warnings, err
}
