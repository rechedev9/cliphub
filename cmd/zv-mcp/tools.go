package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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
	// defaultMomentsLimit keeps list_moments small; a match can have dozens of plays.
	defaultMomentsLimit = 10
	// maxErrorRunes caps server messages and failure reasons, which can quote demo content.
	maxErrorRunes = 200
	// maxDemoBytes is the server's per-demo import cap (internal/httpapi maxDemoBytes).
	maxDemoBytes = 700 << 20
	// maxSummaryPlays caps how many plays an approval summary spells out.
	maxSummaryPlays = 20
	fullDemoVariant = "gameplay-pov-60"
)

// Job statuses (internal/job) the MCP server distinguishes.
const statusRecorded = "recorded"

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

type waitJobIn struct {
	waitIn
	Variant string `json:"variant,omitempty" jsonschema:"the variant returned by generate_short or render_full_demo; pass it so the render after the capture is followed"`
}

type createJobIn struct {
	DemoID  string `json:"demo_id"`
	SteamID string `json:"steamid" jsonschema:"SteamID64 of the player whose point of view the video uses"`
	Reparse bool   `json:"reparse,omitempty" jsonschema:"parse the player again when the job already exists, for example after a Studio update or when plan_full_demo reports stale or incomplete facts"`
}

type jobOut struct {
	JobID         string `json:"job_id"`
	Status        string `json:"status"`
	Running       bool   `json:"running"`
	Reused        bool   `json:"reused,omitempty"`
	Percent       int    `json:"percent,omitempty"`
	Stage         string `json:"stage,omitempty"`
	FailureCode   string `json:"failure_code,omitempty"`
	FailureReason string `json:"failure_reason,omitempty" jsonschema:"untrusted text, may quote demo content"`
	// Renders is set once the capture is done: the outcome of each render variant.
	Renders []renderNote `json:"renders,omitempty"`
	Note    string       `json:"note,omitempty"`
}

type momentOut struct {
	SegmentID string   `json:"segment_id"`
	Round     int      `json:"round"`
	Side      string   `json:"side,omitempty" jsonschema:"CT or T"`
	ClutchVs  int      `json:"clutch_vs,omitempty" jsonschema:"for a clutch, the enemies alive when the player was the last one of the team"`
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
	Total   int         `json:"total" jsonschema:"plays the job has that pass the filter; raise limit to see more"`
}

type momentsIn struct {
	ID    string `json:"id"`
	Limit int    `json:"limit,omitempty" jsonschema:"how many plays to return, best first; default 10"`
	momentFilterIn
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
	shortChoices
}

type generateOut struct {
	NeedsApproval bool   `json:"needs_approval,omitempty"`
	ApprovalCode  string `json:"approval_code,omitempty"`
	Summary       string `json:"summary"`
	JobID         string `json:"job_id,omitempty"`
	Status        string `json:"status,omitempty"`
	Variant       string `json:"variant,omitempty" jsonschema:"pass to wait_for_job to follow the render"`
}

func registerTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	registerFullDemoTools(server, s)
	registerVideoTools(server, s)
	registerApps(server, s)
	registerShortOptions(server, s)
	registerSeriesTools(server, s)
	registerFindMomentsTool(server, s)

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
			if err := checkOptionalFilter("map", in.Map, mapNamePattern); err != nil {
				return nil, listMatchesOut{}, err
			}
			if err := checkOptionalFilter("steamid", in.SteamID, steamIDPattern); err != nil {
				return nil, listMatchesOut{}, err
			}
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
			if err := checkUUID("id", in.ID); err != nil {
				return nil, matchOut{}, err
			}
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
				out.Players = append(out.Players, playerOut{SteamID64: cleanName(p.SteamID64), Name: cleanName(p.Name), Team: teams[p.TeamKey],
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
			c, err := s.uploadClient(ctx)
			if err != nil {
				return nil, demoOut{}, toolError(err)
			}
			res, err := c.ImportLibraryDemo(ctx, in.Path)
			if err != nil {
				return nil, demoOut{}, toolError(err)
			}
			if res.Demo == nil {
				return nil, demoOut{}, fmt.Errorf("Studio rejected the demo (%s): %s", cleanName(res.ErrorCode), cleanText(res.Error, maxErrorRunes))
			}
			out := describeDemo(*res.Demo)
			out.Duplicate = res.Status == "duplicate"
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "wait_for_demo", Annotations: readOnly,
		Description: "Wait until an imported demo is analysed (state ready or failed). Returns its match_id when ready."},
		func(ctx context.Context, req *mcp.CallToolRequest, in waitIn) (*mcp.CallToolResult, demoOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, demoOut{}, err
			}
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
				done := d.State == "ready" || d.State == "failed"
				notify(ctx, req, donePercent(done, 0), "analysing demo: "+d.State)
				return done, nil
			})
			return nil, out, toolError(err)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "create_job",
		Description: "Create a video job from a library demo for one player's point of view. The job parses that player; follow with wait_for_job until \"parsed\". " +
			"An existing job for the same player is reused as it is; set reparse to parse it again."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in createJobIn) (*mcp.CallToolResult, jobOut, error) {
			if err := checkUUID("demo_id", in.DemoID); err != nil {
				return nil, jobOut{}, err
			}
			if err := checkSteamID(in.SteamID); err != nil {
				return nil, jobOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, jobOut{}, toolError(err)
			}
			res, err := c.CreatePlayerJob(ctx, in.DemoID, in.SteamID)
			if err != nil {
				return nil, jobOut{}, toolError(err)
			}
			if res.Reused && in.Reparse {
				parse, err := c.StartParse(ctx, res.JobID, in.SteamID)
				if err != nil {
					return nil, jobOut{}, toolError(err)
				}
				return nil, jobOut{JobID: res.JobID, Status: cleanName(parse.Status), Running: workerOwnsJob(parse.Status), Reused: true}, nil
			}
			return nil, jobOut{JobID: res.JobID, Status: cleanName(res.Status), Running: workerOwnsJob(res.Status), Reused: res.Reused}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "wait_for_job", Annotations: readOnly,
		Description: "Wait for a job to leave its running state (parsing, recording, rendering). Reports progress; call again while running is true. " +
			"After generate_short or render_full_demo, pass the variant they returned: the capture ends before the render does, and only the variant tells when the video is finished."},
		func(ctx context.Context, req *mcp.CallToolRequest, in waitJobIn) (*mcp.CallToolResult, jobOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, jobOut{}, err
			}
			if in.Variant != "" {
				if err := checkArtifactName("variant", in.Variant); err != nil {
					return nil, jobOut{}, err
				}
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, jobOut{}, toolError(err)
			}
			out := jobOut{JobID: in.ID}
			var variants []string
			err = poll(ctx, in.WaitSeconds, func() (bool, error) {
				st, err := c.GetJobStatus(ctx, in.ID)
				if err != nil {
					return false, err
				}
				var renders []tuiclient.RenderVariantState
				if st.Status == statusRecorded {
					if variants == nil {
						if variants, err = renderVariants(ctx, c, in.Variant); err != nil {
							return false, err
						}
					}
					if renders, err = loadRenderStates(ctx, c, in.ID, variants); err != nil {
						return false, err
					}
				}
				out = describeJob(in.ID, st, renders)
				switch {
				case out.Running:
					recentCaptures.forget(in.ID)
				case !out.Running && out.FailureCode == "" && recentCaptures.queued(in.ID):
					out.Running, out.Stage, out.Note, out.Renders = true, "capture queued", "", nil
				}
				notify(ctx, req, donePercent(!out.Running, out.Percent), jobMessage(out))
				return !out.Running, nil
			})
			return nil, out, toolError(err)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_moments", Annotations: readOnly,
		Description: "List the plays of a parsed job, best first: the same list Studio shows under \"Prepara tu Short\". Returns the best 10 unless limit says otherwise. " +
			"weapon, kinds and side keep only matching plays, for example only AWP, only clutches and aces, or only CT."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in momentsIn) (*mcp.CallToolResult, momentsOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, momentsOut{}, err
			}
			q, err := in.query(in.ID)
			if err != nil {
				return nil, momentsOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, momentsOut{}, toolError(err)
			}
			doc, err := c.FilterMoments(ctx, q)
			if err != nil {
				return nil, momentsOut{}, toolError(err)
			}
			limit := in.Limit
			if limit <= 0 {
				limit = defaultMomentsLimit
			}
			out := momentsOut{Moments: make([]momentOut, 0, min(limit, len(doc.Moments))), Total: len(doc.Moments)}
			for _, m := range doc.Moments[:min(limit, len(doc.Moments))] {
				out.Moments = append(out.Moments, describeMoment(m))
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
		Description: "Capture and edit a Short from chosen plays of one job. For a series, the plays of one call must share one job_id (one map); a Short that mixes maps is not supported, so make one Short per map. Optional format, kill_effect, transition, music and extras come from list_short_options; unset keeps the defaults. This opens CS2 full screen and records for minutes, so the user must approve it. " +
			"If the result says needs_approval, show the summary, ask the user, and call again with approve set to approval_code only if they agree. " +
			"Follow with wait_for_job, passing the returned variant."},
		func(ctx context.Context, req *mcp.CallToolRequest, in generateIn) (*mcp.CallToolResult, generateOut, error) {
			if err := checkUUID("job_id", in.JobID); err != nil {
				return nil, generateOut{}, err
			}
			if err := checkSegmentIDs(in.SegmentIDs); err != nil {
				return nil, generateOut{}, err
			}
			if in.Preset != "" {
				if err := checkArtifactName("preset", in.Preset); err != nil {
					return nil, generateOut{}, err
				}
			}
			edit, err := resolveEdit(in.shortChoices)
			if err != nil {
				return nil, generateOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, generateOut{}, toolError(err)
			}
			if in.Music != "" {
				songs, err := c.ListSongs(ctx)
				if err != nil {
					return nil, generateOut{}, toolError(err)
				}
				if err := checkMusic(in.Music, songs); err != nil {
					return nil, generateOut{}, err
				}
			}
			presets, err := c.Presets(ctx)
			if err != nil {
				return nil, generateOut{}, toolError(err)
			}
			preset, err := pickPreset(in.Preset, presets)
			if err != nil {
				return nil, generateOut{}, err
			}
			doc, err := c.GetMoments(ctx, in.JobID)
			if err != nil {
				return nil, generateOut{}, toolError(err)
			}
			plays, err := describePlays(doc.Moments, in.SegmentIDs)
			if err != nil {
				return nil, generateOut{}, err
			}
			body := tuiclient.GenerateRequest{Preset: preset, SegmentIDs: in.SegmentIDs, Music: in.Music, Edit: edit}
			summary := fmt.Sprintf("Grabar un Short con %s y el preset %s: %s. Opciones: %s. CS2 se abrirá a pantalla completa y grabará durante varios minutos.",
				plural(len(in.SegmentIDs), "jugada", "jugadas"), preset, plays, describeShortRequest(body))
			ok, err := sessionApprovals.approve(req, approvalSpec{Tool: "generate_short", JobID: in.JobID, Request: body, Summary: summary, Given: in.Approve})
			if err != nil {
				return nil, generateOut{}, err
			}
			if ok.Ask != nil {
				return ok.Ask, generateOut{}, nil
			}
			if !ok.Approved {
				return nil, generateOut{NeedsApproval: true, ApprovalCode: ok.Code, Summary: summary}, nil
			}
			res, err := c.StartShort(ctx, in.JobID, body)
			if err != nil {
				return nil, generateOut{}, toolError(err)
			}
			recentCaptures.mark(in.JobID)
			return nil, startedOut(summary, res, ""), nil
		})
}

