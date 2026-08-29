package main

import (
	"fmt"
	"strings"
	"sync"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/streamclips"
)

func findWorkflow(name string) (workflowInfo, bool) {
	w, ok := workflowCatalogByName()[name]
	return w, ok
}

// The workflow catalog is static data, identical for the life of the process.
// Build it (and its name index) once: validation rebuilt and re-scanned it on
// every documented command line across every doc and skill.
var (
	workflowCatalogOnce   = sync.OnceValue(buildWorkflowCatalog)
	workflowCatalogByName = sync.OnceValue(func() map[string]workflowInfo {
		catalog := workflowCatalogOnce()
		byName := make(map[string]workflowInfo, len(catalog))
		for _, w := range catalog {
			byName[w.Name] = w
		}
		return byName
	})
)

// workflowCatalog returns the shared catalog. Callers must treat it as read-only.
func workflowCatalog() []workflowInfo {
	return workflowCatalogOnce()
}

// buildWorkflowCatalog lists the stable workflows exposed to automation. It
// includes the composite `zv short` product flow as well as its granular stages.
func buildWorkflowCatalog() []workflowInfo {
	return withWorkflowRunCommands([]workflowInfo{
		{
			Name:        "short",
			Description: "Create one upload-ready Short through parse, capture, render, and publish stages.",
			Command:     "zv short <demo.dem> --prompt <prompt>",
			RunArgs:     []string{"short"},
		},
		{
			Name:        "capabilities",
			Description: "Inspect local capture and render tool readiness without starting work.",
			Command:     "zv capabilities",
			RunArgs:     []string{"capabilities"},
		},
		{
			Name:        "faceit-index",
			Description: "Index a FACEIT player's CS2 matches, statistics, demo availability, and manual room links.",
			Command:     "zv faceit index --profile <url-or-nickname> --out <demo-index.json>",
			RunArgs:     []string{"faceit", "index"},
		},
		{
			Name:        "demo-parse",
			Description: "Parse a CS2 demo into a kill or utility plan.",
			Command:     "zv demo parse --demo <demo.dem> --steamid <SteamID64> --out <plan.json>",
			RunArgs:     []string{"demo", "parse"},
		},
		{
			Name:        "demo-players",
			Description: "List demo participants and SteamID64 values as text or structured JSON.",
			Command:     "zv demo players --demo <demo.dem>",
			RunArgs:     []string{"demo", "players"},
		},
		{
			Name:        "demo-moments",
			Description: "Score and rank planned demo segments before capture.",
			Command:     "zv demo moments --killplan <plan.json>",
			RunArgs:     []string{"demo", "moments"},
		},
		{
			Name:        "demo-select",
			Description: "Create a recorder-ready plan containing an ordered segment selection.",
			Command:     "zv demo select --killplan <plan.json> --segments <ids> --out <selected-plan.json>",
			RunArgs:     []string{"demo", "select"},
		},
		{
			Name:        "demo-probe",
			Description: "Classify whether CS2 can start a demo without the tick-0 playdemo crash.",
			Command:     "zv demo probe --demo <demo.dem> --out <playability.json>",
			RunArgs:     []string{"demo", "probe"},
		},
		{
			Name:        "demo-voice",
			Description: "Probe whether a demo carries voice-comms packets for the POV team.",
			Command:     "zv demo voice --demo <demo.dem> --steamid <SteamID64> --out <voice-probe.json>",
			RunArgs:     []string{"demo", "voice"},
		},
		{
			Name:        "utility-audit",
			Description: "Audit utility destinations/actions against the lineup catalog.",
			Command:     "zv utility audit --plan <plan-utility.json> --lineup-catalog data/lineups --out <utility-audit.csv>",
			RunArgs:     []string{"utility", "audit"},
		},
		{
			Name:        "record",
			Description: "Record planned demo segments with HLAE/CS2.",
			Command:     "zv record --killplan <plan.json> --demo <demo.dem> --out <recording-dir>",
			RunArgs:     []string{"record"},
		},
		{
			Name:        "compose-final",
			Description: "Concatenate recorded segment clips into a final MP4.",
			Command:     "zv compose final --recording-result <recording-result.json> --out <final.mp4>",
			RunArgs:     []string{"compose", "final"},
		},
		{
			Name:        "music-analyze",
			Description: "Analyze music beats and build optional kill-to-beat sync suggestions.",
			Command:     "zv music analyze --input <audio-or-video> --out <rhythm.json>",
			RunArgs:     []string{"music", "analyze"},
		},
		{
			Name:        "shorts-render",
			Description: "Render vertical or landscape videos; the upload-ready pack defaults to <shorts-dir>/shortslistosparasubir.",
			Command:     "zv shorts render --recording-result <recording-result.json> --out <shorts-dir>",
			RunArgs:     []string{"shorts", "render"},
		},
		{
			Name:        "stream-fetch",
			Description: "Download an allowlisted Twitch or YouTube clip/VOD to a local MP4.",
			Command:     "zv stream fetch --url <https://...> --out <stream.mp4>",
			RunArgs:     []string{"stream", "fetch"},
		},
		{
			Name:        "stream-variants",
			Description: "List local stream layout variants and default crops.",
			Command:     "zv stream variants",
			RunArgs:     []string{"stream", "variants"},
		},
		{
			Name:        "stream-plan",
			Description: "Probe a stream video and create a validated local edit plan.",
			Command:     "zv stream plan --input <stream.mp4> --out <edit-plan.json>",
			RunArgs:     []string{"stream", "plan"},
		},
		{
			Name:        "stream-render",
			Description: "Render stream clips into an upload-ready local pack.",
			Command:     "zv stream render --input <stream.mp4> --plan <edit-plan.json> --out <run-dir>",
			RunArgs:     []string{"stream", "render"},
		},
		{
			Name:        "analysis-tactical",
			Description: "Scan a demo into the durable tactical document and its position blob.",
			Command:     "zv analysis tactical --demo <match.dem> --out <tactical.json>",
			RunArgs:     []string{"analysis", "tactical"},
		},
		{
			Name:        "analysis-rounds",
			Description: "List the rounds a tactical filter selects.",
			Command:     "zv analysis rounds --tactical <tactical.json>",
			RunArgs:     []string{"analysis", "rounds"},
		},
		{
			Name:        "analysis-tendencies",
			Description: "Aggregate filtered rounds into buys, sites, patterns, openings, and players.",
			Command:     "zv analysis tendencies --tactical <tactical.json>",
			RunArgs:     []string{"analysis", "tendencies"},
		},
		{
			Name:        "analysis-tactical-data",
			Description: "Export sampled tactical data for replay experiments.",
			Command:     "zv analysis tactical-data --demo <demo.dem> --out <tactical.json> --start <tick> --end <tick>",
			RunArgs:     []string{"analysis", "tactical-data"},
		},
		{
			Name:        "analysis-viewer",
			Description: "Serve a local analysis review UI.",
			Command:     "zv analysis view --json <analysis.json>",
			RunArgs:     []string{"analysis", "view"},
		},
		{
			Name:        "gallery-open",
			Description: "Open a generated publish gallery for review.",
			Command:     "zv gallery open --path <run>/shortslistosparasubir/index.html",
			RunArgs:     []string{"gallery", "open"},
		},
		{
			Name:        "flows-run",
			Description: "Chain a whole demo or stream journey in --dry-run mode into a run directory.",
			Command:     "zv flows run <demo|stream> --run-dir <run-dir> --dry-run",
			RunArgs:     []string{"flows", "run"},
		},
		{
			Name:        "serve",
			Description: "Start the orchestrator API and workers.",
			Command:     "zv serve",
			RunArgs:     []string{"serve"},
		},
		{
			Name:        "skills-check",
			Description: "Validate repo-local Codex skills.",
			Command:     "zv skills check",
			RunArgs:     []string{"skills", "check"},
		},
		{
			Name:        "workflows-check",
			Description: "Validate skills, workflow catalog, and current workflow docs.",
			Command:     "zv workflows check",
			RunArgs:     []string{"workflows", "check"},
		},
		{
			Name:        "project-check",
			Description: "Run the full ClipHub CLI, workflow, docs, and skills contract.",
			Command:     "zv check",
			RunArgs:     []string{"check"},
		},
	})
}

