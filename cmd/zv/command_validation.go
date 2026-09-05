package main

import (
	"fmt"
	"strconv"
	"strings"
)

func validateSkillCommand(command []string) string {
	if len(command) == 0 {
		return "missing zv command"
	}
	switch command[0] {
	case "short":
		return validateShortCommand(command[1:])
	case "batch":
		return validateBatchCommand(command[1:])
	case "metrics", "errors":
		// Read-only observability commands; all flags are optional.
		return ""
	case "presets":
		if issue := validateFormattedCommand("presets", command[1:]); issue != "" {
			return issue
		}
	case "capabilities":
		if issue := validateFormattedCommand("capabilities", command[1:]); issue != "" {
			return issue
		}
	case "verify":
		return validateVerifyCommand(command[1:])
	case "full-demo":
		if len(command) < 2 || !containsString([]string{"defaults", "import", "asset", "plan", "inspect", "execute"}, command[1]) {
			return `expected full-demo defaults, import, asset, plan, inspect, or execute`
		}
		if issue := validateRequiredFlags(fmt.Sprintf("%q", "full-demo "+command[1]), command[2:], requiredFlagsForRunArgs("full-demo", command[1])...); issue != "" {
			return issue
		}
		if isSingleHelp(command[2:]) {
			return ""
		}
		if command[1] == "inspect" && hasFlagValue(command[2:], "--plan") == hasFlagValue(command[2:], "--job") {
			return "full-demo inspect requires exactly one of --plan or --job"
		}
		if command[1] == "execute" && !containsString(command[2:], "--allow-safe-tail-trim=true") && !containsString(command[2:], "--allow-safe-tail-trim=false") {
			return "full-demo execute requires explicit --allow-safe-tail-trim=true or =false matching the plan"
		}
		return ""
	case "faceit":
		if len(command) < 2 || command[1] != "index" {
			return `uses non-standard zv command "faceit"; expected "faceit index"`
		}
		return validateRequiredFlags(`"faceit index"`, command[2:], requiredFlagsForRunArgs("faceit", "index")...)
	case "check":
		if issue := validateFormattedCommand("check", command[1:]); issue != "" {
			return issue
		}
	case "demo":
		if len(command) < 2 || (command[1] != "parse" && command[1] != "players" && command[1] != "moments" && command[1] != "select" && command[1] != "anticheat" && command[1] != "probe" && command[1] != "voice") {
			return `uses non-standard zv command "demo"; expected "demo parse", "demo players", "demo moments", "demo select", "demo anticheat", "demo probe", or "demo voice"`
		}
		switch command[1] {
		case "parse":
			return validateRequiredFlags(`"demo parse"`, command[2:], requiredFlagsForRunArgs("demo", "parse")...)
		case "players":
			return validateRequiredFlags(`"demo players"`, command[2:], requiredFlagsForRunArgs("demo", "players")...)
		case "moments":
			return validateRequiredFlags(`"demo moments"`, command[2:], requiredFlagsForRunArgs("demo", "moments")...)
		case "select":
			return validateDemoSelectCommand(command[2:])
		case "probe":
			return validateRequiredFlags(`"demo probe"`, command[2:], requiredFlagsForRunArgs("demo", "probe")...)
		case "voice":
			return validateRequiredFlags(`"demo voice"`, command[2:], requiredFlagsForRunArgs("demo", "voice")...)
		case "anticheat":
			// The screening pass has no workflow entry, so its required flags
			// are stated here rather than derived from the catalog. Without
			// this a documented line missing --demo passed the canonical check
			// and only failed once someone ran it.
			if len(command) > 2 && command[2] == "calibrate" {
				return validateRequiredFlags(`"demo anticheat calibrate"`, command[3:], "--demos", "--id")
			}
			return validateRequiredFlags(`"demo anticheat"`, command[2:], "--demo")
		}
	case "utility":
		if len(command) < 2 || command[1] != "audit" {
			return `uses non-standard zv command "utility"; expected "utility audit"`
		}
		return validateRequiredFlags(`"utility audit"`, command[2:], requiredFlagsForRunArgs("utility", "audit")...)
	case "compose":
		if len(command) < 2 || command[1] != "final" {
			return `uses non-standard zv command "compose"; expected "compose final"`
		}
		return validateRequiredFlags(`"compose final"`, command[2:], requiredFlagsForRunArgs("compose", "final")...)
	case "shorts":
		if len(command) < 2 || command[1] != "render" {
			return `uses non-standard zv command "shorts"; expected "shorts render"`
		}
		if issue := validateRequiredFlags(`"shorts render"`, command[2:], requiredFlagsForRunArgs("shorts", "render")...); issue != "" {
			return issue
		}
		if preset, ok := flagValue(command[2:], "--preset"); ok && !containsString(supportedPresetNames(), preset) {
			return fmt.Sprintf("unsupported preset %q for \"shorts render\"; supported presets: %s", preset, strings.Join(supportedPresetNames(), ", "))
		}
		return ""
	case "stream":
		if len(command) < 2 || (command[1] != "fetch" && command[1] != "variants" && command[1] != "plan" && command[1] != "render") {
			return `uses non-standard zv command "stream"; expected "stream fetch", "stream variants", "stream plan", or "stream render"`
		}
		switch command[1] {
		case "variants":
			return validateFormattedCommand("stream variants", command[2:])
		case "fetch":
			return validateRequiredFlags(`"stream fetch"`, command[2:], requiredFlagsForRunArgs("stream", "fetch")...)
		case "plan":
			return validateRequiredFlags(`"stream plan"`, command[2:], requiredFlagsForRunArgs("stream", "plan")...)
		case "render":
			return validateRequiredFlags(`"stream render"`, command[2:], requiredFlagsForRunArgs("stream", "render")...)
		}
	case "music":
		if len(command) < 2 || command[1] != "analyze" {
			return `uses non-standard zv command "music"; expected "music analyze"`
		}
		return validateMusicAnalyzeCommand(command[2:])
	case "analysis":
		if len(command) < 2 || !containsString(analysisSubcommands(), command[1]) {
			return `uses non-standard zv command "analysis"; expected "analysis tactical", "analysis rounds", "analysis tendencies", "analysis tactical-data", or "analysis view"`
		}
		return validateRequiredFlags(
			fmt.Sprintf("%q", "analysis "+command[1]),
			command[2:],
			requiredFlagsForRunArgs("analysis", command[1])...)
	case "gallery":
		if len(command) < 2 || command[1] != "open" {
			return `uses non-standard zv command "gallery"; expected "gallery open"`
		}
		return validateRequiredFlags(`"gallery open"`, command[2:], requiredFlagsForRunArgs("gallery", "open")...)
	case "skills":
		if len(command) < 2 || (command[1] != "list" && command[1] != "show" && command[1] != "check") {
			return `uses non-standard zv command "skills"; expected "skills list", "skills show", or "skills check"`
		}
		switch command[1] {
		case "list", "check":
			if issue := validateFormattedCommand(strings.Join(command[:2], " "), command[2:]); issue != "" {
				return issue
			}
		case "show":
			if issue := validateSkillShowCommand(command[2:]); issue != "" {
				return issue
			}
		}
	case "record":
		return validateRequiredFlags(`"record"`, command[1:], requiredFlagsForRunArgs("record")...)
	case "serve":
		if isSingleHelp(command[1:]) {
			return ""
		}
		if len(command) != 1 {
			return `unexpected extra args for "serve"`
		}
	case "workflows":
		if len(command) < 2 || (command[1] != "list" && command[1] != "show" && command[1] != "validate" && command[1] != "run" && command[1] != "check") {
			return `uses non-standard zv command "workflows"; expected "workflows list", "workflows show", "workflows validate", "workflows run", or "workflows check"`
		}
		switch command[1] {
		case "list":
			if issue := validateWorkflowListCommand(command[2:]); issue != "" {
				return issue
			}
		case "show":
			if issue := validateWorkflowShowCommand(command[2:]); issue != "" {
				return issue
			}
		case "validate":
			if issue := validateWorkflowValidateCommand(command[2:]); issue != "" {
				return issue
			}
		case "run":
			if issue := validateWorkflowRunCommand(command[2:]); issue != "" {
				return issue
			}
		case "check":
			if issue := validateFormattedCommand(strings.Join(command[:2], " "), command[2:]); issue != "" {
				return issue
			}
		}
	case "flows":
		if len(command) < 2 || (command[1] != "list" && command[1] != "show" && command[1] != "run") {
			return `uses non-standard zv command "flows"; expected "flows list", "flows show", or "flows run"`
		}
		switch command[1] {
		case "list":
			return validateFormattedCommand("flows list", command[2:])
		case "show":
			format, rest, err := parseFormatArgs(command[2:])
			_ = format
			if err != nil {
				return err.Error()
			}
			if len(rest) != 1 || (rest[0] != "demo" && rest[0] != "stream") {
				return `"flows show" requires exactly one of: demo, stream`
			}
		case "run":
			return validateFlowsRunCommand(command[2:])
		}
	default:
		return fmt.Sprintf("uses non-standard zv command %q", command[0])
	}
	return ""
}

