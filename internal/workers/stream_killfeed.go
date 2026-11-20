package workers

import (
	"context"
	"errors"
	"fmt"

	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/vodplays"
)

type killFeedRequest struct {
	ffmpeg string
	source string
	plan   streamclips.EditPlan
	clip   streamclips.ClipRange
	probe  streamclips.SourceProbe
}

// clipKillFeed finds where the streamer's notices sat during the clip, with
// times relative to the clip start, and turns them into the live feed cut.
func clipKillFeed(ctx context.Context, req killFeedRequest) (*streamclips.KillFeedOverlay, error) {
	if !req.plan.KillFeedEnabled() {
		return nil, nil
	}
	if req.probe.Width <= 0 || req.probe.Height <= 0 {
		return nil, errors.New("kill feed needs the source size")
	}
	spots, err := vodplays.FindNotices(ctx, vodplays.NoticeOptions{
		FFmpeg: req.ffmpeg, Input: req.source, StartSeconds: req.clip.StartSeconds, EndSeconds: req.clip.EndSeconds,
	})
	if err != nil {
		return nil, err
	}
	masks := make([]streamclips.KillFeedMask, 0, len(spots))
	for _, spot := range spots {
		x, y, w, h := spot.SourceArea()
		masks = append(masks, streamclips.KillFeedMask{Area: streamclips.CropRect{X: x, Y: y, Width: w, Height: h},
			From: spot.From - req.clip.StartSeconds, To: spot.To - req.clip.StartSeconds})
	}
	return streamclips.NewKillFeedOverlay(masks, req.probe.Width, req.probe.Height, req.clip.EndSeconds-req.clip.StartSeconds), nil
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

func hasSourceMask(o *streamclips.KillFeedOverlay) bool {
	return o != nil && len(o.Masks) > 0
}

func withoutSourceMask(o *streamclips.KillFeedOverlay) *streamclips.KillFeedOverlay {
	out := *o
	out.Masks = nil
	return &out
}
