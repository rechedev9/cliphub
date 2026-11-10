package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// proFacetRows caps the teams, events and maps list_pro_matches names for filtering.
const proFacetRows = 25

type listProMatchesIn struct {
	Days  int    `json:"days,omitempty" jsonschema:"period in days back from today, 1 to 120; default 7"`
	Team  string `json:"team,omitempty" jsonschema:"an HLTV team id or part of a team name"`
	Event string `json:"event,omitempty" jsonschema:"part of an event name"`
	Map   string `json:"map,omitempty" jsonschema:"such as de_mirage"`
	Page  int    `json:"page,omitempty" jsonschema:"default 1; 30 matches a page"`
}

type proMatchTeamOut struct {
	ID       int    `json:"id" jsonschema:"HLTV team id, for team and follow_pro_team"`
	Name     string `json:"name" jsonschema:"untrusted team name"`
	Rank     int    `json:"rank,omitempty" jsonschema:"HLTV world ranking, missing outside it"`
	Followed bool   `json:"followed"`
}

type proMatchMapOut struct {
	Entry           string `json:"entry" jsonschema:"the map to pass to import_pro_match, as the catalog names it"`
	Map             string `json:"map"`
	Rounds          int    `json:"rounds"`
	Demos           int    `json:"demos" jsonschema:"demos the map was played in; above 1 the server restarted and Studio joins them"`
	AnalysisVersion int    `json:"analysis_version"`
	OldAnalysis     bool   `json:"old_analysis,omitempty" jsonschema:"older than the current scan: some numbers may be missing until the server prepares it again"`
	LibraryMatchID  string `json:"library_match_id,omitempty" jsonschema:"set once the map is in the library"`
	RadarMayBeOff   bool   `json:"radar_may_be_off,omitempty" jsonschema:"the analysis is older than this map's radar calibration, so the 2D may draw positions off"`
}

type proMatchOut struct {
	HLTVMatchID int               `json:"hltv_match_id"`
	Event       string            `json:"event" jsonschema:"untrusted event name"`
	PlayedAt    string            `json:"played_at" jsonschema:"YYYY-MM-DD"`
	HLTVURL     string            `json:"hltv_url" jsonschema:"the HLTV match page, where its .dem files can be downloaded to record clips"`
	Teams       []proMatchTeamOut `json:"teams"`
	Maps        []proMatchMapOut  `json:"maps"`
}

type proCountOut struct {
	ID      int    `json:"id,omitempty"`
	Name    string `json:"name"`
	Matches int    `json:"matches"`
}

type proFollowOut struct {
	TeamID     int    `json:"team_id"`
	Name       string `json:"name"`
	FollowedAt string `json:"followed_at" jsonschema:"YYYY-MM-DD"`
	SyncedAt   string `json:"synced_at,omitempty" jsonschema:"when Studio last imported its new maps"`
}

type listProMatchesOut struct {
	UpdatedAt string         `json:"updated_at,omitempty"`
	Days      int            `json:"days"`
	Page      int            `json:"page"`
	Pages     int            `json:"pages"`
	Total     int            `json:"total" jsonschema:"matches the filter matched"`
	Matches   []proMatchOut  `json:"matches"`
	Teams     []proCountOut  `json:"teams" jsonschema:"teams of the period by matches, for the team filter"`
	Events    []proCountOut  `json:"events"`
	Maps      []proCountOut  `json:"maps"`
	Follows   []proFollowOut `json:"follows"`
}

type importProMatchIn struct {
	HLTVMatchID int    `json:"hltv_match_id" jsonschema:"from list_pro_matches"`
	Entry       string `json:"entry" jsonschema:"the map's entry from list_pro_matches"`
}

type importProMatchOut struct {
	MatchID       string `json:"match_id" jsonschema:"the library match, for get_match and the analysis tools"`
	DemoID        string `json:"demo_id"`
	Created       bool   `json:"created" jsonschema:"false when the map was already in the library"`
	StudioPath    string `json:"studio_path" jsonschema:"the path that opens the match on its Replay tab in Studio"`
	RadarMayBeOff bool   `json:"radar_may_be_off,omitempty" jsonschema:"the analysis is older than this map's radar calibration, so the 2D may draw positions off"`
}

