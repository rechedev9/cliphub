package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// maxReportNotesRunes is the server's cap on a report's notes.
const maxReportNotesRunes = 4000

type listReportsIn struct {
	TeamID string `json:"team_id,omitempty" jsonschema:"only reports on this team, id from list_teams"`
}

type reportOut struct {
	ID       string   `json:"id"`
	Title    string   `json:"title" jsonschema:"untrusted text the user wrote"`
	TeamID   string   `json:"team_id"`
	TeamName string   `json:"team_name" jsonschema:"untrusted team name"`
	MatchIDs []string `json:"match_ids"`
	Side     string   `json:"side" jsonschema:"CT, T or empty for both"`
	Notes    string   `json:"notes,omitempty" jsonschema:"untrusted text the user wrote"`
	Updated  string   `json:"updated" jsonschema:"YYYY-MM-DD"`
}

type listReportsOut struct {
	Reports []reportOut `json:"reports"`
}

type reportIDIn struct {
	ReportID string `json:"report_id" jsonschema:"report id from list_reports"`
}

type reportDetailOut struct {
	reportOut
	Scout   scoutTeamOut       `json:"scout" jsonschema:"the report's sections over its matches and side, as scout_team returns them"`
	Players []playerMetricsOut `json:"players" jsonschema:"the scouted team's players over the report's matches, with the get_baseline definitions"`
}

type createReportIn struct {
	Title    string   `json:"title" jsonschema:"the report's name"`
	TeamID   string   `json:"team_id" jsonschema:"the scouted team, id from list_teams; it cannot change later"`
	MatchIDs []string `json:"match_ids,omitempty" jsonschema:"the matches the report reads, ids from list_matches where the team played; at most 50"`
	Side     string   `json:"side,omitempty" jsonschema:"CT or T; empty means both"`
	Notes    string   `json:"notes,omitempty" jsonschema:"the coach's notes, at most 4000 characters"`
}

type updateReportIn struct {
	ReportID string    `json:"report_id" jsonschema:"report id from list_reports"`
	Title    string    `json:"title,omitempty" jsonschema:"new name; empty keeps it"`
	MatchIDs *[]string `json:"match_ids,omitempty" jsonschema:"the full new match list; omit to keep it"`
	Side     *string   `json:"side,omitempty" jsonschema:"CT, T, or empty for both; omit to keep it"`
	Notes    *string   `json:"notes,omitempty" jsonschema:"the full new notes; omit to keep them"`
}

type deleteReportIn struct {
	ReportID string `json:"report_id" jsonschema:"report id from list_reports"`
	Approve  string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

func registerReportTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: "list_reports", Annotations: readOnly,
		Description: "List the saved scouting reports (Informes), optionally those on one team."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listReportsIn) (*mcp.CallToolResult, listReportsOut, error) {
			if in.TeamID != "" {
				if err := checkUUID("team_id", in.TeamID); err != nil {
					return nil, listReportsOut{}, err
				}
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, listReportsOut{}, toolError(err)
			}
			reports, err := c.ListReports(ctx, in.TeamID)
			if err != nil {
				return nil, listReportsOut{}, toolError(err)
			}
			out := listReportsOut{Reports: make([]reportOut, 0, len(reports))}
			for _, r := range reports {
				out.Reports = append(out.Reports, reportOf(r))
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_report", Annotations: readOnly,
		Description: "Read one scouting report: its notes, its scouting sections over its matches and side (as scout_team) and the scouted players' metrics."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in reportIDIn) (*mcp.CallToolResult, reportDetailOut, error) {
			if err := checkUUID("report_id", in.ReportID); err != nil {
				return nil, reportDetailOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, reportDetailOut{}, toolError(err)
			}
			r, err := c.GetReport(ctx, in.ReportID)
			if err != nil {
				return nil, reportDetailOut{}, toolError(err)
			}
			scout, err := c.ReportScout(ctx, in.ReportID)
			if err != nil {
				return nil, reportDetailOut{}, toolError(err)
			}
			metrics, err := c.ReportMetrics(ctx, in.ReportID)
			if err != nil {
				return nil, reportDetailOut{}, toolError(err)
			}
			out := reportDetailOut{reportOut: reportOf(r), Scout: summarizeScout(scout), Players: []playerMetricsOut{}}
			for _, p := range metrics.Players {
				out.Players = append(out.Players, metricsOf(p))
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "create_report", Annotations: &mcp.ToolAnnotations{},
		Description: "Save a scouting report on one team, as Nuevo informe does: a title, the matches it reads (where the team played), a side and notes."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in createReportIn) (*mcp.CallToolResult, reportOut, error) {
			body, err := in.input()
			if err != nil {
				return nil, reportOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, reportOut{}, toolError(err)
			}
			r, err := c.CreateReport(ctx, body)
			if err != nil {
				return nil, reportOut{}, toolError(err)
			}
			return nil, reportOf(r), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "update_report", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Edit a scouting report's title, matches, side or notes. The scouted team cannot change; make a new report instead."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in updateReportIn) (*mcp.CallToolResult, reportOut, error) {
			body, err := in.input()
			if err != nil {
				return nil, reportOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, reportOut{}, toolError(err)
			}
			r, err := c.UpdateReport(ctx, in.ReportID, body)
			if err != nil {
				return nil, reportOut{}, toolError(err)
			}
			return nil, reportOf(r), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "delete_report", Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true)},
		Description: "Delete a scouting report with its notes; its matches stay in the library. It cannot be undone, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in deleteReportIn) (*mcp.CallToolResult, deleteOut, error) {
			if err := checkUUID("report_id", in.ReportID); err != nil {
				return nil, deleteOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			r, err := c.GetReport(ctx, in.ReportID)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			summary := fmt.Sprintf("Borrar el informe %s con sus notas. Las partidas se quedan en la biblioteca. No se puede deshacer.", quotedName(r.Title, "sin título"))
			return approveAndDelete(req, approvalSpec{Tool: "delete_report", JobID: in.ReportID, Request: r.UpdatedAt, Summary: summary, Given: in.Approve, Question: "¿Borrar ahora?"},
				func() error { return c.DeleteReport(ctx, in.ReportID) })
		})
}