func withWorkflowRunCommands(workflows []workflowInfo) []workflowInfo {
	for i := range workflows {
		if workflows[i].Name != "" && workflows[i].RunCommand == "" {
			workflows[i].RunCommand = workflowRunCommand(workflows[i].Name)
		}
		if workflows[i].Name != "" && workflows[i].ValidateCommand == "" {
			workflows[i].ValidateCommand = workflowValidateCommand(workflows[i].Name)
		}
		workflows[i].Arguments = workflowArgumentMetadata(workflows[i])
		workflows[i].Safety = workflowSafetyMetadata(workflows[i], workflows[i].Arguments)
		workflows[i].Contract = workflowContractMetadata(workflows[i])
	}
	return workflows
}

func workflowArgumentMetadata(workflow workflowInfo) workflowArguments {
	required := workflowRequiredFlags(workflow)
	commandName := fmt.Sprintf("%q", strings.Join(workflow.RunArgs, " "))
	valueFlags := commandValueFlags(commandName, required)
	if workflow.Name == "capabilities" || workflow.Name == "stream-variants" || workflow.Name == "skills-check" || workflow.Name == "workflows-check" || workflow.Name == "project-check" {
		valueFlags = append(valueFlags, "--format")
	}

	positionals := []workflowPositionalArgument{}
	conditional := []workflowConditionalRequirement{}
	switch workflow.Name {
	case "short":
		positionals = append(positionals, workflowPositionalArgument{
			Name:        "demo",
			Placeholder: "<demo.dem>",
			Required:    false,
		})
		conditional = append(conditional, workflowConditionalRequirement{
			Description:         "a demo path is required unless an existing recording result is supplied",
			UnlessAnyFlags:      []string{"--from-recording"},
			RequiredFlags:       []string{},
			RequiredPositionals: []string{"demo"},
		})
	case "flows-run":
		positionals = append(positionals, workflowPositionalArgument{
			Name:        "flow",
			Placeholder: "<demo|stream>",
			Required:    true,
		})
		conditional = append(conditional,
			workflowConditionalRequirement{
				Description:         "the demo flow requires a demo path (--demo) unless an existing kill plan (--killplan) is supplied",
				UnlessAnyFlags:      []string{"--killplan"},
				RequiredFlags:       []string{"--demo"},
				RequiredPositionals: []string{},
			},
			workflowConditionalRequirement{
				Description:         "the stream flow requires a source video (--input)",
				UnlessAnyFlags:      []string{},
				RequiredFlags:       []string{"--input"},
				RequiredPositionals: []string{},
			},
		)
	}

	return workflowArguments{
		Positionals:             positionals,
		RequiredFlags:           copyStrings(required),
		OptionalValueFlags:      flagsExcept(valueFlags, required),
		BooleanFlags:            copyStrings(commandBoolFlags(commandName)),
		ValueConstraints:        workflowValueConstraints(workflow),
		ConditionalRequirements: conditional,
	}
}