// validateFlowsRunCommand validates the canonical "flows run" contract: the flow
// name (demo/stream, or the <demo|stream> template used in docs and the catalog)
// must be the FIRST token, followed by the flow flags. Requiring the flow name
// first avoids the earlier "first non-dash token" scan, which misparsed
// "flows run --run-dir X demo" by stealing a flag value as the flow. Required-flag
// reporting is delegated to validateRequiredFlags so the message matches every
// other workflow. The remaining checks are intentionally shared with workflow
// preflight so it never approves argv that the direct runner will reject before
// starting a flow.
func validateFlowsRunCommand(args []string) string {
	if isSingleHelp(args) {
		return ""
	}
	if len(args) == 0 {
		// A bare invocation surfaces the shared missing-required-flag contract
		// first (every other workflow does), then the missing flow name.
		if issue := validateRequiredFlags(`"flows run"`, args, "--run-dir"); issue != "" {
			return issue
		}
		return `missing flow name for "flows run"; expected demo or stream`
	}
	if strings.HasPrefix(args[0], "-") {
		// There ARE tokens but the flow name is not first: never scan later tokens
		// for it, which stole flag values like in "flows run --run-dir X demo".
		return `missing flow name for "flows run"; expected demo or stream`
	}
	flowName := args[0]
	rest := args[1:]
	if flowName != "demo" && flowName != "stream" && flowName != "<demo|stream>" {
		return fmt.Sprintf(`unknown flow %q for "flows run"; expected demo or stream`, flowName)
	}
	if issue := validateRequiredFlags(`"flows run"`, rest, "--run-dir"); issue != "" {
		return issue
	}
	// The template is catalog metadata rather than runnable argv. It remains
	// accepted here so the catalog can validate itself; runFlowsRun rejects it.
	if flowName == "<demo|stream>" {
		return ""
	}
	if !booleanFlagIsTrue(rest, "--dry-run") {
		return `"flows run" currently supports only --dry-run; real execution remains stage by stage behind the creative gates`
	}
	switch flowName {
	case "demo":
		if !hasFlagValue(rest, "--demo") {
			return `the demo flow requires --demo for capture and render; --killplan only skips parse`
		}
		if !hasFlagValue(rest, "--killplan") && !hasFlagValue(rest, "--steamid") {
			return `--demo requires --steamid for "demo parse"`
		}
	case "stream":
		if !hasFlagValue(rest, "--input") {
			return `the stream flow requires --input`
		}
	}
	return ""
}

