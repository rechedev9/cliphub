package editor

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

type FullDemoDeliveryEvidence struct {
	FullDecode      bool    `json:"full_decode"`
	FrameCount      int64   `json:"frame_count"`
	SampleRate      int     `json:"sample_rate"`
	Channels        int     `json:"channels"`
	DurationSeconds float64 `json:"duration_seconds"`
	ContentSHA256   string  `json:"content_sha256"`
}

func verifyFullDemoDelivery(ctx context.Context, ffmpeg, ffprobe, path string, frames int64) (*FullDemoDeliveryEvidence, error) {
	if ffprobe == "" {
		return nil, fmt.Errorf("full_demo_output_invalid: ffprobe is required")
	}
	output, err := runFFmpegOutput(ctx, []string{ffprobe, "-v", "error", "-count_frames", "-show_entries", "stream=codec_type,codec_name,width,height,r_frame_rate,nb_read_frames,sample_rate,channels,duration", "-of", "json", path}, "Full Demo delivery probe")
	if err != nil {
		return nil, err
	}
	var probe struct {
		Streams []struct {
			Type       string `json:"codec_type"`
			Codec      string `json:"codec_name"`
			Width      int    `json:"width"`
			Height     int    `json:"height"`
			FrameRate  string `json:"r_frame_rate"`
			Frames     string `json:"nb_read_frames"`
			SampleRate string `json:"sample_rate"`
			Channels   int    `json:"channels"`
			Duration   string `json:"duration"`
		} `json:"streams"`
	}
	if len(output) > 1<<20 {
		return nil, fmt.Errorf("full demo delivery probe exceeds resource limit")
	}
	if err := json.Unmarshal([]byte(output), &probe); err != nil {
		return nil, err
	}
	e := &FullDemoDeliveryEvidence{DurationSeconds: float64(frames) / recapplan.OutputFPS}
	video, audio := false, false
	for _, stream := range probe.Streams {
		duration, parseErr := strconv.ParseFloat(stream.Duration, 64)
		if parseErr != nil || math.IsNaN(duration) || math.IsInf(duration, 0) || math.Abs(duration-e.DurationSeconds) > 1.0/recapplan.OutputFPS {
			return nil, fmt.Errorf("full_demo_output_invalid: stream duration differs from the frame/sample timeline")
		}
		switch stream.Type {
		case "video":
			count, err := strconv.ParseInt(stream.Frames, 10, 64)
			if err != nil || count != frames || video || stream.Codec != "h264" || stream.Width != 1920 || stream.Height != 1080 || !frameRateMatches(stream.FrameRate, 60) {
				return nil, fmt.Errorf("full_demo_output_invalid: delivered video differs from 1080p60 H.264 or canonical frame count")
			}
			video, e.FrameCount = true, count
		case "audio":
			if audio || stream.Codec != "aac" || stream.SampleRate != "48000" || stream.Channels != 2 {
				return nil, fmt.Errorf("full_demo_output_invalid: delivered audio is not stereo AAC at 48 kHz")
			}
			audio, e.SampleRate, e.Channels = true, 48000, 2
		default:
			return nil, fmt.Errorf("full_demo_output_invalid: unexpected delivery stream")
		}
	}
	if !video || !audio {
		return nil, fmt.Errorf("full_demo_output_invalid: missing video or audio")
	}
	if _, err := runFFmpegOutput(ctx, []string{ffmpeg, "-v", "error", "-xerror", "-i", path, "-map", "0:v:0", "-map", "0:a:0", "-f", "null", "-"}, "Full Demo complete delivery decode"); err != nil {
		return nil, err
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		return nil, fmt.Errorf("full_demo_output_invalid: missing delivered file")
	}
	e.ContentSHA256, err = mediaassets.FileDigest(ctx, path, 64<<30)
	if err != nil {
		return nil, err
	}
	e.FullDecode = true
	return e, nil
}

func (e *FullDemoRenderEvidence) ValidateCompleted() error {
	if e == nil || e.SchemaVersion != "1.0" {
		return fmt.Errorf("missing Full Demo render evidence")
	}
	if err := e.Approved.Validate(); err != nil {
		return err
	}
	if err := e.Effective.Validate(); err != nil {
		return err
	}
	ends := map[string]int{}
	for _, round := range e.Effective.Rounds {
		ends[round.ID] = round.EffectiveEndTick
	}
	expected, err := recapplan.ApplyCertifiedEnds(e.Approved, ends)
	if err != nil {
		return err
	}
	if expected.PlanHash != e.Effective.PlanHash {
		return fmt.Errorf("full demo effective plan differs from approved changes")
	}
	frames := e.Effective.Timeline[len(e.Effective.Timeline)-1].EndFrame
	if e.Delivery == nil || !e.Delivery.FullDecode || e.Delivery.FrameCount != frames || e.Delivery.SampleRate != 48000 || e.Delivery.Channels != 2 || !recapplan.ValidHash(e.Delivery.ContentSHA256) || math.IsNaN(e.Delivery.DurationSeconds) || math.IsInf(e.Delivery.DurationSeconds, 0) || math.Abs(e.Delivery.DurationSeconds-float64(frames)/recapplan.OutputFPS) > 1.0/recapplan.OutputFPS {
		return fmt.Errorf("full_demo_output_invalid: missing complete delivery evidence")
	}
	if e.ProgramLoudness == nil || len(e.ProgramLoudness.DecodedAAC) == 0 {
		return fmt.Errorf("audio_loudness_failed: missing decoded AAC evidence")
	}
	last := e.ProgramLoudness.DecodedAAC[len(e.ProgramLoudness.DecodedAAC)-1]
	a := e.Effective.Options.Audio
	if last.Status == "silent" && e.ProgramLoudness.Status == "silent-approved" && a.Game.Gain == 0 && (!a.Voice.Enabled || a.Voice.Gain == 0) && !a.Music.Enabled && !e.Effective.Options.Sponsor.Enabled {
		return nil
	}
	if e.ProgramLoudness.Status != "verified-decoded-aac" || last.Status != "measured" || last.IntegratedLUFS == nil || last.TruePeakDBTP == nil || math.IsNaN(*last.IntegratedLUFS) || math.IsNaN(*last.TruePeakDBTP) || math.IsInf(*last.IntegratedLUFS, 0) || math.IsInf(*last.TruePeakDBTP, 0) || math.Abs(*last.IntegratedLUFS-a.Loudness.TargetILUFS) > .5 || *last.TruePeakDBTP > a.Loudness.TargetTPDBTP {
		return fmt.Errorf("audio_loudness_failed: final decoded AAC misses its approved targets")
	}
	return nil
}
