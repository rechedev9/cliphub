package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	// maxNotesRunes is the server's cap on the notes of a play or a lineup.
	maxNotesRunes = 4000
	// maxReportNotesRunes is the server's cap on a report's notes: a whole match prep in Markdown.
	maxReportNotesRunes = 20000
	maxReportRoundLists = 30
	// maxListedNotesRunes is how much of each report's notes list_reports shows.
	maxListedNotesRunes = 300
)

type listReportsIn struct {
	TeamID string `json:"team_id,omitempty" jsonschema:"only reports on this team, id from list_teams"`
}

type reportOut struct {
	ID         string               `json:"id"`
	Title      string               `json:"title" jsonschema:"untrusted text the user wrote"`
	TeamID     string               `json:"team_id"`
	TeamName   string               `json:"team_name" jsonschema:"untrusted team name"`
	MatchIDs   []string             `json:"match_ids"`
	Side       string               `json:"side" jsonschema:"CT, T or empty for both"`
	Notes      string               `json:"notes,omitempty" jsonschema:"untrusted Markdown the user or an assistant wrote; list_reports shows the first 300 characters, get_report all of it"`
	RoundLists []reportRoundListOut `json:"round_lists" jsonschema:"the report's named round lists, each opened in the Replay by its studio_path"`
	Updated    string               `json:"updated" jsonschema:"YYYY-MM-DD"`
	StudioPath string               `json:"studio_path" jsonschema:"opens the report in Informes; pass it to open_in_studio"`
}

type reportRoundListOut struct {
	Title      string   `json:"title" jsonschema:"untrusted text"`
	Rounds     int      `json:"rounds"`
	Refs       []string `json:"refs,omitempty" jsonschema:"round refs (match_id/round) in play order; list_reports leaves them out"`
	StudioPath string   `json:"studio_path,omitempty" jsonschema:"opens the list's rounds in the Replay, as its button in Informes does"`
}

type reportRoundListIn struct {
	Title  string   `json:"title" jsonschema:"the list's name, such as Split B con humo en ventana; at most 120 characters"`
	Rounds []string `json:"rounds" jsonschema:"round refs (match_id/round) in play order, from find_rounds or a finding's refs; 1 to 200"`
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
	Attack  []teamAttackOut    `json:"attack" jsonschema:"the report's Ataque tab: team_attack over its matches, one per map; empty for a CT report"`
}

// maxReportAttackMaps caps the maps get_report reads the T side of, most played first.
const maxReportAttackMaps = 3

type createReportIn struct {
	Title      string              `json:"title" jsonschema:"the report's name"`
	TeamID     string              `json:"team_id" jsonschema:"the scouted team, id from list_teams; it cannot change later"`
	MatchIDs   []string            `json:"match_ids,omitempty" jsonschema:"the matches the report reads, ids from list_matches where the team played; at most 50"`
	Side       string              `json:"side,omitempty" jsonschema:"CT or T; empty means both"`
	Notes      string              `json:"notes,omitempty" jsonschema:"the prep in Markdown: # headings, - and 1. lists, **bold**, *italic*, code spans, pipe tables, bare http(s) addresses and [text](studio_path) links to Studio pages (a relative path such as /replay?...), at most 20000 characters; Informes and the exported report render it"`
	RoundLists []reportRoundListIn `json:"round_lists,omitempty" jsonschema:"named round lists, at most 30: one per finding the user should watch, each round of one of match_ids; Informes shows each with a button that opens its rounds in the Replay"`
}

type updateReportIn struct {
	ReportID   string               `json:"report_id" jsonschema:"report id from list_reports"`
	Title      string               `json:"title,omitempty" jsonschema:"new name; empty keeps it"`
	MatchIDs   *[]string            `json:"match_ids,omitempty" jsonschema:"the full new match list; omit to keep it"`
	Side       *string              `json:"side,omitempty" jsonschema:"CT, T, or empty for both; omit to keep it"`
	Notes      *string              `json:"notes,omitempty" jsonschema:"the full new notes in Markdown, as create_report takes them, at most 20000 characters; omit to keep them"`
	RoundLists *[]reportRoundListIn `json:"round_lists,omitempty" jsonschema:"the full new set of named round lists, each round of one of the report's matches; omit to keep them, [] clears them. Changing match_ids alone drops the listed rounds of matches taken out"`
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
				out.Reports = append(out.Reports, reportSummaryOf(r))
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_report", Annotations: readOnly,
		Description: "Read one scouting report: its notes, its scouting sections over its matches and side (as scout_team), the scouted players' metrics " +
			"and its Ataque tab (team_attack per map, with a studio_path on every finding)."},
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
			out := reportDetailOut{reportOut: reportOf(r), Scout: summarizeScout(scout), Players: []playerMetricsOut{}, Attack: []teamAttackOut{}}
			for _, p := range metrics.Players {
				out.Players = append(out.Players, metricsOf(p))
			}
			if r.Side == "CT" {
				return nil, out, nil
			}
			for i, m := range scout.Maps {
				if i == maxReportAttackMaps {
					break
				}
				attack, err := c.TeamAttackReport(ctx, r.TeamID, tuiclient.AnalysisQuery{Map: m.Map, Matches: r.MatchIDs})
				if err != nil {
					return nil, reportDetailOut{}, toolError(err)
				}
				row := summarizeAttack(attack)
				row.StudioPath = preparePath(r.TeamID, tuiclient.AnalysisQuery{Map: m.Map})
				out.Attack = append(out.Attack, row)
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "create_report", Annotations: &mcp.ToolAnnotations{},
		Description: "Save a scouting report on one team, as Nuevo informe does: a title, the matches it reads (where the team played), a side, " +
			"notes in Markdown (up to 20000 characters, a whole match prep) and named round lists that Informes opens in the Replay."},
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
		Description: "Edit a scouting report's title, matches, side, notes or round lists, to keep refining a prep in the same conversation. " +
			"Notes and round_lists are replaced whole. The scouted team cannot change; make a new report instead."},
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
	if body.Notes, err = checkNotes(&in.Notes, maxReportNotesRunes); err != nil {
		return body, err
	}
	body.RoundLists, err = reportRoundLists(in.RoundLists)
	return body, err
}

