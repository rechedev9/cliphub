package workers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	mapdata "github.com/rechedev9/cliphub/data/maps"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/radaroverlay"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

type fullDemoRadarInput struct {
	job      job.Job
	document recapplan.Document
	dir      string
}

func (w *RenderWorker) materializeFullDemoRadar(ctx context.Context, in fullDemoRadarInput) (*editor.FullDemoLocalRadar, error) {
	j, d := in.job, in.document
	if j.KillPlan == nil {
		return nil, fmt.Errorf("ClipHub radar needs the parsed demo's map")
	}
	mapName := j.KillPlan.Demo.Map
	windows := fullDemoRadarWindows(d, j.KillPlan.Demo.DurationTicks)
	if len(windows) == 0 {
		return nil, fmt.Errorf("ClipHub radar has no rounds to draw")
	}
	if err := mediaassets.VerifyContent(ctx, w.storage, j.DemoPath, d.Input.DemoSHA256, 8<<30); err != nil {
		return nil, err
	}
	demo, err := w.storage.Open(j.DemoPath)
	if err != nil {
		return nil, err
	}
	track, err := parser.ExtractRadarTrack(ctx, demo, parser.RadarTrackOptions{DemoSHA256: d.Input.DemoSHA256, TickRate: d.Clock.TickRate, Map: mapName, Windows: windows})
	if closeErr := demo.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	out := &editor.FullDemoLocalRadar{Rounds: map[string]editor.FullDemoLocalHUD{}}
	for _, segment := range track.Segments {
		one := track
		one.Segments = []radaroverlay.Segment{segment}
		body, err := json.Marshal(one)
		if err != nil {
			return nil, err
		}
		file, err := writeFullDemoLocal(filepath.Join(in.dir, "radar-"+segment.ID+".json"), body)
		if err != nil {
			return nil, err
		}
		out.Rounds[segment.ID] = file
	}
	upper := mapdata.Radar(mapName, "upper")
	if len(upper) == 0 {
		return nil, fmt.Errorf("ClipHub radar has no map art for %s", mapName)
	}
	if out.Upper, err = writeFullDemoLocal(filepath.Join(in.dir, "radar-upper.png"), upper); err != nil {
		return nil, err
	}
	if lower := mapdata.Radar(mapName, "lower"); len(lower) > 0 && track.Calibration.MultiLevel() {
		file, err := writeFullDemoLocal(filepath.Join(in.dir, "radar-lower.png"), lower)
		if err != nil {
			return nil, err
		}
		out.Lower = &file
	}
	return out, nil
}

func writeFullDemoLocal(path string, body []byte) (editor.FullDemoLocalHUD, error) {
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return editor.FullDemoLocalHUD{}, err
	}
	digest := sha256.Sum256(body)
	return editor.FullDemoLocalHUD{Path: path, SHA256: hex.EncodeToString(digest[:])}, nil
}

func fullDemoRadarWindows(d recapplan.Document, demoTicks int) []radaroverlay.Window {
	inTimeline := map[string]bool{}
	for _, item := range d.Timeline {
		if item.Role == "round" {
			inTimeline[item.SourceRef] = true
		}
	}
	var windows []radaroverlay.Window
	for _, r := range d.Rounds {
		if !inTimeline[r.ID] {
			continue
		}
		start := min(r.CaptureStartTick, r.RequestedStartTick)
		end := max(r.CaptureEndTick, r.EffectiveEndTick, r.RequestedEndTick) + worldTrackTailSeconds*d.Clock.TickRate
		if demoTicks > 0 {
			end = min(end, demoTicks)
		}
		if end > start {
			windows = append(windows, radaroverlay.Window{ID: r.ID, POVSteamID64: d.Input.TargetSteamID64, TickStart: start, TickEnd: end})
		}
	}
	return windows
}