func validateBatchCommand(args []string) string {
	if isSingleHelp(args) {
		return ""
	}
	hasDir := false
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			hasDir = true
			break
		}
	}
	if !hasDir {
		return `missing directory for "batch"; pass <dir>`
	}
	return ""
}

func validateShortCommand(args []string) string {
	if isSingleHelp(args) {
		return ""
	}
	rest := args
	hasDemo := len(rest) > 0 && !strings.HasPrefix(rest[0], "-")
	if hasDemo {
		rest = rest[1:]
	}
	if issue := validateRequiredFlags(`"short"`, rest, "--prompt"); issue != "" {
		return issue
	}
	if !hasDemo && !hasFlagValue(rest, "--from-recording") {
		return `missing demo path for "short"; pass <demo.dem> or --from-recording <recording-result.json>`
	}
	if format, ok := flagValue(rest, "--format"); ok {
		if format != "text" && format != "json" {
			return fmt.Sprintf("unsupported format %q for \"short\"", format)
		}
		if format == "json" && !booleanFlagIsTrue(rest, "--dry-run") {
			return `--format json requires --dry-run for "short"`
		}
	}
	return ""
}

// analysisSubcommands lists the analysis group members in the order the usage
// text presents them.
func analysisSubcommands() []string {
	return []string{"tactical", "rounds", "tendencies", "tactical-data", "view"}
}

