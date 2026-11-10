package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	defaultPlayList = 50
	// maxPlayList caps one list_plays page so the answer stays small; offset reads the rest.
	maxPlayList          = 200
	defaultDetectedPlays = 20
	maxDetectedPlays     = 100
	// maxImportListed caps the plays and rejections import_plays lists; the counts cover all.
	maxImportListed = 50
	// maxPlayExamples is the server's cap on a play's example rounds.
	maxPlayExamples = 50
	// maxStratbookBytes is the server's cap on an imported stratbook file.
	maxStratbookBytes = 8 << 20
	maxPlayRound      = 99
	// maxEntryTags and maxTagRunes are the server's caps on a Libreta entry's tags.
	maxEntryTags = 32
	maxTagRunes  = 40
)

var (
	playSites = []string{"A", "B", "mid", "none"}
	playKinds = []string{"execute", "default", "split", "fast", "eco_rush", "save", "hold", "retake", "aggression", "stack", "unknown"}
	statuses  = []string{"active", "practice", "draft", "review", "archived"}
)

type listPlaysIn struct {
	TeamID          string `json:"team_id,omitempty" jsonschema:"only plays of this team, id from list_teams"`
	Map             string `json:"map,omitempty" jsonschema:"only plays on this map, such as de_mirage"`
	Side            string `json:"side,omitempty" jsonschema:"CT or T"`
	Kind            string `json:"kind,omitempty" jsonschema:"only plays of this kind: execute, default, split, fast, eco_rush, save (T) or hold, retake, aggression, stack, save (CT)"`
	Status          string `json:"status,omitempty" jsonschema:"only plays with this status: active, practice, draft, review or archived"`
	Tag             string `json:"tag,omitempty" jsonschema:"only plays with this tag"`
	IncludeArchived bool   `json:"include_archived,omitempty" jsonschema:"true also lists archived plays, which are left out by default"`
	Limit           int    `json:"limit,omitempty" jsonschema:"default 50, at most 200"`
	Offset          int    `json:"offset,omitempty" jsonschema:"plays to skip, for the next page when truncated is true"`
}

type playSummaryOut struct {
	ID       string   `json:"id"`
	Title    string   `json:"title" jsonschema:"untrusted text the user wrote"`
	TeamID   string   `json:"team_id,omitempty"`
	TeamName string   `json:"team_name,omitempty" jsonschema:"untrusted team name"`
	Map      string   `json:"map"`
	Side     string   `json:"side"`
	Kind     string   `json:"kind"`
	Site     string   `json:"site"`
	Origin   string   `json:"origin" jsonschema:"round, detected, import, manual or playbook"`
	Status   string   `json:"status"`
	Tags     []string `json:"tags,omitempty" jsonschema:"untrusted text the user wrote"`
	Grenades int      `json:"grenades"`
	Examples int      `json:"examples" jsonschema:"rounds the play was read from"`
}

type listPlaysOut struct {
	Plays     []playSummaryOut `json:"plays"`
	Truncated bool             `json:"truncated,omitempty" jsonschema:"true when more plays match; call again with offset"`
}

type playIDIn struct {
	PlayID string `json:"play_id" jsonschema:"play id from list_plays"`
}

type playGrenadeOut struct {
	Kind     string  `json:"kind"`
	Seconds  float64 `json:"seconds" jsonschema:"seconds after freeze end"`
	Callout  string  `json:"callout,omitempty"`
	Player   string  `json:"player,omitempty" jsonschema:"untrusted player name from the demo"`
	Lineup   string  `json:"lineup,omitempty" jsonschema:"the known smoke lineup it matches"`
	SharePct float64 `json:"share_pct,omitempty" jsonschema:"for a detected play, the share of its rounds with this grenade"`
}

type playPositionOut struct {
	Player   string  `json:"player,omitempty" jsonschema:"untrusted player name from the demo"`
	Callout  string  `json:"callout,omitempty"`
	SharePct float64 `json:"share_pct,omitempty"`
}

