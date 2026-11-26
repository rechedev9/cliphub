package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/hudthemes"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	nativeHUDID   = "native"
	nativeHUDName = "Original de CS2"
	nativeHUDText = "El HUD nativo del juego: vida, munición, radar, killfeed y la mira del jugador, con la barra de equipos de CS2 arriba."
	// Previews are about 15 KB each; the cap only guards against a wrong file.
	maxPreviewBytes = 512 << 10
	previewTimeout  = 10 * time.Second
	maxNotices      = 8
	maxRounds       = 999
)

var overlaySources = []string{"demo", "faceit", "premier", "professional"}

type hudThemeOut struct {
	ID          string `json:"id" jsonschema:"value for plan_full_demo hud"`
	Name        string `json:"name"`
	Description string `json:"description" jsonschema:"one line, in Spanish"`
	Preview     bool   `json:"preview" jsonschema:"true when a preview image of this design is attached"`
}

type hudThemesOut struct {
	Themes  []hudThemeOut     `json:"themes"`
	Custom  []userHUDThemeOut `json:"custom" jsonschema:"the user's own themes (save_hud_theme); their id also goes in plan_full_demo hud"`
	Max     int               `json:"max_custom" jsonschema:"how many own themes Studio keeps"`
	Shows   []string          `json:"shows" jsonschema:"round facts every custom design draws under the scoreboard, in Spanish; the native HUD has none of them"`
	Options []string          `json:"options" jsonschema:"plan_full_demo switches every custom design takes, in Spanish"`
	Note    string            `json:"note,omitempty"`
}

// customHUDShows mirrors the round facts line of the web HUD selector.
var customHUDShows = []string{
	"jugadores vivos de cada bando junto al marcador",
	"racha de un equipo con 4 o más rondas seguidas",
	"3K, 4K o ACE del jugador en la ronda",
	"barra de desactivación de la bomba, con o sin kit",
	"tarjeta MVP al cerrar cada ronda",
}

type planFullDemoIn struct {
	JobID               string `json:"job_id" jsonschema:"a parsed job from create_job"`
	CheckOnly           bool   `json:"check_only,omitempty" jsonschema:"only say whether these options need CS2 to record again and the disk they need, as the Full Demo editor does before saving; saves nothing and returns no plan_hash"`
	HUD                 string `json:"hud,omitempty" jsonschema:"HUD design id or user theme id from list_hud_themes, or native for the original CS2 HUD; empty keeps Studio's default design"`
	OverlaySource       string `json:"overlay_source,omitempty" jsonschema:"origin of the match data in the intro and scoreboard: demo (local demo), faceit, premier or professional"`
	IntroDesign         string `json:"intro_design,omitempty" jsonschema:"look of the player intro and final scoreboard: neon (default) or hud, in the colours of the chosen HUD or own theme (neutral over the native CS2 HUD); render only"`
	TeamVoices          *bool  `json:"team_voices,omitempty" jsonschema:"include the player's team voice chat; default on"`
	VoiceLevel          *int   `json:"voice_level,omitempty" jsonschema:"team voices level in percent, 0 to 200; default 110"`
	GameLevel           *int   `json:"game_level,omitempty" jsonschema:"game sound level in percent, 0 to 200; default 100"`
	BetweenRoundEffects *bool  `json:"between_round_effects,omitempty" jsonschema:"dynamic transition effects between rounds; default on"`
	TrueView            *bool  `json:"true_view,omitempty" jsonschema:"original 1:1 POV (CS2 TrueView) instead of the standard view; default off"`
	ClipHubRadar        *bool  `json:"cliphub_radar,omitempty" jsonschema:"ClipHub's own radar in the top left corner in place of CS2's: the 2D Replay map art, every living player with where it looks, smokes, fires, flashes, deaths and the bomb, with no zone names; custom HUD only, changes the capture; default off"`
	Rounds              []int  `json:"rounds,omitempty" jsonschema:"only these round numbers; empty means every round"`
	fullDemoExtrasIn
	hudLookIn
	brandChoiceIn
	storyOverlaysIn
	subtitleChoices
	custom *hudthemes.Style // A user theme resolved from hud by resolveUserHUD.
}

