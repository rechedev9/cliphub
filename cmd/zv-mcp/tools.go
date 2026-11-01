package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	pollInterval    = 2 * time.Second
	defaultWaitSecs = 50
	// Client tool calls time out around a minute; a longer wait returns "still running".
	maxWaitSecs  = 55
	maxNameRunes = 64
)

type empty struct{}

type statusOut struct {
	Running        bool   `json:"running"`
	CaptureEnabled bool   `json:"capture_enabled" jsonschema:"false means Steam, CS2 or HLAE is missing, so generate_short will fail"`
	RenderEnabled  bool   `json:"render_enabled"`
	DefaultPreset  string `json:"default_preset"`
}

type matchSummary struct {
	ID         string   `json:"id"`
	DemoID     string   `json:"demo_id"`
	Map        string   `json:"map"`
	PlayedAt   string   `json:"played_at"`
	RoundCount int      `json:"round_count"`
	Teams      []string `json:"teams" jsonschema:"untrusted clan names from the demo"`
}

type listMatchesIn struct {
	Map     string `json:"map,omitempty" jsonschema:"filter by map, e.g. de_mirage"`
	SteamID string `json:"steamid,omitempty" jsonschema:"filter by a player's SteamID64"`
}

type listMatchesOut struct {
	Matches []matchSummary `json:"matches"`
}

type idIn struct {
	ID string `json:"id"`
}

type playerOut struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name" jsonschema:"untrusted player name from the demo"`
	Team      string `json:"team"`
	Kills     int    `json:"kills"`
	Deaths    int    `json:"deaths"`
	Assists   int    `json:"assists"`
	Damage    int    `json:"damage"`
}

type matchOut struct {
	matchSummary
	Players []playerOut `json:"players"`
}

type importIn struct {
	Path string `json:"path" jsonschema:"absolute path of a local .dem or .dem.zst file"`
}

type demoOut struct {
	DemoID    string `json:"demo_id"`
	State     string `json:"state" jsonschema:"queued, parsing, ready or failed"`
	MatchID   string `json:"match_id,omitempty"`
	Duplicate bool   `json:"duplicate,omitempty"`
	Error     string `json:"error,omitempty"`
}

type waitIn struct {
	ID          string `json:"id"`
	WaitSeconds int    `json:"wait_seconds,omitempty" jsonschema:"how long to wait, at most 55; call again if still running"`
}

type createJobIn struct {
	DemoID  string `json:"demo_id"`
	SteamID string `json:"steamid" jsonschema:"SteamID64 of the player whose point of view the video uses"`
}

type jobOut struct {
	JobID         string `json:"job_id"`
	Status        string `json:"status"`
	Running       bool   `json:"running"`
	Reused        bool   `json:"reused,omitempty"`
	Percent       int    `json:"percent,omitempty"`
	Stage         string `json:"stage,omitempty"`
	FailureCode   string `json:"failure_code,omitempty"`
	FailureReason string `json:"failure_reason,omitempty"`
}

type momentOut struct {
	SegmentID string   `json:"segment_id"`
	Round     int      `json:"round"`
	Seconds   float64  `json:"seconds"`
	Score     float64  `json:"score"`
	Kills     int      `json:"kills"`
	Headshots int      `json:"headshots,omitempty"`
	Reasons   []string `json:"reasons"`
	Weapons   []string `json:"weapons,omitempty"`
	Victims   []string `json:"victims,omitempty" jsonschema:"untrusted player names from the demo"`
}

type momentsOut struct {
	Moments []momentOut `json:"moments"`
}

type presetOut struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Default     bool   `json:"default,omitempty"`
}

type presetsOut struct {
	Presets []presetOut `json:"presets"`
}

