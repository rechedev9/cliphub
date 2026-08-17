// Package timelinerender builds and runs the multitrack FFmpeg compositor.
package timelinerender

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/timelineplan"
)

const (
	defaultVideoCRF   = 18
	defaultAACBitrate = "192k"
	defaultPreset     = "slow"
	gradeFilter       = "eq=contrast=1.05:saturation=1.15"
)

// AssetInput is one resolved media file referenced by the timeline.
type AssetInput struct {
	Path     string
	HasAudio bool
}

// Inputs are machine-resolved paths for one render. The document stores only
// asset ids and music keys.
type Inputs struct {
	Assets           map[string]AssetInput
	OutputPath       string
	MusicPath        string
	FontPath         string
	TextOverlayPaths []string
}

func BuildFFmpegArgs(in Inputs, doc timelineplan.Document) ([]string, error) {
	doc = timelineplan.Normalize(doc)
	if err := doc.ValidateForRender(); err != nil {
		return nil, err
	}
	if in.OutputPath == "" {
		return nil, fmt.Errorf("output path is required")
	}
	if len(doc.Overlays) > 0 {
		if in.FontPath == "" {
			return nil, fmt.Errorf("text overlay font path is required")
		}
		if len(in.TextOverlayPaths) != len(doc.Overlays) {
			return nil, fmt.Errorf("text overlay paths must match overlays")
		}
	}

	assetOrder, err := collectAssets(doc, in.Assets)
	if err != nil {
		return nil, err
	}

	duration := doc.DurationSeconds()
	if duration <= 0 {
		return nil, fmt.Errorf("timeline duration must be > 0")
	}

	args := []string{"-hide_banner", "-y"}
	for _, id := range assetOrder {
		args = append(args, "-i", in.Assets[id].Path)
	}
	musicIndex := -1
	if in.MusicPath != "" && doc.Music.Key != "" {
		musicIndex = len(assetOrder)
		args = append(args, "-stream_loop", "-1", "-i", in.MusicPath)
	}

	graph, err := buildFilterGraph(doc, in, assetOrder, musicIndex, duration)
	if err != nil {
		return nil, err
	}
	args = append(args,
		"-filter_complex", graph,
		"-map", "[vout]",
		"-map", "[aout]",
		"-c:v", "libx264",
		"-preset", defaultPreset,
		"-crf", strconv.Itoa(defaultVideoCRF),
		"-pix_fmt", "yuv420p",
		"-r", strconv.Itoa(doc.Canvas.FPS),
		"-c:a", "aac",
		"-b:a", defaultAACBitrate,
		"-movflags", "+faststart",
		"-t", secondsArg(duration),
		in.OutputPath,
	)
	return args, nil
}

func collectAssets(doc timelineplan.Document, assets map[string]AssetInput) ([]string, error) {
	var order []string
	seen := map[string]bool{}
	for _, track := range doc.Tracks {
		for _, item := range track.Items {
			if seen[item.AssetID] {
				continue
			}
			input, ok := assets[item.AssetID]
			if !ok || strings.TrimSpace(input.Path) == "" {
				return nil, fmt.Errorf("missing media for asset %s", item.AssetID)
			}
			seen[item.AssetID] = true
			order = append(order, item.AssetID)
		}
	}
	if len(order) == 0 {
		return nil, fmt.Errorf("timeline has no assets")
	}
	return order, nil
}

