package workers

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/vodplays"
)

type killFeedRequest struct {
	ffmpeg  string
	source  string
	workDir string
	plan    streamclips.EditPlan
	clip    streamclips.ClipRange
	probe   streamclips.SourceProbe
}

// clipKillFeed rebuilds the streamer's notices seen during the clip and
// scales them to the output, with times relative to the clip start.
func clipKillFeed(ctx context.Context, req killFeedRequest) ([]streamclips.KillFeedNotice, error) {
	if !req.plan.KillFeedEnabled() {
		return nil, nil
	}
	scale, err := streamclips.KillFeedScale(req.plan, req.probe.Width, req.probe.Height)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(req.workDir, "killfeed", req.clip.ID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	sprites, err := vodplays.ExtractNotices(ctx, vodplays.NoticeOptions{
		FFmpeg: req.ffmpeg, Input: req.source, WorkDir: dir,
		StartSeconds: req.clip.StartSeconds, EndSeconds: req.clip.EndSeconds,
	})
	if err != nil {
		return nil, err
	}
	duration := req.clip.EndSeconds - req.clip.StartSeconds
	notices := make([]streamclips.KillFeedNotice, 0, len(sprites))
	for i, sprite := range sprites {
		scaled, err := vodplays.ScaleNotice(sprite, scale, filepath.Join(dir, fmt.Sprintf("scaled-%03d.png", i+1)))
		if err != nil {
			return nil, err
		}
		masks := make([]streamclips.KillFeedMask, 0, len(sprite.Spots))
		for _, spot := range sprite.Spots {
			x, y, w, h := spot.SourceArea()
			masks = append(masks, streamclips.KillFeedMask{Area: streamclips.CropRect{X: x, Y: y, Width: w, Height: h},
				From: spot.From - req.clip.StartSeconds, To: spot.To - req.clip.StartSeconds})
		}
		notices = append(notices, streamclips.KillFeedNotice{
			Path: scaled.Path, Width: scaled.Width, Height: scaled.Height,
			Masks:            masks,
			AppearSeconds:    scaled.AppearSeconds - req.clip.StartSeconds,
			DisappearSeconds: math.Min(scaled.DisappearSeconds-req.clip.StartSeconds, duration),
		})
	}
	return notices, nil
}

type streamClipRender struct {
	run  func(ctx context.Context, args []string) error
	in   streamclips.FFmpegInputs
	plan streamclips.EditPlan
	clip streamclips.ClipRange
}

// renderStreamClip runs one clip render. When hiding the original feed breaks
// FFmpeg (a probed size that differs from the frames), it renders unmasked
// and returns a warning instead of failing the whole render.
func renderStreamClip(ctx context.Context, r streamClipRender) (string, error) {
	args, err := streamclips.BuildFFmpegArgs(r.in, r.plan, r.clip)
	if err != nil {
		return "", err
	}
	runErr := r.run(ctx, args)
	if runErr == nil || ctx.Err() != nil || !hasSourceMask(r.in.KillFeed) {
		return "", runErr
	}
	r.in.KillFeed = withoutSourceMask(r.in.KillFeed)
	if args, err = streamclips.BuildFFmpegArgs(r.in, r.plan, r.clip); err != nil {
		return "", err
	}
	if err := r.run(ctx, args); err != nil {
		return "", err
	}
	// FFmpeg's whole stderr is in runErr; the warning keeps only the outcome.
	return fmt.Sprintf("clip %s: original kill feed not hidden, FFmpeg refused the mask", r.clip.ID), nil
}

func hasSourceMask(notices []streamclips.KillFeedNotice) bool {
	for _, n := range notices {
		if len(n.Masks) > 0 {
			return true
		}
	}
	return false
}

func withoutSourceMask(notices []streamclips.KillFeedNotice) []streamclips.KillFeedNotice {
	out := make([]streamclips.KillFeedNotice, len(notices))
	for i, n := range notices {
		n.Masks = nil
		out[i] = n
	}
	return out
}
