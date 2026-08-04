package streamclips

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/rechedev9/tickcut/internal/keydropbanner"
	"github.com/rechedev9/tickcut/internal/mediafont"
)

const (
	outputFPS             = 60
	defaultVideoCRF       = 18
	defaultAACBitrate     = "192k"
	defaultPreset         = "slow"
	bannerHeight          = 96
	bannerSlideSeconds    = 0.35
	bannerColor           = "0x9146ff"
	bannerAccentColor     = "0x5b1ba9"
	landscapeBannerWidth  = 520
	landscapeBannerHeight = 64
	landscapeBannerX      = 32

	// gradeFilter is the light contrast/saturation lift EffectsPlan.Grade
	// applies — the same restrained look TickCut's viral presets use.
	gradeFilter = "eq=contrast=1.05:saturation=1.15"
)

// FFmpegInputs carries the machine-resolved inputs for one clip render. The
// edit plan stores only the music track KEY; the worker resolves it to an
// on-disk path (MusicPath) and reports whether the probed source has an audio
// stream, which decides how music is mixed.
type FFmpegInputs struct {
	SourcePath     string
	OutputPath     string
	MusicPath      string // resolved track file; empty renders without music
	BannerFontPath string // resolved bold font file; required when the banner has a nick
	// KeyDropImagePath is the pre-composited plate PNG with the live sponsor
	// code already burned in (keydropbanner.CompositeWithCode). Required when
	// the plan enables a KeyDrop banner.
	KeyDropImagePath string
	SourceHasAudio   bool
	// TextOverlayPaths holds materialized text files, index-aligned with the
	// clip's Edit.TextOverlays. drawtext reads each file with expansion=none,
	// so arbitrary user text never needs filtergraph escaping.
	TextOverlayPaths []string
}

func BuildFFmpegArgs(in FFmpegInputs, plan EditPlan, clip ClipRange) ([]string, error) {
	plan = NormalizeEditPlan(plan)
	clip = normalizeClipRange(clip)
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	if err := clip.Validate(); err != nil {
		return nil, err
	}
	layout, ok := VariantByName(plan.Variant)
	if !ok {
		return nil, unknownVariantError(plan.Variant)
	}
	if plan.StreamerBanner.Nick != "" && in.BannerFontPath == "" {
		return nil, fmt.Errorf("streamer banner font path is required")
	}
	if plan.KeyDropBanner.Enabled() {
		if in.KeyDropImagePath == "" {
			return nil, fmt.Errorf("keydrop banner image path is required")
		}
	}
	if clip.Edit != nil && len(clip.Edit.TextOverlays) > 0 {
		if in.BannerFontPath == "" {
			return nil, fmt.Errorf("text overlay font path is required")
		}
		if len(in.TextOverlayPaths) != len(clip.Edit.TextOverlays) {
			return nil, fmt.Errorf(
				"clip %s text overlay paths length %d must match %d text overlays",
				clip.ID, len(in.TextOverlayPaths), len(clip.Edit.TextOverlays),
			)
		}
	}
	duration := clip.EndSeconds - clip.StartSeconds

	// Input 0 is always the source. Music (optional) stays at 1 so existing
	// audio filters keep their [1:a] labels. The KeyDrop plate is appended
	// after music so its index is 1 without music and 2 with music.
	nextInput := 1
	musicInput := -1
	keyDropInput := -1
	if in.MusicPath != "" {
		musicInput = nextInput
		nextInput++
	}
	if plan.KeyDropBanner.Enabled() {
		keyDropInput = nextInput
	}

	filter, err := buildStandardFilterGraph(layout, plan, clip, in.BannerFontPath, in.TextOverlayPaths, duration, keyDropInput)
	if err != nil {
		return nil, err
	}

	args := []string{
		"-y",
		"-ss", secondsArg(clip.StartSeconds),
		"-t", secondsArg(duration),
		"-i", in.SourcePath,
	}
	if musicInput >= 0 {
		args = append(args, "-stream_loop", "-1", "-i", in.MusicPath)
	}
	if keyDropInput >= 0 {
		args = append(args, "-loop", "1", "-t", secondsArg(duration), "-i", in.KeyDropImagePath)
	}
	audioMap := "0:a?"
	shortest := false
	srcFilters := sourceAudioFilters(clip.Edit)
	fadeFilters := boundaryFades(clip.Edit, clip.OutputDurationSeconds(), "afade")
	if musicInput >= 0 {
		// Loop the track so it always covers the clip; amix/-shortest bound it.
		volume := plan.Music.Volume
		if volume == 0 {
			volume = defaultMusicVolume
		}
		musicLabel := fmt.Sprintf("[%d:a]", musicInput)
		if in.SourceHasAudio {
			// Gain and tempo edits apply to the source before the mix so the
			// music keeps its own volume and pace; fades apply to the mix.
			mixInput := "[0:a]"
			if srcFilters != "" {
				filter += ";[0:a]" + srcFilters + "[srca]"
				mixInput = "[srca]"
			}
			filter += fmt.Sprintf(";%svolume=%s[bgm];%s[bgm]amix=inputs=2:duration=first:dropout_transition=0:normalize=0", musicLabel, floatArg(volume), mixInput)
			if fadeFilters != "" {
				filter += "," + fadeFilters
			}
			filter += "[a]"
		} else {
			// No original audio to bound the mix: -shortest ends with the video.
			filter += fmt.Sprintf(";%svolume=%s", musicLabel, floatArg(volume))
			if fadeFilters != "" {
				filter += "," + fadeFilters
			}
			filter += "[a]"
			shortest = true
		}
		audioMap = "[a]"
	} else if in.SourceHasAudio {
		chain := srcFilters
		if fadeFilters != "" {
			if chain != "" {
				chain += ","
			}
			chain += fadeFilters
		}
		if chain != "" {
			filter += ";[0:a]" + chain + "[a]"
			audioMap = "[a]"
		}
	} else if clip.Edit.speed() != 1 {
		// A probed-silent source renders without an audio track when speed
		// changes the timeline: with a correct probe 0:a? maps nothing anyway,
		// and with a stale probe passing the stream untouched would desync it.
		audioMap = ""
	}
	args = append(args,
		"-filter_complex", filter,
		"-map", "[v]",
	)
	if audioMap != "" {
		args = append(args, "-map", audioMap)
	}
	args = append(args,
		"-c:v", "libx264",
		"-preset", defaultPreset,
		"-crf", strconv.Itoa(defaultVideoCRF),
		"-c:a", "aac",
		"-b:a", defaultAACBitrate,
		"-movflags", "+faststart",
	)
	if shortest {
		args = append(args, "-shortest")
	}
	return append(args, in.OutputPath), nil
}

