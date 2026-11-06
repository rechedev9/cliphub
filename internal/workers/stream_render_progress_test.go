package workers

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/streamclips"
)

// progressScriptRunner writes FFmpeg -progress records and remembers the
// board's percent after each one.
type progressScriptRunner struct {
	board   *streamclips.RenderProgressBoard
	jobID   uuid.UUID
	outUs   []int64
	args    []string
	percent []float64
}

func (r *progressScriptRunner) Run(context.Context, string, ...string) ([]byte, error) {
	return nil, fmt.Errorf("Run called; want RunProgress")
}

func (r *progressScriptRunner) RunProgress(_ context.Context, _ string, progress io.Writer, args ...string) ([]byte, error) {
	r.args = args
	for _, us := range r.outUs {
		if _, err := fmt.Fprintf(progress, "frame=1\nout_time_us=%d\nprogress=continue\n", us); err != nil {
			return nil, err
		}
		p, _ := r.board.Get(r.jobID)
		r.percent = append(r.percent, p.Percent)
	}
	return nil, nil
}

func TestRenderStreamClipReportsProgressAcrossClips(t *testing.T) {
	board := streamclips.NewRenderProgressBoard()
	jobID := uuid.New()
	runner := &progressScriptRunner{board: board, jobID: jobID, outUs: []int64{2_500_000, 5_000_000}}
	w := &StreamRenderWorker{runner: runner}
	cfg := StreamRenderWorkerConfig{FFmpegPath: "ffmpeg", Progress: board}
	plan := streamclips.EditPlan{
		Variant: streamclips.VariantStreamerFullframeNoCam,
		Clips: []streamclips.ClipRange{
			{ID: "a", StartSeconds: 0, EndSeconds: 10},
			{ID: "b", StartSeconds: 20, EndSeconds: 30},
		},
	}
	err := w.runClipWithProgress(context.Background(), cfg, streamClipProgress{jobID: jobID, plan: plan, index: 1, args: []string{"-i", "in.mp4", "out.mp4"}})
	if err != nil {
		t.Fatal(err)
	}
	// Clip 2 of 2: 50 % for the first clip plus a quarter, then half, of the second.
	if len(runner.percent) != 2 || runner.percent[0] != 62.5 || runner.percent[1] != 75 {
		t.Fatalf("percent = %v, want [62.5 75]", runner.percent)
	}
	if got, _ := board.Get(jobID); got.Clip != 2 || got.Clips != 2 {
		t.Fatalf("progress = %+v, want clip 2 of 2", got)
	}
	if len(runner.args) < 3 || runner.args[0] != "-progress" || runner.args[1] != "pipe:1" {
		t.Fatalf("args = %v, want -progress pipe:1 first", runner.args)
	}
}
