package workers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/streamclips"
)

type streamPublishArtifact struct {
	key  string
	path string
}

// streamRenderedClip is one moment encoded on its own, before groups join.
type streamRenderedClip struct {
	path    string
	elapsed time.Duration
}

type streamOutputAssembly struct {
	ffmpeg   string
	output   streamclips.PlanOutput
	rendered map[string]streamRenderedClip
	outDir   string
	// hasAudio says every part carries one audio track (music or source audio).
	hasAudio bool
	// progressAt is posted while a group joins, after every moment has encoded.
	progress   *streamclips.RenderProgressBoard
	jobID      uuid.UUID
	progressAt streamclips.RenderProgress
}

// assembleStreamOutput writes one output video into the delivery folder: a
// lone moment's render as is, or a group's renders joined back to back.
func (w *StreamRenderWorker) assembleStreamOutput(ctx context.Context, a streamOutputAssembly) (string, time.Duration, error) {
	outPath := filepath.Join(a.outDir, a.output.ID+".mp4")
	var elapsed time.Duration
	parts := make([]string, 0, len(a.output.Clips))
	for _, clip := range a.output.Clips {
		rendered, ok := a.rendered[clip.ID]
		if !ok {
			return "", 0, fmt.Errorf("clip %s was not rendered", clip.ID)
		}
		parts = append(parts, rendered.path)
		elapsed += rendered.elapsed
	}
	if len(parts) == 1 {
		return outPath, elapsed, os.Rename(parts[0], outPath)
	}
	started := time.Now()
	a.progress.Set(a.jobID, a.progressAt)
	// Every member shares the codec, size and frame rate, so the concat
	// demuxer joins the video with a hard cut and no second encode.
	var list strings.Builder
	for _, part := range parts {
		fmt.Fprintf(&list, "file '%s'\n", strings.ReplaceAll(filepath.ToSlash(part), "'", `'\''`))
	}
	listPath := filepath.Join(filepath.Dir(parts[0]), a.output.ID+".concat.txt")
	if err := os.WriteFile(listPath, []byte(list.String()), 0o600); err != nil {
		return "", 0, err
	}
	if _, err := w.runner.Run(ctx, a.ffmpeg, streamJoinArgs(listPath, parts, a.hasAudio, outPath)...); err != nil {
		return "", 0, fmt.Errorf("join group %s: %w", a.output.ID, err)
	}
	return outPath, elapsed + time.Since(started), nil
}

type streamOutputPublish struct {
	jobID      uuid.UUID
	variant    string
	revisionID uuid.UUID
	output     streamclips.PlanOutput
	path       string
	elapsed    time.Duration
}

type streamPublishedOutput struct {
	video     streamclips.VideoEntry
	delivery  streamclips.DeliveryEntry
	artifacts []streamPublishArtifact
}

// publishStreamOutput names the revision keys of one output video: its
// gallery video and its copy in the upload-ready delivery folder.
func publishStreamOutput(p streamOutputPublish) (streamPublishedOutput, error) {
	key, err := streamclips.RenderRevisionVideoKey(p.jobID, p.variant, p.revisionID, p.output.ID)
	if err != nil {
		return streamPublishedOutput{}, err
	}
	deliveryName := p.output.ID + ".mp4"
	deliveryKey, err := streamclips.RenderRevisionDeliveryKey(p.jobID, p.variant, p.revisionID, deliveryName)
	if err != nil {
		return streamPublishedOutput{}, err
	}
	var outputBytes int64
	if info, statErr := os.Stat(p.path); statErr == nil {
		outputBytes = info.Size()
	}
	video := streamclips.NewVideoEntry(p.output, key)
	video.Performance = streamclips.NewVideoPerformance(p.elapsed, video.DurationSeconds, outputBytes)
	return streamPublishedOutput{
		video:     video,
		delivery:  streamclips.DeliveryEntry{Name: deliveryName, Kind: "video", Key: deliveryKey},
		artifacts: []streamPublishArtifact{{key: key, path: p.path}, {key: deliveryKey, path: p.path}},
	}, nil
}

// streamJoinArgs copies the video through the concat demuxer and re-encodes
// the audio with the concat filter: copying AAC parts keeps each part's
// encoder delay, which shifts the audio about 23 ms at every cut.
func streamJoinArgs(listPath string, parts []string, hasAudio bool, outPath string) []string {
	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-f", "concat", "-safe", "0", "-i", listPath}
	if !hasAudio {
		return append(args, "-map", "0:v", "-c:v", "copy", "-movflags", "+faststart", outPath)
	}
	var filter strings.Builder
	for i, part := range parts {
		args = append(args, "-i", part)
		fmt.Fprintf(&filter, "[%d:a]", i+1)
	}
	fmt.Fprintf(&filter, "concat=n=%d:v=0:a=1[a]", len(parts))
	return append(args, "-filter_complex", filter.String(), "-map", "0:v", "-map", "[a]",
		"-c:v", "copy", "-c:a", "aac", "-b:a", streamclips.AACBitrate, "-movflags", "+faststart", outPath)
}

func (w *StreamRenderWorker) writeStreamCover(ctx context.Context, ffmpegPath, videoPath, filename string) error {
	if strings.TrimSpace(videoPath) == "" {
		return errors.New("rendered video is required for cover generation")
	}
	_, err := w.runner.Run(ctx, ffmpegPath,
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", videoPath,
		"-frames:v", "1", "-vf", "scale=720:-2", "-q:v", "2", filename,
	)
	if err != nil {
		return err
	}
	info, err := os.Stat(filename)
	if err != nil {
		return fmt.Errorf("verify cover output: %w", err)
	}
	if info.Size() == 0 {
		return errors.New("cover output is empty")
	}
	return nil
}