type playDetailOut struct {
	playSummaryOut
	Notes            string            `json:"notes,omitempty" jsonschema:"untrusted text the user wrote"`
	PositionsSeconds float64           `json:"positions_seconds" jsonschema:"when the positions are read, seconds after freeze end"`
	GrenadeList      []playGrenadeOut  `json:"grenade_list"`
	Positions        []playPositionOut `json:"positions"`
	Refs             []string          `json:"refs" jsonschema:"example rounds (match_id/round)"`
	MissingMatchIDs  []string          `json:"missing_match_ids,omitempty" jsonschema:"example matches no longer in the library"`
	Roles            []roleOut         `json:"roles,omitempty" jsonschema:"who does what in the play"`
}

type saveRoundPlayIn struct {
	MatchID string `json:"match_id" jsonschema:"match id from list_matches or a round ref"`
	Round   int    `json:"round"`
	TeamKey string `json:"team_key" jsonschema:"whose play: the side key from get_match, ct-start or t-start"`
	Title   string `json:"title" jsonschema:"the play's name"`
	Notes   string `json:"notes,omitempty"`
}

type detectPlaysIn struct {
	TeamID string `json:"team_id" jsonschema:"team id from list_teams"`
	analysisFilterIn
	teamScopeIn
}

type detectPlaysListIn struct {
	detectPlaysIn
	Limit int `json:"limit,omitempty" jsonschema:"most repeated plays first; default 20, at most 100"`
}

type detectedPlayOut struct {
	Ref         string           `json:"ref" jsonschema:"the seed round (match_id/round); pass it to save_detected_play"`
	Map         string           `json:"map"`
	Side        string           `json:"side"`
	Kind        string           `json:"kind"`
	Site        string           `json:"site"`
	Rounds      int              `json:"rounds"`
	SharePct    float64          `json:"share_pct"`
	WinPct      float64          `json:"win_pct"`
	Reliable    bool             `json:"reliable"`
	GrenadeList []playGrenadeOut `json:"grenade_list"`
	Refs        []string         `json:"refs" jsonschema:"first rounds of the group, at most 5"`
	linkOut
}

type detectPlaysOut struct {
	Team           string            `json:"team" jsonschema:"untrusted team name"`
	Matches        int               `json:"matches"`
	Rounds         int               `json:"rounds"`
	Compared       int               `json:"compared" jsonschema:"rounds with enough utility and a buy worth comparing"`
	Plays          []detectedPlayOut `json:"plays"`
	Truncated      bool              `json:"truncated,omitempty" jsonschema:"true when more plays were found; raise limit or narrow the filter"`
	SkippedMatches int               `json:"skipped_matches,omitempty"`
}

type saveDetectedPlayIn struct {
	detectPlaysIn
	PlayRef string `json:"play_ref" jsonschema:"the ref of a play from detect_team_plays, called with the same filter"`
	Title   string `json:"title" jsonschema:"the play's name"`
}

type updatePlayIn struct {
	PlayID     string    `json:"play_id" jsonschema:"play id from list_plays"`
	Title      string    `json:"title,omitempty" jsonschema:"new name; empty keeps it"`
	Kind       string    `json:"kind,omitempty" jsonschema:"new kind, a round pattern of the play's side; empty keeps it"`
	Site       string    `json:"site,omitempty" jsonschema:"A, B, mid or none; empty keeps it"`
	Notes      *string   `json:"notes,omitempty" jsonschema:"the full new notes; omit to keep them"`
	TeamID     string    `json:"team_id,omitempty" jsonschema:"the team that runs it, id from list_teams"`
	DetachTeam bool      `json:"detach_team,omitempty" jsonschema:"true leaves the play without a team"`
	Status     string    `json:"status,omitempty" jsonschema:"active, practice, draft, review or archived; empty keeps it"`
	Tags       *[]string `json:"tags,omitempty" jsonschema:"the full new tag list; omit to keep them, [] clears them"`
	Roles      *[]roleIn `json:"roles,omitempty" jsonschema:"up to 5 roles, the whole list; omit to keep them, [] clears them"`
}