func buildFilterGraph(doc timelineplan.Document, in Inputs, assetOrder []string, musicIndex int, duration float64) (string, error) {
	indexOf := map[string]int{}
	for i, id := range assetOrder {
		indexOf[id] = i
	}
	w, h, fps := doc.Canvas.Width, doc.Canvas.Height, doc.Canvas.FPS
	var b strings.Builder
	fmt.Fprintf(&b, "color=c=black:s=%dx%d:d=%s:r=%d,format=yuva420p[bg];", w, h, secondsArg(duration), fps)

	type videoLabel struct {
		label string
		item  timelineplan.Item
	}
	var videos []videoLabel
	var audioLabels []string
	videoN := 0
	audioN := 0
	for _, track := range doc.Tracks {
		for _, item := range track.Items {
			idx := indexOf[item.AssetID]
			src := in.Assets[item.AssetID]
			if track.Kind == timelineplan.KindVideo {
				label := fmt.Sprintf("v%d", videoN)
				videoN++
				chain, err := videoChain(item, w, h, duration)
				if err != nil {
					return "", err
				}
				fmt.Fprintf(&b, "[%d:v]%s[%s];", idx, chain, label)
				videos = append(videos, videoLabel{label: label, item: item})
			}
			if !src.HasAudio {
				continue
			}
			if item.Volume != nil && *item.Volume == 0 {
				continue
			}
			alabel := fmt.Sprintf("a%d", audioN)
			audioN++
			fmt.Fprintf(&b, "[%d:a]%s[%s];", idx, audioChain(item, duration), alabel)
			audioLabels = append(audioLabels, alabel)
		}
	}

	prev := "bg"
	for i, layer := range videos {
		next := fmt.Sprintf("ov%d", i)
		if i == len(videos)-1 && len(doc.Overlays) == 0 {
			next = "vout"
		}
		tf := layer.item.ResolvedTransform()
		x := fmt.Sprintf("%d*%s", w, floatArg(tf.X))
		y := fmt.Sprintf("%d*%s", h, floatArg(tf.Y))
		fmt.Fprintf(&b, "[%s][%s]overlay=x=%s:y=%s:eof_action=pass:shortest=0:format=auto[%s];", prev, layer.label, x, y, next)
		prev = next
	}
	if len(videos) == 0 {
		return "", fmt.Errorf("timeline has no video items")
	}

	if len(doc.Overlays) > 0 {
		cur := prev
		for i, overlay := range doc.Overlays {
			next := fmt.Sprintf("tx%d", i)
			if i == len(doc.Overlays)-1 {
				next = "vout"
			}
			fmt.Fprintf(&b, "[%s]%s[%s];", cur, drawtextFilter(overlay, in.FontPath, in.TextOverlayPaths[i], duration), next)
			cur = next
		}
	}

	if musicIndex >= 0 {
		alabel := fmt.Sprintf("a%d", audioN)
		fmt.Fprintf(&b, "[%d:a]atrim=0:%s,asetpts=PTS-STARTPTS,volume=%s[%s];", musicIndex, secondsArg(duration), floatArg(doc.Music.Volume), alabel)
		audioLabels = append(audioLabels, alabel)
	}
	if len(audioLabels) == 0 {
		fmt.Fprintf(&b, "anullsrc=r=48000:cl=stereo:d=%s[aout]", secondsArg(duration))
		return b.String(), nil
	}
	if len(audioLabels) == 1 {
		fmt.Fprintf(&b, "[%s]aresample=48000,aformat=sample_fmts=fltp:channel_layouts=stereo,apad=whole_dur=%s,atrim=0:%s[aout]", audioLabels[0], secondsArg(duration), secondsArg(duration))
		return b.String(), nil
	}
	for _, label := range audioLabels {
		b.WriteByte('[')
		b.WriteString(label)
		b.WriteByte(']')
	}
	fmt.Fprintf(&b, "amix=inputs=%d:duration=longest:normalize=0,aresample=48000,aformat=sample_fmts=fltp:channel_layouts=stereo,apad=whole_dur=%s,atrim=0:%s[aout]", len(audioLabels), secondsArg(duration), secondsArg(duration))
	return b.String(), nil
}