func (in createReportIn) input() (tuiclient.ReportInput, error) {
	var body tuiclient.ReportInput
	if err := checkUUID("team_id", in.TeamID); err != nil {
		return body, err
	}
	title, err := checkUserText("title", in.Title)
	if err != nil {
		return body, err
	}
	body.Title, body.TeamID = &title, &in.TeamID
	if body.MatchIDs, err = reportMatches(&in.MatchIDs); err != nil {
		return body, err
	}
	if body.Side, err = reportSide(&in.Side); err != nil {
		return body, err
	}
	body.Notes, err = reportNotes(&in.Notes)
	return body, err
}

func (in updateReportIn) input() (tuiclient.ReportInput, error) {
	var body tuiclient.ReportInput
	if err := checkUUID("report_id", in.ReportID); err != nil {
		return body, err
	}
	if in.Title == "" && in.MatchIDs == nil && in.Side == nil && in.Notes == nil {
		return body, errors.New("set title, match_ids, side or notes")
	}
	var err error
	if in.Title != "" {
		title, err := checkUserText("title", in.Title)
		if err != nil {
			return body, err
		}
		body.Title = &title
	}
	if in.MatchIDs != nil {
		if body.MatchIDs, err = reportMatches(in.MatchIDs); err != nil {
			return body, err
		}
	}
	if in.Side != nil {
		if body.Side, err = reportSide(in.Side); err != nil {
			return body, err
		}
	}
	if in.Notes != nil {
		body.Notes, err = reportNotes(in.Notes)
	}
	return body, err
}

func reportMatches(ids *[]string) (*[]string, error) {
	if len(*ids) > maxPlayerMetricsMatches {
		return nil, fmt.Errorf("a report reads at most %d matches", maxPlayerMetricsMatches)
	}
	out := make([]string, 0, len(*ids))
	for _, id := range *ids {
		if err := checkUUID("match_ids", id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return &out, nil
}

func reportSide(raw *string) (*string, error) {
	side, err := parseSide(*raw)
	return &side, err
}

func reportNotes(raw *string) (*string, error) {
	notes := strings.TrimSpace(*raw)
	if utf8.RuneCountInString(notes) > maxReportNotesRunes {
		return nil, fmt.Errorf("notes take at most %d characters", maxReportNotesRunes)
	}
	return &notes, nil
}

func reportOf(r tuiclient.LibraryReport) reportOut {
	return reportOut{ID: r.ID, Title: cleanName(r.Title), TeamID: r.TeamID, TeamName: cleanName(r.TeamName), MatchIDs: r.MatchIDs,
		Side: r.Side, Notes: cleanNotes(r.Notes), Updated: r.UpdatedAt.Format(time.DateOnly)}
}