type fullDemoPlanOut struct {
	JobID               string   `json:"job_id"`
	PlanHash            string   `json:"plan_hash" jsonschema:"pass to render_full_demo"`
	Ready               bool     `json:"ready" jsonschema:"false when blockers stop the render"`
	Summary             string   `json:"summary"`
	DurationSeconds     int      `json:"duration_seconds"`
	Rounds              int      `json:"rounds"`
	HUD                 string   `json:"hud" jsonschema:"HUD design id, or native"`
	OverlaySource       string   `json:"overlay_source"`
	IntroDesign         string   `json:"intro_design" jsonschema:"neon or hud"`
	HalftimeScoreboard  bool     `json:"halftime_scoreboard"`
	SubscribeButton     bool     `json:"subscribe_button"`
	Hook                bool     `json:"hook" jsonschema:"the opening hook was asked for"`
	HookRound           int      `json:"hook_round,omitempty" jsonschema:"the round of the best play the hook teases; absent when the hook is off or was skipped (see warnings)"`
	TeamVoices          bool     `json:"team_voices"`
	VoiceStatus         string   `json:"voice_status,omitempty" jsonschema:"what the demo holds for team voices, e.g. available or no_packets"`
	VoiceLevel          int      `json:"voice_level_percent"`
	GameLevel           int      `json:"game_level_percent"`
	BetweenRoundEffects bool     `json:"between_round_effects"`
	TrueView            bool     `json:"true_view"`
	ClipHubRadar        bool     `json:"cliphub_radar"`
	ProSettings         string   `json:"pro_settings,omitempty" jsonschema:"the pro whose resolution and viewmodel the capture uses; empty is the user's own 1920x1080"`
	HUDPortrait         bool     `json:"hud_portrait,omitempty"`
	Bumpers             []string `json:"bumpers,omitempty" jsonschema:"the added clips: intro, sponsor, outro"`
	Warnings            []string `json:"warnings,omitempty"`
	Blockers            []string `json:"blockers,omitempty" jsonschema:"why the video cannot be rendered yet"`
	NeedsCapture        bool     `json:"needs_capture" jsonschema:"false when Studio still keeps the capture of every round: render_full_demo only renders again and CS2 does not open"`
	MissingRounds       []int    `json:"missing_rounds,omitempty" jsonschema:"the source rounds render_full_demo records in CS2"`
	LowSpace            string   `json:"low_space,omitempty" jsonschema:"the data disk may lack the space this capture and render need; Almacenamiento frees it"`
}