// tacticalFilterFlagNames lists the round-filter flags "analysis rounds" and
// "analysis tendencies" accept. They map onto tacticalplan.FilterFromValues, so
// this list must stay in step with tacticalFilterFlags.
func tacticalFilterFlagNames() []string {
	names := make([]string, 0, len(tacticalFilterFlags()))
	for _, mapping := range tacticalFilterFlags() {
		names = append(names, "--"+mapping.Name)
	}
	return names
}

func requiredFlagsForRunArgs(args ...string) []string {
	if equalArgs(args, []string{"record"}) {
		return []string{"--killplan", "--demo", "--out"}
	}
	for _, workflow := range workflowCatalog() {
		if !equalArgs(workflow.RunArgs, args) {
			continue
		}
		return workflowRequiredFlags(workflow)
	}
	return nil
}

func requiredFlagsFromCommand(command string) []string {
	fields, ok := splitCommandFields(command)
	if !ok {
		return nil
	}
	var flags []string
	for _, field := range fields {
		if !strings.HasPrefix(field, "--") {
			continue
		}
		// --dry-run appears in a Command template only to make the documented line
		// runnable (flows-run rejects non-dry-run); it is a boolean, not a required
		// value flag, so it never counts as a required flag.
		if field == "--dry-run" {
			continue
		}
		flags = append(flags, field)
	}
	return flags
}

func validateFormattedCommand(commandName string, args []string) string {
	if isSingleHelp(args) {
		return ""
	}
	if _, rest, err := parseFormatArgs(args); err != nil {
		return err.Error()
	} else if len(rest) != 0 {
		return fmt.Sprintf("unexpected extra args for %q", commandName)
	}
	return ""
}

func validateWorkflowListCommand(args []string) string {
	return validateFormattedCommand("workflows list", args)
}

func validateSkillShowCommand(args []string) string {
	if isSingleHelp(args) {
		return ""
	}
	if _, rest, err := parseFormatArgs(args); err != nil {
		return err.Error()
	} else if len(rest) == 0 {
		return `missing skill name for "skills show"`
	} else if len(rest) > 1 {
		return `unexpected extra args for "skills show"`
	}
	return ""
}

func validateWorkflowShowCommand(args []string) string {
	if isSingleHelp(args) {
		return ""
	}
	if _, rest, err := parseFormatArgs(args); err != nil {
		return err.Error()
	} else if len(rest) == 0 {
		return `missing workflow name for "workflows show"`
	} else if len(rest) > 1 {
		return `unexpected extra args for "workflows show"`
	} else if _, ok := findWorkflow(rest[0]); !ok {
		return fmt.Sprintf(`unknown workflow name %q for "workflows show"`, rest[0])
	}
	return ""
}

func validateWorkflowRunCommand(args []string) string {
	if len(args) == 0 {
		return `missing workflow name for "workflows run"`
	}
	workflow, ok := findWorkflow(args[0])
	if !ok {
		return fmt.Sprintf(`unknown workflow name %q for "workflows run"`, args[0])
	}
	rest := args[1:]
	if issue := validateWorkflowRunForwardedArgs(workflow, rest); issue != "" {
		return issue
	}
	return ""
}

