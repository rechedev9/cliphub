package workers

import (
	"context"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/overlayassets"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

func (w *RenderWorker) materializeOverlayScreenshots(options recapplan.OverlayOptions, workDir string) (*demooverlay.Screenshots, error) {
	result := &demooverlay.Screenshots{}
	load := func(ref *recapplan.AssetRef) (*demooverlay.ScreenshotFile, error) {
		if ref == nil {
			return nil, &recapplan.Error{Code: recapplan.ErrAssetMissing, Detail: "Falta una captura del overlay"}
		}
		id, err := uuid.Parse(ref.ID)
		if err != nil {
			return nil, err
		}
		key := overlayassets.MediaKey(id)
		if err := mediaassets.VerifyContent(context.Background(), w.storage, key, ref.SHA256, overlayassets.MaxBytes); err != nil {
			return nil, err
		}
		file := filepath.Join(workDir, "overlay-image-"+id.String()+".image")
		if err := materializeStorageFile(w.storage, key, file); err != nil {
			return nil, err
		}
		return &demooverlay.ScreenshotFile{Path: file, SHA256: ref.SHA256}, nil
	}
	var err error
	if options.Roster {
		result.Team1, err = load(options.Team1Image)
		if err != nil {
			return nil, err
		}
		result.Team2, err = load(options.Team2Image)
		if err != nil {
			return nil, err
		}
	}
	if options.Scoreboard {
		result.Scoreboard, err = load(options.ScoreboardImage)
	}
	return result, err
}
