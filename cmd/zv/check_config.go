package main

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
		{Command: "hud-designs", Binary: "zv-hud-designs"},
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
		"zv-hud-designs",
		"zv-tui",
	}
}