func validateWorkflowValidateCommand(args []string) string {
	control, forwarded := splitWorkflowValidateArgs(args)
	_, rest, err := parseFormatArgs(control)
	if err != nil {
		return err.Error()
	}
	if len(rest) == 0 {
		return `missing workflow name for "workflows validate"`
	}
	if len(rest) > 1 {
		return `unexpected extra args for "workflows validate"; use "--" before workflow flags`
	}
	workflow, ok := findWorkflow(rest[0])
	if !ok {
		return fmt.Sprintf(`unknown workflow name %q for "workflows validate"`, rest[0])
	}
	command := append([]string(nil), workflow.RunArgs...)
	command = append(command, forwarded...)
	if issue := validateSkillCommand(command); issue != "" {
		return issue
	}
	return validateWorkflowValueConstraints(workflow, forwarded)
}

func validateWorkflowRunForwardedArgs(workflow workflowInfo, rest []string) string {
	if len(rest) > 0 && rest[0] != "--" {
		return `missing "--" separator before forwarded args for "workflows run"`
	}
	var forwarded []string
	if len(rest) > 0 {
		forwarded = rest[1:]
	}
	if isSingleHelp(forwarded) {
		return ""
	}
	command := append([]string(nil), workflow.RunArgs...)
	command = append(command, forwarded...)
	if issue := validateSkillCommand(command); issue != "" {
		return issue
	}
	return validateWorkflowValueConstraints(workflow, forwarded)
}

func validateWorkflowValueConstraints(workflow workflowInfo, args []string) string {
	for _, constraint := range workflow.Arguments.ValueConstraints {
		value, ok := flagValue(args, constraint.Flag)
		if !ok || containsString(constraint.AllowedValues, value) {
			continue
		}
		return fmt.Sprintf("invalid value %q for flag %s in workflow %q; allowed values: %s",
			value, constraint.Flag, workflow.Name, strings.Join(constraint.AllowedValues, ", "))
	}
	if workflow.Name == "demo-moments" {
		if value, ok := flagValue(args, "--top"); ok {
			if _, err := parseNonNegativeIntFlag("--top", value); err != nil {
				return err.Error()
			}
		}
	}
	return ""
}

func parseNonNegativeIntFlag(name, raw string) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}
	if value < 0 {
		return 0, fmt.Errorf("%s must be >= 0", name)
	}
	return value, nil
}

