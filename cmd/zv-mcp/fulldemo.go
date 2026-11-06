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
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/customhud"
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
	Themes []hudThemeOut `json:"themes"`
	Note   string        `json:"note,omitempty"`
}

type planFullDemoIn struct {
	JobID               string `json:"job_id" jsonschema:"a parsed job from create_job"`
	HUD                 string `json:"hud,omitempty" jsonschema:"HUD design id from list_hud_themes, or native for the original CS2 HUD; empty keeps Studio's default design"`
	OverlaySource       string `json:"overlay_source,omitempty" jsonschema:"origin of the match data in the intro and scoreboard: demo (local demo), faceit, premier or professional"`
	TeamVoices          *bool  `json:"team_voices,omitempty" jsonschema:"include the player's team voice chat; default on"`
	VoiceLevel          *int   `json:"voice_level,omitempty" jsonschema:"team voices level in percent, 0 to 200; default 110"`
	GameLevel           *int   `json:"game_level,omitempty" jsonschema:"game sound level in percent, 0 to 200; default 100"`
	BetweenRoundEffects *bool  `json:"between_round_effects,omitempty" jsonschema:"dynamic transition effects between rounds; default on"`
	TrueView            *bool  `json:"true_view,omitempty" jsonschema:"original 1:1 POV (CS2 TrueView) instead of the standard view; default off"`
	Rounds              []int  `json:"rounds,omitempty" jsonschema:"only these round numbers; empty means every round"`
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
	TeamVoices          bool     `json:"team_voices"`
	VoiceStatus         string   `json:"voice_status,omitempty" jsonschema:"what the demo holds for team voices, e.g. available or no_packets"`
	VoiceLevel          int      `json:"voice_level_percent"`
	GameLevel           int      `json:"game_level_percent"`
	BetweenRoundEffects bool     `json:"between_round_effects"`
	TrueView            bool     `json:"true_view"`
	Warnings            []string `json:"warnings,omitempty"`
	Blockers            []string `json:"blockers,omitempty" jsonschema:"why the video cannot be rendered yet"`
}

type renderFullDemoIn struct {
	JobID    string `json:"job_id"`
	PlanHash string `json:"plan_hash" jsonschema:"plan_hash from plan_full_demo"`
	Approve  string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

// fullDemoState is what GET /full-demo/plan returns: the saved plan, if any,
// and the defaults a new plan starts from.
type fullDemoState struct {
	Document *recapplan.Document
	Defaults recapplan.Options
}

func registerFullDemoTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: "list_hud_themes", Annotations: readOnly,
		Description: "List the HUD designs a Full Demo (long video of a whole match) can use, with a preview image of each. " +
			"Show the previews and ask the user which design they want before plan_full_demo. Names and descriptions are in Spanish."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, hudThemesOut, error) {
			previews, note := fetchHUDPreviews(ctx, s)
			out := hudThemesOut{Note: note}
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
			add(nativeHUDID, nativeHUDName, nativeHUDText)
			if note != "" {
				text.WriteString(note + "\n")
			}
			content = append([]mcp.Content{&mcp.TextContent{Text: strings.TrimSpace(text.String())}}, content...)
			return &mcp.CallToolResult{Content: content}, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "plan_full_demo",
		Description: "Prepare a Full Demo: one long video of the whole match from one player's point of view. This only builds and saves the plan, it never starts CS2. " +
			"Choose the HUD with list_hud_themes first. Options left out keep Studio's defaults. " +
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
			options, err := applyFullDemoChoices(state.Defaults, in)
			if err != nil {
				return nil, fullDemoPlanOut{}, err
			}
			body, err := json.Marshal(options)
			if err != nil {
				return nil, fullDemoPlanOut{}, err
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
			return nil, describeFullDemoPlan(in.JobID, doc), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "render_full_demo",
		Description: "Capture and render a planned Full Demo. This opens CS2 full screen and records for a long time, so the user must approve it. " +
			"If the result says needs_approval, show the summary, ask the user, and call again with approve set to approval_code only if they agree. " +
			"Follow with wait_for_job, passing the returned variant."},
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
			plan := describeFullDemoPlan(in.JobID, *doc)
			if !plan.Ready {
				return nil, generateOut{}, fmt.Errorf("the plan cannot be rendered yet: %s", strings.Join(plan.Blockers, "; "))
			}
			caps, err := c.Capabilities(ctx)
			if err != nil {
				return nil, generateOut{}, toolError(err)
			}
			if !caps.Record.Enabled {
				return nil, generateOut{}, errors.New("capture is not available: Steam, CS2 or HLAE is missing, so the video cannot be recorded")
			}
			summary := plan.Summary + " CS2 se abrirá a pantalla completa y grabará durante bastante tiempo."
			ok, err := sessionApprovals.approve(req, approvalSpec{Tool: "render_full_demo", JobID: in.JobID,
				Request: struct{ PlanHash string }{doc.PlanHash}, Summary: summary, Given: in.Approve})
			if err != nil {
				return nil, generateOut{}, err
			}
			if ok.Ask != nil {
				return ok.Ask, generateOut{}, nil
			}
			if !ok.Approved {
				return nil, generateOut{NeedsApproval: true, ApprovalCode: ok.Code, Summary: summary}, nil
			}
			edit, err := fullDemoEdit(*doc)
			if err != nil {
				return nil, generateOut{}, err
			}
			res, err := c.GenerateFullDemo(ctx, in.JobID, edit)
			if err != nil {
				return nil, generateOut{}, toolError(err)
			}
			recentCaptures.mark(in.JobID)
			return nil, startedOut(summary, res, fullDemoVariant), nil
		})
}

