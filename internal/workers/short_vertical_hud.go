package workers

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/tasks"
)

// writeShortVerticalHUD reads the scoreboard of each captured play from the job's HUD telemetry for
// the 9:16 HUD. A missing demo or a failed read renders the Short without it instead of failing it.
func (w *RenderWorker) writeShortVerticalHUD(ctx context.Context, in shortDataInput) string {
	edit, plan := in.edit, in.result.Plan
	if !edit.VerticalHUD || edit.FullDemo != nil || edit.MatchRecap {
		return ""
	}
	fail := func(err error) string {
		logWorkerError(in.job.ID, tasks.TypeRenderVariant, fmt.Errorf("vertical HUD left out: %w", err))
		return ""
	}
	windows, err := verticalHUDWindows(plan)
	if err != nil {
		return fail(err)
	}
	if strings.TrimSpace(in.job.DemoPath) == "" {
		return fail(fmt.Errorf("the job has no stored demo"))
	}
	timeline, _, err := w.hudTimeline(ctx, in.job, hudSource{demoSHA: plan.DemoSHA256, target: plan.TargetSteamID64, tickRate: plan.Tickrate})
	if err != nil {
		return fail(err)
	}
	track, err := timeline.VerticalTrack(windows)
	if err != nil {
		return fail(err)
	}
	path := filepath.Join(in.workDir, "vertical-hud.json")
	if err := writeJSONFile(path, track); err != nil {
		return fail(err)
	}
	return path
}

// verticalHUDWindows are the captured gameplay clips of the plan; a victim replay shows another view.
func verticalHUDWindows(plan recording.RecordingPlan) ([]customhud.VerticalWindow, error) {
	if plan.Tickrate <= 0 {
		return nil, fmt.Errorf("the capture has no tick rate")
	}
	var windows []customhud.VerticalWindow
	for _, s := range plan.Segments {
		if strings.HasSuffix(s.ID, recording.VictimReplaySuffix) {
			continue
		}
		start := recording.EffectiveRecordStartTick(s, plan.Tickrate)
		end := recording.EffectiveRecordEndTick(s, plan) + worldTrackTailSeconds*plan.Tickrate
		if plan.DemoDurationTicks > 0 {
			end = min(end, plan.DemoDurationTicks)
		}
		if end > start {
			windows = append(windows, customhud.VerticalWindow{ID: s.ID, TickStart: start, TickEnd: end})
		}
	}
	if len(windows) == 0 {
		return nil, fmt.Errorf("the capture has no plays")
	}
	return windows, nil
}
