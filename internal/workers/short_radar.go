package workers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mapdata "github.com/rechedev9/cliphub/data/maps"
	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/radaroverlay"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/tasks"
)

// writeShortRadar reads the radar of each captured play from the demo and writes it with the map art,
// returning the editor flags. A missing demo, an uncalibrated map or a failed read renders the Short
// without the radar instead of failing it.
func (w *RenderWorker) writeShortRadar(ctx context.Context, in shortDataInput) []string {
	edit, plan := in.edit, in.result.Plan
	if !edit.Radar || edit.FullDemo != nil || edit.MatchRecap {
		return nil
	}
	fail := func(err error) []string {
		logWorkerError(in.job.ID, tasks.TypeRenderVariant, fmt.Errorf("radar left out: %w", err))
		return nil
	}
	windows, err := radarWindows(plan)
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
	track, err := parser.ExtractRadarTrack(ctx, demo, parser.RadarTrackOptions{DemoSHA256: plan.DemoSHA256, TickRate: plan.Tickrate, Map: plan.DemoMap, Windows: windows})
	if closeErr := demo.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fail(err)
	}
	trackPath := filepath.Join(in.workDir, "radar-track.json")
	if err := writeJSONFile(trackPath, track); err != nil {
		return fail(err)
	}
	args := []string{"--radar-track", trackPath}
	for _, art := range []struct{ layer, flag string }{{"upper", "--radar-image"}, {"lower", "--radar-image-lower"}} {
		raw := mapdata.Radar(plan.DemoMap, art.layer)
		if len(raw) == 0 || (art.layer == "lower" && !track.Calibration.MultiLevel()) {
			continue
		}
		path := filepath.Join(in.workDir, "radar-"+art.layer+".png")
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			return fail(err)
		}
		args = append(args, art.flag, path)
	}
	return args
}

// radarWindows are the captured gameplay clips of the plan, each seen from the player it films;
// a victim replay shows another view and gets no radar.
func radarWindows(plan recording.RecordingPlan) ([]radaroverlay.Window, error) {
	if plan.Tickrate <= 0 {
		return nil, fmt.Errorf("the capture has no tick rate")
	}
	var windows []radaroverlay.Window
	total := 0
	for _, s := range plan.Segments {
		if strings.HasSuffix(s.ID, recording.VictimReplaySuffix) {
			continue
		}
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
		windows = append(windows, radaroverlay.Window{ID: s.ID, POVSteamID64: pov, TickStart: start, TickEnd: end})
	}
	if len(windows) == 0 {
		return nil, fmt.Errorf("the capture has no plays")
	}
	if total > maxWorldTrackSeconds*plan.Tickrate {
		return nil, fmt.Errorf("the plays last more than %d s", maxWorldTrackSeconds)
	}
	return windows, nil
}