func describeMoment(m tuiclient.Moment) momentOut {
	return momentOut{SegmentID: m.SegmentID, Round: m.Round, Side: cleanName(m.Side), ClutchVs: m.ClutchVs, Seconds: m.DurationSeconds,
		Score: m.Score, Kills: m.Events.Kills, Headshots: m.Events.Headshots, Reasons: cleanNames(m.ReasonCodes),
		Weapons: cleanNames(m.Weapons), Victims: cleanNames(m.Victims)}
}

// startedOut is the result of a started capture. The render outlives the
// capture, so the variant is what wait_for_job needs to see it finish.
func startedOut(summary string, res tuiclient.EnqueueResponse, fallbackVariant string) generateOut {
	variant := res.Variant
	if variant == "" {
		variant = fallbackVariant
	}
	follow := " Started; follow with wait_for_job."
	if variant != "" {
		follow = fmt.Sprintf(" Started; follow with wait_for_job and variant %q.", cleanName(variant))
	}
	return generateOut{Summary: summary + follow, JobID: res.ID, Status: cleanName(res.Status), Variant: cleanName(variant)}
}

func pickPreset(chosen string, list tuiclient.PresetList) (string, error) {
	if chosen == "" {
		return list.Default, nil
	}
	names := make([]string, 0, len(list.Presets))
	for _, p := range list.Presets {
		if p.Name == chosen {
			return chosen, nil
		}
		names = append(names, p.Name)
	}
	return "", fmt.Errorf("unknown preset %q; valid presets: %s", cleanName(chosen), strings.Join(names, ", "))
}

