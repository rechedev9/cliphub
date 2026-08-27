package workers

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/tasks"
)

func TestRenderWorkerFullDemoPOVExtractsTeamComms(t *testing.T) {
	const steamID = "76561197960265729"
	voice := 0.85
	tests := []struct {
		name        string
		tracks      int
		wantDir     bool
		wantVolume  string
		wantExtract bool
	}{
		{
			name:        "native POV with team tracks passes voice-dir at 85%",
			tracks:      2,
			wantDir:     true,
			wantVolume:  "0.85",
			wantExtract: true,
		},
		{
			name:        "native POV with no tracks still extracts and skips --voice-dir",
			wantExtract: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			store := newFakeStorage()
			id := uuid.New()
			plan := minimalKillPlan()
			repo.jobs[id] = &job.Job{
				ID:            id,
				Status:        job.StatusRecorded,
				DemoPath:      "demos/test.dem",
				TargetSteamID: steamID,
				Rules:         rules.Default(),
				KillPlan:      &plan,
			}
			putJSON(t, store, recording.ResultArtifactKey(id), recordingResultWithSegment("", "C:/stale/seg-001.mp4"))
			_ = store.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("clip")))
			if err := store.Put("demos/test.dem", bytes.NewReader([]byte("demo"))); err != nil {
				t.Fatal(err)
			}

			var extracted bool
			var gotTarget string
			var gotArgs []string
			stop := errors.New("stop after args")
			w := NewRenderWorker(repo, store, RenderWorkerConfig{
				WorkDir:    t.TempDir(),
				EditorPath: "zv-editor",
			})
			w.voiceExtract = func(_, target, dir string) (int, error) {
				extracted = true
				gotTarget = target
				if tc.tracks > 0 {
					if err := os.MkdirAll(dir, 0o700); err != nil {
						return 0, err
					}
					if err := os.WriteFile(filepath.Join(dir, "track.ogg"), []byte("ogg"), 0o600); err != nil {
						return 0, err
					}
				}
				return tc.tracks, nil
			}
			w.runner = &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
				gotArgs = append([]string(nil), args...)
				return nil, stop
			}}

			edit := renderplan.DefaultEditRequest()
			edit.VoiceComms = true
			edit.VoiceVolume = &voice
			edit.MatchRecap = true
			edit.NativeHUD = true
			edit.Format = renderplan.FormatLandscape16x9
			edit.KillEffect = renderplan.KillEffectClean
			edit.Intro = false
			edit.Outro = false
			task, err := tasks.NewRenderVariantTask(id, editor.PresetGameplayPOV60, "", 0, nil, edit)
			if err != nil {
				t.Fatal(err)
			}
			if err := w.HandleRenderVariant(context.Background(), task); !errors.Is(err, stop) {
				t.Fatalf("HandleRenderVariant error = %v, want stop sentinel", err)
			}
			if extracted != tc.wantExtract {
				t.Fatalf("extract called = %v, want %v", extracted, tc.wantExtract)
			}
			if gotTarget != steamID {
				t.Fatalf("extract target = %q, want POV %q", gotTarget, steamID)
			}
			if got := hasArg(gotArgs, "--voice-dir"); got != tc.wantDir {
				t.Fatalf("--voice-dir present = %v, want %v: %#v", got, tc.wantDir, gotArgs)
			}
			if tc.wantDir {
				if got := argValue(gotArgs, "--voice-volume"); got != tc.wantVolume {
					t.Fatalf("--voice-volume = %q, want %s", got, tc.wantVolume)
				}
			}
			if hasArg(gotArgs, "--music") {
				t.Fatalf("full demo passed --music: %#v", gotArgs)
			}
		})
	}
}