type renderFullDemoIn struct {
	JobID    string `json:"job_id"`
	PlanHash string `json:"plan_hash" jsonschema:"plan_hash from plan_full_demo"`
	Approve  string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

// fullDemoState is what GET /full-demo/plan returns: the saved plan, if any,
// the defaults a new plan starts from and what the saved plan must capture.
type fullDemoState struct {
	Document *recapplan.Document
	Defaults recapplan.Options
	Capture  *fullDemoCapture
}

// fullDemoCapture is the server's answer to "does this plan open CS2?".
type fullDemoCapture struct {
	Needed        bool  `json:"needed"`
	MissingRounds []int `json:"missing_rounds"`
	Space         *struct {
		FreeBytes   int64 `json:"free_bytes"`
		NeededBytes int64 `json:"needed_bytes"`
		Short       bool  `json:"short"`
	} `json:"space"`
}

func registerFullDemoTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: "list_hud_themes", Annotations: readOnly,
		Description: "List the HUD designs a Full Demo (long video of a whole match) can use, with a preview image of each. " +
			"Show the previews and ask the user which design they want before plan_full_demo. Names and descriptions are in Spanish. " +
			"Every custom design also shows the round facts in shows: players alive, a win streak of 4 or more, 3K/4K/ACE, the defuse bar with or without kit and the round MVP. " +
			"custom lists the user's own themes (save_hud_theme), without previews; options lists the plan_full_demo switches every custom design takes."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, hudThemesOut, error) {
			previews, note := fetchHUDPreviews(ctx, s)
			own, ownNote := fetchUserHUDThemes(ctx, s)
			out := hudThemesOut{Custom: []userHUDThemeOut{}, Max: hudthemes.MaxThemes, Shows: customHUDShows, Options: hudLookChoices, Note: strings.TrimSpace(note + " " + ownNote)}
			var content []mcp.Content
			var text strings.Builder
			add := func(id, name, description string) {
				image := previews[id]
				out.Themes = append(out.Themes, hudThemeOut{ID: id, Name: name, Description: description, Preview: image != nil})
				fmt.Fprintf(&text, "%s (%s): %s\n", id, name, description)
				if image != nil {
					content = append(content, &mcp.TextContent{Text: id + ": " + name}, &mcp.ImageContent{Data: image, MIMEType: "image/jpeg"})
				}
			}
			for _, t := range customhud.Themes() {
				add(t.ID, t.Name, t.Description)
			}
			for _, t := range own {
				theme := userHUDTheme(t)
				out.Custom = append(out.Custom, theme)
				fmt.Fprintf(&text, "%s (tema propio %s, sobre %s): CT #%s, T #%s, fondo #%s\n", theme.ID, theme.Name, theme.Base, theme.CT, theme.T, theme.Background)
			}
			fmt.Fprintf(&text, "Todos los diseños personalizados muestran: %s.\n", strings.Join(customHUDShows, "; "))
			fmt.Fprintf(&text, "Y admiten en plan_full_demo: %s.\n", strings.Join(hudLookChoices, "; "))
			add(nativeHUDID, nativeHUDName, nativeHUDText)
			if out.Note != "" {
				text.WriteString(out.Note + "\n")
			}
			content = append([]mcp.Content{&mcp.TextContent{Text: strings.TrimSpace(text.String())}}, content...)
			return &mcp.CallToolResult{Content: content}, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "plan_full_demo",
		Description: "Prepare a Full Demo: one long video of the whole match from one player's point of view. This only builds and saves the plan, it never starts CS2. " +
			"Choose the HUD with list_hud_themes first. Options left out keep Studio's defaults. " +
			"halftime_scoreboard adds a score card at the side switch and subscribe_button a SUSCRÍBETE button twice over the rounds; both are off by default and only change the render. " +
			"hook opens the video with an 8 to 20 s teaser of the player's best play before round 1; it is off by default, only changes the render and the reply's hook_round says which round it shows. " +
			"Show the user the summary, then call render_full_demo with the plan_hash if they agree."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in planFullDemoIn) (*mcp.CallToolResult, fullDemoPlanOut, error) {
			if err := checkUUID("job_id", in.JobID); err != nil {
				return nil, fullDemoPlanOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, fullDemoPlanOut{}, toolError(err)
			}
			state, err := loadFullDemoState(ctx, c, in.JobID)
			if err != nil {
				return nil, fullDemoPlanOut{}, toolError(err)
			}
			if err := checkFullDemoExtras(in, state.Defaults); err != nil {
				return nil, fullDemoPlanOut{}, err
			}
			options := state.Defaults
			if in.fullDemoExtrasIn != (fullDemoExtrasIn{}) && !in.CheckOnly {
				uploader, err := s.uploadClient(ctx)
				if err != nil {
					return nil, fullDemoPlanOut{}, toolError(err)
				}
				if err := applyFullDemoExtras(ctx, uploader, &options, in); err != nil {
					return nil, fullDemoPlanOut{}, err
				}
			}
			if err := resolveUserHUD(ctx, c, &in); err != nil {
				return nil, fullDemoPlanOut{}, err
			}
			if err := resolveBrandKit(ctx, c, &in.brandChoiceIn); err != nil {
				return nil, fullDemoPlanOut{}, err
			}
			options, err = applyFullDemoChoices(options, in)
			if err != nil {
				return nil, fullDemoPlanOut{}, err
			}
			body, err := json.Marshal(options)
			if err != nil {
				return nil, fullDemoPlanOut{}, err
			}
			if in.CheckOnly {
				return checkFullDemoCapture(ctx, c, in.JobID, body)
			}
			raw, err := c.PlanFullDemo(ctx, in.JobID, body)
			if err != nil {
				return nil, fullDemoPlanOut{}, toolError(err)
			}
			doc, err := decodeFullDemoDocument(raw)
			if err != nil {
				return nil, fullDemoPlanOut{}, err
			}
			if err := doc.Validate(); err != nil {
				return nil, fullDemoPlanOut{}, fmt.Errorf("Studio returned a plan that does not validate: %w", err)
			}
			out := describeFullDemoPlan(in.JobID, doc)
			if saved, err := loadFullDemoState(ctx, c, in.JobID); err == nil && saved.Document != nil && saved.Document.PlanHash == doc.PlanHash {
				describeFullDemoCapture(&out, saved.Capture)
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "render_full_demo", Meta: appToolMeta(jobProgressAppURI),
		Description: "Capture and render a planned Full Demo. This opens CS2 full screen and records for a long time. " +
			startApprovalHint + " Follow with wait_for_job, passing the returned variant."},
		func(ctx context.Context, req *mcp.CallToolRequest, in renderFullDemoIn) (*mcp.CallToolResult, generateOut, error) {
			if err := checkUUID("job_id", in.JobID); err != nil {
				return nil, generateOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, generateOut{}, toolError(err)
			}
			state, err := loadFullDemoState(ctx, c, in.JobID)
			if err != nil {
				return nil, generateOut{}, toolError(err)
			}
			doc := state.Document
			if doc == nil {
				return nil, generateOut{}, errors.New("this job has no Full Demo plan; call plan_full_demo first")
			}
			if doc.PlanHash == "" || doc.PlanHash != in.PlanHash {
				return nil, generateOut{}, errors.New("plan_hash is not the saved plan of this job (it was replaced or is stale); call plan_full_demo again")
			}
			return startFullDemo(ctx, fullDemoStart{Request: req, Client: c, Tool: "render_full_demo", JobID: in.JobID, Doc: *doc, Approve: in.Approve, Capture: state.Capture})
		})
}