// describePlays spells out, for the approval, the plays the user agrees to. The
// data comes from list_moments, so it is what the user may have seen in Studio.
func describePlays(moments []tuiclient.Moment, ids []string) (string, error) {
	bySegment := make(map[string]tuiclient.Moment, len(moments))
	for _, m := range moments {
		bySegment[m.SegmentID] = m
	}
	parts := make([]string, 0, min(len(ids), maxSummaryPlays+1))
	for i, id := range ids {
		m, ok := bySegment[id]
		if !ok {
			return "", fmt.Errorf("segment id %s is not a play of this job; use the ids from list_moments, or for a series the segment ids listed with this job_id", id)
		}
		switch {
		case i < maxSummaryPlays:
			details := nonEmpty(cleanName(m.Player), cleanName(m.Map), fmt.Sprintf("ronda %d", m.Round), plural(m.Events.Kills, "baja", "bajas"))
			parts = append(parts, fmt.Sprintf("%s (%s)", id, strings.Join(details, ", ")))
		case i == maxSummaryPlays:
			parts = append(parts, fmt.Sprintf("y %d más", len(ids)-maxSummaryPlays))
		}
	}
	return strings.Join(parts, "; "), nil
}

// plural spells a count for the Spanish approval text: "1 jugada", "3 jugadas".
func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
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

// donePercent reports a finished wait as complete, whatever the last progress said.
func donePercent(done bool, percent int) int {
	if done {
		return 100
	}
	return percent
}

func notify(ctx context.Context, req *mcp.CallToolRequest, percent int, message string) {
	token := req.Params.GetProgressToken()
	if token == nil {
		return
	}
	_ = req.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{ProgressToken: token, Progress: float64(percent), Total: 100, Message: message})
}

// describeJob reports a job. After the capture ("recorded") the job record says
// nothing more: the render outcome lives in the render variant states, and the
// persisted progress file is never deleted, so it is no proof of work.
func describeJob(id string, st tuiclient.JobStatus, renders []tuiclient.RenderVariantState) jobOut {
	out := jobOut{JobID: id, Status: cleanName(st.Status), FailureCode: cleanName(st.FailureCode), FailureReason: cleanText(st.FailureReason, maxErrorRunes)}
	if st.Status != statusRecorded {
		out.Running = workerOwnsJob(st.Status)
		if st.Progress != nil {
			out.Percent, out.Stage = st.Progress.Percent, cleanName(st.Progress.Stage)
		}
		return out
	}
	var failed *tuiclient.RenderVariantState
	for i, r := range renders {
		out.Renders = append(out.Renders, renderNote{Variant: cleanName(r.Variant), Status: cleanName(r.Status), Error: cleanText(r.Error, maxErrorRunes)})
		switch r.Status {
		case tuiclient.RenderQueued:
			out.Running = true
			if out.Stage == "" {
				out.Stage = "render queued"
			}
		case tuiclient.RenderRendering:
			out.Running = true
			out.Stage = "rendering"
			if st.Progress != nil {
				out.Percent = st.Progress.Percent
				if st.Progress.Stage != "" {
					out.Stage = cleanName(st.Progress.Stage)
				}
			}
		case tuiclient.RenderFailed:
			failed = &renders[i]
		}
	}
	switch {
	case out.Running:
	case failed != nil:
		out.FailureCode = "render_failed"
		out.FailureReason = cleanText(failed.Error, maxErrorRunes)
		out.Note = "The render of variant " + cleanName(failed.Variant) + " failed."
	case len(renders) > 0:
		out.Note = "The render finished; list_videos has the videos."
	default:
		out.Note = "The capture is done and no render was found; pass the variant returned by generate_short or render_full_demo."
	}
	return out
}