func loadFullDemoState(ctx context.Context, c *tuiclient.Client, jobID string) (fullDemoState, error) {
	raw, err := c.GetFullDemoPlan(ctx, jobID)
	if err != nil {
		return fullDemoState{}, err
	}
	var wire struct {
		Document json.RawMessage   `json:"document"`
		Defaults recapplan.Options `json:"defaults"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return fullDemoState{}, fmt.Errorf("Studio returned an unreadable Full Demo state: %w", err)
	}
	state := fullDemoState{Defaults: wire.Defaults}
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
var errOutdatedMCP = errors.New("Studio sent a Full Demo plan this zv-mcp cannot read. " +
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
		if options.Transitions == nil {
			dynamic := recapplan.DynamicTransitions()
			options.Transitions = &dynamic
		}
		options.Transitions.Enabled = *in.BetweenRoundEffects
	}
	if in.TrueView != nil {
		options.Capture.TrueView = *in.TrueView
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
		options.Capture.HUDProfile = recapplan.NativeHUDProfile
		options.Overlays.HUDTheme = ""
		options.Overlays.HUDPortrait = nil
		return nil
	}
	theme, ok := customhud.Lookup(id)
	if !ok {
		ids := []string{nativeHUDID}
		for _, t := range customhud.Themes() {
			ids = append(ids, t.ID)
		}
		return fmt.Errorf("unknown HUD design %q; use one of: %s", cleanName(choice), strings.Join(ids, ", "))
	}
	options.Capture.HUDProfile = customhud.CaptureProfile
	options.Overlays.HUDTheme = theme.ID
	if theme.ID != "focus" {
		options.Overlays.HUDPortrait = nil
	}
	return nil
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
	}
	source := o.SourceKind
	effects := o.Transitions != nil && o.Transitions.Enabled
	out := fullDemoPlanOut{
		JobID: jobID, PlanHash: doc.PlanHash, Ready: len(doc.Blockers) == 0, DurationSeconds: seconds, Rounds: len(doc.Rounds),
		HUD: hud, OverlaySource: source, TeamVoices: o.Audio.Voice.Enabled, VoiceStatus: doc.Voice.Availability,
		VoiceLevel: percent(o.Audio.Voice.Gain), GameLevel: percent(o.Audio.Game.Gain), BetweenRoundEffects: effects,
		TrueView: o.Capture.TrueView, Warnings: noticeMessages(doc.Warnings), Blockers: noticeMessages(doc.Blockers),
	}
	voices := "sin voces del equipo"
	if out.TeamVoices {
		voices = fmt.Sprintf("voces del equipo al %d%%", out.VoiceLevel)
	}
	effectsText := "sin efectos entre rondas"
	if effects {
		effectsText = "con efectos entre rondas"
	}
	out.Summary = fmt.Sprintf("Demo completa: %s, unos %d min, HUD %s, datos de la partida de %s, %s, sonido del juego al %d%%, %s.",
		plural(out.Rounds, "ronda", "rondas"), (seconds+30)/60, hudName, sourceLabel(source), voices, out.GameLevel, effectsText)
	return out
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
