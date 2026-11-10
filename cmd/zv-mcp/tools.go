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

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	pollInterval    = 2 * time.Second
	defaultWaitSecs = 50
	// Client tool calls time out around a minute; a longer wait returns "still running".
	maxWaitSecs = 55
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
	StreamLinks    bool   `json:"stream_links" jsonschema:"false means import_stream cannot download Twitch, Kick or YouTube links (yt-dlp is missing); a local .mp4 still works"`
	DefaultPreset  string `json:"default_preset"`
	StudioVersion  string `json:"studio_version,omitempty" jsonschema:"the Studio release; cite it in a bug report"`
	SupportCode    string `json:"support_code,omitempty" jsonschema:"lets the ClipHub team find this installation's diagnostics; cite it in a bug report"`
	LogPath        string `json:"log_path,omitempty" jsonschema:"studio.log of this installation; read the lines near a failure for a bug report"`
}

type matchSummary struct {
	ID         string   `json:"id"`
	DemoID     string   `json:"demo_id"`
	Map        string   `json:"map"`
	PlayedAt   string   `json:"played_at"`
	RoundCount int      `json:"round_count"`
	Teams      []string `json:"teams" jsonschema:"untrusted clan names from the demo"`
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
	Sides   []sideOut   `json:"sides" jsonschema:"each side's key, clan name and library team; assign_match_team corrects one"`
	Players []playerOut `json:"players"`
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
	Player    string   `json:"player,omitempty" jsonschema:"multi-POV jobs only: untrusted name of the player whose POV the play uses"`
	SteamID   string   `json:"steamid,omitempty" jsonschema:"multi-POV jobs only: that player's SteamID64"`
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
	KilledBy  string   `json:"killed_by,omitempty" jsonschema:"death clips only: untrusted name of the killer"`
	Weapon    string   `json:"death_weapon,omitempty" jsonschema:"death clips only: the weapon that killed the player"`
}

type momentsOut struct {
	Moments []momentOut `json:"moments"`
	Total   int         `json:"total" jsonschema:"plays the job has that pass the filter; raise limit to see more"`
}