func flagValue(args []string, name string) (string, bool) {
	for i, arg := range args {
		if value, ok := strings.CutPrefix(arg, name+"="); ok {
			return value, true
		}
		if arg == name && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", false
}

func validateRequiredFlags(commandName string, args []string, required ...string) string {
	if isSingleHelp(args) {
		return ""
	}
	valueFlags := commandValueFlags(commandName, required)
	boolFlags := commandBoolFlags(commandName)
	if flag := duplicateFlag(args, commandRepeatableFlags(commandName)...); flag != "" {
		return fmt.Sprintf("duplicate flag %s for %s", flag, commandName)
	}
	if flag := unknownFlag(args, valueFlags, boolFlags); flag != "" {
		return fmt.Sprintf("unknown flag %s for %s", flag, commandName)
	}
	if flag, value := invalidBooleanFlagValue(args, boolFlags); flag != "" {
		return fmt.Sprintf("invalid boolean value %q for flag %s for %s", value, flag, commandName)
	}
	var missing []string
	for _, name := range required {
		if !hasFlagValue(args, name) {
			missing = append(missing, name)
		}
	}
	if len(missing) == 1 {
		return fmt.Sprintf("missing required flag %s for %s", missing[0], commandName)
	}
	if len(missing) > 1 {
		return fmt.Sprintf("missing required flags %s for %s", strings.Join(missing, ", "), commandName)
	}
	if flag, value := booleanFlagSeparateValue(args, boolFlags); flag != "" {
		return fmt.Sprintf("boolean flag %s for %s does not take separate value %q; use %s=%s", flag, commandName, value, flag, value)
	}
	if flag := optionalValueFlagMissingValue(args, valueFlags, required); flag != "" {
		return fmt.Sprintf("missing value for flag %s for %s", flag, commandName)
	}
	if arg := unexpectedPositionalArg(args, valueFlags); arg != "" {
		return fmt.Sprintf("unexpected positional arg %q for %s; quote paths with spaces", arg, commandName)
	}
	return ""
}

func validateDemoSelectCommand(args []string) string {
	if isSingleHelp(args) {
		return ""
	}
	if issue := validateRequiredFlags(`"demo select"`, args, "--killplan", "--out"); issue != "" {
		return issue
	}
	segments := hasFlagValue(args, "--segments")
	topValue, top := flagValue(args, "--top")
	if segments == top {
		return `"demo select" requires exactly one of --segments or --top`
	}
	if top {
		value, err := strconv.Atoi(topValue)
		if err != nil {
			return fmt.Sprintf("invalid value %q for flag --top for \"demo select\": %v", topValue, err)
		}
		if value <= 0 {
			return `flag --top for "demo select" must be > 0`
		}
	}
	return ""
}

func validateMusicAnalyzeCommand(args []string) string {
	if issue := validateRequiredFlags(`"music analyze"`, args, requiredFlagsForRunArgs("music", "analyze")...); issue != "" {
		return issue
	}
	_, hasKillPlan := flagValue(args, "--killplan")
	_, hasRecordingResult := flagValue(args, "--recording-result")
	if hasKillPlan && hasRecordingResult {
		return "--killplan and --recording-result are mutually exclusive"
	}
	if raw, ok := flagValue(args, "--limit"); ok {
		limit, err := parseNonNegativeIntFlag("--limit", raw)
		if err != nil {
			return err.Error()
		}
		if limit > 0 && !booleanFlagIsTrue(args, "--rank-moments") {
			return `--limit requires --rank-moments for "music analyze"`
		}
	}
	return ""
}

func commandValueFlags(commandName string, required []string) []string {
	flags := append([]string(nil), required...)
	switch commandName {
	case `"full-demo defaults"`:
		flags = append(flags, "--out", "--format")
	case `"full-demo import"`, `"full-demo asset"`, `"full-demo plan"`, `"full-demo execute"`:
		flags = append(flags, "--url", "--format")
	case `"full-demo inspect"`:
		flags = append(flags, "--plan", "--job", "--document", "--out", "--url", "--format")
	case `"demo parse"`:
		flags = append(flags, "--segment-mode", "--rules")
	case `"faceit index"`:
		flags = append(flags, "--from", "--to", "--format")
	case `"demo players"`:
		flags = append(flags, "--contains", "--out", "--format")
	case `"demo moments"`:
		flags = append(flags, "--out", "--top", "--format")
	case `"demo select"`:
		flags = append(flags, "--segments", "--top", "--format")
	case `"demo probe"`:
		flags = append(flags, "--format")
	case `"demo voice"`:
		flags = append(flags, "--format", "--extract")
	case `"demo anticheat"`:
		flags = append(flags, "--baseline", "--out", "--dossier", "--format")
	case `"demo anticheat calibrate"`:
		flags = append(flags, "--out", "--format")
	case `"utility audit"`:
		flags = append(flags, "--format")
	case `"short"`:
		flags = append(flags, "--preset", "--out", "--music", "--target-steamid", "--hlae", "--cs2", "--from-recording", "--format", "--output-format", "--kill-effect", "--transition", "--encoder", "--gap-timescale", "--settle-seconds", "--threads")
	case `"record"`:
		flags = append(flags, "--hlae", "--cs2", "--hud", "--fps", "--video-crf", "--encoder", "--gap-timescale", "--settle-seconds", "--timeout", "--format", "--full-demo-plan")
	case `"compose final"`:
		flags = append(flags, "--ffmpeg", "--timeout", "--format")
	case `"shorts render"`:
		flags = append(flags,
			"--full-demo-execution",
			"--killplan",
			"--publish-dir",
			"--preset",
			"--effects",
			"--effects-preset",
			"--music",
			"--music-volume",
			"--game-volume",
			"--voice-dir",
			"--voice-volume",
			"--rhythm",
			"--output-format",
			"--kill-effect",
			"--transition",
			"--intro-text",
			"--outro-text",
			"--tail-trim",
			"--fps",
			"--lineup-catalog",
			"--segments",
			"--limit",
			"--video-crf",
			"--video-preset",
			"--render-jobs",
			"--threads",
			"--ffmpeg",
			"--ffprobe",
			"--format",
		)
	case `"stream fetch"`:
		flags = append(flags, "--max-bytes", "--ytdlp", "--timeout", "--format")
	case `"stream plan"`:
		flags = append(flags,
			"--variant",
			"--clip-id",
			"--clip-start",
			"--clip-end",
			"--title",
			"--streamer",
			"--face-crop",
			"--gameplay-crop",
			"--ffprobe",
			"--format",
		)
	case `"stream render"`:
		flags = append(flags,
			"--title",
			"--ffmpeg",
			"--ffprobe",
			"--timeout",
			"--work-dir",
			"--music-dir",
			"--format",
		)
	case `"music analyze"`:
		flags = append(flags,
			"--killplan",
			"--recording-result",
			"--ffmpeg",
			"--sample-rate",
			"--min-bpm",
			"--max-bpm",
			"--kill-offset-ms",
			"--max-beats",
			"--max-onsets",
			"--tail-trim",
			"--limit",
		)
	case `"analysis tactical-data"`:
		flags = append(flags, "--sample")
	case `"analysis view"`:
		flags = append(flags, "--addr")
	case `"analysis tactical"`:
		flags = append(flags, "--positions", "--hz", "--cell-size", "--format")
	case `"analysis rounds"`, `"analysis tendencies"`:
		flags = append(flags, "--format")
		flags = append(flags, tacticalFilterFlagNames()...)
	case `"flows run"`:
		flags = append(flags, "--demo", "--steamid", "--killplan", "--input", "--format")
	}
	return flags
}

func commandBoolFlags(commandName string) []string {
	switch commandName {
	case `"full-demo import"`, `"full-demo asset"`, `"full-demo plan"`:
		return []string{"--dry-run"}
	case `"full-demo execute"`:
		return []string{"--dry-run", "--allow-safe-tail-trim"}
	case `"demo parse"`:
		return []string{"--verbose", "--dry-run"}
	case `"faceit index"`:
		return []string{"--dry-run"}
	case `"demo moments"`:
		return []string{"--dry-run"}
	case `"demo probe"`:
		return []string{"--dry-run"}
	case `"demo voice"`:
		return []string{"--dry-run"}
	case `"demo select"`, `"demo anticheat"`, `"demo anticheat calibrate"`:
		return []string{"--dry-run"}
	case `"short"`:
		return []string{"--dry-run", "--intro", "--outro", "--hook", "--kill-counter", "--covers", "--cover-sheets", "--cover-first-frame"}
	case `"compose final"`:
		return []string{"--dry-run"}
	case `"record"`:
		return []string{"--dry-run", "--fake", "--portrait-safe-killfeed"}
	case `"shorts render"`:
		return []string{
			"--audio-normalize",
			"--cover-first-frame",
			"--cover-sheets",
			"--covers",
			"--dry-run",
			"--hq-filters",
			"--intro",
			"--outro",
			"--hook",
			"--kill-counter",
			"--rank-moments",
			"--killfeed-overlay",
			"--no-covers",
			"--open-gallery",
			"--quality-checks",
			"--skip-existing",
			"--temporal-smoothing",
			"--compile-segments",
		}
	case `"stream fetch"`, `"stream plan"`, `"stream render"`:
		return []string{"--dry-run"}
	case `"music analyze"`:
		return []string{"--rank-moments"}
	case `"flows run"`:
		return []string{"--dry-run"}
	case `"analysis tactical"`:
		return []string{"--dry-run"}
	default:
		return nil
	}
}

func commandRepeatableFlags(commandName string) []string {
	switch commandName {
	case `"analysis rounds"`, `"analysis tendencies"`:
		// Repeating a filter flag ORs its values, exactly as repeating a query
		// parameter does on the HTTP API.
		return []string{"--buy", "--opponent-buy", "--site", "--t-pattern", "--ct-pattern", "--tag", "--slot"}
	default:
		return nil
	}
}

func duplicateFlag(args []string, repeatable ...string) string {
	seen := make(map[string]struct{})
	for _, arg := range args {
		if !strings.HasPrefix(arg, "--") {
			continue
		}
		name := arg
		if before, _, ok := strings.Cut(arg, "="); ok {
			name = before
		}
		if _, ok := seen[name]; ok && !containsString(repeatable, name) {
			return name
		}
		seen[name] = struct{}{}
	}
	return ""
}

func unknownFlag(args []string, valueFlags []string, boolFlags []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			continue
		}
		flag := arg
		hasEquals := false
		if before, _, ok := strings.Cut(arg, "="); ok {
			flag = before
			hasEquals = true
		}
		if !isLongFlag(flag) {
			return flag
		}
		if flagTakesValue(flag, valueFlags) {
			if !hasEquals && i+1 < len(args) && !isLongFlag(args[i+1]) {
				i++
			}
			continue
		}
		if flagIsBoolean(flag, boolFlags) {
			continue
		}
		return flag
	}
	return ""
}

