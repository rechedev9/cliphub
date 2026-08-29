package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/streamclips"
)

type productionFlow struct {
	Name                 string       `json:"name"`
	Description          string       `json:"description"`
	Source               string       `json:"source"`
	RequiredArtifacts    []string     `json:"required_artifacts"`
	ProducedArtifactKeys []string     `json:"produced_artifact_keys"`
	SafetyGates          []string     `json:"safety_gates"`
	DryRunBehavior       string       `json:"dry_run_behavior"`
	LiveBehavior         string       `json:"live_behavior"`
	ResumePolicy         string       `json:"resume_policy"`
	Outputs              []flowOutput `json:"outputs"`
	Phases               []flowPhase  `json:"phases"`
}

type flowOutput struct {
	Name        string `json:"name"`
	Format      string `json:"format"`
	Resolution  string `json:"resolution"`
	Destination string `json:"destination"`
}

type flowPhase struct {
	ID        string `json:"id"`
	Goal      string `json:"goal"`
	Command   string `json:"command,omitempty"`
	Decision  string `json:"decision,omitempty"`
	Produces  string `json:"produces,omitempty"`
	When      string `json:"when,omitempty"`
	Gate      bool   `json:"gate,omitempty"`
	ReadOnly  bool   `json:"read_only"`
	Expensive bool   `json:"expensive"`
}

type flowListRow struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ShowCommand string `json:"show_command"`
}

func runFlows(args []string, stdout, stderr io.Writer, stdin io.Reader, runner commandRunner) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, flowsUsage)
		return exitInvalidArgs
	}
	if isHelp(args[0]) {
		fmt.Fprint(stdout, flowsUsage)
		return exitSuccess
	}
	switch args[0] {
	case "list":
		return runFlowsList(args[1:], stdout, stderr)
	case "show":
		return runFlowsShow(args[1:], stdout, stderr)
	case "run":
		return runFlowsRun(args[1:], stdout, stderr, stdin, runner)
	default:
		fmt.Fprintf(stderr, "unknown flows command %q\n%s", args[0], flowsUsage)
		return exitInvalidArgs
	}
}

func runFlowsList(args []string, stdout, stderr io.Writer) int {
	if isSingleHelp(args) {
		fmt.Fprint(stdout, flowsListUsage)
		return exitSuccess
	}
	format, rest, err := parseFormatArgs(args)
	if err != nil || len(rest) != 0 {
		if err == nil {
			err = fmt.Errorf("unexpected extra args for flows list")
		}
		return writeFlowError(args, stdout, stderr, err, flowsListUsage)
	}
	flows := productionFlows()
	rows := make([]flowListRow, 0, len(flows))
	for _, flow := range flows {
		rows = append(rows, flowListRow{
			Name: flow.Name, Description: flow.Description, ShowCommand: "zv flows show " + flow.Name + " --format json",
		})
	}
	if format == "json" {
		if err := writeJSON(stdout, map[string]any{"ok": true, "flows": rows}); err != nil {
			fmt.Fprintf(stderr, "error: write flow list: %v\n", err)
			return exitUnexpected
		}
		return exitSuccess
	}
	for _, row := range rows {
		fmt.Fprintf(stdout, "%s\t%s\n", row.Name, row.Description)
	}
	return exitSuccess
}