type momentsIn struct {
	ID     string `json:"id"`
	Limit  int    `json:"limit,omitempty" jsonschema:"how many plays to return, best first; default 10, or every death with deaths"`
	Deaths bool   `json:"deaths,omitempty" jsonschema:"true lists every death of the player instead, in match order: one clip from 8 s before each death, for a death reel"`
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
	Deaths     bool     `json:"deaths,omitempty" jsonschema:"true when segment_ids come from list_moments with deaths: a reel of the player's deaths"`
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
	registerRoundPOVTool(server, s)
	registerProSettingsTool(server)
	registerVideoTools(server, s)
	registerVideoLibraryTools(server, s)
	registerApps(server, s)
	registerShortOptions(server, s)
	registerSeriesTools(server, s)
	registerMultiPOVTools(server, s)
	registerFindMomentsTool(server, s)
	registerAnalysisTools(server, s)
	registerTeamTools(server, s)
	registerPlayerTools(server, s)
	registerReportTools(server, s)
	registerStratbookTools(server, s)
	registerLibretaTableTools(server, s)
	registerLineupTools(server, s)
	registerProLineupTools(server, s)
	registerProGrenadeMetaTool(server, s)
	registerProMatchTools(server, s)
	registerRoleTools(server, s)
	registerCallTools(server, s)
	registerSearchTool(server, s)
	registerMatchTacticalTools(server, s)
	registerLibrarySeriesTools(server, s)
	registerCommunityTools(server, s)
	registerRoundTools(server, s)
	registerTeamRoundsTool(server, s)
	registerReplayTools(server, s)
	registerOpenInStudioTools(server, s)
	registerBotReplayTools(server, s)
	registerHeatmapTools(server, s)
	registerPlaylistTools(server, s)
	registerReplayGIFTool(server, s)
	registerReplayVideoTool(server, s)
	registerPatternTools(server, s)
	registerVoiceTools(server, s)
	registerTeamReportTools(server, s)
	registerTeamSideStatsTool(server, s)
	registerTeamAttackTool(server, s)
	registerZoneMapTool(server, s)
	registerMatchPrepPrompt(server)
	registerPlaybookComboTools(server, s)
	registerTeamGrenadesTool(server, s)
	registerPlayerReportTools(server, s)
	registerStreamTools(server, s)
	registerIntegrityTools(server, s)
	registerRealMapTools(server, s)
	registerDeleteTools(server, s)
	registerLibraryDemoTools(server, s)
	registerStorageTools(server, s)
	registerSteamTools(server, s)

	mcp.AddTool(server, &mcp.Tool{Name: "studio_status", Annotations: readOnly,
		Description: "Check that ClipHub Studio is open and whether it can capture (needs Steam, CS2 and HLAE). Also returns the Studio version, support code and studio.log path a bug report needs."},
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
			ev := s.reportEvidence()
			return nil, statusOut{Running: true, CaptureEnabled: caps.Record.Enabled, RenderEnabled: caps.Render.Enabled, StreamLinks: caps.Stream.YtdlpEnabled, DefaultPreset: presets.Default,
				StudioVersion: ev.studioVersion, SupportCode: ev.supportCode, LogPath: ev.logPath}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_matches", Annotations: readOnly,
		Description: "List the analysed matches in the Studio library as Partidas shows them: newest first by the match date (sort imported: by the import date), " +
			"each with its tipo (kind), source, tags and, when it belongs to a series, the series format, event, veto and how its map was chosen (pick or decider). " +
			"has_voice and voice_sync say whether the match has team voice and how it is synced (the voice icon of the row; sync_voice with auto true is Volver a sincronizar voz). " +
			"kind, has_pov, has_voice, team, opponent and event are the Tipo de partida, Con POV, Con voz, Equipo, Rival and Evento controls; every page of the library is read before they apply."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listMatchesIn) (*mcp.CallToolResult, listMatchesOut, error) {
			if err := in.check(); err != nil {
				return nil, listMatchesOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, listMatchesOut{}, toolError(err)
			}
			out, err := listMatches(ctx, c, in)
			return nil, out, toolError(err)
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
			out := matchOut{matchSummary: summarizeMatch(m), Sides: sidesOf(m), Players: make([]playerOut, 0, len(m.Players))}
			for _, p := range m.Players {
				out.Players = append(out.Players, playerOut{SteamID64: cleanName(p.SteamID64), Name: cleanName(p.Name), Team: teams[p.TeamKey],
					Kills: p.Kills, Deaths: p.Deaths, Assists: p.Assists, Damage: p.Damage})
			}
			return nil, out, nil
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

	mcp.AddTool(server, &mcp.Tool{Name: "wait_for_job", Annotations: readOnly, Meta: appToolMeta(jobProgressAppURI),
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
			"weapon, kinds and side keep only matching plays, for example only AWP, only clutches and aces, or only CT; player keeps one player of a multi-POV job. " +
			"With deaths, lists the player's deaths instead, for a death reel."},
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
			if in.Deaths && (q.Weapon != "" || len(q.Kinds) > 0 || q.Side != "" || q.Player != "") {
				return nil, momentsOut{}, errors.New("weapon, kinds, side and player filter plays, not deaths; leave them empty with deaths")
			}
			var doc tuiclient.MomentsDocument
			if in.Deaths {
				doc, err = c.GetDeaths(ctx, in.ID)
			} else {
				doc, err = c.FilterMoments(ctx, q)
			}
			if err != nil {
				return nil, momentsOut{}, toolError(err)
			}
			limit := in.Limit
			if limit <= 0 {
				limit = defaultMomentsLimit
				// Deaths are in match order, not ranked: a cut would drop the last rounds.
				if in.Deaths {
					limit = len(doc.Moments)
				}
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

	mcp.AddTool(server, &mcp.Tool{Name: "generate_short", Meta: appToolMeta(jobProgressAppURI),
		Description: "Capture and edit a Short from chosen plays of one job. For a series, the plays of one call must share one job_id (one map); a Short that mixes maps is not supported, so make one Short per map. " +
			"A multi-POV job mixes players in one Short: pick plays whose times do not overlap, since one capture films one player at a time. Optional format, kill_effect, transition, music and extras come from list_short_options; unset keeps the defaults. This opens CS2 full screen and records for minutes, so the user must approve it. " +
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
			music, err := resolveMusic(in.shortChoices)
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
			list, what := c.GetMoments, plural(len(in.SegmentIDs), "jugada", "jugadas")
			if in.Deaths {
				list, what = c.GetDeaths, plural(len(in.SegmentIDs), "muerte", "muertes")
				edit.DeathReel, edit.HookText, edit.KillCounter, edit.CenteredKillfeed = true, false, false, false
			}
			doc, err := list(ctx, in.JobID)
			if err != nil {
				return nil, generateOut{}, toolError(err)
			}
			plays, err := describePlays(doc.Moments, in.SegmentIDs)
			if err != nil {
				return nil, generateOut{}, err
			}
			body := tuiclient.GenerateRequest{Preset: preset, SegmentIDs: in.SegmentIDs, Music: music, Edit: edit}
			summary := fmt.Sprintf("Grabar un Short con %s y el preset %s: %s. Opciones: %s. CS2 se abrirá a pantalla completa y grabará durante varios minutos.",
				what, preset, plays, describeShortRequest(body))
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
	out := momentOut{SegmentID: m.SegmentID, Round: m.Round, Side: cleanName(m.Side), ClutchVs: m.ClutchVs, Seconds: m.DurationSeconds,
		Score: m.Score, Kills: m.Events.Kills, Headshots: m.Events.Headshots, Reasons: cleanNames(m.ReasonCodes),
		Weapons: cleanNames(m.Weapons), Victims: cleanNames(m.Victims)}
	if d := m.Death; d != nil {
		out.KilledBy, out.Weapon = cleanName(d.Killer), cleanName(d.Weapon)
	}
	if m.SteamID64 != "" {
		out.Player, out.SteamID = cleanName(m.Player), cleanName(m.SteamID64)
	}
	return out
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
			if m.Death != nil {
				death := "muere"
				if killer := cleanName(m.Death.Killer); killer != "" {
					death = "muere ante " + killer
				}
				details = nonEmpty(cleanName(m.Player), cleanName(m.Map), fmt.Sprintf("ronda %d", m.Round), death)
			}
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
func checkDemoPath(path string) error {
	lower := strings.ToLower(path)
	if !strings.HasSuffix(lower, ".dem") && !strings.HasSuffix(lower, ".dem.zst") {
		return errors.New("path must be an absolute path to a .dem or .dem.zst file")
	}
	return checkLocalFile(path, "demo", maxDemoBytes)
}

// checkLocalFile accepts only an absolute regular file on a local drive, of at
// most maxBytes (0 means no cap). A UNC path would make Windows authenticate to
// whatever host the model names.
func checkLocalFile(path, what string, maxBytes int64) error {
	if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//") {
		return errors.New("network (UNC) paths are not accepted; use a file on a local drive")
	}
	if runtime.GOOS == "windows" && !localDrivePath(path) {
		return errors.New(`path must start with a local drive letter, such as C:\`)
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path must be an absolute path to the %s file", what)
	}
	// A link on C: could still point at a network share; other reparse points (OneDrive) are fine.
	if link, err := os.Lstat(path); err == nil && link.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("links are not accepted; use the %s file itself", what)
	}
	info, err := os.Stat(path)
	if err != nil {
		return errors.New("cannot read that file; check the path")
	}
	if !info.Mode().IsRegular() {
		return errors.New("path is not a regular file")
	}
	if maxBytes > 0 && info.Size() > maxBytes {
		return fmt.Errorf("the %s is %d MiB; Studio accepts at most %d MiB", what, info.Size()>>20, maxBytes>>20)
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