func invalidBooleanFlagValue(args []string, boolFlags []string) (string, string) {
	for _, arg := range args {
		flag, value, ok := strings.Cut(arg, "=")
		if !ok || !flagIsBoolean(flag, boolFlags) {
			continue
		}
		if _, err := strconv.ParseBool(value); err != nil {
			return flag, value
		}
	}
	return "", ""
}

func booleanFlagSeparateValue(args []string, boolFlags []string) (string, string) {
	for i, arg := range args {
		if !flagIsBoolean(arg, boolFlags) || i+1 >= len(args) || isLongFlag(args[i+1]) {
			continue
		}
		if _, err := strconv.ParseBool(args[i+1]); err == nil {
			return arg, args[i+1]
		}
	}
	return "", ""
}

func booleanFlagIsTrue(args []string, name string) bool {
	for _, arg := range args {
		if arg == name {
			return true
		}
		if value, ok := strings.CutPrefix(arg, name+"="); ok {
			parsed, err := strconv.ParseBool(value)
			return err == nil && parsed
		}
	}
	return false
}

func hasFlagValue(args []string, name string) bool {
	for i, arg := range args {
		if strings.HasPrefix(arg, name+"=") {
			return strings.TrimSpace(strings.TrimPrefix(arg, name+"=")) != ""
		}
		if arg == name {
			return i+1 < len(args) && !isLongFlag(args[i+1]) && strings.TrimSpace(args[i+1]) != ""
		}
	}
	return false
}