type createPlayIn struct {
	Title  string   `json:"title" jsonschema:"the play's name"`
	Map    string   `json:"map" jsonschema:"such as de_mirage"`
	Side   string   `json:"side" jsonschema:"CT or T"`
	Kind   string   `json:"kind" jsonschema:"a round pattern of the side: execute, default, split, fast, eco_rush, save (T) or hold, retake, aggression, stack, save (CT)"`
	Site   string   `json:"site,omitempty" jsonschema:"A, B, mid or none (the default)"`
	TeamID string   `json:"team_id,omitempty" jsonschema:"the team that runs it, id from list_teams"`
	Notes  string   `json:"notes,omitempty"`
	Status string   `json:"status,omitempty" jsonschema:"active (the default), practice, draft, review or archived"`
	Tags   []string `json:"tags,omitempty"`
}

type deletePlayIn struct {
	PlayID  string `json:"play_id" jsonschema:"play id from list_plays"`
	Approve string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type exportPlaysIn struct {
	Path            string `json:"path" jsonschema:"absolute path of the new .json file on a local drive; an existing file is not overwritten"`
	TeamID          string `json:"team_id,omitempty" jsonschema:"only plays of this team"`
	Map             string `json:"map,omitempty"`
	Side            string `json:"side,omitempty"`
	Kind            string `json:"kind,omitempty"`
	IncludeArchived bool   `json:"include_archived,omitempty" jsonschema:"true also exports archived plays"`
}

type exportPlaysOut struct {
	Path  string `json:"path"`
	Bytes int    `json:"bytes"`
}

type importPlaysIn struct {
	Path string `json:"path" jsonschema:"absolute path of a ClipHub stratbook .json file on a local drive"`
}

type importPlaysOut struct {
	Imported      int              `json:"imported"`
	Duplicates    int              `json:"duplicates" jsonschema:"plays already in the stratbook"`
	Rejected      []string         `json:"rejected" jsonschema:"plays that were not stored, with the reason; at most 50 listed"`
	RejectedCount int              `json:"rejected_count"`
	Plays         []playSummaryOut `json:"plays" jsonschema:"the imported plays, at most 50 listed"`
	Sections      []string         `json:"sections,omitempty" jsonschema:"what the file's other sections did, such as lineups: 3 imported, 1 duplicate, 0 rejected"`
}

func registerStratbookTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: "list_plays", Annotations: readOnly,
		Description: "List the plays of the Libreta (its Jugadas tab), filtered by team, map, side, kind, status and tag. Archived plays are left out unless include_archived is true."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listPlaysIn) (*mcp.CallToolResult, listPlaysOut, error) {
			if in.Limit < 0 || in.Limit > maxPlayList {
				return nil, listPlaysOut{}, fmt.Errorf("limit must be from 1 to %d", maxPlayList)
			}
			if in.Offset < 0 {
				return nil, listPlaysOut{}, errors.New("offset must not be negative")
			}
			q, err := playQuery(in.TeamID, in.Map, in.Side, in.Kind)
			if err != nil {
				return nil, listPlaysOut{}, err
			}
			if q.Status, err = checkStatus(in.Status); err != nil {
				return nil, listPlaysOut{}, err
			}
			if in.Tag != "" {
				tags, err := checkTags([]string{in.Tag})
				if err != nil {
					return nil, listPlaysOut{}, err
				}
				q.Tag = tags[0]
			}
			q.IncludeArchived = in.IncludeArchived
			limit := cmp.Or(in.Limit, defaultPlayList)
			q.Limit, q.Offset = limit+1, in.Offset
			c, err := s.client(ctx)
			if err != nil {
				return nil, listPlaysOut{}, toolError(err)
			}
			plays, err := c.ListPlays(ctx, q)
			if err != nil {
				return nil, listPlaysOut{}, toolError(err)
			}
			out := listPlaysOut{Plays: make([]playSummaryOut, 0, min(len(plays), limit)), Truncated: len(plays) > limit}
			for _, p := range plays[:min(len(plays), limit)] {
				out.Plays = append(out.Plays, playSummaryOf(p))
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_play", Annotations: readOnly,
		Description: "Read one stratbook play: its grenades (kind, second, callout, thrower, lineup), where each player stands, its notes and example rounds."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in playIDIn) (*mcp.CallToolResult, playDetailOut, error) {
			if err := checkUUID("play_id", in.PlayID); err != nil {
				return nil, playDetailOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, playDetailOut{}, toolError(err)
			}
			p, err := c.GetPlay(ctx, in.PlayID)
			if err != nil {
				return nil, playDetailOut{}, toolError(err)
			}
			return nil, playDetailOf(p), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "save_round_as_play", Annotations: &mcp.ToolAnnotations{},
		Description: "Save the play one team ran in one round to the stratbook, as Guardar como jugada does in the round view: " +
			"its grenades up to the commit and its positions. Round refs come from find_rounds and the team reports."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in saveRoundPlayIn) (*mcp.CallToolResult, playDetailOut, error) {
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, playDetailOut{}, err
			}
			if in.Round < 1 || in.Round > maxPlayRound {
				return nil, playDetailOut{}, fmt.Errorf("round must be from 1 to %d", maxPlayRound)
			}
			if in.TeamKey != "ct-start" && in.TeamKey != "t-start" {
				return nil, playDetailOut{}, fmt.Errorf("team_key must be ct-start or t-start, not %q", cleanName(in.TeamKey))
			}
			title, err := checkUserText("title", in.Title)
			if err != nil {
				return nil, playDetailOut{}, err
			}
			notes, err := checkNotes(&in.Notes, maxNotesRunes)
			if err != nil {
				return nil, playDetailOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, playDetailOut{}, toolError(err)
			}
			draft, err := c.RoundPlayDraft(ctx, in.MatchID, in.Round, in.TeamKey, 0)
			if err != nil {
				return nil, playDetailOut{}, toolError(err)
			}
			draft.Title, draft.Notes, draft.Origin = title, *notes, "round"
			draft.Examples = draft.Examples[:min(len(draft.Examples), maxPlayExamples)]
			p, err := c.CreatePlay(ctx, draft)
			if err != nil {
				return nil, playDetailOut{}, toolError(err)
			}
			return nil, playDetailOf(p), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "detect_team_plays", Annotations: readOnly,
		Description: "Find the plays a team repeats: its rounds grouped by shared utility and positions, per map and side, with how often and how well each runs. " +
			"Filter like scout_team. save_detected_play stores one in the stratbook. " +
			"Each play has a studio_path that opens its example rounds in the 2D Replay (rounds_linked when they are fewer than rounds)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in detectPlaysListIn) (*mcp.CallToolResult, detectPlaysOut, error) {
			if in.Limit < 0 || in.Limit > maxDetectedPlays {
				return nil, detectPlaysOut{}, fmt.Errorf("limit must be from 1 to %d", maxDetectedPlays)
			}
			limit := cmp.Or(in.Limit, defaultDetectedPlays)
			det, err := in.detect(ctx, s)
			if err != nil {
				return nil, detectPlaysOut{}, err
			}
			out := detectPlaysOut{Team: cleanName(det.TeamName), Matches: det.Matches, Rounds: det.Rounds, Compared: det.Compared,
				Plays: []detectedPlayOut{}, Truncated: len(det.Plays) > limit, SkippedMatches: len(det.Skipped)}
			for _, p := range det.Plays[:min(len(det.Plays), limit)] {
				out.Plays = append(out.Plays, detectedPlayOut{Ref: refString(p.Seed), Map: cleanName(p.Map), Side: p.Side, Kind: cleanName(p.Kind),
					Site: cleanName(p.Site), Rounds: p.Rounds, SharePct: p.Share.Pct, WinPct: p.WinRate.Pct, Reliable: p.WinRate.Reliable,
					GrenadeList: grenadesOf(p.Grenades), Refs: refStrings(p.Examples),
					linkOut: findingLink{title: joinTitle(out.Team, p.Map, p.Side, p.Kind, p.Site), refs: p.Examples}.outFor(p.Rounds)})
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "save_detected_play", Annotations: &mcp.ToolAnnotations{},
		Description: "Save a play detect_team_plays found to the stratbook, with all its rounds as examples, as Guardar does in the detection view. " +
			"Pass the same team and filter as the detect_team_plays call and the play's ref."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in saveDetectedPlayIn) (*mcp.CallToolResult, playDetailOut, error) {
			title, err := checkUserText("title", in.Title)
			if err != nil {
				return nil, playDetailOut{}, err
			}
			det, err := in.detect(ctx, s)
			if err != nil {
				return nil, playDetailOut{}, err
			}
			i := slices.IndexFunc(det.Plays, func(p tuiclient.DetectedPlay) bool { return refString(p.Seed) == in.PlayRef })
			if i < 0 {
				return nil, playDetailOut{}, errors.New("no detected play has that ref with this filter; call detect_team_plays again")
			}
			found := det.Plays[i]
			play := tuiclient.Play{Title: title, TeamID: &in.TeamID, Map: found.Map, Side: found.Side, Kind: found.Kind, Site: found.Site,
				Origin: "detected", Grenades: found.Grenades, Positions: found.Positions,
				Examples: found.Examples[:min(len(found.Examples), maxPlayExamples)]}
			if len(found.Positions) > 0 {
				play.PositionsSeconds = det.Thresholds.PositionSeconds
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, playDetailOut{}, toolError(err)
			}
			p, err := c.CreatePlay(ctx, play)
			if err != nil {
				return nil, playDetailOut{}, toolError(err)
			}
			return nil, playDetailOf(p), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "update_play", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Edit a Libreta play's title, kind, site, notes, team, status or tags. Its map and side are what the play is and do not change."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in updatePlayIn) (*mcp.CallToolResult, playDetailOut, error) {
			patch, err := in.patch()
			if err != nil {
				return nil, playDetailOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, playDetailOut{}, toolError(err)
			}
			p, err := c.UpdatePlay(ctx, in.PlayID, patch)
			if err != nil {
				return nil, playDetailOut{}, toolError(err)
			}
			return nil, playDetailOf(p), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "create_play", Annotations: &mcp.ToolAnnotations{},
		Description: "Write a new play by hand in the Libreta, as Nueva jugada does: a name, map, side and kind, with no grenades or positions yet. " +
			"Plays read from a round or a detection come from save_round_as_play and save_detected_play."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in createPlayIn) (*mcp.CallToolResult, playDetailOut, error) {
			play, err := in.play()
			if err != nil {
				return nil, playDetailOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, playDetailOut{}, toolError(err)
			}
			p, err := c.CreatePlay(ctx, play)
			if err != nil {
				return nil, playDetailOut{}, toolError(err)
			}
			return nil, playDetailOf(p), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "delete_play", Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true)},
		Description: "Delete a play from the stratbook. It cannot be undone, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in deletePlayIn) (*mcp.CallToolResult, deleteOut, error) {
			if err := checkUUID("play_id", in.PlayID); err != nil {
				return nil, deleteOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			p, err := c.GetPlay(ctx, in.PlayID)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			summary := fmt.Sprintf("Borrar la jugada %s (%s, %s) de la Libreta. No se puede deshacer.", quotedName(p.Title, "sin título"), cleanName(p.Map), p.Side)
			return approveAndDelete(req, approvalSpec{Tool: "delete_play", JobID: in.PlayID, Request: p.UpdatedAt, Summary: summary, Given: in.Approve, Question: "¿Borrar ahora?"},
				func() error { return c.DeletePlay(ctx, in.PlayID) })
		})

	mcp.AddTool(server, &mcp.Tool{Name: "export_plays", Annotations: &mcp.ToolAnnotations{},
		Description: "Export the stratbook's plays (all, or those of one team, map, side or kind) to a new stratbook .json file on this PC, as Exportar does."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in exportPlaysIn) (*mcp.CallToolResult, exportPlaysOut, error) {
			if err := checkNewLocalFile(in.Path, ".json"); err != nil {
				return nil, exportPlaysOut{}, err
			}
			q, err := playQuery(in.TeamID, in.Map, in.Side, in.Kind)
			if err != nil {
				return nil, exportPlaysOut{}, err
			}
			q.IncludeArchived = in.IncludeArchived
			c, err := s.client(ctx)
			if err != nil {
				return nil, exportPlaysOut{}, toolError(err)
			}
			file, err := c.ExportPlays(ctx, q)
			if err != nil {
				return nil, exportPlaysOut{}, toolError(err)
			}
			if err := writeNewFile(in.Path, file); err != nil {
				return nil, exportPlaysOut{}, err
			}
			return nil, exportPlaysOut{Path: in.Path, Bytes: len(file)}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "import_plays", Annotations: &mcp.ToolAnnotations{},
		Description: "Import a ClipHub Libreta (stratbook) .json file, as Importar does: its plays and, from a whole Libreta file, its lineups, " +
			"role sheets, calls and callout names. Each record is stored on its own: ones already there count as duplicates, " +
			"bad ones are rejected, and teams are found by name or created."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in importPlaysIn) (*mcp.CallToolResult, importPlaysOut, error) {
			if !strings.EqualFold(filepath.Ext(in.Path), ".json") {
				return nil, importPlaysOut{}, errors.New("path must be a .json stratbook file")
			}
			if err := checkLocalFile(in.Path, "stratbook", maxStratbookBytes); err != nil {
				return nil, importPlaysOut{}, err
			}
			file, err := readJSONFile(in.Path)
			if err != nil {
				return nil, importPlaysOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, importPlaysOut{}, toolError(err)
			}
			res, err := c.ImportPlays(ctx, file)
			if err != nil {
				return nil, importPlaysOut{}, toolError(err)
			}
			out := importPlaysOut{Imported: res.Imported, Duplicates: res.Duplicates, Rejected: []string{}, RejectedCount: len(res.Rejected), Plays: []playSummaryOut{}}
			for _, r := range res.Rejected[:min(len(res.Rejected), maxImportListed)] {
				out.Rejected = append(out.Rejected, fmt.Sprintf("%d %s: %s", r.Index, quotedName(r.Title, "sin título"), cleanText(r.Error, maxErrorRunes)))
			}
			for _, p := range res.Plays[:min(len(res.Plays), maxImportListed)] {
				out.Plays = append(out.Plays, playSummaryOf(p))
			}
			for _, key := range []string{"lineups", "team_roles", "calls", "callout_names"} {
				if sec, ok := res.Sections[key]; ok {
					out.Sections = append(out.Sections, fmt.Sprintf("%s: %d imported, %d duplicate, %d rejected", key, sec.Imported, sec.Duplicates, sec.Rejected))
				}
			}
			return nil, out, nil
		})
}