func videoChain(item timelineplan.Item, canvasW, canvasH int, _ float64) (string, error) {
	tf := item.ResolvedTransform()
	outW := int(math.Round(float64(canvasW) * tf.Width))
	outH := int(math.Round(float64(canvasH) * tf.Height))
	if outW < 2 {
		outW = 2
	}
	if outH < 2 {
		outH = 2
	}
	speed := item.EffectiveSpeed()
	parts := []string{
		fmt.Sprintf("trim=start=%s:end=%s", secondsArg(item.SourceIn), secondsArg(item.SourceOut)),
		"setpts=(PTS-STARTPTS)/" + floatArg(speed) + "+" + secondsArg(item.TimelineStart) + "/TB",
		fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", outW, outH),
		fmt.Sprintf("pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black@0", outW, outH),
		"setsar=1",
		"format=yuva420p",
	}
	if item.Filter == timelineplan.FilterGrade {
		parts = append(parts, gradeFilter)
	}
	if fade := videoFades(item); fade != "" {
		parts = append(parts, fade)
	}
	if opacity := item.EffectiveOpacity(); opacity < 1 {
		parts = append(parts, "colorchannelmixer=aa="+floatArg(opacity))
	}
	return strings.Join(parts, ","), nil
}

func videoFades(item timelineplan.Item) string {
	var parts []string
	if item.FadeIn > 0 {
		parts = append(parts, fmt.Sprintf("fade=t=in:st=%s:d=%s:alpha=1", secondsArg(item.TimelineStart), secondsArg(item.FadeIn)))
	}
	if item.FadeOut > 0 {
		start := item.TimelineEnd() - item.FadeOut
		parts = append(parts, fmt.Sprintf("fade=t=out:st=%s:d=%s:alpha=1", secondsArg(start), secondsArg(item.FadeOut)))
	}
	return strings.Join(parts, ",")
}

func audioChain(item timelineplan.Item, duration float64) string {
	speed := item.EffectiveSpeed()
	parts := []string{
		fmt.Sprintf("atrim=start=%s:end=%s", secondsArg(item.SourceIn), secondsArg(item.SourceOut)),
		"asetpts=PTS-STARTPTS",
	}
	if speed != 1 {
		parts = append(parts, atempoChain(speed))
	}
	delayMS := int(math.Round(item.TimelineStart * 1000))
	if delayMS > 0 {
		parts = append(parts, fmt.Sprintf("adelay=%d:all=1", delayMS))
	}
	if item.Volume != nil {
		parts = append(parts, "volume="+floatArg(*item.Volume))
	}
	parts = append(parts, "apad=whole_dur="+secondsArg(duration), "atrim=0:"+secondsArg(duration))
	return strings.Join(parts, ",")
}

func drawtextFilter(overlay timelineplan.TextOverlay, fontPath, textPath string, duration float64) string {
	end := duration
	if overlay.EndSeconds != nil {
		end = *overlay.EndSeconds
	}
	size := overlay.FontSize
	if size == 0 {
		size = 64
	}
	y := fmt.Sprintf("h*%s-th/2", floatArg(overlay.PositionY))
	return fmt.Sprintf(
		"drawtext=fontfile='%s':textfile='%s':expansion=none:fontsize=%d:fontcolor=white:borderw=4:bordercolor=black:x=(w-tw)/2:y=%s:enable='between(t\\,%s\\,%s)'",
		ffmpegFilterPath(fontPath),
		ffmpegFilterPath(textPath),
		size,
		y,
		secondsArg(overlay.StartSeconds),
		secondsArg(end),
	)
}

func atempoChain(speed float64) string {
	switch {
	case speed > 2:
		return "atempo=2.000000,atempo=" + floatArg(speed/2)
	case speed < 0.5:
		return "atempo=0.500000,atempo=" + floatArg(speed/0.5)
	default:
		return "atempo=" + floatArg(speed)
	}
}

func ffmpegFilterPath(value string) string {
	value = strings.ReplaceAll(value, `\`, "/")
	value = strings.ReplaceAll(value, ":", `\:`)
	return strings.ReplaceAll(value, "'", `\'`)
}

func secondsArg(v float64) string {
	return strconv.FormatFloat(v, 'f', 9, 64)
}

func floatArg(v float64) string {
	return strconv.FormatFloat(v, 'f', 6, 64)
}

func WriteOverlayTexts(dir string, overlays []timelineplan.TextOverlay) ([]string, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(overlays))
	for i, overlay := range overlays {
		path := filepath.Join(dir, fmt.Sprintf("overlay-%d.txt", i))
		if err := os.WriteFile(path, []byte(overlay.Text), 0o600); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}