func runFlowsShow(args []string, stdout, stderr io.Writer) int {
	if isSingleHelp(args) {
		fmt.Fprint(stdout, flowsShowUsage)
		return exitSuccess
	}
	format, rest, err := parseFormatArgs(args)
	if err != nil || len(rest) != 1 {
		if err == nil {
			err = fmt.Errorf("flows show requires exactly one flow name")
		}
		return writeFlowError(args, stdout, stderr, err, flowsShowUsage)
	}
	flow, ok := findProductionFlow(rest[0])
	if !ok {
		return writeFlowError(args, stdout, stderr, fmt.Errorf("unknown production flow %q (valid: demo, stream)", rest[0]), flowsShowUsage)
	}
	if format == "json" {
		if err := writeJSON(stdout, map[string]any{"ok": true, "flow": flow}); err != nil {
			fmt.Fprintf(stderr, "error: write production flow: %v\n", err)
			return exitUnexpected
		}
		return exitSuccess
	}
	fmt.Fprintf(stdout, "%s: %s\n", flow.Name, flow.Description)
	for i, phase := range flow.Phases {
		fmt.Fprintf(stdout, "%d. %s — %s\n", i+1, phase.ID, phase.Goal)
		if phase.Command != "" {
			fmt.Fprintf(stdout, "   %s\n", phase.Command)
		}
		if phase.Decision != "" {
			fmt.Fprintf(stdout, "   decide: %s\n", phase.Decision)
		}
	}
	return exitSuccess
}