func (in detectPlaysIn) detect(ctx context.Context, s *studio) (tuiclient.PlayDetection, error) {
	if err := checkUUID("team_id", in.TeamID); err != nil {
		return tuiclient.PlayDetection{}, err
	}
	q, err := teamQuery(in.analysisFilterIn, in.teamScopeIn)
	if err != nil {
		return tuiclient.PlayDetection{}, err
	}
	c, err := s.client(ctx)
	if err != nil {
		return tuiclient.PlayDetection{}, toolError(err)
	}
	det, err := c.DetectTeamPlays(ctx, in.TeamID, q)
	if err != nil {
		return tuiclient.PlayDetection{}, toolError(err)
	}
	return det, nil
}

func (in updatePlayIn) patch() (tuiclient.PlayPatch, error) {
	var patch tuiclient.PlayPatch
	if err := checkUUID("play_id", in.PlayID); err != nil {
		return patch, err
	}
	if in.Title == "" && in.Kind == "" && in.Site == "" && in.Notes == nil && in.TeamID == "" && !in.DetachTeam && in.Status == "" && in.Tags == nil && in.Roles == nil {
		return patch, errors.New("set title, kind, site, notes, team_id, detach_team, status, tags or roles")
	}
	if in.TeamID != "" && in.DetachTeam {
		return patch, errors.New("set team_id or detach_team, not both")
	}
	if in.Title != "" {
		title, err := checkUserText("title", in.Title)
		if err != nil {
			return patch, err
		}
		patch.Title = &title
	}
	if in.Kind != "" {
		if !slices.Contains(playKinds, in.Kind) {
			return patch, fmt.Errorf("kind must be one of %s", strings.Join(playKinds, ", "))
		}
		patch.Kind = &in.Kind
	}
	if in.Site != "" {
		if !slices.Contains(playSites, in.Site) {
			return patch, errors.New("site must be A, B, mid or none")
		}
		patch.Site = &in.Site
	}
	if in.Notes != nil {
		notes, err := checkNotes(in.Notes, maxNotesRunes)
		if err != nil {
			return patch, err
		}
		patch.Notes = notes
	}
	if in.TeamID != "" {
		if err := checkUUID("team_id", in.TeamID); err != nil {
			return patch, err
		}
		patch.TeamID = &in.TeamID
	}
	patch.ClearTeam = in.DetachTeam
	if in.Status != "" {
		status, err := checkStatus(in.Status)
		if err != nil {
			return patch, err
		}
		patch.Status = &status
	}
	if in.Tags != nil {
		tags, err := checkTags(*in.Tags)
		if err != nil {
			return patch, err
		}
		patch.Tags = &tags
	}
	if in.Roles != nil {
		roles, err := checkRoles("roles", *in.Roles)
		if err != nil {
			return patch, err
		}
		patch.Roles = &roles
	}
	return patch, nil
}

