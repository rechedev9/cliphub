package main

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rechedev9/tickcut/internal/pathguard"
)

func TestValidateCompositionOutputsRejectsRecordingResultAlias(t *testing.T) {
	dir := t.TempDir()
	resultPath := filepath.Join(dir, "composition-result.json")
	err := validateCompositionOutputs(
		filepath.Join(dir, "final.mp4"),
		resultPath,
		pathguard.Input{Flag: "--recording-result", Path: resultPath},
	)
	if err == nil {
		t.Fatal("validateCompositionOutputs error = nil, want recording result alias rejection")
	}
}

func TestValidateCompositionOutputsRejectsCompositionResultAlias(t *testing.T) {
	dir := t.TempDir()
	resultPath := filepath.Join(dir, "composition-result.json")
	outputs := []struct {
		name string
		path string
	}{
		{name: "exact", path: resultPath},
		{name: "cleaned", path: filepath.Join(dir, ".", "composition-result.json")},
	}
	if runtime.GOOS == "windows" {
		outputs = append(outputs, struct {
			name string
			path string
		}{name: "windows case alias", path: strings.ToUpper(resultPath)})
	}

	for _, output := range outputs {
		t.Run(output.name, func(t *testing.T) {
			err := validateCompositionOutputs(output.path, resultPath)
			if err == nil || !strings.Contains(err.Error(), "must not overwrite composition result") {
				t.Fatalf("validateCompositionOutputs error = %v, want composition result alias rejection", err)
			}
		})
	}
}
