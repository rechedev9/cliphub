package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

func TestFinalizeFullDemoCaptureEvidencePersistsFailures(t *testing.T) {
	for _, scenario := range []string{"missing console log", "invalid evidence", "settings restoration", "success"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			result := recording.RecordingResult{
				Plan: recording.RecordingPlan{
					OutputDir: dir,
					FullDemo:  &recapplan.Document{Options: recapplan.DefaultOptions()},
					Segments:  []recording.RecordingSegment{{ID: "round-001", TickStart: 10, TickEnd: 20, LiveEndTick: 18}},
				},
				CaptureVerified: true,
				CaptureMode:     recording.CaptureModeReal,
			}
			// A stale result must not survive a capture evidence failure.
			if err := writeResult(dir, recording.RecordingResult{Error: "stale attempt"}); err != nil {
				t.Fatal(err)
			}
			logPath := filepath.Join(dir, "console.log")
			if scenario != "missing console log" {
				content := fullDemoEvidenceLog(t)
				if scenario == "invalid evidence" {
					content = "ZV_FULL_DEMO:test:{invalid-json}\n"
				}
				if err := os.WriteFile(logPath, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			restored := false
			restoreFailure := errors.New("settings file is locked")
			err := finalizeFullDemoCaptureEvidence(&result, logPath, "test", func() error {
				if scenario == "settings restoration" {
					return restoreFailure
				}
				restored = true
				return nil
			})
			if scenario == "success" {
				if err != nil || !restored || result.FullDemoEvidence == nil || !result.FullDemoEvidence.FilesRestored || result.Error != "" {
					t.Fatalf("successful evidence finalization: %+v; %v", result, err)
				}
				return
			}
			if err == nil || restored {
				t.Fatalf("expected failed finalization: restored=%v error=%v", restored, err)
			}
			if scenario == "settings restoration" && !errors.Is(err, restoreFailure) {
				t.Fatalf("restoration failure was lost: %v", err)
			}
			data, readErr := os.ReadFile(filepath.Join(dir, "recording-result.json"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			var persisted recording.RecordingResult
			if decodeErr := json.Unmarshal(data, &persisted); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if persisted.Error != err.Error() || !persisted.CaptureVerified || persisted.CaptureMode != recording.CaptureModeReal || len(persisted.Plan.Segments) != 1 {
				t.Fatalf("failure result missing or stale: %+v; error: %v", persisted, err)
			}
			if persisted.FullDemoEvidence != nil && persisted.FullDemoEvidence.FilesRestored {
				t.Fatal("failed restoration was certified")
			}
		})
	}
}

func fullDemoEvidenceLog(t *testing.T) string {
	t.Helper()
	values := []recording.CvarValue{}
	for _, name := range []string{"voice_modenable", "snd_voipvolume", "tv_listen_voice_indices", "tv_listen_voice_indices_h", "spec_show_xray", "spec_autodirector", "cl_draw_only_deathnotices", "cl_demo_predict", "cl_trueview_show_status", "cl_spec_show_bindings", "cl_drawhud_specvote", "cl_teamid_overhead_mode", "hud_showtargetid"} {
		values = append(values, recording.CvarValue{Name: name, Value: json.RawMessage("0")})
	}
	values = append(values,
		recording.CvarValue{Name: "cl_drawhud_force_teamid_overhead", Value: json.RawMessage("-1")},
		recording.CvarValue{Name: "cl_show_observer_crosshair", Value: json.RawMessage("2")},
		recording.CvarValue{Name: "cl_drawhud", Value: json.RawMessage("1")},
		recording.CvarValue{Name: "crosshair", Value: json.RawMessage("1")},
	)
	var log strings.Builder
	for _, event := range []any{
		map[string]any{"kind": "settings_before", "values": values},
		map[string]any{"kind": "settings_applied", "values": values},
		map[string]any{"kind": "settings_restored", "success": true},
		map[string]any{"kind": "certified_end", "round_id": "round-001", "end_tick": 20},
	} {
		data, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		log.WriteString("ZV_FULL_DEMO:test:" + string(data) + "\n")
	}
	return log.String()
}