type fullDemoStart struct {
	Request *mcp.CallToolRequest
	Client  *tuiclient.Client
	Tool    string
	JobID   string
	Doc     recapplan.Document
	Approve string
	// Capture is nil when unknown: the start then assumes CS2 records.
	Capture *fullDemoCapture
}

// startFullDemo asks for approval and starts the capture of a saved plan; the
// tool name keeps an approval code from one tool out of another.
func startFullDemo(ctx context.Context, start fullDemoStart) (*mcp.CallToolResult, generateOut, error) {
	c, req, doc := start.Client, start.Request, start.Doc
	plan := describeFullDemoPlan(start.JobID, doc)
	if !plan.Ready {
		return nil, generateOut{}, fmt.Errorf("the plan cannot be rendered yet: %s", strings.Join(plan.Blockers, "; "))
	}
	describeFullDemoCapture(&plan, start.Capture)
	summary := plan.Summary + " Studio vuelve a montar el vídeo con las capturas guardadas: CS2 no se abre."
	if plan.NeedsCapture {
		caps, err := c.Capabilities(ctx)
		if err != nil {
			return nil, generateOut{}, toolError(err)
		}
		if !caps.Record.Enabled {
			return nil, generateOut{}, errors.New("capture is not available: Steam, CS2 or HLAE is missing, so the video cannot be recorded")
		}
		summary = plan.Summary + " CS2 se abrirá a pantalla completa y grabará durante bastante tiempo."
	}
	ok, err := sessionApprovals.approve(req, approvalSpec{Tool: start.Tool, JobID: start.JobID,
		Request: struct{ PlanHash string }{doc.PlanHash}, Summary: summary, Given: start.Approve, Auto: autoStart})
	if err != nil {
		return nil, generateOut{}, err
	}
	if ok.Ask != nil {
		return ok.Ask, generateOut{}, nil
	}
	if !ok.Approved {
		return nil, generateOut{NeedsApproval: true, ApprovalCode: ok.Code, Summary: summary}, nil
	}
	edit, err := fullDemoEdit(doc)
	if err != nil {
		return nil, generateOut{}, err
	}
	res, err := c.GenerateFullDemo(ctx, start.JobID, edit)
	if err != nil {
		return nil, generateOut{}, toolError(err)
	}
	recentCaptures.mark(start.JobID)
	return nil, startedOut(summary, res, fullDemoVariant), nil
}

func loadFullDemoState(ctx context.Context, c *tuiclient.Client, jobID string) (fullDemoState, error) {
	raw, err := c.GetFullDemoPlan(ctx, jobID)
	if err != nil {
		return fullDemoState{}, err
	}
	return decodeFullDemoState(raw)
}