func (in createPlayIn) play() (tuiclient.Play, error) {
	title, err := checkUserText("title", in.Title)
	if err != nil {
		return tuiclient.Play{}, err
	}
	if in.Map == "" || in.Side == "" || in.Kind == "" {
		return tuiclient.Play{}, errors.New("map, side and kind are required")
	}
	q, err := playQuery(in.TeamID, in.Map, in.Side, in.Kind)
	if err != nil {
		return tuiclient.Play{}, err
	}
	site := cmp.Or(in.Site, "none")
	if !slices.Contains(playSites, site) {
		return tuiclient.Play{}, errors.New("site must be A, B, mid or none")
	}
	notes, err := checkNotes(&in.Notes, maxNotesRunes)
	if err != nil {
		return tuiclient.Play{}, err
	}
	status, err := checkStatus(in.Status)
	if err != nil {
		return tuiclient.Play{}, err
	}
	tags, err := checkTags(in.Tags)
	if err != nil {
		return tuiclient.Play{}, err
	}
	p := tuiclient.Play{Title: title, Map: q.Map, Side: q.Side, Kind: q.Kind, Site: site, Origin: "manual",
		Status: status, Tags: tags}
	if notes != nil {
		p.Notes = *notes
	}
	if q.TeamID != "" {
		p.TeamID = &q.TeamID
	}
	return p, nil
}

