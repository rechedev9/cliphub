package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

// labArgsShape keeps what identifies an editor invocation: every flag, and
// the value of the flags that select the program rather than a file path.
func labArgsShape(args []string) []string {
	valued := []string{"--preset", "--output-format", "--kill-effect", "--transition"}
	// The encoder depends on the machine's GPU and the plates directory on
	// Studio's storage; neither is a Full Demo input.
	machine := []string{"--video-encoder", "--ffprobe", "--overlay-assets"}
	var shape []string
	for i := 0; i < len(args); i++ {
		if slices.Contains(machine, args[i]) {
			i++
			continue
		}
		if !strings.HasPrefix(args[i], "--") {
			continue
		}
		shape = append(shape, args[i])
		if slices.Contains(valued, args[i]) && i+1 < len(args) {
			shape = append(shape, args[i+1])
		}
	}
	return shape
}

// The synthetic lab bundle writes the Full Demo editor arguments itself, so it
// cannot drift silently from what a Full Demo render passes the editor.
func TestSyntheticLabBundleArgumentsMatchAFullDemoRender(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is required to generate the synthetic captures")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, _, _ := fullDemoPublicationFixture(t, "clip")
	store := newFakeStorage()
	id := uuid.New()
	for _, artifact := range result.Artifacts {
		key, err := result.SegmentClipKey(id, artifact.SegmentID)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Put(key, bytes.NewReader([]byte("clip"))); err != nil {
			t.Fatal(err)
		}
	}
	roster, err := json.Marshal(parser.RosterResult{Players: []parser.PlayerStat{{SteamID64: result.Plan.TargetSteamID64, Name: "target", Team: "CT"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(artifacts.RosterKey(id), bytes.NewReader(roster)); err != nil {
		t.Fatal(err)
	}
	doc := *result.Plan.FullDemo
	if !doc.Options.Overlays.Roster && !doc.Options.Overlays.Scoreboard {
		t.Fatal("fixture plan has no overlays; the overlay argument would go unchecked")
	}
	snapshot := recapplan.Snapshot{Document: doc, Approval: recapplan.Approval{PlanHash: doc.PlanHash, AllowSafeTailTrim: doc.Options.Editorial.AllowSafeTailTrim, Timestamp: time.Now().UTC()}}
	killPlan := doc.KillPlan(killplan.NewPlan())
	loadout, err := renderplan.LoadoutForVariant(editor.PresetGameplayPOV60)
	if err != nil {
		t.Fatal(err)
	}
	w := NewRenderWorker(newFakeRepo(), store, RenderWorkerConfig{FFmpegPath: ffmpeg})
	render, err := w.writeEditorInputs(ctx, w.cfg, editorInputs{
		job:             job.Job{ID: id, TargetSteamID: doc.Input.TargetSteamID64, KillPlan: &killPlan},
		loadout:         loadout,
		edit:            renderplan.NormalizeEditRequest(renderplan.FullDemoEditRequest(snapshot)),
		recordingResult: result,
	}, t.TempDir())
	if err != nil {
		t.Fatalf("Full Demo editor inputs: %v", err)
	}

	synth, err := editor.SynthLabBundle(ctx, editor.LabSynthOptions{Rounds: 1, Dir: filepath.Join(t.TempDir(), "synth"), FFmpeg: ffmpeg, Width: 64, Height: 36})
	if err != nil {
		t.Fatalf("synthetic bundle: %v", err)
	}
	if got, want := labArgsShape(synth.Args), labArgsShape(render.args); !slices.Equal(got, want) {
		t.Fatalf("synthetic bundle editor arguments differ from a Full Demo render's\n got %q\nwant %q", got, want)
	}
}