// videoTail is the filter chain every graph applies after the layout and any
// banner overlay: text overlays and the speed change first (both in
// source time up to setpts), then boundary fades in output time, then the
// grade and the output format. An unedited clip keeps the pre-edit chain.
func videoTail(plan EditPlan, clip ClipRange, fontPath string, textPaths []string) string {
	var parts []string
	if clip.Edit != nil {
		for i, overlay := range clip.Edit.TextOverlays {
			parts = append(parts, textOverlayFilter(overlay, fontPath, textPaths[i]))
		}
		if speed := clip.Edit.speed(); speed != 1 {
			parts = append(parts, "setpts=PTS/"+floatArg(speed))
		}
		if fades := boundaryFades(clip.Edit, clip.OutputDurationSeconds(), "fade"); fades != "" {
			parts = append(parts, fades)
		}
	}
	if plan.Effects.Grade {
		parts = append(parts, gradeFilter)
	}
	parts = append(parts, fmt.Sprintf("fps=%d,format=yuv420p[v]", outputFPS))
	return strings.Join(parts, ",")
}

// boundaryFades emits the clip-edge fades in output (post-speed) time. The
// name is the FFmpeg filter to use — "fade" for video, "afade" for audio — so
// both timelines share one timing implementation and can never drift apart.
func boundaryFades(edit *ClipEdit, outputDuration float64, name string) string {
	if edit == nil {
		return ""
	}
	var parts []string
	if fadeIn := edit.FadeInSeconds; fadeIn > 0 {
		parts = append(parts, fmt.Sprintf("%s=t=in:st=0:d=%s", name, floatArg(fadeIn)))
	}
	if fadeOut := edit.FadeOutSeconds; fadeOut > 0 {
		parts = append(parts, fmt.Sprintf("%s=t=out:st=%s:d=%s", name, floatArg(outputDuration-fadeOut), floatArg(fadeOut)))
	}
	return strings.Join(parts, ",")
}

