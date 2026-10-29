package editor

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

// FullDemoBumperPad records a bumper whose video holds fewer frames than its
// planned length; its item clones the last frame PaddedFrames times.
type FullDemoBumperPad struct {
	Reason        string `json:"reason"`
	VideoFrames   int64  `json:"video_frames"`
	PlannedFrames int64  `json:"planned_frames"`
	PaddedFrames  int64  `json:"padded_frames"`
}

// measureFullDemoBumpers decodes each bumper's video at the output rate before
// any item encodes. The plan sizes a bumper from its container duration, which
// the audio track or a late video start can set, so the video can hold fewer
// frames. Up to FullDemoTailPadToleranceFrames are cloned and recorded; a
// larger shortfall fails here, naming the clip, not after the whole render.
func measureFullDemoBumpers(ctx context.Context, short *ShortEdit) error {
	short.FullDemo.BumperPads = nil
	for _, item := range short.FullDemo.Effective.Timeline {
		if item.Role != "bumper" {
			continue
		}
		ref, _, err := fullDemoBumperAsset(short.FullDemo.Effective, item)
		if err != nil {
			return err
		}
		path, err := short.fullDemo.execution.assetPath(ref)
		if err != nil {
			return err
		}
		command := []string{short.fullDemo.ffmpeg, "-v", "error", "-xerror", "-i", path, "-map", "0:v:0", "-vf", "fps=60", "-fps_mode", "passthrough", "-f", "null", "-"}
		var decoded bytes.Buffer
		if _, err := runFFmpegOutputProgressTo(ctx, command, "Full Demo bumper frame count", 0, nil, &decoded); err != nil {
			return err
		}
		frames, err := decodedDeliveryFrames(decoded.String())
		if err != nil {
			return fmt.Errorf("full_demo_asset_invalid: count %s frames: %w", bumperName(item.Reason), err)
		}
		planned := item.EndFrame - item.StartFrame
		missing := planned - frames
		if missing <= 0 {
			continue
		}
		if missing > recording.FullDemoTailPadToleranceFrames {
			return fmt.Errorf("full_demo_asset_invalid: the %s video has %d frames at 60 fps but the plan needs %d; re-export it so the video lasts as long as its audio", bumperName(item.Reason), frames, planned)
		}
		short.FullDemo.BumperPads = append(short.FullDemo.BumperPads, FullDemoBumperPad{Reason: item.Reason, VideoFrames: frames, PlannedFrames: planned, PaddedFrames: missing})
	}
	return nil
}

func bumperName(reason string) string {
	return strings.TrimSuffix(reason, "-bumper")
}

// fullDemoBumperTailPad returns the measured clone count for a bumper item.
func fullDemoBumperTailPad(pads []FullDemoBumperPad, reason string) int64 {
	for _, pad := range pads {
		if pad.Reason == reason {
			return pad.PaddedFrames
		}
	}
	return 0
}

// validateFullDemoBumperPads accepts only bounded, self-consistent pads of
// distinct bumper items. The delivery frame count then proves the padded
// items still produced the canonical program length.
func validateFullDemoBumperPads(pads []FullDemoBumperPad, effective recapplan.Document) error {
	planned := map[string]int64{}
	for _, item := range effective.Timeline {
		if item.Role == "bumper" {
			planned[item.Reason] = item.EndFrame - item.StartFrame
		}
	}
	seen := map[string]bool{}
	for _, pad := range pads {
		frames, ok := planned[pad.Reason]
		if !ok || seen[pad.Reason] || pad.PlannedFrames != frames || pad.PaddedFrames < 1 || pad.PaddedFrames > recording.FullDemoTailPadToleranceFrames || pad.PlannedFrames-pad.VideoFrames != pad.PaddedFrames {
			return fmt.Errorf("full_demo_output_invalid: bumper pad %s does not match one bounded bumper item", pad.Reason)
		}
		seen[pad.Reason] = true
	}
	return nil
}