// checkStatus accepts an empty status (unset) or one of statuses.
func checkStatus(raw string) (string, error) {
	if raw != "" && !slices.Contains(statuses, raw) {
		return "", fmt.Errorf("status must be one of %s", strings.Join(statuses, ", "))
	}
	return raw, nil
}

// checkTags refuses empty, long or control-character tags; the server dedupes and sorts them.
func checkTags(raw []string) ([]string, error) {
	if len(raw) > maxEntryTags {
		return nil, fmt.Errorf("at most %d tags", maxEntryTags)
	}
	out := make([]string, 0, len(raw))
	for _, tag := range raw {
		clean, err := checkUserText("tag", tag)
		if err != nil {
			return nil, err
		}
		if len([]rune(clean)) > maxTagRunes {
			return nil, fmt.Errorf("a tag has at most %d characters", maxTagRunes)
		}
		out = append(out, clean)
	}
	return out, nil
}

func playQuery(teamID, mapName, side, kind string) (tuiclient.PlayQuery, error) {
	var q tuiclient.PlayQuery
	if teamID != "" {
		if err := checkUUID("team_id", teamID); err != nil {
			return q, err
		}
	}
	if err := checkOptionalFilter("map", mapName, mapNamePattern); err != nil {
		return q, err
	}
	parsed, err := parseSide(side)
	if err != nil {
		return q, err
	}
	if kind != "" && !slices.Contains(playKinds, kind) {
		return q, fmt.Errorf("kind must be one of %s", strings.Join(playKinds, ", "))
	}
	return tuiclient.PlayQuery{TeamID: teamID, Map: strings.ToLower(mapName), Side: parsed, Kind: kind}, nil
}