// textOverlayFilter burns one centered text line. The text comes from a
// materialized file read with expansion=none, so no user character can reach
// FFmpeg's filtergraph or drawtext expansion syntax. The enable window is in
// source-relative seconds because it runs before the speed setpts.
func textOverlayFilter(overlay TextOverlay, fontPath, textPath string) string {
	size := overlay.FontSize
	if size == 0 {
		size = defaultOverlayFontSize
	}
	filter := fmt.Sprintf(
		"drawtext=fontfile='%s':textfile='%s':expansion=none:fontcolor=white:fontsize=%d:borderw=3:bordercolor=black:"+
			"shadowcolor=black@0.35:shadowx=2:shadowy=2:x=(w-text_w)/2:y=h*%s-text_h/2",
		ffmpegFilterPath(fontPath), ffmpegFilterPath(textPath), size, floatArg(overlay.PositionY),
	)
	switch {
	case overlay.StartSeconds != nil && overlay.EndSeconds != nil:
		filter += fmt.Sprintf(`:enable='between(t\,%s\,%s)'`, floatArg(*overlay.StartSeconds), floatArg(*overlay.EndSeconds))
	case overlay.StartSeconds != nil:
		filter += fmt.Sprintf(`:enable='gte(t\,%s)'`, floatArg(*overlay.StartSeconds))
	case overlay.EndSeconds != nil:
		filter += fmt.Sprintf(`:enable='lte(t\,%s)'`, floatArg(*overlay.EndSeconds))
	}
	return filter
}

// sourceAudioFilters is the chain applied to the clip's original audio: gain
// first, then the tempo chain. Empty means the stream passes through.
func sourceAudioFilters(edit *ClipEdit) string {
	if edit == nil {
		return ""
	}
	var parts []string
	if edit.SourceVolume != nil {
		parts = append(parts, "volume="+floatArg(*edit.SourceVolume))
	}
	if speed := edit.speed(); speed != 1 {
		parts = append(parts, atempoChain(speed))
	}
	return strings.Join(parts, ",")
}

// atempoChain expresses a rate in [0.25, 3] as chained atempo filters, since a
// single atempo instance only covers [0.5, 2].
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

func buildStandardFilterGraph(layout LayoutVariant, plan EditPlan, clip ClipRange, bannerFontPath string, textPaths []string, duration float64, keyDropInput int) (string, error) {
	tail := videoTail(plan, clip, bannerFontPath, textPaths)
	needsNamedContent := plan.StreamerBanner.Nick != "" || plan.KeyDropBanner.Enabled()

	outputLabel := ""
	if needsNamedContent {
		outputLabel = "[content]"
	}

	var content string
	if layout.Name == VariantStreamerLandscape16x9 {
		content = fmt.Sprintf(
			"[0:v]%s,scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black%s",
			cropFilter(plan.GameplayCrop),
			layout.OutputWidth, layout.GameOutputHeight, layout.OutputWidth, layout.GameOutputHeight,
			outputLabel,
		)
	} else if layout.FullFrame {
		content = fmt.Sprintf(
			"[0:v]%s,scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d%s",
			cropFilter(plan.GameplayCrop),
			layout.OutputWidth, layout.GameOutputHeight, layout.OutputWidth, layout.GameOutputHeight,
			outputLabel,
		)
	} else {
		content = fmt.Sprintf(
			"[0:v]split=2[facein][gamein];"+
				"[facein]%s,scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d[face];"+
				"[gamein]%s,scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d[game];"+
				"[face][game]vstack=inputs=2%s",
			cropFilter(plan.FaceCrop),
			layout.OutputWidth, layout.FaceOutputHeight, layout.OutputWidth, layout.FaceOutputHeight,
			cropFilter(plan.GameplayCrop),
			layout.OutputWidth, layout.GameOutputHeight, layout.OutputWidth, layout.GameOutputHeight,
			outputLabel,
		)
	}

	if !needsNamedContent {
		return content + "," + tail, nil
	}

	current := "content"
	graph := content
	if plan.StreamerBanner.Nick != "" {
		graph += ";" + streamerBannerFilter(layout, plan.StreamerBanner, bannerFontPath, duration)
		current = "bannered"
	}
	if plan.KeyDropBanner.Enabled() {
		if keyDropInput < 0 {
			return "", fmt.Errorf("keydrop banner input index is required")
		}
		kdFilter, err := keyDropBannerFilter(layout, plan.KeyDropBanner, duration, current, keyDropInput)
		if err != nil {
			return "", err
		}
		graph += ";" + kdFilter
		current = "keydropped"
	}
	return graph + ";[" + current + "]" + tail, nil
}

