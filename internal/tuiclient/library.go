package tuiclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// LibraryDemo is one demo in the Analizar library.
type LibraryDemo struct {
	ID       string `json:"id"`
	FileName string `json:"file_name"`
	State    string `json:"state"`
	Source   string `json:"source"`
	// SourceManual is true once a person picked the source; re-analysis keeps it.
	SourceManual bool      `json:"source_manual"`
	Tags         []string  `json:"tags"`
	ErrorCode    string    `json:"error_code,omitempty"`
	Error        string    `json:"error,omitempty"`
	MatchID      *string   `json:"match_id,omitempty"`
	JobIDs       []string  `json:"job_ids"`
	SizeBytes    int64     `json:"size_bytes"`
	Imported     time.Time `json:"imported_at"`
}

// LibraryImportResult is one file's outcome of POST /api/library/demos.
type LibraryImportResult struct {
	FileName  string       `json:"file_name"`
	Status    string       `json:"status"`
	Demo      *LibraryDemo `json:"demo,omitempty"`
	ErrorCode string       `json:"error_code,omitempty"`
	Error     string       `json:"error,omitempty"`
}

// LibraryMatch is one analysed match; Players is present on single reads.
type LibraryMatch struct {
	ID         string               `json:"id"`
	DemoID     string               `json:"demo_id"`
	Map        string               `json:"map"`
	PlayedAt   time.Time            `json:"played_at"`
	RoundCount int                  `json:"round_count"`
	Teams      []LibraryMatchTeam   `json:"teams"`
	Players    []LibraryMatchPlayer `json:"players,omitempty"`
}

type LibraryMatchTeam struct {
	Key        string `json:"key"`
	StartSide  string `json:"start_side"`
	ClanName   string `json:"clan_name"`
	TeamName   string `json:"team_name,omitempty"`
	TeamID     string `json:"team_id,omitempty"`
	Assignment string `json:"assignment"`
}

type LibraryMatchPlayer struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name"`
	TeamKey   string `json:"team_key"`
	Kills     int    `json:"kills"`
	Deaths    int    `json:"deaths"`
	Assists   int    `json:"assists"`
	Damage    int    `json:"damage"`
}

// PlayerJobResponse is the body of POST /api/library/demos/{id}/jobs.
type PlayerJobResponse struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
	Reused bool   `json:"reused"`
}

// JobStatus is the GET /api/jobs/{id}?view=status document.
type JobStatus struct {
	Status        string       `json:"status"`
	FailureReason string       `json:"failure_reason,omitempty"`
	FailureCode   string       `json:"failure_code,omitempty"`
	Progress      *JobProgress `json:"progress,omitempty"`
}

type JobProgress struct {
	Stage   string `json:"stage,omitempty"`
	Done    int    `json:"done"`
	Total   int    `json:"total"`
	Percent int    `json:"percent"`
}