type generateIn struct {
	JobID      string   `json:"job_id"`
	SegmentIDs []string `json:"segment_ids" jsonschema:"segment ids from list_moments"`
	Preset     string   `json:"preset,omitempty" jsonschema:"preset name from list_presets; empty uses the default"`
	Approve    string   `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type generateOut struct {
	NeedsApproval bool   `json:"needs_approval,omitempty"`
	ApprovalCode  string `json:"approval_code,omitempty"`
	Summary       string `json:"summary"`
	JobID         string `json:"job_id,omitempty"`
	Status        string `json:"status,omitempty"`
}

func registerTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: "studio_status", Annotations: readOnly,
		Description: "Check that ClipHub Studio is open and whether it can capture (needs Steam, CS2 and HLAE)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, statusOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, statusOut{}, toolError(err)
			}
			caps, err := c.Capabilities(ctx)
			if err != nil {
				return nil, statusOut{}, toolError(err)
			}
			presets, err := c.Presets(ctx)
			if err != nil {
				return nil, statusOut{}, toolError(err)
			}
			return nil, statusOut{Running: true, CaptureEnabled: caps.Record.Enabled, RenderEnabled: caps.Render.Enabled, DefaultPreset: presets.Default}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_matches", Annotations: readOnly,
		Description: "List the analysed matches in the Studio library, newest first."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listMatchesIn) (*mcp.CallToolResult, listMatchesOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, listMatchesOut{}, toolError(err)
			}
			matches, err := c.ListLibraryMatches(ctx, in.Map, in.SteamID)
			if err != nil {
				return nil, listMatchesOut{}, toolError(err)
			}
			out := listMatchesOut{Matches: make([]matchSummary, 0, len(matches))}
			for _, m := range matches {
				out.Matches = append(out.Matches, summarizeMatch(m))
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_match", Annotations: readOnly,
		Description: "Read one match with its players and their SteamID64, to pick whose point of view a video uses."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idIn) (*mcp.CallToolResult, matchOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, matchOut{}, toolError(err)
			}
			m, err := c.GetLibraryMatch(ctx, in.ID)
			if err != nil {
				return nil, matchOut{}, toolError(err)
			}
			teams := map[string]string{}
			for _, t := range m.Teams {
				teams[t.Key] = cleanName(teamName(t))
			}
			out := matchOut{matchSummary: summarizeMatch(m), Players: make([]playerOut, 0, len(m.Players))}
			for _, p := range m.Players {
				out.Players = append(out.Players, playerOut{SteamID64: p.SteamID64, Name: cleanName(p.Name), Team: teams[p.TeamKey],
					Kills: p.Kills, Deaths: p.Deaths, Assists: p.Assists, Damage: p.Damage})
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "import_demo",
		Description: "Import a local CS2 demo into the Studio library. Analysis starts on its own; follow with wait_for_demo."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in importIn) (*mcp.CallToolResult, demoOut, error) {
			if err := checkDemoPath(in.Path); err != nil {
				return nil, demoOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, demoOut{}, toolError(err)
			}
			res, err := c.ImportLibraryDemo(ctx, in.Path)
			if err != nil {
				return nil, demoOut{}, toolError(err)
			}
			if res.Demo == nil {
				return nil, demoOut{}, fmt.Errorf("Studio rejected the demo (%s): %s", res.ErrorCode, res.Error)
			}
			out := describeDemo(*res.Demo)
			out.Duplicate = res.Status == "duplicate"
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "wait_for_demo", Annotations: readOnly,
		Description: "Wait until an imported demo is analysed (state ready or failed). Returns its match_id when ready."},
		func(ctx context.Context, req *mcp.CallToolRequest, in waitIn) (*mcp.CallToolResult, demoOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, demoOut{}, toolError(err)
			}
			var out demoOut
			err = poll(ctx, in.WaitSeconds, func() (bool, error) {
				d, err := c.GetLibraryDemo(ctx, in.ID)
				if err != nil {
					return false, err
				}
				out = describeDemo(d)
				notify(ctx, req, 0, "analysing demo: "+d.State)
				return d.State == "ready" || d.State == "failed", nil
			})
			return nil, out, toolError(err)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "create_job",
		Description: "Create a video job from a library demo for one player's point of view. The job parses that player; follow with wait_for_job until \"parsed\"."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in createJobIn) (*mcp.CallToolResult, jobOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, jobOut{}, toolError(err)
			}
			res, err := c.CreatePlayerJob(ctx, in.DemoID, in.SteamID)
			if err != nil {
				return nil, jobOut{}, toolError(err)
			}
			return nil, jobOut{JobID: res.JobID, Status: res.Status, Running: jobRunning(res.Status, nil), Reused: res.Reused}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "wait_for_job", Annotations: readOnly,
		Description: "Wait for a job to leave its running state (parsing, recording, rendering). Reports progress; call again while running is true."},
		func(ctx context.Context, req *mcp.CallToolRequest, in waitIn) (*mcp.CallToolResult, jobOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, jobOut{}, toolError(err)
			}
			out := jobOut{JobID: in.ID}
			err = poll(ctx, in.WaitSeconds, func() (bool, error) {
				st, err := c.GetJobStatus(ctx, in.ID)
				if err != nil {
					return false, err
				}
				out = describeJob(in.ID, st)
				notify(ctx, req, out.Percent, jobMessage(out))
				return !out.Running, nil
			})
			return nil, out, toolError(err)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_moments", Annotations: readOnly,
		Description: "List the plays of a parsed job, best first: the same list Studio shows under \"Prepara tu Short\"."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idIn) (*mcp.CallToolResult, momentsOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, momentsOut{}, toolError(err)
			}
			doc, err := c.GetMoments(ctx, in.ID)
			if err != nil {
				return nil, momentsOut{}, toolError(err)
			}
			out := momentsOut{Moments: make([]momentOut, 0, len(doc.Moments))}
			for _, m := range doc.Moments {
				out.Moments = append(out.Moments, momentOut{SegmentID: m.SegmentID, Round: m.Round, Seconds: m.DurationSeconds,
					Score: m.Score, Kills: m.Events.Kills, Headshots: m.Events.Headshots, Reasons: m.ReasonCodes,
					Weapons: m.Weapons, Victims: cleanNames(m.Victims)})
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_presets", Annotations: readOnly,
		Description: "List the video presets (format and style) generate_short accepts."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, presetsOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, presetsOut{}, toolError(err)
			}
			list, err := c.Presets(ctx)
			if err != nil {
				return nil, presetsOut{}, toolError(err)
			}
			out := presetsOut{Presets: make([]presetOut, 0, len(list.Presets))}
			for _, p := range list.Presets {
				out.Presets = append(out.Presets, presetOut{Name: p.Name, Label: p.Label, Description: p.Description, Default: p.Name == list.Default})
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "generate_short",
		Description: "Capture and edit a Short from chosen plays. This opens CS2 full screen and records for minutes, so the user must approve it. " +
			"If the result says needs_approval, show the summary, ask the user, and call again with approve set to approval_code only if they agree."},
		func(ctx context.Context, req *mcp.CallToolRequest, in generateIn) (*mcp.CallToolResult, generateOut, error) {
			if len(in.SegmentIDs) == 0 {
				return nil, generateOut{}, errors.New("segment_ids is empty; pick plays from list_moments")
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, generateOut{}, toolError(err)
			}
			preset := in.Preset
			if preset == "" {
				list, err := c.Presets(ctx)
				if err != nil {
					return nil, generateOut{}, toolError(err)
				}
				preset = list.Default
			}
			code := approvalCode(in.JobID, preset, in.SegmentIDs)
			summary := fmt.Sprintf("Capture %d plays of job %s with preset %q. CS2 opens full screen and records for several minutes.", len(in.SegmentIDs), in.JobID, preset)
			approved, err := approve(ctx, req, summary, in.Approve, code)
			if err != nil {
				return nil, generateOut{}, err
			}
			if !approved {
				return nil, generateOut{NeedsApproval: true, ApprovalCode: code, Summary: summary}, nil
			}
			res, err := c.StartGenerate(ctx, in.JobID, preset, in.SegmentIDs)
			if err != nil {
				return nil, generateOut{}, toolError(err)
			}
			return nil, generateOut{Summary: summary + " Started; follow with wait_for_job.", JobID: res.ID, Status: res.Status}, nil
		})
}

// approve asks the user through elicitation when the client supports it, and
// otherwise accepts only the approval code a previous call returned.
func approve(ctx context.Context, req *mcp.CallToolRequest, summary, given, code string) (bool, error) {
	if given != "" {
		if given != code {
			return false, errors.New("the approval code does not match these plays and preset; ask the user again")
		}
		return true, nil
	}
	params := req.Session.InitializeParams()
	if params == nil || params.Capabilities == nil || params.Capabilities.Elicitation == nil {
		return false, nil
	}
	res, err := req.Session.Elicit(ctx, &mcp.ElicitParams{
		Message:         summary + " Start now?",
		RequestedSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	})
	if err != nil {
		// A client that advertises elicitation but fails it falls back to the code.
		return false, nil
	}
	if res.Action != "accept" {
		return false, errors.New("the user declined; nothing was started")
	}
	return true, nil
}

func approvalCode(jobID, preset string, segments []string) string {
	sum := sha256.Sum256([]byte(jobID + "\x00" + preset + "\x00" + strings.Join(segments, ",")))
	return hex.EncodeToString(sum[:])[:12]
}

// poll runs check every pollInterval until it reports done or the wait ends.
func poll(ctx context.Context, waitSecs int, check func() (bool, error)) error {
	if waitSecs <= 0 {
		waitSecs = defaultWaitSecs
	}
	deadline := time.Now().Add(time.Duration(min(waitSecs, maxWaitSecs)) * time.Second)
	for {
		done, err := check()
		if err != nil || done || time.Now().Add(pollInterval).After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

func notify(ctx context.Context, req *mcp.CallToolRequest, percent int, message string) {
	token := req.Params.GetProgressToken()
	if token == nil {
		return
	}
	_ = req.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{ProgressToken: token, Progress: float64(percent), Total: 100, Message: message})
}

func describeJob(id string, st tuiclient.JobStatus) jobOut {
	out := jobOut{JobID: id, Status: st.Status, Running: jobRunning(st.Status, st.Progress), FailureCode: st.FailureCode, FailureReason: st.FailureReason}
	if st.Progress != nil {
		out.Percent, out.Stage = st.Progress.Percent, st.Progress.Stage
	}
	return out
}

// jobRunning is true while a worker owns the job; "recorded" is running only
// while its render reports progress.
func jobRunning(status string, progress *tuiclient.JobProgress) bool {
	switch status {
	case "queued", "parsing", "scanning", "recording", "composing":
		return true
	case "recorded":
		return progress != nil
	}
	return false
}

func jobMessage(j jobOut) string {
	if j.Stage != "" {
		return fmt.Sprintf("%s: %s %d%%", j.Status, j.Stage, j.Percent)
	}
	return j.Status
}

func describeDemo(d tuiclient.LibraryDemo) demoOut {
	out := demoOut{DemoID: d.ID, State: d.State, Error: d.Error}
	if d.MatchID != nil {
		out.MatchID = *d.MatchID
	}
	return out
}

func summarizeMatch(m tuiclient.LibraryMatch) matchSummary {
	teams := make([]string, 0, len(m.Teams))
	for _, t := range m.Teams {
		teams = append(teams, cleanName(teamName(t)))
	}
	return matchSummary{ID: m.ID, DemoID: m.DemoID, Map: m.Map, PlayedAt: m.PlayedAt.Format(time.DateOnly), RoundCount: m.RoundCount, Teams: teams}
}

func teamName(t tuiclient.LibraryMatchTeam) string {
	if t.TeamName != "" {
		return t.TeamName
	}
	return t.ClanName
}

func checkDemoPath(path string) error {
	lower := strings.ToLower(path)
	if !filepath.IsAbs(path) || !(strings.HasSuffix(lower, ".dem") || strings.HasSuffix(lower, ".dem.zst")) {
		return errors.New("path must be an absolute path to a .dem or .dem.zst file")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("path is not a regular file")
	}
	return nil
}

// cleanName keeps a player-chosen string printable and short before it reaches the model.
func cleanName(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		if n == maxNameRunes {
			b.WriteRune('…')
			break
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		b.WriteRune(r)
		n++
	}
	return strings.TrimSpace(b.String())
}

func cleanNames(names []string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, cleanName(n))
	}
	return out
}