func productionFlows() []productionFlow {
	demoOutputs := []flowOutput{
		{Name: "TikTok / Shorts", Format: editor.OutputFormatShort9x16, Resolution: "1080x1920@60", Destination: "<run>/shortslistosparasubir"},
		{Name: "YouTube long-form", Format: editor.OutputFormatLandscape16x9, Resolution: "1920x1080@60", Destination: "<run>/shortslistosparasubir"},
	}
	streamOutputs := []flowOutput{
		{Name: "TikTok / Shorts", Format: editor.OutputFormatShort9x16, Resolution: "1080x1920@60", Destination: "<run>/render/shortslistosparasubir"},
		{Name: "YouTube long-form", Format: editor.OutputFormatLandscape16x9, Resolution: "1920x1080@60", Destination: "<run>/render/shortslistosparasubir"},
	}
	return []productionFlow{
		{
			Name:                 "demo",
			Description:          "CS2 demo to selected HLAE capture and upload-ready edit",
			Source:               ".dem",
			RequiredArtifacts:    []string{"demo path", "approved creative brief before live media work"},
			ProducedArtifactKeys: []string{"playability", "demo-roster", "killplan", "moments", "selected-plan", "recording-result", "render-manifest", "qa-report", "publish-pack"},
			SafetyGates:          []string{"target player selection", "creative brief approval", "live HLAE/CS2 capture approval", "long FFmpeg render approval", "thumbnail selection when covers are enabled"},
			DryRunBehavior:       "use preflight phases and flows run --dry-run to plan commands without launching CS2 or rendering media",
			LiveBehavior:         "runs parser-derived selected ranges through HLAE/CS2 capture and FFmpeg/Lua render into a local publish pack",
			ResumePolicy:         "durable artifacts can be inspected and skipped on retry; failed live recording requires a fresh output namespace and demo_incompatible is not retried",
			Outputs:              append([]flowOutput(nil), demoOutputs...),
			Phases: []flowPhase{
				{ID: "doctor", Goal: "verify local parser, HLAE, CS2, FFmpeg, and editor readiness", Command: "zv capabilities --format json", ReadOnly: true},
				{ID: "probe", Goal: "classify playdemo tick-0 safety without launching CS2", Command: "zv demo probe --demo <match.dem> --out <run>/playability.json --format json", Decision: "playable or stop", ReadOnly: false},
				{ID: "players", Goal: "inspect the roster and choose the POV SteamID64", Command: "zv demo players --demo <match.dem> --format json", Decision: "target player", ReadOnly: true},
				{ID: "parse-preflight", Goal: "validate the demo, target, and output path without parsing", Command: "zv demo parse --demo <match.dem> --steamid <SteamID64> --out <run>/killplan.json --dry-run", Decision: "approve deterministic parse inputs", ReadOnly: true},
				{ID: "parse", Goal: "derive a deterministic kill plan from the demo", Command: "zv demo parse --demo <match.dem> --steamid <SteamID64> --out <run>/killplan.json", Produces: "killplan.json"},
				{ID: "moments-preflight", Goal: "score candidate plays in memory without writing the review artifact", Command: "zv demo moments --killplan <run>/killplan.json --out <run>/moments.json --dry-run --format json", Decision: "approve factual ranking inputs", ReadOnly: true},
				{ID: "moments", Goal: "rank factual candidate plays before GPU capture", Command: "zv demo moments --killplan <run>/killplan.json --out <run>/moments.json --format json", Decision: "segments and narrative order", Produces: "moments.json", ReadOnly: false},
				{ID: "creative-brief", Goal: "ask only unanswered creative questions and receive explicit approval before expensive media work; ambiguous words like go/hazlo are not approval until they answer a shown brief", Decision: "delivery format; HUD and killfeed; kill effect; transition; kill numbering or counter; intro/outro; music; generate gameplay thumbnail candidates or no cover", Produces: "approved creative brief and exact render choices", Gate: true, ReadOnly: true},
				{ID: "select-preflight", Goal: "validate the chosen plays and narrative order without writing", Command: "zv demo select --killplan <run>/killplan.json --segments <seg-ids> --out <run>/selected-plan.json --dry-run --format json", Decision: "approve selected segments", ReadOnly: true},
				{ID: "select", Goal: "persist only the chosen plays in their requested order", Command: "zv demo select --killplan <run>/killplan.json --segments <seg-ids> --out <run>/selected-plan.json --format json", Produces: "selected-plan.json"},
				{ID: "capture-preflight", Goal: "generate and inspect the HLAE capture contract without launching CS2", Command: "zv record --killplan <run>/selected-plan.json --demo <match.dem> --out <run>/recording --hud <gameplay|clean|deathnotices> --portrait-safe-killfeed=<true|false> --dry-run --format json", Decision: "approve capture plan", Produces: "recording dry-run artifacts", ReadOnly: false},
				{ID: "capture", Goal: "record the selected POV ranges with HLAE and CS2", Command: "zv record --killplan <run>/selected-plan.json --demo <match.dem> --out <run>/recording --hud <gameplay|clean|deathnotices> --portrait-safe-killfeed=<true|false> --format json", Produces: "recording-result.json", Expensive: true},
				{ID: "edit-preflight", Goal: "validate the approved creative brief without rendering", Command: "zv shorts render --recording-result <run>/recording/recording-result.json --killplan <run>/selected-plan.json --out <run>/render --publish-dir <run>/shortslistosparasubir --preset viral-60-clean --output-format <approved-format> --kill-effect <approved-effect> --transition <approved-transition> --hook=<true|false> --kill-counter=<true|false> --intro=<true|false> --outro=<true|false> [--intro-text <text>] [--outro-text <text>] [--music <track>] --covers=<true|false> --cover-sheets=<true|false> --compile-segments --dry-run", Decision: "approve exact render argv", Produces: "editor dry-run manifests and publish metadata", ReadOnly: false},
				{ID: "edit", Goal: "render one polished compilation with the approved creative brief", Command: "zv shorts render --recording-result <run>/recording/recording-result.json --killplan <run>/selected-plan.json --out <run>/render --publish-dir <run>/shortslistosparasubir --preset viral-60-clean --output-format <approved-format> --kill-effect <approved-effect> --transition <approved-transition> --hook=<true|false> --kill-counter=<true|false> --intro=<true|false> --outro=<true|false> [--intro-text <text>] [--outro-text <text>] [--music <track>] --covers=<true|false> --cover-sheets=<true|false> --compile-segments", Produces: "candidate pack when covers are enabled; upload-ready pack when covers are disabled", Expensive: true},
				{ID: "thumbnail-selection", Goal: "show the generated cover candidates and receive the user's final thumbnail choice", Command: "zv gallery open --path <run>/shortslistosparasubir/index.html", Decision: "select a cover candidate or explicitly delegate automatic selection", Produces: "upload-ready pack with approved thumbnail", When: "covers enabled", Gate: true, ReadOnly: true},
				{ID: "review", Goal: "inspect the gallery, covers, manifest, and QA before upload", Command: "zv gallery open --path <run>/shortslistosparasubir/index.html", ReadOnly: true},
			},
		},
		{
			Name:                 "stream",
			Description:          "stream/VOD clips",
			Source:               "video",
			RequiredArtifacts:    []string{"local stream MP4 or approved fetch URL", "persisted stream edit plan"},
			ProducedArtifactKeys: []string{"stream-mp4", "stream-variant-catalog", "stream-edit-plan", "stream-render-manifest", "publish-pack"},
			SafetyGates:          []string{"clip bounds/title approval", "crop/framing approval", "source-audio treatment", "long FFmpeg render approval", "third-party music provenance when music is supplied"},
			DryRunBehavior:       "stream dry-runs validate/probe but do not create the --out edit plan artifact; persist the approved plan before dependent render stages",
			LiveBehavior:         "renders the persisted edit plan into a local upload-ready pack without ad hoc FFmpeg flags outside the plan",
			ResumePolicy:         "changing the persisted stream plan invalidates the creative brief; settle the brief again before the next non-dry-run render",
			Outputs:              append([]flowOutput(nil), streamOutputs...),
			Phases: []flowPhase{
				{ID: "doctor", Goal: "verify FFmpeg readiness", Command: "zv capabilities --format json", ReadOnly: true},
				{ID: "layouts", Goal: "discover vertical and landscape output geometry", Command: "zv stream variants --format json", Decision: "layout and delivery format", ReadOnly: true},
				{ID: "creative-brief", Goal: "ask only unanswered stream edit questions and receive explicit approval before rendering; ambiguous words like go/hazlo are not approval until they answer a shown brief", Decision: "layout; clip boundaries/title; clean crop/framing; music; delivery shape; cover strategy", Produces: "approved stream brief and exact render choices", Gate: true, ReadOnly: true},
				{ID: "plan-preflight", Goal: "probe media and preview the clip/crop contract without writing", Command: "zv stream plan --input <stream.mp4> --out <run>/edit-plan.json --variant <" + strings.Join(streamclips.VariantNames(), "|") + "> --dry-run --format json", Decision: "clip ranges and clean crops", ReadOnly: true},
				{ID: "plan", Goal: "persist the approved stream edit contract", Command: "zv stream plan --input <stream.mp4> --out <run>/edit-plan.json --variant <" + strings.Join(streamclips.VariantNames(), "|") + "> --format json", Produces: "edit-plan.json"},
				{ID: "plan-review", Goal: "inspect the persisted edit plan and settle any change before render", Command: "zv stream render --input <stream.mp4> --plan <run>/edit-plan.json --out <run>/render --dry-run --format json", Decision: "approve clip ranges, order, crop, source audio, fades, text, and music volume", Gate: true, ReadOnly: true},
				{ID: "render-preflight", Goal: "validate tools, media, and the final plan without rendering", Command: "zv stream render --input <stream.mp4> --plan <run>/edit-plan.json --out <run>/render --dry-run --format json", Decision: "approve final output", ReadOnly: true, Expensive: true},
				{ID: "render", Goal: "render video, audio, cover, manifest, and gallery", Command: "zv stream render --input <stream.mp4> --plan <run>/edit-plan.json --out <run>/render --format json", Produces: "upload-ready pack", Expensive: true},
				{ID: "review", Goal: "inspect the final video and selected cover before upload", Command: "zv gallery open --path <run>/render/shortslistosparasubir/index.html", ReadOnly: true},
			},
		},
	}
}

func findProductionFlow(name string) (productionFlow, bool) {
	for _, flow := range productionFlows() {
		if flow.Name == name {
			return flow, true
		}
	}
	return productionFlow{}, false
}

func writeFlowError(args []string, stdout, stderr io.Writer, err error, commandUsage string) int {
	if shortJSONRequested(args) {
		if writeErr := writeJSON(stdout, map[string]any{"ok": false, "error": err.Error()}); writeErr != nil {
			fmt.Fprintf(stderr, "error: write flow json error: %v\n", writeErr)
			return exitUnexpected
		}
		return exitInvalidArgs
	}
	fmt.Fprintf(stderr, "error: %v\n", err)
	fmt.Fprint(stderr, commandUsage)
	return exitInvalidArgs
}