// Healthy reports whether the orchestrator answers /healthz, which needs no token.
func (c *Client) Healthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/healthz", nil)
	if err != nil {
		return false
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// ListLibraryMatches lists analysed matches, optionally for one map or player.
func (c *Client) ListLibraryMatches(ctx context.Context, mapName, steamID string) ([]LibraryMatch, error) {
	return c.SearchLibraryMatches(ctx, LibraryMatchQuery{Map: mapName, SteamID: steamID})
}

// LibraryMatchQuery filters GET /api/library/matches; zero fields are unset.
// From and To bound played_at, both included; Limit 0 is the server default.
type LibraryMatchQuery struct {
	Map     string
	SteamID string
	TeamID  string
	From    time.Time
	To      time.Time
	Limit   int
}

// SearchLibraryMatches lists analysed matches newest first, filtered on the server.
func (c *Client) SearchLibraryMatches(ctx context.Context, q LibraryMatchQuery) ([]LibraryMatch, error) {
	params := map[string]string{"map": q.Map, "steamid": q.SteamID, "team_id": q.TeamID}
	if !q.From.IsZero() {
		params["from"] = q.From.UTC().Format(time.RFC3339Nano)
	}
	if !q.To.IsZero() {
		params["to"] = q.To.UTC().Format(time.RFC3339Nano)
	}
	if q.Limit > 0 {
		params["limit"] = strconv.Itoa(q.Limit)
	}
	var out struct {
		Matches []LibraryMatch `json:"matches"`
	}
	err := c.getJSON(ctx, "/api/library/matches"+query(params), &out)
	return out.Matches, err
}

// GetLibraryMatch returns one match with its players.
func (c *Client) GetLibraryMatch(ctx context.Context, id string) (LibraryMatch, error) {
	var out LibraryMatch
	err := c.getJSON(ctx, "/api/library/matches/"+url.PathEscape(id), &out)
	return out, err
}

// GetLibraryDemo returns one library demo with its parse state.
func (c *Client) GetLibraryDemo(ctx context.Context, id string) (LibraryDemo, error) {
	var out LibraryDemo
	err := c.getJSON(ctx, "/api/library/demos/"+url.PathEscape(id), &out)
	return out, err
}

// ImportLibraryDemo uploads one local demo file with its tags; the scan starts on its own.
func (c *Client) ImportLibraryDemo(ctx context.Context, demoPath string, tags []string) (LibraryImportResult, error) {
	var out struct {
		Demos []LibraryImportResult `json:"demos"`
	}
	fields := make([]formField, 0, len(tags))
	for _, tag := range tags {
		fields = append(fields, formField{Name: "tag", Value: tag})
	}
	if err := c.uploadMultipart(ctx, upload{Path: "/api/library/demos", FileField: "demo", FilePath: demoPath, Fields: fields}, &out); err != nil {
		return LibraryImportResult{}, err
	}
	if len(out.Demos) != 1 {
		return LibraryImportResult{}, fmt.Errorf("import returned %d results for one demo", len(out.Demos))
	}
	return out.Demos[0], nil
}

// LibraryDemoPatch is the body of PATCH /api/library/demos/{id}; nil fields stay as they are.
type LibraryDemoPatch struct {
	Source *string   `json:"source,omitempty"`
	Tags   *[]string `json:"tags,omitempty"`
}

// UpdateLibraryDemo sets a demo's source by hand or replaces its whole tag set.
func (c *Client) UpdateLibraryDemo(ctx context.Context, id string, patch LibraryDemoPatch) (LibraryDemo, error) {
	var out LibraryDemo
	err := c.doJSON(ctx, http.MethodPatch, "/api/library/demos/"+url.PathEscape(id), patch, &out)
	return out, err
}

// AnalyzeLibraryDemo queues the library scan of a demo again.
func (c *Client) AnalyzeLibraryDemo(ctx context.Context, id string) (LibraryDemo, error) {
	var out LibraryDemo
	err := c.doJSON(ctx, http.MethodPost, "/api/library/demos/"+url.PathEscape(id)+"/analyze", nil, &out)
	return out, err
}

// DeleteLibraryDemo removes a demo with its match and analysis; videos made from it stay.
func (c *Client) DeleteLibraryDemo(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, "/api/library/demos/"+url.PathEscape(id), nil, nil)
}

// CreatePlayerJob makes a job that parses one player's POV of a library demo.
func (c *Client) CreatePlayerJob(ctx context.Context, demoID, steamID string) (PlayerJobResponse, error) {
	var out PlayerJobResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/library/demos/"+url.PathEscape(demoID)+"/jobs",
		map[string]string{"target_steamid": steamID}, &out)
	return out, err
}

// CreateMultiPOVJob makes one job that parses several teammates of a library
// demo, so one Short can mix their plays; the first SteamID is the anchor.
func (c *Client) CreateMultiPOVJob(ctx context.Context, demoID string, steamIDs []string) (PlayerJobResponse, error) {
	var out PlayerJobResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/library/demos/"+url.PathEscape(demoID)+"/jobs",
		map[string][]string{"target_steamids": steamIDs}, &out)
	return out, err
}

// GetJobStatus returns the lightweight status view of a job.
func (c *Client) GetJobStatus(ctx context.Context, id string) (JobStatus, error) {
	var out JobStatus
	err := c.getJSON(ctx, jobPath(id)+"?view=status", &out)
	return out, err
}
