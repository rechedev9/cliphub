package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

// This fixture tests storage publication only. Its byte strings are not media
// and its modeled attestation is never submitted to a production worker.
func fullDemoPublicationFixture(t *testing.T, content string, mutations ...func(*recapplan.Facts, *recapplan.Options)) (recording.RecordingResult, string, string) {
	t.Helper()
	f := recapplan.Facts{SchemaVersion: recapplan.DocumentVersion, DemoSHA256: strings.Repeat("a", 64), TargetSteamID64: "76561198000000001", ClockKind: recapplan.ClockIngame, TickRate: 64, EndTick: 2000, Complete: true,
		Rounds: []recapplan.RoundFacts{{ID: "round-001", Number: 1, StartTick: 200, FreezeEndTick: 400, RoundEndTick: 1000, Evidence: "round-events", Kills: []killplan.Kill{}, Utility: []killplan.UtilityThrow{}}}}
	o := recapplan.DefaultOptions()
	o.Audio.Music.Enabled, o.Audio.Voice.Enabled, o.Editorial.KeepFreezeVoice, o.Sponsor.Enabled = false, false, false, false
	o.Capture.Crosshair.AllowCaptureDefault = true
	for _, mutate := range mutations {
		mutate(&f, &o)
	}
	d, err := recapplan.Plan(f, o, recapplan.VoiceEvidence{Availability: "not_requested"}, nil, "facts.json")
	if err != nil {
		t.Fatal(err)
	}
	kp := killplan.NewPlan()
	kp.Demo = killplan.Demo{SHA256: f.DemoSHA256, Tickrate: 64, DurationTicks: 2000}
	kp.Target.SteamID64 = f.TargetSteamID64
	dir := t.TempDir()
	p, err := recording.NewPlanFromKillPlan(d.KillPlan(kp), "source.dem", dir, recording.DefaultStreamConfig(), &d)
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "recording.js")
	if err := os.MkdirAll(filepath.Join(dir, "segments"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("script-"+content), 0600); err != nil {
		t.Fatal(err)
	}
	evidence := &recording.FullDemoCaptureEvidence{SchemaVersion: "1.0", Restored: true, FilesRestored: true, CertifiedEnds: map[string]int{}}
	artifacts := []recording.RecordingArtifact{}
	for _, segment := range p.Segments {
		clip := filepath.Join(dir, "segments", segment.ID+".mp4")
		if err := os.WriteFile(clip, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		artifacts = append(artifacts, recording.RecordingArtifact{SegmentID: segment.ID, Role: "segment", Type: "video", Path: clip, SizeBytes: int64(len(content))})
		evidence.CertifiedEnds[segment.ID] = segment.TickEnd
	}
	for _, name := range []string{"voice_modenable", "snd_voipvolume", "tv_listen_voice_indices", "tv_listen_voice_indices_h", "spec_show_xray", "spec_autodirector", "cl_show_observer_crosshair"} {
		value := json.RawMessage("0")
		if name == "cl_show_observer_crosshair" {
			value = json.RawMessage("2")
		}
		evidence.Before = append(evidence.Before, recording.CvarValue{Name: name, Value: json.RawMessage("1")})
		evidence.Applied = append(evidence.Applied, recording.CvarValue{Name: name, Value: value})
	}
	for name, value := range map[string]float64{"cl_drawhud": 1, "cl_draw_only_deathnotices": 0, "crosshair": 1, "cl_demo_predict": 0, "cl_trueview_show_status": 0, "cl_spec_show_bindings": 0, "cl_drawhud_specvote": 0, "cl_teamid_overhead_mode": 0, "cl_drawhud_force_teamid_overhead": -1, "hud_showtargetid": 0} {
		encoded, _ := json.Marshal(value)
		evidence.Before = append(evidence.Before, recording.CvarValue{Name: name, Value: json.RawMessage("1")})
		evidence.Applied = append(evidence.Applied, recording.CvarValue{Name: name, Value: encoded})
	}
	fingerprint, err := recording.CaptureInputFingerprint(p)
	if err != nil {
		t.Fatal(err)
	}
	result := recording.RecordingResult{Plan: p, Script: script, CaptureMode: recording.CaptureModeReal, CaptureVerified: true, CaptureRevision: uuid.NewString(), CaptureInputFingerprint: fingerprint, FullDemoEvidence: evidence,
		Artifacts: artifacts}
	if err := result.DigestSegmentFiles(context.Background()); err != nil {
		t.Fatal(err)
	}
	return result, dir, filepath.Join(dir, "recording-result.json")
}

func TestFullDemoRecordingPublicationPreservesPriorRevisionOnFailure(t *testing.T) {
	for _, stage := range []string{"script", "clip", "revision", "pointer"} {
		t.Run(stage, func(t *testing.T) {
			store := newFakeStorage()
			id := uuid.New()
			old, out, path := fullDemoPublicationFixture(t, "old-clip")
			if _, err := uploadFullDemoRecordingOutputs(store, id, out, path, old, old, false); err != nil {
				t.Fatal(err)
			}
			old, err := decodeStoredRecordingResult(store, id)
			if err != nil {
				t.Fatal(err)
			}
			pointerBefore := bytes.Clone(store.files[recording.ResultArtifactKey(id)])
			oldClipKey, err := old.SegmentClipKey(id, "round-001")
			if err != nil {
				t.Fatal(err)
			}
			replacement, newOut, newPath := fullDemoPublicationFixture(t, "replacement-clip")
			failKey := ""
			switch stage {
			case "script":
				failKey, err = replacement.ScriptKey(id)
			case "clip":
				failKey, err = replacement.SegmentClipKey(id, "round-001")
			case "revision":
				failKey, err = replacement.RevisionResultKey(id)
			case "pointer":
				failKey = recording.ResultArtifactKey(id)
			}
			if err != nil {
				t.Fatal(err)
			}
			failed := &failPutAtStorage{fakeStorage: store, key: failKey, failAt: 1, err: errors.New("simulated disk full")}
			if _, err := uploadFullDemoRecordingOutputs(failed, id, newOut, newPath, replacement, replacement, true); err == nil {
				t.Fatal("publication failure swallowed")
			}
			if !bytes.Equal(pointerBefore, store.files[recording.ResultArtifactKey(id)]) || string(store.files[oldClipKey]) != "old-clip" {
				t.Fatal("failed replacement mutated the previous revision")
			}
			ready, err := recordingCommitReady(store, id, old)
			if err != nil || !ready {
				t.Fatalf("old recording cannot be reopened: %v", err)
			}
			localizations, err := recording.NewSegmentClipLocalizations(id, t.TempDir(), old)
			if err != nil || len(localizations) != 1 || localizations[0].Key != oldClipKey {
				t.Fatalf("localizer lost immutable revision: %v", err)
			}
		})
	}
}