func playSummaryOf(p tuiclient.Play) playSummaryOut {
	out := playSummaryOut{ID: p.ID, Title: cleanName(p.Title), TeamName: cleanName(p.TeamName), Map: cleanName(p.Map), Side: p.Side,
		Kind: cleanName(p.Kind), Site: cleanName(p.Site), Origin: p.Origin, Status: cleanName(p.Status),
		Grenades: len(p.Grenades), Examples: len(p.Examples)}
	for _, tag := range p.Tags {
		out.Tags = append(out.Tags, cleanName(tag))
	}
	if p.TeamID != nil {
		out.TeamID = *p.TeamID
	}
	return out
}

func playDetailOf(p tuiclient.Play) playDetailOut {
	out := playDetailOut{playSummaryOut: playSummaryOf(p), Notes: cleanText(p.Notes, maxNotesRunes), PositionsSeconds: p.PositionsSeconds,
		GrenadeList: grenadesOf(p.Grenades), Positions: []playPositionOut{}, Refs: []string{}, MissingMatchIDs: p.MissingMatchIDs}
	for _, pos := range p.Positions {
		out.Positions = append(out.Positions, playPositionOut{Player: cleanName(pos.Player), Callout: cleanName(pos.Callout), SharePct: sharePct(pos.Share)})
	}
	for _, e := range p.Examples {
		out.Refs = append(out.Refs, refString(e))
	}
	out.Roles = rolesOf(p.Roles)
	return out
}

func grenadesOf(gs []tuiclient.PlayGrenade) []playGrenadeOut {
	out := make([]playGrenadeOut, 0, len(gs))
	for _, g := range gs {
		out = append(out, playGrenadeOut{Kind: cleanName(g.Kind), Seconds: math.Round(g.Seconds*10) / 10, Callout: cleanName(g.Callout),
			Player: cleanName(g.Player), Lineup: cleanName(g.Lineup), SharePct: sharePct(g.Share)})
	}
	return out
}

// sharePct turns a 0..1 share into a percent with one decimal.
func sharePct(share float64) float64 { return math.Round(share*1000) / 10 }

func refString(e tuiclient.Evidence) string {
	return cleanName(e.MatchID) + "/" + strconv.Itoa(e.Round)
}