func workflowValueConstraints(workflow workflowInfo) []workflowValueConstraint {
	constraint := func(flag, defaultValue, discoveryCommand string, allowed ...string) workflowValueConstraint {
		return workflowValueConstraint{
			Flag:             flag,
			AllowedValues:    copyStrings(allowed),
			Default:          defaultValue,
			DiscoveryCommand: discoveryCommand,
		}
	}

	switch workflow.Name {
	case "short":
		return []workflowValueConstraint{
			constraint("--preset", editor.DefaultPreset().Name, "zv presets --format json", supportedPresetNames()...),
			constraint("--output-format", editor.OutputFormatShort9x16, "", editor.OutputFormatShort9x16, editor.OutputFormatLandscape16x9),
			constraint("--kill-effect", editor.KillEffectPunchIn, "", editor.KillEffectClean, editor.KillEffectPunchIn, editor.KillEffectVelocity, editor.KillEffectFreezeFlash, editor.KillEffectShake, editor.KillEffectGlitch),
			constraint("--transition", editor.TransitionFlash, "", editor.TransitionCut, editor.TransitionFlash, editor.TransitionWhip, editor.TransitionDip, editor.TransitionGlitch, editor.TransitionZoomWhip),
			constraint("--format", "text", "", "text", "json"),
		}
	case "demo-parse":
		return []workflowValueConstraint{
			constraint("--segment-mode", string(parser.SegmentModeKills), "",
				string(parser.SegmentModeKills), string(parser.SegmentModeSmokes), string(parser.SegmentModeUtility), string(parser.SegmentModeRecap)),
		}
	case "utility-audit":
		return []workflowValueConstraint{
			constraint("--format", "csv", "", "csv", "json"),
		}
	case "record":
		return []workflowValueConstraint{
			constraint("--hud", string(recording.HUDModeGameplay), "",
				string(recording.HUDModeGameplay), string(recording.HUDModeClean), string(recording.HUDModeDeathnotices)),
			constraint("--format", "text", "", "text", "json"),
		}
	case "shorts-render":
		defaultPreset := editor.DefaultPreset()
		return []workflowValueConstraint{
			constraint("--preset", defaultPreset.Name, "zv presets --format json", supportedPresetNames()...),
			constraint("--effects-preset", defaultPreset.EffectsPreset, "", editor.EffectsPresetViralUltraClean, editor.EffectsPresetViralAggressive, editor.EffectsPresetGameplayNative),
			constraint("--output-format", editor.OutputFormatShort9x16, "", editor.OutputFormatShort9x16, editor.OutputFormatLandscape16x9),
			constraint("--kill-effect", editor.KillEffectPunchIn, "", editor.KillEffectClean, editor.KillEffectPunchIn, editor.KillEffectVelocity, editor.KillEffectFreezeFlash, editor.KillEffectShake, editor.KillEffectGlitch),
			constraint("--transition", editor.TransitionFlash, "", editor.TransitionCut, editor.TransitionFlash, editor.TransitionWhip, editor.TransitionDip, editor.TransitionGlitch, editor.TransitionZoomWhip),
			constraint("--video-preset", defaultPreset.VideoPreset, "",
				"ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow"),
			constraint("--format", "text", "", "text", "json"),
		}
	case "compose-final":
		return []workflowValueConstraint{
			constraint("--format", "text", "", "text", "json"),
		}
	case "stream-plan":
		return []workflowValueConstraint{
			constraint("--variant", streamclips.DefaultVariant().Name, "zv stream variants --format json", streamclips.VariantNames()...),
			constraint("--format", "text", "", "text", "json"),
		}
	case "stream-fetch":
		return []workflowValueConstraint{
			constraint("--format", "text", "", "text", "json"),
		}
	case "faceit-index", "stream-render", "stream-variants", "demo-players", "demo-moments", "demo-select", "demo-probe", "demo-voice", "flows-run",
		"analysis-tactical", "analysis-rounds", "analysis-tendencies":
		return []workflowValueConstraint{
			constraint("--format", "text", "", "text", "json"),
		}
	case "capabilities", "skills-check", "workflows-check", "project-check":
		return []workflowValueConstraint{
			constraint("--format", "text", "", "text", "json"),
		}
	default:
		return []workflowValueConstraint{}
	}
}