// keyDropBannerFilter overlays the pre-composited sponsor plate on the named
// content label. The plate PNG already carries the live code.
func keyDropBannerFilter(layout LayoutVariant, banner KeyDropBannerPlan, duration float64, contentLabel string, inputIndex int) (string, error) {
	style, ok := keydropbanner.Lookup(banner.Style)
	if !ok {
		return "", fmt.Errorf("unknown keydrop banner style %q", banner.Style)
	}
	outputHeight := layout.FaceOutputHeight + layout.GameOutputHeight
	positionY := keydropbanner.DefaultPositionY
	if banner.PositionY != nil {
		positionY = *banner.PositionY
	}
	start := 0.0
	if banner.StartSeconds != nil {
		start = *banner.StartSeconds
	}
	end := 0.0
	if banner.EndSeconds != nil {
		end = *banner.EndSeconds
	}
	return keydropbanner.BuildOverlayFilter(keydropbanner.OverlayParams{
		Style:           style,
		OutputWidth:     layout.OutputWidth,
		OutputHeight:    outputHeight,
		PositionY:       positionY,
		SlideEnabled:    banner.SlideEnabled,
		DurationSeconds: duration,
		StartSeconds:    start,
		EndSeconds:      end,
		ContentLabel:    contentLabel,
		OutputLabel:     "keydropped",
		InputIndex:      inputIndex,
	})
}

// streamerBannerFilter builds the strip independently and overlays it on the
// completed layout so the entire banner can move as one unit.
func streamerBannerFilter(layout LayoutVariant, banner StreamerBannerPlan, fontPath string, duration float64) string {
	if layout.Name == VariantStreamerLandscape16x9 {
		return landscapeStreamerBannerFilter(layout, banner, fontPath, duration)
	}
	outputHeight := layout.FaceOutputHeight + layout.GameOutputHeight
	centerY := int(math.Round(layout.DefaultBannerPositionY * float64(outputHeight)))
	if banner.PositionY != nil {
		centerY = int(math.Round(*banner.PositionY * float64(outputHeight)))
	}
	top := centerY - bannerHeight/2
	x := "0"
	if banner.SlideEnabled {
		phase := math.Min(bannerSlideSeconds, duration/2)
		exitStart := duration - phase
		x = fmt.Sprintf(
			`if(lt(t\,%s)\,-w*(1-t/%s)\,if(lt(t\,%s)\,0\,-w*(t-%s)/%s))`,
			floatArg(phase), floatArg(phase), floatArg(exitStart), floatArg(exitStart), floatArg(phase),
		)
	}

	return fmt.Sprintf(
		"color=c=%s:s=%dx%d:r=%d:d=%s,"+
			"setpts=PTS-STARTPTS,"+
			"drawbox=x=0:y=0:w=116:h=%d:color=%s:t=fill,"+
			"drawbox=x=34:y=27:w=48:h=36:color=white:t=fill,"+
			"drawbox=x=41:y=34:w=34:h=22:color=%s:t=fill,"+
			"drawbox=x=43:y=61:w=11:h=9:color=white:t=fill,"+
			"drawbox=x=50:y=38:w=5:h=12:color=white:t=fill,"+
			"drawbox=x=64:y=38:w=5:h=12:color=white:t=fill,"+
			"drawtext=fontfile='%s':text='%s':fontcolor=white:fontsize=52:borderw=1:bordercolor=%s:"+
			"shadowcolor=black@0.35:shadowx=2:shadowy=2:x=140:y=(%d-text_h)/2[banner];"+
			"[content]setpts=PTS-STARTPTS[contentpts];"+
			"[contentpts][banner]overlay=x='%s':y=%d:eval=frame:eof_action=pass:shortest=0[bannered]",
		bannerColor, layout.OutputWidth, bannerHeight, outputFPS, secondsArg(duration),
		bannerHeight, bannerAccentColor,
		bannerAccentColor,
		ffmpegFilterPath(fontPath), ffmpegDrawtextText(banner.Nick), bannerAccentColor, bannerHeight,
		x, top,
	)
}

