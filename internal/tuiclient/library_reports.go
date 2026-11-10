package tuiclient

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// LibraryReport is a saved scouting report on one rival.
type LibraryReport struct {
	ID         string            `json:"id"`
	Title      string            `json:"title"`
	TeamID     string            `json:"team_id"`
	TeamName   string            `json:"team_name"`
	MatchIDs   []string          `json:"match_ids"`
	Side       string            `json:"side"`
	Notes      string            `json:"notes"`
	RoundLists []ReportRoundList `json:"round_lists"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// ReportRound is one round of a report's list.
type ReportRound struct {
	MatchID string `json:"match_id"`
	Round   int    `json:"round"`
}

// ReportRoundList is a named list of rounds on a report.
type ReportRoundList struct {
	Title  string        `json:"title"`
	Rounds []ReportRound `json:"rounds"`
}

// ReportInput is the body of a report create or edit; nil fields are left out.
// A report's team is fixed once it is created.
type ReportInput struct {
	Title      *string            `json:"title,omitempty"`
	TeamID     *string            `json:"team_id,omitempty"`
	MatchIDs   *[]string          `json:"match_ids,omitempty"`
	Side       *string            `json:"side,omitempty"`
	Notes      *string            `json:"notes,omitempty"`
	RoundLists *[]ReportRoundList `json:"round_lists,omitempty"`
}

func reportPath(id string) string { return "/api/library/reports/" + url.PathEscape(id) }

// ListReports lists the saved reports, optionally those on one team.
func (c *Client) ListReports(ctx context.Context, teamID string) ([]LibraryReport, error) {
	var out struct {
		Reports []LibraryReport `json:"reports"`
	}
	err := c.getJSON(ctx, "/api/library/reports"+query(map[string]string{"team_id": teamID, "limit": "200"}), &out)
	return out.Reports, err
}

// GetReport returns one report.
func (c *Client) GetReport(ctx context.Context, id string) (LibraryReport, error) {
	var out LibraryReport
	err := c.getJSON(ctx, reportPath(id), &out)
	return out, err
}

// CreateReport saves a new report; title and team are required.
func (c *Client) CreateReport(ctx context.Context, in ReportInput) (LibraryReport, error) {
	var out LibraryReport
	err := c.doJSON(ctx, http.MethodPost, "/api/library/reports", in, &out)
	return out, err
}

// UpdateReport edits a report's title, matches, side or notes.
func (c *Client) UpdateReport(ctx context.Context, id string, in ReportInput) (LibraryReport, error) {
	var out LibraryReport
	err := c.doJSON(ctx, http.MethodPatch, reportPath(id), in, &out)
	return out, err
}

// DeleteReport removes a report; its matches stay in the library.
func (c *Client) DeleteReport(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, reportPath(id), nil, nil)
}

// ReportScout returns the scouting sections of a report's matches and side.
func (c *Client) ReportScout(ctx context.Context, id string) (ScoutReport, error) {
	var out ScoutReport
	err := c.getJSON(ctx, reportPath(id)+"/scout", &out)
	return out, err
}

// ReportMetrics returns the scouted team's player metrics over the report's matches.
func (c *Client) ReportMetrics(ctx context.Context, id string) (PlayerMetricsReport, error) {
	var out PlayerMetricsReport
	err := c.getJSON(ctx, reportPath(id)+"/metrics", &out)
	return out, err
}
