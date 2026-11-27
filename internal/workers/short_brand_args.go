package workers

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/brandkit"
	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/overlayassets"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

// brandArgs copies the edit's brand images into workDir, checked against the
// hashes the edit recorded, and returns their editor flags.
func (w *RenderWorker) brandArgs(ctx context.Context, edit renderplan.EditRequest, workDir string) ([]string, error) {
	var args []string
	if wm := edit.BrandWatermark; wm != nil {
		path, err := w.materializeBrandImage(ctx, wm.Image, filepath.Join(workDir, "brand-watermark.media"))
		if err != nil {
			return nil, fmt.Errorf("brand watermark: %w", err)
		}
		args = append(args, "--watermark-image", path, "--watermark-corner", wm.Corner,
			"--watermark-opacity", strconv.FormatFloat(wm.Opacity, 'f', -1, 64))
	}
	if banner := edit.BrandBanner; banner != nil {
		path, err := w.materializeBrandImage(ctx, banner.Image, filepath.Join(workDir, "brand-banner.media"))
		if err != nil {
			return nil, fmt.Errorf("sponsor banner: %w", err)
		}
		args = append(args, "--sponsor-image", path, "--sponsor-start", strconv.FormatFloat(banner.StartSeconds, 'f', -1, 64))
		if banner.EndSeconds > 0 {
			args = append(args, "--sponsor-end", strconv.FormatFloat(banner.EndSeconds, 'f', -1, 64))
		}
	}
	if card := edit.SubscribeBanner; card != nil {
		args = append(args, "--subscribe")
		if card.Logo != nil {
			path, err := w.materializeBrandImage(ctx, *card.Logo, filepath.Join(workDir, "brand-subscribe-logo.media"))
			if err != nil {
				return nil, fmt.Errorf("subscribe logo: %w", err)
			}
			args = append(args, "--subscribe-logo", path)
		}
		if card.ChannelName != "" {
			args = append(args, "--subscribe-channel", card.ChannelName)
		}
	}
	return args, nil
}

func (w *RenderWorker) materializeBrandImage(ctx context.Context, ref brandkit.ImageRef, path string) (string, error) {
	id, err := uuid.Parse(ref.ID)
	if err != nil {
		return "", err
	}
	key := overlayassets.MediaKey(id)
	if err := mediaassets.VerifyContent(ctx, w.storage, key, ref.SHA256, overlayassets.MaxBytes); err != nil {
		return "", err
	}
	if err := materializeStorageFile(w.storage, key, path); err != nil {
		return "", err
	}
	return path, nil
}

// bookendTextArgs are the editor flags of the custom intro and outro text.
func bookendTextArgs(edit renderplan.EditRequest) []string {
	var args []string
	if edit.IntroText != "" {
		args = append(args, "--intro-text", edit.IntroText)
	}
	if edit.OutroText != "" {
		args = append(args, "--outro-text", edit.OutroText)
	}
	return args
}
