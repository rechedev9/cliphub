package editor

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// LabMediaEvidence is what the picture and sound of one produced file look
// like. A clean decode proves nothing about content: a black capture decodes
// fine, so the lab reports luma, black spans, bitrate and loudness as well.
type LabMediaEvidence struct {
	Path            string        `json:"path"`
	SizeBytes       int64         `json:"size_bytes"`
	DurationSeconds float64       `json:"duration_seconds"`
	BitrateKbps     float64       `json:"bitrate_kbps"`
	VideoCodec      string        `json:"video_codec,omitempty"`
	Width           int           `json:"width,omitempty"`
	Height          int           `json:"height,omitempty"`
	FrameRate       string        `json:"frame_rate,omitempty"`
	Frames          int64         `json:"frames"`
	ExpectedFrames  int64         `json:"expected_frames,omitempty"`
	FramesMatch     bool          `json:"frames_match"`
	MeanLuma        float64       `json:"mean_luma"`
	MinLuma         float64       `json:"min_luma"`
	BlackSeconds    float64       `json:"black_seconds"`
	BlackIntervals  []LabInterval `json:"black_intervals,omitempty"`
	AudioCodec      string        `json:"audio_codec,omitempty"`
	IntegratedLUFS  *float64      `json:"integrated_lufs,omitempty"`
	TruePeakDBTP    *float64      `json:"true_peak_dbtp,omitempty"`
	Stills          []string      `json:"stills,omitempty"`
}

type LabInterval struct {
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds"`
}

var (
	labLumaPattern     = regexp.MustCompile(`lavfi\.signalstats\.YAVG=([0-9.]+)`)
	labBlackPattern    = regexp.MustCompile(`black_start:\s*([0-9.]+)\s+black_end:\s*([0-9.]+)`)
	labIntegratedMatch = regexp.MustCompile(`(?m)^\s*I:\s+(-?[0-9.]+|-inf) LUFS`)
	labTruePeakMatch   = regexp.MustCompile(`(?m)^\s*Peak:\s+(-?[0-9.]+|-inf) dBFS`)
)

// measureLabMedia probes path, decodes it once with blackdetect, signalstats
// and ebur128, and writes three stills (10 %, 50 % and 90 % of the duration)
// next to stillPrefix.
func measureLabMedia(ctx context.Context, ffmpeg, ffprobe, path, stillPrefix string, expectedFrames int64) (LabMediaEvidence, error) {
	m := LabMediaEvidence{Path: path, ExpectedFrames: expectedFrames}
	if ffprobe == "" {
		return m, fmt.Errorf("ffprobe is required to measure %s", path)
	}
	probe, err := runFFmpegOutput(ctx, []string{ffprobe, "-v", "error", "-show_entries", "format=duration,size,bit_rate:stream=codec_type,codec_name,width,height,avg_frame_rate", "-of", "json", path}, "lab probe")
	if err != nil {
		return m, err
	}
	var probed struct {
		Format struct {
			Duration string `json:"duration"`
			Size     string `json:"size"`
			BitRate  string `json:"bit_rate"`
		} `json:"format"`
		Streams []struct {
			CodecType    string `json:"codec_type"`
			CodecName    string `json:"codec_name"`
			Width        int    `json:"width"`
			Height       int    `json:"height"`
			AvgFrameRate string `json:"avg_frame_rate"`
		} `json:"streams"`
	}
	if err := json.Unmarshal([]byte(probe), &probed); err != nil {
		return m, fmt.Errorf("decode lab probe: %w", err)
	}
	m.DurationSeconds, _ = strconv.ParseFloat(probed.Format.Duration, 64)
	m.SizeBytes, _ = strconv.ParseInt(probed.Format.Size, 10, 64)
	if bitrate, err := strconv.ParseFloat(probed.Format.BitRate, 64); err == nil {
		m.BitrateKbps = bitrate / 1000
	} else if m.DurationSeconds > 0 {
		m.BitrateKbps = float64(m.SizeBytes) * 8 / m.DurationSeconds / 1000
	}
	hasVideo, hasAudio := false, false
	for _, stream := range probed.Streams {
		switch stream.CodecType {
		case "video":
			if !hasVideo {
				hasVideo = true
				m.VideoCodec, m.Width, m.Height, m.FrameRate = stream.CodecName, stream.Width, stream.Height, stream.AvgFrameRate
			}
		case "audio":
			if !hasAudio {
				hasAudio = true
				m.AudioCodec = stream.CodecName
			}
		}
	}
	command := []string{ffmpeg, "-hide_banner", "-nostats", "-v", "info", "-i", path}
	if hasVideo {
		command = append(command, "-map", "0:v:0", "-vf", "blackdetect=d=0.1:pix_th=0.10,signalstats,metadata=mode=print:key=lavfi.signalstats.YAVG", "-f", "null", "-")
	}
	if hasAudio {
		command = append(command, "-map", "0:a:0", "-af", "ebur128=peak=true", "-f", "null", "-")
	}
	if !hasVideo && !hasAudio {
		return m, fmt.Errorf("%s has no audio or video stream", path)
	}
	output, err := runFFmpegOutput(ctx, command, "lab analysis")
	if err != nil {
		return m, err
	}
	parseLabAnalysis(output, &m)
	if hasVideo && m.DurationSeconds > 0 {
		for i, fraction := range []float64{.1, .5, .9} {
			still := fmt.Sprintf("%s-still-%d.png", stillPrefix, i+1)
			at := strconv.FormatFloat(m.DurationSeconds*fraction, 'f', 3, 64)
			if _, err := runFFmpegOutput(ctx, []string{ffmpeg, "-y", "-v", "error", "-ss", at, "-i", path, "-frames:v", "1", still}, "lab still"); err != nil {
				return m, err
			}
			m.Stills = append(m.Stills, still)
		}
	}
	return m, nil
}

// parseLabAnalysis reads the per-frame luma lines, the blackdetect spans and
// the ebur128 summary out of one FFmpeg analysis log.
func parseLabAnalysis(output string, m *LabMediaEvidence) {
	var sum float64
	m.MinLuma = math.Inf(1)
	for _, match := range labLumaPattern.FindAllStringSubmatch(output, -1) {
		luma, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			continue
		}
		m.Frames++
		sum += luma
		m.MinLuma = math.Min(m.MinLuma, luma)
	}
	if m.Frames > 0 {
		m.MeanLuma = sum / float64(m.Frames)
	} else {
		m.MinLuma = 0
	}
	m.FramesMatch = m.ExpectedFrames == 0 || m.Frames == m.ExpectedFrames
	for _, match := range labBlackPattern.FindAllStringSubmatch(output, -1) {
		start, _ := strconv.ParseFloat(match[1], 64)
		end, _ := strconv.ParseFloat(match[2], 64)
		m.BlackIntervals = append(m.BlackIntervals, LabInterval{StartSeconds: start, EndSeconds: end})
		m.BlackSeconds += end - start
	}
	m.IntegratedLUFS = lastLabLevel(labIntegratedMatch, output)
	m.TruePeakDBTP = lastLabLevel(labTruePeakMatch, output)
}

func lastLabLevel(pattern *regexp.Regexp, output string) *float64 {
	matches := pattern.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return nil
	}
	value := strings.TrimSpace(matches[len(matches)-1][1])
	// Silence has no finite level, and JSON cannot carry an infinity.
	if value == "-inf" {
		return nil
	}
	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil
	}
	return &v
}
