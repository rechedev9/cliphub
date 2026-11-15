package workers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/storage"
)

func (w *RenderWorker) materializeFullDemoHUD(ctx context.Context, j job.Job, d recapplan.Document, dir string) (*editor.FullDemoLocalHUD, error) {
	_, body, err := w.hudTimeline(ctx, j, hudSource{demoSHA: d.Input.DemoSHA256, target: d.Input.TargetSteamID64, tickRate: d.Clock.TickRate})
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(body)
	path := filepath.Join(dir, "full-demo-hud-telemetry.json")
	if err := os.WriteFile(path, body, 0600); err != nil {
		return nil, err
	}
	return &editor.FullDemoLocalHUD{Path: path, SHA256: hex.EncodeToString(digest[:])}, nil
}

// hudSource names the demo, the observed player and the clock of a job's HUD telemetry.
type hudSource struct {
	demoSHA, target string
	tickRate        int
}

// hudTimeline reads the job's cached HUD telemetry, or extracts it from the verified demo and caches it.
// Full Demo and the 9:16 Short HUD share the cache. It returns the timeline and its JSON body.
func (w *RenderWorker) hudTimeline(ctx context.Context, j job.Job, src hudSource) (customhud.Timeline, []byte, error) {
	if err := mediaassets.VerifyContent(ctx, w.storage, j.DemoPath, src.demoSHA, 8<<30); err != nil {
		return customhud.Timeline{}, nil, err
	}
	key := fmt.Sprintf("jobs/%s/full-demo/hud/%s/%s-%s.json", j.ID, customhud.TelemetryVersion, src.demoSHA, src.target)
	var timeline customhud.Timeline
	valid := false
	reader, err := w.storage.Open(key)
	if err == nil {
		timeline, err = customhud.Decode(reader)
		closeErr := reader.Close()
		if closeErr != nil {
			return customhud.Timeline{}, nil, closeErr
		}
		valid = err == nil && timeline.DemoSHA256 == src.demoSHA && timeline.TargetSteamID == src.target && timeline.TickRate == src.tickRate
	} else if !storage.IsNotExist(err) {
		return customhud.Timeline{}, nil, err
	}
	if !valid {
		input, err := w.storage.Open(j.DemoPath)
		if err != nil {
			return customhud.Timeline{}, nil, err
		}
		timeline, err = customhud.Extract(ctx, input, src.demoSHA, src.target, src.tickRate)
		closeErr := input.Close()
		if err != nil {
			return customhud.Timeline{}, nil, err
		}
		if closeErr != nil {
			return customhud.Timeline{}, nil, closeErr
		}
	}
	body, err := json.Marshal(timeline)
	if err != nil {
		return customhud.Timeline{}, nil, err
	}
	if len(body) > customhud.MaxTelemetryBytes {
		return customhud.Timeline{}, nil, fmt.Errorf("custom HUD telemetry exceeds byte limit")
	}
	if !valid {
		if err := w.storage.Put(key, bytes.NewReader(body)); err != nil {
			return customhud.Timeline{}, nil, err
		}
	}
	return timeline, body, nil
}