type followProTeamIn struct {
	TeamID int    `json:"team_id" jsonschema:"HLTV team id from list_pro_matches"`
	Name   string `json:"name" jsonschema:"the team's name, as list_pro_matches shows it"`
}

type unfollowProTeamIn struct {
	TeamID int `json:"team_id" jsonschema:"HLTV team id from list_pro_follows"`
}

type proFollowsOut struct {
	Follows []proFollowOut `json:"follows"`
}

func registerProMatchTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "list_pro_matches", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "List recent HLTV pro matches the ClipHub community server analysed (Partidas pro), newest first, filtered by period, team, event and map. " +
			"Each map has an entry for import_pro_match and library_match_id once imported. The answer also lists the period's teams, events and maps and the followed teams."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listProMatchesIn) (*mcp.CallToolResult, listProMatchesOut, error) {
			q, err := in.query()
			if err != nil {
				return nil, listProMatchesOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, listProMatchesOut{}, toolError(err)
			}
			page, err := c.ListProMatches(ctx, q)
			if err != nil {
				return nil, listProMatchesOut{}, proMatchesError(err)
			}
			if err := proCatalogStatus(page.Status); err != nil {
				return nil, listProMatchesOut{}, err
			}
			return nil, proMatchesOf(page), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "import_pro_match", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Import one map of a pro match into the library, as Abrir en el Replay does on Partidas pro: Studio downloads its analysis (never the demo) " +
			"and the match opens in the Replay and the analysis tools at once. It cannot be analysed again or recorded until its .dem is imported with import_demo. " +
			"A map played over several demos after a server restart comes in as one match. Importing it again returns the same match, " +
			"or swaps in the newer analysis when the server prepared the map again; delete_demo removes it."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in importProMatchIn) (*mcp.CallToolResult, importProMatchOut, error) {
			entry := strings.TrimSuffix(strings.TrimSpace(in.Entry), ".dem")
			if in.HLTVMatchID <= 0 || !community.ProEntryPattern.MatchString(entry) {
				return nil, importProMatchOut{}, errors.New("hltv_match_id and entry must come from list_pro_matches")
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, importProMatchOut{}, toolError(err)
			}
			res, err := c.ImportProMatch(ctx, in.HLTVMatchID, entry)
			if err != nil {
				return nil, importProMatchOut{}, proMatchesError(err)
			}
			return nil, importProMatchOut{MatchID: res.MatchID.String(), DemoID: res.DemoID.String(), Created: res.Created,
				StudioPath: res.StudioPath, RadarMayBeOff: res.RadarMayBeOff}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "follow_pro_team", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Follow an HLTV team, as Seguir equipo does on Partidas pro: Studio imports its last five maps, its maps of the last two weeks and every new one after each catalog refresh. " +
			"unfollow_pro_team stops it."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in followProTeamIn) (*mcp.CallToolResult, proFollowOut, error) {
			name := strings.TrimSpace(in.Name)
			if in.TeamID <= 0 || name == "" {
				return nil, proFollowOut{}, errors.New("team_id and name must come from list_pro_matches")
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, proFollowOut{}, toolError(err)
			}
			f, err := c.FollowProTeam(ctx, in.TeamID, name)
			if err != nil {
				return nil, proFollowOut{}, toolError(err)
			}
			return nil, proFollowOf(f), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "unfollow_pro_team", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Stop following an HLTV team. The maps already imported stay in the library."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in unfollowProTeamIn) (*mcp.CallToolResult, empty, error) {
			if in.TeamID <= 0 {
				return nil, empty{}, errors.New("team_id must come from list_pro_follows")
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, empty{}, toolError(err)
			}
			return nil, empty{}, toolError(c.UnfollowProTeam(ctx, in.TeamID))
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_pro_follows", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "List the HLTV teams Studio follows, with when it last imported their new pro maps."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, proFollowsOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, proFollowsOut{}, toolError(err)
			}
			follows, err := c.ListProFollows(ctx)
			if err != nil {
				return nil, proFollowsOut{}, toolError(err)
			}
			out := proFollowsOut{Follows: make([]proFollowOut, 0, len(follows))}
			for _, f := range follows {
				out.Follows = append(out.Follows, proFollowOf(f))
			}
			return nil, out, nil
		})
}