func optionalValueFlagMissingValue(args []string, valueFlags []string, required []string) string {
	for i, arg := range args {
		if !strings.HasPrefix(arg, "--") {
			continue
		}
		flag := arg
		if before, value, ok := strings.Cut(arg, "="); ok {
			flag = before
			if flagTakesValue(flag, valueFlags) && !flagTakesValue(flag, required) && strings.TrimSpace(value) == "" {
				return flag
			}
			continue
		}
		if !flagTakesValue(flag, valueFlags) || flagTakesValue(flag, required) {
			continue
		}
		if i+1 >= len(args) || isLongFlag(args[i+1]) || strings.TrimSpace(args[i+1]) == "" {
			return flag
		}
	}
	return ""
}

func unexpectedPositionalArg(args []string, valueFlags []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			if strings.Contains(arg, "=") {
				continue
			}
			if flagTakesValue(arg, valueFlags) && i+1 < len(args) && !isLongFlag(args[i+1]) {
				i++
			}
			continue
		}
		return arg
	}
	return ""
}

func flagTakesValue(flag string, valueFlags []string) bool {
	for _, valueFlag := range valueFlags {
		if flag == valueFlag {
			return true
		}
	}
	return false
}

func isLongFlag(arg string) bool {
	return strings.HasPrefix(arg, "--")
}

func flagIsBoolean(flag string, boolFlags []string) bool {
	for _, boolFlag := range boolFlags {
		if flag == boolFlag {
			return true
		}
	}
	return false
}