func (in updateReportIn) input() (tuiclient.ReportInput, error) {
	var body tuiclient.ReportInput
	if err := checkUUID("report_id", in.ReportID); err != nil {
		return body, err
	}
	if in.Title == "" && in.MatchIDs == nil && in.Side == nil && in.Notes == nil && in.RoundLists == nil {
		return body, errors.New("set title, match_ids, side, notes or round_lists")
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
		if body.Notes, err = checkNotes(in.Notes, maxReportNotesRunes); err != nil {
			return body, err
		}
	}
	if in.RoundLists != nil {
		body.RoundLists, err = reportRoundLists(*in.RoundLists)
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

func checkNotes(raw *string, limit int) (*string, error) {
	notes := strings.TrimSpace(*raw)
	if utf8.RuneCountInString(notes) > limit {
		return nil, fmt.Errorf("notes take at most %d characters", limit)
	}
	return &notes, nil
}

func reportRoundLists(in []reportRoundListIn) (*[]tuiclient.ReportRoundList, error) {
	if len(in) > maxReportRoundLists {
		return nil, fmt.Errorf("a report holds at most %d round lists", maxReportRoundLists)
	}
	out := make([]tuiclient.ReportRoundList, 0, len(in))
	for _, l := range in {
		title, err := checkUserText("round list title", l.Title)
		if err != nil {
			return nil, err
		}
		if len(l.Rounds) == 0 || len(l.Rounds) > maxPlaylistRounds {
			return nil, fmt.Errorf("round list %q holds from 1 to %d round refs", title, maxPlaylistRounds)
		}
		list := tuiclient.ReportRoundList{Title: title, Rounds: make([]tuiclient.ReportRound, 0, len(l.Rounds))}
		for _, ref := range l.Rounds {
			matchID, round, err := parseRoundRef(ref)
			if err != nil {
				return nil, err
			}
			list.Rounds = append(list.Rounds, tuiclient.ReportRound{MatchID: matchID, Round: round})
		}
		out = append(out, list)
	}
	return &out, nil
}

func reportOf(r tuiclient.LibraryReport) reportOut {
	return reportOut{ID: r.ID, Title: cleanName(r.Title), TeamID: r.TeamID, TeamName: cleanName(r.TeamName), MatchIDs: r.MatchIDs,
		Side: r.Side, Notes: cleanText(r.Notes, maxReportNotesRunes), RoundLists: roundListsOf(r.RoundLists, true), Updated: r.UpdatedAt.Format(time.DateOnly),
		StudioPath: "/informes/" + url.PathEscape(r.ID)}
}

// reportSummaryOf is a list row: the start of the notes and each list without its refs.
func reportSummaryOf(r tuiclient.LibraryReport) reportOut {
	out := reportOf(r)
	out.Notes = cleanText(r.Notes, maxListedNotesRunes)
	out.RoundLists = roundListsOf(r.RoundLists, false)
	return out
}

func roundListsOf(lists []tuiclient.ReportRoundList, withRefs bool) []reportRoundListOut {
	out := make([]reportRoundListOut, 0, len(lists))
	for _, l := range lists {
		row := reportRoundListOut{Title: cleanName(l.Title), Rounds: len(l.Rounds)}
		refs := make([]string, 0, len(l.Rounds))
		for _, r := range l.Rounds {
			refs = append(refs, refString(tuiclient.Evidence{MatchID: r.MatchID, Round: r.Round}))
		}
		if path, err := playlistPath(refs[:min(len(refs), maxPlaylistRounds)], row.Title); err == nil {
			row.StudioPath = path
		}
		if withRefs {
			row.Refs = refs
		}
		out = append(out, row)
	}
	return out
}