func workflowRequiredFlags(workflow workflowInfo) []string {
	if workflow.Name == "record" {
		return []string{"--killplan", "--demo", "--out"}
	}
	// requiredFlagsFromCommand already drops the boolean --dry-run that the
	// flows-run Command documents, so only --run-dir remains required there.
	return requiredFlagsFromCommand(workflow.Command)
}

func workflowContractMetadata(workflow workflowInfo) workflowContract {
	contract := workflowContract{
		RequiredArtifacts:    []string{},
		ProducedArtifactKeys: []string{},
		SafetyGates:          []string{},
		DryRunBehavior:       "not supported; use the workflow validate command for argument-only preflight",
		LiveBehavior:         "executes the documented command exactly as delegated by zv workflows run",
		ResumePolicy:         "inspect command output and artifacts before rerun",
	}
	if workflow.Safety.SupportsDryRun {
		contract.DryRunBehavior = "forward --dry-run in the workflow args to validate inputs without live media work when the command supports it"
	}
	if workflow.Safety.ReadOnly {
		contract.LiveBehavior = "read-only inspection; no pipeline artifacts are written by the command contract"
		contract.ResumePolicy = "safe to rerun; output is derived from current local state"
	}
	if workflow.Name == "record" {
		contract.ResumePolicy = "recording uses a fresh output namespace after failed live capture; deterministic demo_incompatible failures are not retried"
	} else if workflow.Name == "short" || workflow.Name == "flows-run" {
		contract.ResumePolicy = "workers and staged runs skip completed durable artifacts when safe; live capture still requires explicit approval and a fresh failed-recording namespace"
	} else if workflow.Name == "serve" {
		contract.ResumePolicy = "long-running service; stop the owning process instead of starting duplicate orchestrators"
	}

	switch workflow.Name {
	case "short":
		contract.RequiredArtifacts = []string{"demo path or existing recording result"}
		contract.ProducedArtifactKeys = []string{"killplan", "selected-plan", "recording-result", "publish-pack"}
		contract.SafetyGates = []string{"creative brief approval", "live HLAE/CS2 capture approval", "long FFmpeg render approval", "thumbnail selection when covers are enabled"}
		contract.LiveBehavior = "runs parse, capture, render, and publish-pack stages using explicit approved flags"
	case "capabilities":
		contract.ProducedArtifactKeys = []string{"tool-readiness-report"}
	case "faceit-index":
		contract.ProducedArtifactKeys = []string{"faceit-demo-index"}
		contract.SafetyGates = []string{"FACEIT Data API credentials must remain in environment or server-side secret storage"}
	case "demo-parse":
		contract.RequiredArtifacts = []string{"demo"}
		contract.ProducedArtifactKeys = []string{"killplan"}
	case "demo-players":
		contract.RequiredArtifacts = []string{"demo"}
		contract.ProducedArtifactKeys = []string{"demo-roster"}
	case "demo-moments":
		contract.RequiredArtifacts = []string{"killplan"}
		contract.ProducedArtifactKeys = []string{"moments"}
	case "demo-select":
		contract.RequiredArtifacts = []string{"killplan"}
		contract.ProducedArtifactKeys = []string{"selected-plan"}
	case "demo-probe":
		contract.RequiredArtifacts = []string{"demo"}
		contract.ProducedArtifactKeys = []string{"playability"}
	case "demo-voice":
		contract.RequiredArtifacts = []string{"demo", "SteamID64"}
		contract.ProducedArtifactKeys = []string{"voice-probe"}
	case "utility-audit":
		contract.RequiredArtifacts = []string{"utility-plan", "lineup-catalog"}
		contract.ProducedArtifactKeys = []string{"utility-audit"}
	case "record":
		contract.RequiredArtifacts = []string{"selected killplan", "demo"}
		contract.ProducedArtifactKeys = []string{"recording-result", "capture-script", "segment-clips"}
		contract.SafetyGates = []string{"creative brief HUD/killfeed choices", "live HLAE/CS2 capture approval"}
		contract.LiveBehavior = "launches HLAE/CS2 and records selected POV ranges; all captures contend for one cs2.exe lane"
	case "compose-final":
		contract.RequiredArtifacts = []string{"recording-result"}
		contract.ProducedArtifactKeys = []string{"final-mp4"}
	case "music-analyze":
		contract.RequiredArtifacts = []string{"audio-or-video"}
		contract.ProducedArtifactKeys = []string{"rhythm-plan"}
	case "shorts-render":
		contract.RequiredArtifacts = []string{"recording-result"}
		contract.ProducedArtifactKeys = []string{"render-manifest", "qa-report", "publish-pack"}
		contract.SafetyGates = []string{"creative brief approval", "long FFmpeg render approval", "thumbnail selection when covers are enabled", "third-party music provenance when music is supplied"}
		contract.LiveBehavior = "renders the approved recording result into a local publish pack and QA artifacts"
	case "stream-fetch":
		contract.RequiredArtifacts = []string{"allowlisted stream URL"}
		contract.ProducedArtifactKeys = []string{"stream-mp4"}
		contract.SafetyGates = []string{"network download approval for the requested URL"}
	case "stream-variants":
		contract.ProducedArtifactKeys = []string{"stream-variant-catalog"}
	case "stream-plan":
		contract.RequiredArtifacts = []string{"stream-mp4"}
		contract.ProducedArtifactKeys = []string{"stream-edit-plan"}
		contract.SafetyGates = []string{"stream bounds, crop/framing, title, audio, and music choices"}
	case "stream-render":
		contract.RequiredArtifacts = []string{"stream-mp4", "stream-edit-plan"}
		contract.ProducedArtifactKeys = []string{"stream-render-manifest", "publish-pack"}
		contract.SafetyGates = []string{"approved persisted edit plan", "long FFmpeg render approval", "third-party music provenance when music is supplied"}
	case "analysis-tactical":
		contract.RequiredArtifacts = []string{"demo"}
		contract.ProducedArtifactKeys = []string{"tactical-document", "position-blob"}
	case "analysis-rounds", "analysis-tendencies":
		contract.RequiredArtifacts = []string{"tactical-document"}
		contract.ProducedArtifactKeys = []string{"tactical-review"}
	case "analysis-tactical-data":
		contract.RequiredArtifacts = []string{"demo", "tick-window"}
		contract.ProducedArtifactKeys = []string{"tactical-data-export"}
	case "analysis-viewer":
		contract.RequiredArtifacts = []string{"analysis-json"}
	case "gallery-open":
		contract.RequiredArtifacts = []string{"publish-gallery"}
	case "flows-run":
		contract.RequiredArtifacts = []string{"demo flow: demo or killplan", "stream flow: stream-mp4"}
		contract.ProducedArtifactKeys = []string{"run-directory-plan"}
		contract.SafetyGates = []string{"--dry-run is required for whole-flow planning"}
		contract.LiveBehavior = "whole-flow execution remains dry-run only through this command contract"
	case "serve":
		contract.ProducedArtifactKeys = []string{"local-http-api", "worker-queue"}
	case "skills-check", "workflows-check", "project-check":
		contract.ProducedArtifactKeys = []string{"contract-check-report"}
	}
	return contract
}

