package main

import "github.com/rechedev9/cliphub/internal/editor"

func supportedPresetNames() []string {
	return []string{editor.PresetViral60Clean, editor.PresetViralAggressive60}
}

func supportedPresetByName(name string) (editor.RenderPreset, bool) {
	for _, supported := range supportedPresetNames() {
		if name != supported {
			continue
		}
		return editor.PresetByName(name)
	}
	return editor.RenderPreset{}, false
}