// workerOwnsJob is true while a worker holds the job. "recorded" is not in the
// list: whether its render is still going depends on the render variant.
func workerOwnsJob(status string) bool {
	return slices.Contains([]string{"queued", "parsing", "scanning", "recording", "composing"}, status)
}

// renderVariants lists the variants wait_for_job follows: the one it was given,
// or every preset the job may have rendered.
func renderVariants(ctx context.Context, c *tuiclient.Client, variant string) ([]string, error) {
	if variant != "" {
		return []string{variant}, nil
	}
	presets, err := c.Presets(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(presets.Presets))
	for _, p := range presets.Presets {
		names = append(names, p.Name)
	}
	return names, nil
}

// loadRenderStates reads the state of each variant; one never requested (404) is skipped.
func loadRenderStates(ctx context.Context, c *tuiclient.Client, jobID string, variants []string) ([]tuiclient.RenderVariantState, error) {
	states := make([]tuiclient.RenderVariantState, 0, len(variants))
	for _, v := range variants {
		state, err := c.GetRenderVariant(ctx, jobID, v)
		if tuiclient.StatusCode(err) == http.StatusNotFound {
			continue
		}
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, nil
}

func jobMessage(j jobOut) string {
	if j.Stage != "" {
		return fmt.Sprintf("%s: %s %d%%", j.Status, j.Stage, j.Percent)
	}
	return j.Status
}

func describeDemo(d tuiclient.LibraryDemo) demoOut {
	out := demoOut{DemoID: d.ID, State: cleanName(d.State), Error: cleanText(d.Error, maxErrorRunes)}
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
	return matchSummary{ID: m.ID, DemoID: m.DemoID, Map: cleanName(m.Map), PlayedAt: m.PlayedAt.Format(time.DateOnly), RoundCount: m.RoundCount, Teams: teams}
}

// teamName falls back to the starting side, so players of a match without clan
// names can still be told apart.
func teamName(t tuiclient.LibraryMatchTeam) string {
	switch {
	case t.TeamName != "":
		return t.TeamName
	case t.ClanName != "":
		return t.ClanName
	case t.StartSide != "":
		return "Empieza " + strings.ToUpper(t.StartSide)
	}
	return t.Key
}

// checkDemoPath accepts only a regular .dem or .dem.zst file on a local drive.
// A UNC path would make Windows authenticate to whatever host the model names.
func checkDemoPath(path string) error {
	lower := strings.ToLower(path)
	if !strings.HasSuffix(lower, ".dem") && !strings.HasSuffix(lower, ".dem.zst") {
		return errors.New("path must be an absolute path to a .dem or .dem.zst file")
	}
	if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//") {
		return errors.New("network (UNC) paths are not accepted; use a file on a local drive")
	}
	if runtime.GOOS == "windows" && !localDrivePath(path) {
		return errors.New(`path must start with a local drive letter, such as C:\`)
	}
	if !filepath.IsAbs(path) {
		return errors.New("path must be an absolute path to a .dem or .dem.zst file")
	}
	// A link on C: could still point at a network share; other reparse points (OneDrive) are fine.
	if link, err := os.Lstat(path); err == nil && link.Mode()&os.ModeSymlink != 0 {
		return errors.New("links are not accepted; use the demo file itself")
	}
	info, err := os.Stat(path)
	if err != nil {
		return errors.New("cannot read that file; check the path")
	}
	if !info.Mode().IsRegular() {
		return errors.New("path is not a regular file")
	}
	if info.Size() > maxDemoBytes {
		return fmt.Errorf("the demo is %d MiB; Studio accepts at most %d MiB", info.Size()>>20, maxDemoBytes>>20)
	}
	return nil
}

// localDrivePath is true for C:\ and C:/ style paths.
func localDrivePath(path string) bool {
	if len(path) < 3 || path[1] != ':' || (path[2] != '\\' && path[2] != '/') {
		return false
	}
	letter := path[0] | 0x20
	return letter >= 'a' && letter <= 'z'
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