func decodeFullDemoState(raw json.RawMessage) (fullDemoState, error) {
	var wire struct {
		Document json.RawMessage  `json:"document"`
		Defaults json.RawMessage  `json:"defaults"`
		Capture  *fullDemoCapture `json:"capture"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return fullDemoState{}, fmt.Errorf("Studio returned an unreadable Full Demo state: %w", err)
	}
	state := fullDemoState{Capture: wire.Capture}
	if err := json.Unmarshal(wire.Defaults, &state.Defaults); err != nil {
		return fullDemoState{}, fmt.Errorf("%w (%v)", errOutdatedMCP, err)
	}
	if len(wire.Document) == 0 || bytes.Equal(bytes.TrimSpace(wire.Document), []byte("null")) {
		return state, nil
	}
	doc, err := decodeFullDemoDocument(wire.Document)
	if err != nil {
		return fullDemoState{}, err
	}
	state.Document = &doc
	return state, nil
}

// errOutdatedMCP: assistants keep zv-mcp running across Studio updates, so it can
// be older than the orchestrator that wrote the plan.
var errOutdatedMCP = errors.New("Studio sent Full Demo data this zv-mcp cannot read. " +
	"If ClipHub Studio was updated, restart the assistant so it loads the zv-mcp that came with the update")

// decodeFullDemoDocument reads plans as strictly as Studio stores them: a field
// one side lacks would change the recomputed content hash and pass as a stale plan.
func decodeFullDemoDocument(raw []byte) (recapplan.Document, error) {
	doc, err := recapplan.DecodeDocument(raw)
	if err != nil {
		return recapplan.Document{}, fmt.Errorf("%w (%v)", errOutdatedMCP, err)
	}
	return doc, nil
}

// fullDemoEdit builds the generate body exactly as `zv full-demo execute` does,
// approving the saved document the user just saw.
func fullDemoEdit(doc recapplan.Document) (json.RawMessage, error) {
	snapshot := recapplan.Snapshot{Document: doc, Approval: recapplan.Approval{
		PlanHash: doc.PlanHash, AllowSafeTailTrim: doc.Options.Editorial.AllowSafeTailTrim, Timestamp: time.Now().UTC(),
	}}
	if err := snapshot.Validate(); err != nil {
		return nil, fmt.Errorf("the plan is not approvable: %w", err)
	}
	edit := renderplan.FullDemoEditRequest(snapshot)
	if err := edit.Validate(); err != nil {
		return nil, fmt.Errorf("the render request is invalid: %w", err)
	}
	return json.Marshal(edit)
}

// applyFullDemoChoices sets the user's choices on the server defaults and lets
// the planner's own canonical form fix everything that stays automatic.
func applyFullDemoChoices(options recapplan.Options, in planFullDemoIn) (recapplan.Options, error) {
	if in.HUD != "" {
		if err := setHUD(&options, in.HUD); err != nil {
			return recapplan.Options{}, err
		}
		options.Overlays.HUDCustom = in.custom
	}
	if err := applyHUDLook(&options, in.hudLookIn); err != nil {
		return recapplan.Options{}, err
	}
	applyBrandChoice(&options, in.brandChoiceIn)
	applyStoryOverlays(&options, in.storyOverlaysIn)
	if err := applyIntroDesign(&options, in.IntroDesign); err != nil {
		return recapplan.Options{}, err
	}
	if in.OverlaySource != "" {
		source := strings.ToLower(strings.TrimSpace(in.OverlaySource))
		if source == "local" {
			source = "demo"
		}
		if !slices.Contains(overlaySources, source) {
			return recapplan.Options{}, fmt.Errorf("overlay_source must be one of %s", strings.Join(overlaySources, ", "))
		}
		options.SourceKind = source
	}
	if in.TeamVoices != nil {
		options.Audio.Voice.Enabled = *in.TeamVoices
	}
	var err error
	if options.Audio.Voice.Gain, err = level("voice_level", in.VoiceLevel, options.Audio.Voice.Gain); err != nil {
		return recapplan.Options{}, err
	}
	if options.Audio.Game.Gain, err = level("game_level", in.GameLevel, options.Audio.Game.Gain); err != nil {
		return recapplan.Options{}, err
	}
	if in.BetweenRoundEffects != nil {
		// A copy, so the dry run in checkFullDemoExtras never writes through the defaults' pointer.
		transitions := recapplan.DynamicTransitions()
		if options.Transitions != nil {
			transitions = *options.Transitions
		}
		transitions.Enabled = *in.BetweenRoundEffects
		options.Transitions = &transitions
	}
	if in.TrueView != nil {
		options.Capture.TrueView = *in.TrueView
	}
	if in.ClipHubRadar != nil {
		options.Capture.ClipHubRadar = *in.ClipHubRadar
	}
	if err := applySubtitles(&options, in.subtitleChoices); err != nil {
		return recapplan.Options{}, err
	}
	if len(in.Rounds) > 0 {
		if len(in.Rounds) > recapplan.MaxSelectedRounds {
			return recapplan.Options{}, fmt.Errorf("rounds takes at most %d numbers", recapplan.MaxSelectedRounds)
		}
		for _, r := range in.Rounds {
			if r < 1 || r > maxRounds {
				return recapplan.Options{}, fmt.Errorf("round numbers go from 1 to %d, got %d", maxRounds, r)
			}
		}
		options.Rounds = slices.Compact(slices.Sorted(slices.Values(in.Rounds)))
	}
	canonical, err := recapplan.CanonicalNewOptions(options)
	if err != nil {
		return recapplan.Options{}, fmt.Errorf("these choices cannot be planned: %w", err)
	}
	if err := canonical.Validate(); err != nil {
		return recapplan.Options{}, fmt.Errorf("these choices cannot be planned: %w", err)
	}
	return canonical, nil
}

func setHUD(options *recapplan.Options, choice string) error {
	id := strings.ToLower(strings.TrimSpace(choice))
	switch id {
	case nativeHUDID, "original", "cs2", strings.ToLower(nativeHUDName):
		return options.ChooseHUD("")
	}
	theme, ok := customhud.Lookup(id)
	if !ok {
		ids := []string{nativeHUDID}
		for _, t := range customhud.Themes() {
			ids = append(ids, t.ID)
		}
		return fmt.Errorf("unknown HUD design %q; use one of: %s", cleanName(choice), strings.Join(ids, ", "))
	}
	return options.ChooseHUD(theme.ID)
}

// level turns a percent from the user into the gain the planner stores.
func level(name string, value *int, current float64) (float64, error) {
	if value == nil {
		return current, nil
	}
	if *value < 0 || *value > 200 {
		return 0, fmt.Errorf("%s must be between 0 and 200 percent", name)
	}
	return float64(*value) / 100, nil
}

func describeFullDemoPlan(jobID string, doc recapplan.Document) fullDemoPlanOut {
	o := doc.Options
	var lastFrame int64
	for _, item := range doc.Timeline {
		lastFrame = max(lastFrame, item.EndFrame)
	}
	seconds := 0
	if doc.Clock.FPS > 0 {
		seconds = int(math.Round(float64(lastFrame) / float64(doc.Clock.FPS)))
	}
	hud, hudName := o.Overlays.HUDTheme, nativeHUDName
	if hud == "" {
		hud = nativeHUDID
	} else if theme, ok := customhud.Lookup(hud); ok {
		hudName = theme.Name
		if c := o.Overlays.HUDCustom; c != nil {
			hudName = fmt.Sprintf("%s (tema propio sobre %s)", cleanName(c.Name), theme.Name)
		}
	}
	source := o.SourceKind
	effects := o.Transitions != nil && o.Transitions.Enabled
	out := fullDemoPlanOut{
		JobID: jobID, PlanHash: doc.PlanHash, Ready: len(doc.Blockers) == 0, DurationSeconds: seconds, Rounds: len(doc.Rounds), NeedsCapture: true,
		HUD: hud, OverlaySource: source, IntroDesign: introDesignOf(o.Overlays), HalftimeScoreboard: o.Overlays.Halftime, SubscribeButton: o.Overlays.Subscribe, TeamVoices: o.Audio.Voice.Enabled, VoiceStatus: doc.Voice.Availability,
		VoiceLevel: percent(o.Audio.Voice.Gain), GameLevel: percent(o.Audio.Game.Gain), BetweenRoundEffects: effects,
		TrueView: o.Capture.TrueView, ClipHubRadar: o.Capture.ClipHubRadar, Warnings: noticeMessages(doc.Warnings), Blockers: noticeMessages(doc.Blockers),
	}
	out.ProSettings, out.HUDPortrait, out.Bumpers = describeExtras(o)
	voices := "sin voces del equipo"
	if out.TeamVoices {
		voices = fmt.Sprintf("voces del equipo al %d%%", out.VoiceLevel)
	}
	if s := o.Subtitles; s != nil {
		voices += describeSubtitles(s.Enabled, len(s.Hidden))
	}
	effectsText := "sin efectos entre rondas"
	if effects {
		effectsText = "con efectos entre rondas"
	}
	out.Summary = fmt.Sprintf("Demo completa: %s, unos %d min, HUD %s, datos de la partida de %s, %s, sonido del juego al %d%%, %s.",
		plural(out.Rounds, "ronda", "rondas"), (seconds+30)/60, hudName, sourceLabel(source), voices, out.GameLevel, effectsText)
	if out.ProSettings != "" {
		out.Summary += fmt.Sprintf(" Graba con la configuración de %s.", out.ProSettings)
	}
	if out.HUDPortrait {
		out.Summary += " Con tu retrato en el HUD."
	}
	if out.ClipHubRadar {
		out.Summary += " Con el radar de ClipHub en lugar del de CS2."
	}
	hookText := ""
	out.Hook, out.HookRound, hookText = describeHook(doc)
	out.Summary += hookText + describeHUDLook(o.Overlays) + describeIntroDesign(o.Overlays) + describeStoryOverlays(o.Overlays) + describeBrand(o.Overlays.Brand)
	if len(out.Bumpers) > 0 {
		out.Summary += fmt.Sprintf(" Clips añadidos: %s.", strings.Join(out.Bumpers, ", "))
	}
	return out
}

// describeFullDemoCapture adds what the plan records, from the server's check; nil leaves "records every round".
func describeFullDemoCapture(out *fullDemoPlanOut, capture *fullDemoCapture) {
	if capture == nil {
		return
	}
	out.NeedsCapture, out.MissingRounds = capture.Needed, capture.MissingRounds
	switch {
	case !capture.Needed:
		out.Summary += " Las capturas de todas las rondas están guardadas: solo se vuelve a montar."
	case len(capture.MissingRounds) < out.Rounds:
		rounds := make([]string, len(capture.MissingRounds))
		for i, r := range capture.MissingRounds {
			rounds[i] = strconv.Itoa(r)
		}
		out.Summary += fmt.Sprintf(" Graba de nuevo solo las rondas %s; las demás ya están capturadas.", strings.Join(rounds, ", "))
	}
	if s := capture.Space; s != nil && s.Short {
		out.LowSpace = fmt.Sprintf("Puede faltar espacio: hacen falta unos %s y quedan %s libres. Almacenamiento libera espacio.", formatBytes(s.NeededBytes), formatBytes(s.FreeBytes))
		out.Summary += " " + out.LowSpace
	}
}

func percent(gain float64) int { return int(math.Round(gain * 100)) }

func sourceLabel(kind string) string {
	if kind == "demo" || kind == "" {
		return "la demo local"
	}
	return kind
}

// noticeMessages cleans planner notices, which can quote demo content.
func noticeMessages(notices []recapplan.Notice) []string {
	var out []string
	for _, n := range notices {
		if len(out) == maxNotices {
			break
		}
		out = append(out, cleanText(n.Message, maxErrorRunes))
	}
	return out
}

// fetchHUDPreviews downloads the design previews from the Studio web server,
// which serves /hud/* as public assets. A failure only drops the images.
func fetchHUDPreviews(ctx context.Context, s *studio) (map[string][]byte, string) {
	origin, err := s.webOrigin()
	if err != nil {
		return nil, "Preview images are not available: " + err.Error()
	}
	ctx, cancel := context.WithTimeout(ctx, previewTimeout)
	defer cancel()
	previews := map[string][]byte{}
	for _, t := range customhud.Themes() {
		if thumb, ok := hudThumbs.get(t.ID); ok {
			previews[t.ID] = thumb
			continue
		}
		image, err := fetchPreview(ctx, origin, t.ID)
		if err != nil {
			continue
		}
		if thumb, err := hudThumbnail(image); err == nil {
			previews[t.ID] = thumb
			hudThumbs.put(t.ID, thumb)
		}
	}
	if len(previews) == 0 {
		return nil, "Preview images could not be loaded from Studio."
	}
	return previews, ""
}

// hudThumbs keeps the thumbnails this process made: the previews ship with Studio and do not change.
var hudThumbs = thumbCache{byID: map[string][]byte{}}

type thumbCache struct {
	mu   sync.Mutex
	byID map[string][]byte
}

func (c *thumbCache) get(id string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	thumb, ok := c.byID[id]
	return thumb, ok
}

func (c *thumbCache) put(id string, thumb []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byID[id] = thumb
}

func fetchPreview(ctx context.Context, origin, id string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/hud/"+id+".webp", nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("preview %s: status %d", id, res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxPreviewBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxPreviewBytes || len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return nil, fmt.Errorf("preview %s is not a WebP image", id)
	}
	return data, nil
}