// landscapeStreamerBannerFilter renders a compact broadcast lower-third. The
// vertical product banner is intentionally full-width because it separates the
// stacked facecam/game bands; reusing it on 16:9 obscures gameplay and HUD.
func landscapeStreamerBannerFilter(layout LayoutVariant, banner StreamerBannerPlan, fontPath string, duration float64) string {
	outputHeight := layout.FaceOutputHeight + layout.GameOutputHeight
	centerY := int(math.Round(layout.DefaultBannerPositionY * float64(outputHeight)))
	if banner.PositionY != nil {
		centerY = int(math.Round(*banner.PositionY * float64(outputHeight)))
	}
	top := centerY - landscapeBannerHeight/2
	x := strconv.Itoa(landscapeBannerX)
	if banner.SlideEnabled {
		phase := math.Min(bannerSlideSeconds, duration/2)
		exitStart := duration - phase
		x = fmt.Sprintf(
			`if(lt(t\,%s)\,%d-w*(1-t/%s)\,if(lt(t\,%s)\,%d\,%d-w*(t-%s)/%s))`,
			floatArg(phase), landscapeBannerX, floatArg(phase), floatArg(exitStart), landscapeBannerX,
			landscapeBannerX, floatArg(exitStart), floatArg(phase),
		)
	}
	return fmt.Sprintf(
		"color=c=0x111319:s=%dx%d:r=%d:d=%s,"+
			"setpts=PTS-STARTPTS,"+
			"drawbox=x=0:y=0:w=8:h=%d:color=%s:t=fill,"+
			"drawtext=fontfile='%s':text='@%s':fontcolor=white:fontsize=32:borderw=1:bordercolor=black@0.55:"+
			"shadowcolor=black@0.45:shadowx=2:shadowy=2:x=28:y=(%d-text_h)/2[banner];"+
			"[content]setpts=PTS-STARTPTS[contentpts];"+
			"[contentpts][banner]overlay=x='%s':y=%d:eval=frame:eof_action=pass:shortest=0[bannered]",
		landscapeBannerWidth, landscapeBannerHeight, outputFPS, secondsArg(duration),
		landscapeBannerHeight, bannerColor,
		ffmpegFilterPath(fontPath), ffmpegDrawtextText(banner.Nick), landscapeBannerHeight,
		x, top,
	)
}

// FindBannerFont prefers the bundled Montserrat ExtraBold used across all
// generated text. System fonts remain an exceptional fallback when the
// embedded font cannot be written to the user cache.
func FindBannerFont() string {
	if fontPath, err := mediafont.Materialize(); err == nil {
		return fontPath
	}
	var candidates []string
	switch runtime.GOOS {
	case "windows":
		windowsDir := os.Getenv("WINDIR")
		if windowsDir == "" {
			windowsDir = `C:\Windows`
		}
		candidates = []string{
			filepath.Join(windowsDir, "Fonts", "arialbd.ttf"),
			filepath.Join(windowsDir, "Fonts", "segoeuib.ttf"),
		}
	case "darwin":
		candidates = []string{
			"/System/Library/Fonts/Supplemental/Arial Bold.ttf",
			"/System/Library/Fonts/Supplemental/Arial.ttf",
		}
	default:
		candidates = []string{
			"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
			"/usr/share/fonts/truetype/liberation2/LiberationSans-Bold.ttf",
		}
	}
	for _, candidate := range candidates {
		// #nosec G703 -- candidates are fixed system-font paths beneath the local OS font root.
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// ffmpegFilterPath escapes a path for embedding in an ffmpeg filtergraph
// string (e.g. drawtext's fontfile). ffmpeg filtergraph syntax always wants
// forward slashes and an escaped drive-letter colon, regardless of the OS
// running this code, so backslashes are normalized unconditionally rather
// than with filepath.ToSlash: ToSlash only rewrites the host OS's own
// separator, which is a no-op for a Windows-style path like `C:\Windows\...`
// on a non-Windows build and left it double-escaped instead of slash-joined.
func ffmpegFilterPath(value string) string {
	value = strings.ReplaceAll(value, `\`, "/")
	value = strings.ReplaceAll(value, ":", `\:`)
	return strings.ReplaceAll(value, "'", `\'`)
}

// ffmpegDrawtextText escapes a literal value embedded in drawtext's text
// option. Streamer handles are validated separately, but keeping this boundary
// safe prevents future plan formats from turning text into filtergraph syntax.
func ffmpegDrawtextText(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`'`, `\'`,
		`:`, `\:`,
		`,`, `\,`,
		`[`, `\[`,
		`]`, `\]`,
		`%`, `\%`,
	)
	return replacer.Replace(value)
}

func cropFilter(c CropRect) string {
	return fmt.Sprintf("crop=w=iw*%s:h=ih*%s:x=iw*%s:y=ih*%s",
		floatArg(c.Width), floatArg(c.Height), floatArg(c.X), floatArg(c.Y))
}

func secondsArg(v float64) string {
	// Keep enough decimal precision to address native media timestamps. Three
	// decimals can move an NTSC frame boundary by a measurable fraction of a
	// frame before the clip bounds even reach the filtergraph.
	return strconv.FormatFloat(v, 'f', 9, 64)
}

func floatArg(v float64) string {
	return strconv.FormatFloat(v, 'f', 6, 64)
}
