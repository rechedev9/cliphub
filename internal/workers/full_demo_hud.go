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
	if err := mediaassets.VerifyContent(ctx, w.storage, j.DemoPath, d.Input.DemoSHA256, 8<<30); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("jobs/%s/full-demo/hud/%s/%s-%s.json", j.ID, customhud.TelemetryVersion, d.Input.DemoSHA256, d.Input.TargetSteamID64)
	var timeline customhud.Timeline
	valid := false
	reader, err := w.storage.Open(key)
	if err == nil {
		timeline, err = customhud.Decode(reader)
		closeErr := reader.Close()
		if closeErr != nil {
			return nil, closeErr
		}
		valid = err == nil && timeline.DemoSHA256 == d.Input.DemoSHA256 && timeline.TargetSteamID == d.Input.TargetSteamID64 && timeline.TickRate == d.Clock.TickRate
	} else if !storage.IsNotExist(err) {
		return nil, err
	}
	if !valid {
		input, err := w.storage.Open(j.DemoPath)
		if err != nil {
			return nil, err
		}
		timeline, err = customhud.Extract(ctx, input, d.Input.DemoSHA256, d.Input.TargetSteamID64, d.Clock.TickRate)
		closeErr := input.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
	}
	body, err := json.Marshal(timeline)
	if err != nil {
		return nil, err
	}
	if len(body) > customhud.MaxTelemetryBytes {
		return nil, fmt.Errorf("custom HUD telemetry exceeds byte limit")
	}
	if !valid {
		if err := w.storage.Put(key, bytes.NewReader(body)); err != nil {
			return nil, err
		}
	}
	digest := sha256.Sum256(body)
	path := filepath.Join(dir, "full-demo-hud-telemetry.json")
	if err := os.WriteFile(path, body, 0600); err != nil {
		return nil, err
	}
	return &editor.FullDemoLocalHUD{Path: path, SHA256: hex.EncodeToString(digest[:])}, nil
}