func workflowSafetyMetadata(workflow workflowInfo, arguments workflowArguments) workflowSafety {
	readOnly := false
	switch workflow.Name {
	case "capabilities", "stream-variants", "analysis-viewer", "gallery-open", "skills-check", "workflows-check", "project-check",
		"analysis-rounds", "analysis-tendencies":
		// analysis-rounds and analysis-tendencies only read a tactical document
		// and print; analysis-tactical is not here because it writes artifacts.
		readOnly = true
	}

	longRunning := false
	switch workflow.Name {
	case "short", "faceit-index", "record", "compose-final", "music-analyze", "shorts-render", "stream-fetch", "stream-render", "analysis-viewer", "serve", "flows-run",
		"analysis-tactical", "demo-voice":
		// flows-run really parses demos and probes media across a whole journey,
		// and analysis-tactical parses a whole demo before it writes anything.
		longRunning = true
	}

	return workflowSafety{
		ReadOnly:       readOnly,
		SupportsDryRun: containsString(arguments.BooleanFlags, "--dry-run"),
		LongRunning:    longRunning,
	}
}

func flagsExcept(flags, excluded []string) []string {
	out := make([]string, 0, len(flags))
	seen := make(map[string]struct{}, len(flags))
	for _, flag := range flags {
		if containsString(excluded, flag) {
			continue
		}
		if _, ok := seen[flag]; ok {
			continue
		}
		seen[flag] = struct{}{}
		out = append(out, flag)
	}
	return out
}

func copyStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values...)
}

func workflowRunCommand(name string) string {
	return "zv workflows run " + name
}

func workflowValidateCommand(name string) string {
	return "zv workflows validate " + name
}
