package workers

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/tasks"
	"github.com/rechedev9/cliphub/internal/worldoverlay"
)

const (
	// maxWorldTrackSeconds bounds the demo time sampled for one render, which keeps the track file small.
	maxWorldTrackSeconds = 600
	// worldTrackTailSeconds samples a little past each play's planned end, as the clip can run past it.
	worldTrackTailSeconds = 1
)

// shortDataInput is what a Short's data files need: the job, its edit, its capture and the stage folder.
type shortDataInput struct {
	job     job.Job
	edit    renderplan.EditRequest
	result  recording.RecordingResult
	workDir string
}

// shortDataArgs writes the stats card, the world track, the vertical HUD track and the radar track a
// Short draws and returns their editor flags.
func (w *RenderWorker) shortDataArgs(ctx context.Context, in shortDataInput) []string {
	var args []string
	if cardPath := w.writeShortStatsCard(in.job, in.workDir, in.edit); cardPath != "" {
		args = append(args, "--stats-card", cardPath)
	}
	if trackPath := w.writeShortWorldTrack(ctx, in); trackPath != "" {
		args = append(args, "--world-track", trackPath)
		if in.edit.PlayerLabels {
			args = append(args, "--player-labels")
		}
		if in.edit.GrenadeTrails {
			args = append(args, "--grenade-trails")
		}
	}
	if trackPath := w.writeShortVerticalHUD(ctx, in); trackPath != "" {
		args = append(args, "--vertical-hud-track", trackPath)
	}
	return append(args, w.writeShortRadar(ctx, in)...)
}

// writeShortWorldTrack samples the demo inside each captured play for the player labels and grenade
// trails. A missing demo or a failed read renders the Short without them instead of failing it.
func (w *RenderWorker) writeShortWorldTrack(ctx context.Context, in shortDataInput) string {
	edit, plan := in.edit, in.result.Plan
	if (!edit.PlayerLabels && !edit.GrenadeTrails) || edit.FullDemo != nil || edit.MatchRecap {
		return ""
	}
	fail := func(err error) string {
		logWorkerError(in.job.ID, tasks.TypeRenderVariant, fmt.Errorf("player labels and grenade trails left out: %w", err))
		return ""
	}
	windows, err := worldWindows(plan)
	if err != nil {
		return fail(err)
	}
	if strings.TrimSpace(in.job.DemoPath) == "" {
		return fail(fmt.Errorf("the job has no stored demo"))
	}
	if err := mediaassets.VerifyContent(ctx, w.storage, in.job.DemoPath, plan.DemoSHA256, 8<<30); err != nil {
		return fail(err)
	}
	demo, err := w.storage.Open(in.job.DemoPath)
	if err != nil {
		return fail(err)
	}
	track, err := parser.ExtractWorldTrack(ctx, demo, parser.WorldTrackOptions{DemoSHA256: plan.DemoSHA256, TickRate: plan.Tickrate, Windows: windows})
	if closeErr := demo.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fail(err)
	}
	path := filepath.Join(in.workDir, "world-track.json")
	if err := writeJSONFile(path, track); err != nil {
		return fail(err)
	}
	return path
}

// worldWindows are the captured clips of the plan, each seen from the player it films.
func worldWindows(plan recording.RecordingPlan) ([]worldoverlay.Window, error) {
	if plan.Tickrate <= 0 {
		return nil, fmt.Errorf("the capture has no tick rate")
	}
	var windows []worldoverlay.Window
	total := 0
	for _, s := range plan.Segments {
		pov := plan.TargetSteamID64
		if s.Observe != nil && s.Observe.SteamID64 != "" {
			pov = s.Observe.SteamID64
		}
		start := recording.EffectiveRecordStartTick(s, plan.Tickrate)
		end := recording.EffectiveRecordEndTick(s, plan) + worldTrackTailSeconds*plan.Tickrate
		if plan.DemoDurationTicks > 0 {
			end = min(end, plan.DemoDurationTicks)
		}
		if end <= start {
			continue
		}
		total += end - start
		windows = append(windows, worldoverlay.Window{ID: s.ID, POVSteamID64: pov, TickStart: start, TickEnd: end})
	}
	if len(windows) == 0 {
		return nil, fmt.Errorf("the capture has no plays")
	}
	if total > maxWorldTrackSeconds*plan.Tickrate {
		return nil, fmt.Errorf("the plays last more than %d s", maxWorldTrackSeconds)
	}
	return windows, nil
}