func (in listProMatchesIn) query() (community.ProMatchQuery, error) {
	q := community.ProMatchQuery{Days: in.Days, Team: strings.TrimSpace(in.Team), Event: strings.TrimSpace(in.Event),
		Map: strings.ToLower(strings.TrimSpace(in.Map)), Page: in.Page}
	if q.Days == 0 {
		q.Days = community.DefaultProMatchDays
	}
	if q.Page == 0 {
		q.Page = 1
	}
	_, err := community.ParseProMatchQuery(q.Values())
	return q, err
}

// proCatalogStatus explains a catalog Studio could not read.
func proCatalogStatus(status string) error {
	switch status {
	case "disabled":
		return errors.New("the ClipHub community server is turned off in this Studio, and pro matches come from it")
	case "missing":
		return errors.New("the ClipHub community server has no pro match catalog yet; try again after it is updated")
	case "unavailable":
		return errors.New("the ClipHub community server did not answer; try again later")
	}
	return nil
}

func proMatchesError(err error) error {
	switch tuiclient.ErrorCode(err) {
	case "community_disabled":
		return proCatalogStatus("disabled")
	case "community_unavailable":
		return proCatalogStatus("unavailable")
	case "no_pro_map":
		return errors.New("the community server has no analysis of this map; list_pro_matches lists the maps it has")
	case "pro_map_unreadable":
		return errors.New("the community server sent an analysis of this map that Studio cannot read; import another map or retry after the server is updated")
	case "pro_map_pending":
		return errors.New("the demo of this pro map is in the library and still being analysed; wait_for_demo, then get_match")
	}
	return toolError(err)
}

func proMatchesOf(page tuiclient.ProMatches) listProMatchesOut {
	out := listProMatchesOut{Days: page.Days, Page: page.Page, Pages: page.Pages, Total: page.Total, Matches: make([]proMatchOut, 0, len(page.Matches)),
		Teams: []proCountOut{}, Events: []proCountOut{}, Maps: []proCountOut{}, Follows: make([]proFollowOut, 0, len(page.Follows))}
	if page.UpdatedAt != nil && !page.UpdatedAt.IsZero() {
		out.UpdatedAt = page.UpdatedAt.Format(time.DateOnly)
	}
	for _, m := range page.Matches {
		row := proMatchOut{HLTVMatchID: m.ID, Event: cleanName(m.Event), PlayedAt: m.PlayedAt.Format(time.DateOnly), HLTVURL: community.ProMatchURL(m.ID),
			Teams: []proMatchTeamOut{}, Maps: []proMatchMapOut{}}
		for _, t := range m.Teams {
			row.Teams = append(row.Teams, proMatchTeamOut{ID: t.ID, Name: cleanName(t.Name), Rank: t.Rank, Followed: t.Followed})
		}
		for _, f := range m.Maps {
			mo := proMatchMapOut{Entry: f.Entry, Map: cleanName(f.Map), Rounds: f.Rounds, Demos: len(f.Parts), AnalysisVersion: f.AnalysisVersion,
				OldAnalysis: f.OldAnalysis, RadarMayBeOff: f.RadarMayBeOff}
			if f.LibraryMatchID != nil {
				mo.LibraryMatchID = f.LibraryMatchID.String()
			}
			row.Maps = append(row.Maps, mo)
		}
		out.Matches = append(out.Matches, row)
	}
	for _, t := range page.Facets.Teams[:min(len(page.Facets.Teams), proFacetRows)] {
		out.Teams = append(out.Teams, proCountOut{ID: t.ID, Name: cleanName(t.Name), Matches: t.Matches})
	}
	for _, e := range page.Facets.Events[:min(len(page.Facets.Events), proFacetRows)] {
		out.Events = append(out.Events, proCountOut{Name: cleanName(e.Name), Matches: e.Matches})
	}
	for _, m := range page.Facets.Maps {
		out.Maps = append(out.Maps, proCountOut{Name: cleanName(m.Name), Matches: m.Matches})
	}
	for _, f := range page.Follows {
		out.Follows = append(out.Follows, proFollowOf(f))
	}
	return out
}

func proFollowOf(f library.ProFollow) proFollowOut {
	out := proFollowOut{TeamID: f.TeamID, Name: cleanName(f.Name), FollowedAt: f.FollowedAt.Format(time.DateOnly)}
	if f.SyncedAt != nil {
		out.SyncedAt = f.SyncedAt.Format(time.RFC3339)
	}
	return out
}
