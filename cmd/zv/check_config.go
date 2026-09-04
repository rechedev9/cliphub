package main

// claudeRequiredAllowPermissions is the harness contract for
// .claude/settings.json: the operator wants zero permission prompts, so the
// blanket tool allows must be present and no ask/deny lists are required.
func claudeRequiredAllowPermissions() []string {
	return []string{
		"Bash(*)",
		"Read(*)",
		"Edit(*)",
		"Write(*)",
	}
}

func skillWorkflowRequirementMap() map[string][]string {
	return map[string][]string{
		"zackvideo-cheater-pov-reels":      {"demo-players", "record", "shorts-render"},
		"zackvideo-cs2-utility-shorts":     {"demo-parse", "utility-audit", "record", "shorts-render", "gallery-open"},
		"zackvideo-lineup-audit":           {"utility-audit"},
		"zackvideo-music-scripted-shorts":  {"demo-parse", "demo-players", "record", "music-analyze", "shorts-render", "gallery-open"},
		"zackvideo-shorts-production":      {"demo-parse", "demo-players", "demo-moments", "demo-select", "utility-audit", "record", "shorts-render", "gallery-open"},
		"zackvideo-stream-clips":           {"stream-fetch", "stream-variants", "stream-plan", "stream-render"},
		"zackvideo-youtube-shorts-publish": {"gallery-open"},
	}
}

func groupUsageTexts() map[string]string {
	return map[string]string{
		"faceit":    faceitUsage,
		"demo":      demoUsage,
		"utility":   utilityUsage,
		"compose":   composeUsage,
		"shorts":    shortsUsage,
		"music":     musicUsage,
		"analysis":  analysisUsage,
		"gallery":   galleryUsage,
		"check":     checkUsage,
		"skills":    skillsUsage,
		"workflows": workflowsUsage,
	}
}

type legacyPassThrough struct {
	Command string
	Binary  string
}

func legacyPassThroughs() []legacyPassThrough {
	return []legacyPassThrough{
		{Command: "parser", Binary: "zv-parser"},
		{Command: "editor", Binary: "zv-editor"},
		{Command: "recorder", Binary: "zv-recorder"},
		{Command: "composer", Binary: "zv-composer"},
		{Command: "orchestrator", Binary: "zv-orchestrator"},
		{Command: "analysis-viewer", Binary: "zv-analysis-viewer"},
		{Command: "tactical-data", Binary: "zv-tactical-data"},
		{Command: "rhythm", Binary: "zv-rhythm"},
		{Command: "tui", Binary: "zv-tui"},
	}
}

func defaultLegacyCommandEntrypointNames() []string {
	return []string{
		"zv-parser",
		"zv-analysis-viewer",
		"zv-demo-players",
		"zv-recorder",
		"zv-editor",
		"zv-stream",
		"zv-composer",
		"zv-orchestrator",
		"zv-tactical-data",
		"zv-rhythm",
		"zv-tui",
	}
}
