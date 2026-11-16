package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/mediafont"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/worldoverlay"
)

// worldOverlayTailSeconds covers a clip that runs a little past its planned end tick.
const worldOverlayTailSeconds = 1.0

// WorldOverlay is the player labels and grenade trails script of each captured segment clip.
type WorldOverlay struct {
	FontsDir string            `json:"fonts_dir"`
	Scripts  map[string]string `json:"scripts"`
}

type worldOverlayInput struct {
	cfg    Config
	plan   recording.RecordingPlan
	outDir string
}

// prepareWorldOverlay writes one ASS script per segment clip from the worker's demo track. The script
// is drawn on the clip before any scale or crop, so zooms, crops and transitions carry it like the game.
func prepareWorldOverlay(in worldOverlayInput) (*WorldOverlay, []string, error) {
	cfg, plan := in.cfg, in.plan
	if !cfg.PlayerLabels && !cfg.GrenadeTrails {
		return nil, nil, nil
	}
	if strings.TrimSpace(cfg.WorldTrackPath) == "" {
		return nil, []string{"player labels and grenade trails left out: no world track"}, nil
	}
	track, err := worldoverlay.Load(cfg.WorldTrackPath)
	if err != nil {
		return nil, nil, err
	}
	if track.TickRate != plan.Tickrate || track.DemoSHA256 != plan.DemoSHA256 {
		return nil, nil, fmt.Errorf("world track is for another demo or tick rate")
	}
	bySegment := map[string]worldoverlay.Segment{}
	for _, s := range track.Segments {
		bySegment[s.ID] = s
	}
	out := &WorldOverlay{Scripts: map[string]string{}}
	dir := filepath.Join(in.outDir, "world-overlay")
	for i, segment := range plan.Segments {
		seg, ok := bySegment[segment.ID]
		if !ok {
			continue
		}
		start := recording.EffectiveRecordStartTick(segment, plan.Tickrate)
		end := recording.EffectiveRecordEndTick(segment, plan)
		script, drawn, err := worldoverlay.Script(seg, worldoverlay.Layout{
			Width: plan.Stream.Width, Height: plan.Stream.Height, FPS: plan.Stream.FPS, TickRate: plan.Tickrate,
			StartTick: start, DurationSeconds: float64(end-start)/float64(plan.Tickrate) + worldOverlayTailSeconds,
			Labels: cfg.PlayerLabels, Trails: cfg.GrenadeTrails,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("world overlay for %s: %w", segment.ID, err)
		}
		if !drawn {
			continue
		}
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, nil, err
		}
		path := filepath.Join(dir, fmt.Sprintf("segment-%03d.ass", i+1))
		if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
			return nil, nil, err
		}
		out.Scripts[segment.ID] = path
	}
	if len(out.Scripts) == 0 {
		return nil, []string{"player labels and grenade trails: nothing to draw in these plays"}, nil
	}
	if out.FontsDir, err = mediafont.MaterializeHUD(); err != nil {
		return nil, nil, fmt.Errorf("world overlay fonts: %w", err)
	}
	return out, nil, nil
}

// worldOverlayFilter is the ASS filter, with its trailing comma, that draws a segment's labels and
// trails on its clip; empty without a script or for Full Demo.
func worldOverlayFilter(short ShortEdit, segmentID string) string {
	w := short.Overlays.World
	if w == nil || short.FullDemo != nil {
		return ""
	}
	path, ok := w.Scripts[segmentID]
	if !ok {
		return ""
	}
	return customhud.ASSFilter(path, w.FontsDir, false) + ","
}

// sourceVideoFilter is VideoFilter for a single-clip Short, with the world overlay drawn first.
func sourceVideoFilter(short ShortEdit) string {
	return worldOverlayFilter(short, short.SegmentID) + VideoFilter(short)
}
