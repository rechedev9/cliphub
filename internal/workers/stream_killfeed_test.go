package workers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/streamclips"
)

type maskFailingRunner struct{ graphs []string }

func (r *maskFailingRunner) run(_ context.Context, args []string) error {
	graph := argValue(args, "-filter_complex")
	r.graphs = append(r.graphs, graph)
	if strings.Contains(graph, "delogo") {
		return errors.New("exit status 127")
	}
	return nil
}

func TestStreamClipRendersUnmaskedWhenTheMaskBreaksFFmpeg(t *testing.T) {
	plan := streamclips.DefaultEditPlan()
	clip := streamclips.ClipRange{ID: "c1", StartSeconds: 10, EndSeconds: 20}
	plan.Clips = []streamclips.ClipRange{clip}
	runner := &maskFailingRunner{}
	warning, err := renderStreamClip(context.Background(), streamClipRender{run: runner.run, plan: plan, clip: clip,
		in: streamclips.FFmpegInputs{
			SourcePath: "in.mp4", OutputPath: "out.mp4", SourceWidth: 1920, SourceHeight: 1080,
			KillFeed: []streamclips.KillFeedNotice{{Path: "a.png", Width: 300, Height: 41, DisappearSeconds: 5,
				Masks: []streamclips.KillFeedMask{{Area: streamclips.CropRect{X: 0.75, Y: 0.125, Width: 0.125, Height: 0.0625}, To: 5}}}},
		}})
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if len(runner.graphs) != 2 || strings.Contains(runner.graphs[1], "delogo") || !strings.Contains(runner.graphs[1], "[kf0]") {
		t.Fatalf("want a masked try, then one with the redrawn feed and no mask: %q", runner.graphs)
	}
	if !strings.Contains(warning, "original kill feed not hidden") {
		t.Fatalf("warning = %q", warning)
	}
}
