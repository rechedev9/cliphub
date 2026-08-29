package editor

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/rechedev9/cliphub/internal/mediafont"
)

// effectColorPattern matches the FFmpeg colour forms accepted from Lua presets:
// a named colour, #RRGGBB, or 0xRRGGBB[AA], each optionally followed by
// @opacity. Anything else — notably ':' ',' '[' ']' ';' or whitespace — is
// rejected so a preset cannot smuggle extra filtergraph clauses or stream
// labels into a drawbox/drawtext colour argument.
var effectColorPattern = regexp.MustCompile(`^(?:[A-Za-z][A-Za-z0-9]*|#[0-9A-Fa-f]{6}|0x[0-9A-Fa-f]{6}(?:[0-9A-Fa-f]{2})?)(?:@[0-9]+(?:\.[0-9]+)?)?$`)

// validateEffectColor rejects colour values that are not a plain FFmpeg colour
// spec. It validates the value exactly as given (callers trim before storing),
// so the validated form is the form that reaches the filtergraph. field is used
// only for the error message.
func validateEffectColor(field, value string) error {
	if !effectColorPattern.MatchString(value) {
		return fmt.Errorf("%s %q is not a valid color", field, value)
	}
	return nil
}

// effectPositionPattern matches the FFmpeg position expressions accepted from
// Lua presets: digits, identifiers (W, w, h, text_w, ...), arithmetic,
// parentheses, dots and spaces. It rejects ':' ',' ';' '[' ']' '=' quotes and
// newlines so a preset cannot smuggle extra filtergraph clauses through an x=/y=
// argument, which is interpolated unescaped into drawtext/overlay filters.
var effectPositionPattern = regexp.MustCompile(`^[A-Za-z0-9_.()+\-*/ ]+$`)

// validateEffectPosition rejects position values that are not a plain numeric or
// FFmpeg expression. field is used only for the error message.
func validateEffectPosition(field, value string) error {
	if !effectPositionPattern.MatchString(value) {
		return fmt.Errorf("%s %q is not a valid position expression", field, value)
	}
	return nil
}

func VideoFilter(short ShortEdit) string {
	width, height := outputDimensions(short)
	if presetUsesFullFrame(short.Preset) {
		return FullFrameVideoFilter(short)
	}
	scaleHeight, singleCrop := motionSingleCropHeight(short, height)
	if scaleHeight == "" {
		// No motion-without-zoom re-staging: fall back to the historical
		// height, keeping a dynamic zoom height when a zoom effect is present.
		scaleHeight = fmt.Sprintf("%d", height)
		if expr := zoomHeightExpression(short.Effects, height); expr != "" {
			scaleHeight = "'" + expr + "'"
		}
	}
	filters := []string{
		scaleFilter(scaleHeight, short),
	}
	if !singleCrop {
		filters = append(filters, fmt.Sprintf("crop=%d:%d:(iw-ow)/2:(ih-oh)/2", width, height))
	}
	filters = append(filters, "setsar=1")
	filters = append(filters, fpsFilter(short))
	filters = appendTemporalSmoothingFilter(filters, short)
	filters = appendEffectFilters(filters, short, singleCrop)
	filters = append(filters, "format=yuv420p")
	return strings.Join(filters, ",")
}

func FullFrameVideoFilter(short ShortEdit) string {
	width, height := outputDimensions(short)
	heightExpr, singleCrop := motionSingleCropHeight(short, height)
	if heightExpr == "" {
		heightExpr = fmt.Sprintf("%d", height)
		if expr := zoomHeightExpression(short.Effects, height); expr != "" {
			heightExpr = "'" + expr + "'"
		}
	}
	filters := []string{
		fullFrameBackgroundScaleFilter(short, heightExpr),
	}
	if !singleCrop {
		filters = append(filters, fmt.Sprintf("crop=%d:%d:(iw-ow)/2:(ih-oh)/2", width, height))
	}
	filters = append(filters, "setsar=1")
	filters = append(filters,
		fpsFilter(short),
	)
	filters = appendTemporalSmoothingFilter(filters, short)
	filters = appendEffectFilters(filters, short, singleCrop)
	filters = append(filters, "format=yuv420p")
	return strings.Join(filters, ",")
}

func imageEffects(effects []Effect) []Effect {
	out := []Effect{}
	for _, effect := range effects {
		if effect.Type == EffectImage {
			out = append(out, effect)
		}
	}
	return out
}

func appendImageOverlayClauses(clauses []string, current string, imageInputStart int, images []Effect, short ShortEdit, outputLabel string) []string {
	for i, effect := range images {
		imageInput := imageInputStart + i
		imageLabel := fmt.Sprintf("img%d", i)
		next := fmt.Sprintf("vimg%d", i)
		if i == len(images)-1 {
			next = outputLabel
		}
		clauses = append(clauses,
			fmt.Sprintf("[%d:v]%s[%s]", imageInput, imageOverlayFilter(effect, short), imageLabel),
			fmt.Sprintf("[%s][%s]overlay=x=%s:y=%s:format=auto:enable='%s'[%s]",
				current,
				imageLabel,
				effectPosition(effect.X, "(W-w)/2"),
				effectPosition(effect.Y, "72"),
				betweenExpression(effect.StartSeconds, effect.EndSeconds),
				next,
			),
		)
		current = next
	}
	return clauses
}

func killfeedEffects(effects []Effect) []Effect {
	out := []Effect{}
	for _, effect := range effects {
		if effect.Type == EffectKillfeed {
			out = append(out, effect)
		}
	}
	return out
}

func imageOverlayFilter(effect Effect, short ShortEdit) string {
	filters := []string{
		"format=rgba",
		imageScaleFilter(effect),
	}
	if hasEffectFade(effect) {
		duration := short.DurationSeconds
		if duration <= 0 {
			duration = effect.EndSeconds
		}
		filters = append(filters,
			"loop=loop=-1:size=1:start=0",
			fmt.Sprintf("setpts=N/%d/TB", outputFPS(short)),
		)
		if duration > 0 {
			filters = append(filters, fmt.Sprintf("trim=duration=%.3f", duration))
		}
		filters = append(filters, overlayFadeFilters(effect)...)
	}
	return strings.Join(filters, ",")
}

func imageScaleFilter(effect Effect) string {
	switch {
	case effect.Width > 0 && effect.Height > 0:
		return fmt.Sprintf("scale=w=%d:h=%d:flags=lanczos", effect.Width, effect.Height)
	case effect.Width > 0:
		return fmt.Sprintf("scale=w=%d:h=-1:flags=lanczos", effect.Width)
	case effect.Height > 0:
		return fmt.Sprintf("scale=w=-1:h=%d:flags=lanczos", effect.Height)
	default:
		return "scale=w=760:h=-1:flags=lanczos"
	}
}

func sourceCropScaleFilter(effect Effect) string {
	switch {
	case effect.Width > 0 && effect.Height > 0:
		return fmt.Sprintf("scale=w=%d:h=%d:flags=lanczos", effect.Width, effect.Height)
	case effect.Width > 0:
		return fmt.Sprintf("scale=w=%d:h=-1:flags=lanczos", effect.Width)
	case effect.Height > 0:
		return fmt.Sprintf("scale=w=-1:h=%d:flags=lanczos", effect.Height)
	default:
		return "scale=w=430:h=-1:flags=lanczos"
	}
}

func overlayFadeFilters(effect Effect) []string {
	fadeIn, fadeOut := normalizedFadeDurations(effect)
	filters := []string{}
	if fadeIn > 0 {
		filters = append(filters, fmt.Sprintf("fade=t=in:st=%.3f:d=%.3f:alpha=1", effect.StartSeconds, fadeIn))
	}
	if fadeOut > 0 {
		filters = append(filters, fmt.Sprintf("fade=t=out:st=%.3f:d=%.3f:alpha=1", effect.EndSeconds-fadeOut, fadeOut))
	}
	return filters
}

func hasEffectFade(effect Effect) bool {
	return effect.FadeInSeconds > 0 || effect.FadeOutSeconds > 0
}

func normalizedFadeDurations(effect Effect) (float64, float64) {
	fadeIn := effect.FadeInSeconds
	fadeOut := effect.FadeOutSeconds
	if fadeIn < 0 {
		fadeIn = 0
	}
	if fadeOut < 0 {
		fadeOut = 0
	}
	duration := effect.EndSeconds - effect.StartSeconds
	if duration <= 0 || fadeIn+fadeOut <= duration {
		return fadeIn, fadeOut
	}
	scale := duration / (fadeIn + fadeOut)
	return fadeIn * scale, fadeOut * scale
}

func effectPosition(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func scaleFilter(height string, short ShortEdit) string {
	filter := fmt.Sprintf("scale=w=-2:h=%s:%s", height, scaleEvalMode(height))
	if short.HQFilters {
		filter += ":flags=" + hqScaleFlags(short)
	}
	return filter
}

// scaleEvalMode selects how often FFmpeg evaluates the scale geometry. A plain
// integer height is constant across the timeline, so eval=init lets FFmpeg
// compute the scaled size once and reuse it; a zoom-driven height expression
// depends on t and must be re-evaluated for every frame (eval=frame).
func scaleEvalMode(height string) string {
	if isPlainIntegerHeight(height) {
		return "eval=init"
	}
	return "eval=frame"
}

// isPlainIntegerHeight reports whether a scale height argument is a constant
// integer (e.g. "1920") rather than a quoted dynamic expression ("'if(…)'").
func isPlainIntegerHeight(height string) bool {
	if height == "" {
		return false
	}
	for _, r := range height {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func fpsFilter(short ShortEdit) string {
	return fmt.Sprintf("fps=%d", outputFPS(short))
}

func outputFPS(short ShortEdit) int {
	if short.OutputFPS > 0 {
		return short.OutputFPS
	}
	return 60
}

func fullFrameBackgroundScaleFilter(short ShortEdit, heightExpr string) string {
	width, _ := outputDimensions(short)
	filter := fmt.Sprintf("scale=w=%d:h=%s:force_original_aspect_ratio=increase:%s", width, heightExpr, scaleEvalMode(heightExpr))
	if short.HQFilters {
		filter += ":flags=" + hqScaleFlags(short)
	}
	return filter
}

func hqScaleFlags(short ShortEdit) string {
	return "lanczos+accurate_rnd+full_chroma_int"
}

func appendTemporalSmoothingFilter(filters []string, short ShortEdit) []string {
	if !short.TemporalSmoothing {
		return filters
	}
	return append(filters, "tmix=frames=2:weights='1 2'")
}

func zoomHeightExpression(effects []Effect, baseHeight int) string {
	return zoomHeightExpressionForBase(effects, float64(baseHeight))
}

func zoomHeightExpressionForBase(effects []Effect, baseHeight float64) string {
	var terms []string
	for _, effect := range effects {
		if effect.Type != EffectZoom || effect.Scale <= 1 {
			continue
		}
		terms = append(terms, smoothZoomHeightExpressionForBase(effect, baseHeight))
	}
	if len(terms) == 0 {
		return ""
	}
	combined := terms[0]
	for _, term := range terms[1:] {
		combined = fmt.Sprintf("max(%s\\,%s)", combined, term)
	}
	return combined
}

func smoothZoomHeightExpressionForBase(effect Effect, baseHeight float64) string {
	start := effect.StartSeconds
	end := effect.EndSeconds
	at := effect.AtSeconds
	if at <= start || at >= end {
		at = start + (end-start)/2
	}
	if at <= start || end <= at {
		height := int(math.Round(baseHeight * effect.Scale))
		return fmt.Sprintf("if(%s\\,%d\\,%d)", betweenExpression(start, end), height, int(math.Round(baseHeight)))
	}
	peak := baseHeight * effect.Scale
	rise := smoothZoomRampExpression(start, at, baseHeight, peak)
	fall := smoothZoomRampExpression(at, end, peak, baseHeight)
	return fmt.Sprintf(
		"if(%s\\,%s\\,if(%s\\,%s\\,%d))",
		betweenExpression(start, at),
		rise,
		betweenExpression(at, end),
		fall,
		int(math.Round(baseHeight)),
	)
}

func smoothZoomRampExpression(start, end, from, to float64) string {
	duration := end - start
	if duration <= 0 {
		return fmt.Sprintf("%.3f", to)
	}
	// Smoothstep avoids a visible scale step at the beginning and end of a
	// scripted zoom while keeping the Lua API compact.
	t := fmt.Sprintf("((t-%.3f)/%.3f)", start, duration)
	return fmt.Sprintf("(%.3f+(%.3f-%.3f)*(%s*%s*(3-2*%s)))", from, to, from, t, t, t)
}

func appendEffectFilters(filters []string, short ShortEdit, singleCrop bool) []string {
	effects := short.Effects
	filters = append(filters, gradeFilters(effects)...)
	filters = appendMotionCropFilters(filters, effects, short, singleCrop)
	filters = append(filters, chromaShiftFilters(effects)...)
	for _, effect := range effects {
		if effect.Type != EffectFlash {
			continue
		}
		color := effect.Color
		if color == "" {
			color = "white"
		}
		if converted := ffmpegColor(color); converted != "" {
			color = converted
		}
		opacity := effect.Opacity
		if opacity == 0 {
			opacity = 0.18
		}
		filters = append(filters, fmt.Sprintf(
			"drawbox=x=0:y=0:w=iw:h=ih:color=%s@%.3f:t=fill:enable='%s'",
			color,
			opacity,
			betweenExpression(effect.StartSeconds, effect.EndSeconds),
		))
	}
	for _, effect := range effects {
		if effect.Type != EffectText {
			continue
		}
		x := effect.X
		if x == "" {
			x = "48"
		}
		y := effect.Y
		if y == "" {
			y = "72"
		}
		size := effect.Size
		if size == 0 {
			size = 32
		}
		fontColor := effect.FontColor
		if fontColor == "" {
			fontColor = "white@0.92"
		}
		boxColor := effect.BoxColor
		if boxColor == "" {
			boxColor = "black@0.36"
		}
		boxBorder := effect.BoxBorder
		if boxBorder == 0 {
			boxBorder = 12
		}
		styled := effect
		styled.X = x
		styled.Y = y
		styled.Size = size
		styled.FontColor = fontColor
		styled.BoxColor = boxColor
		styled.BoxBorder = boxBorder
		filters = append(filters, drawTextEffect(styled))
	}
	return filters
}

func effectAmplitude(effect Effect, fallback float64) float64 {
	if effect.Amplitude > 0 {
		return effect.Amplitude
	}
	return fallback
}

func effectFrequency(effect Effect, fallback float64) float64 {
	if effect.Frequency > 0 {
		return effect.Frequency
	}
	return fallback
}

func motionPadAmplitude(effects []Effect) int {
	pad := 0.0
	for _, effect := range effects {
		switch effect.Type {
		case EffectShake:
			pad += effectAmplitude(effect, 14)
		case EffectGlitch:
			pad += effectAmplitude(effect, 10)
		}
	}
	return int(math.Ceil(pad))
}

// motionCropEnabled reports whether the motion crop chain will actually emit:
// it needs at least one shake/glitch effect with offset expressions on both
// the x and y axes.
func motionCropEnabled(effects []Effect) bool {
	if motionPadAmplitude(effects) <= 0 {
		return false
	}
	return motionOffsetExpression(effects, "x") != "" && motionOffsetExpression(effects, "y") != ""
}

// motionSingleCropHeight returns the scale height (and the single-crop flag)
// when the frame should be staged with a motion margin so the shake/glitch
// window can pan without a second resample. That is only possible when motion
// is active and there is no zoom: a dynamic zoom height (eval=frame) sizes the
// frame per frame, so it cannot carry a fixed margin and must keep the legacy
// crop+rescale chain in appendMotionCropFilters. The scaled height is the
// final output height plus the 2*pad margin the pan range needs.
func motionSingleCropHeight(short ShortEdit, height int) (string, bool) {
	if !motionCropEnabled(short.Effects) || zoomHeightExpression(short.Effects, height) != "" {
		return "", false
	}
	return fmt.Sprintf("%d", height+2*motionPadAmplitude(short.Effects)), true
}

func shakeOffsetTerm(effect Effect, axis string) string {
	amp := effectAmplitude(effect, 14)
	freq := effectFrequency(effect, 16)
	phase := 0.0
	if axis == "y" {
		freq *= 0.73
		phase = 1.885
	}
	start := effect.StartSeconds
	end := effect.EndSeconds
	at := effect.AtSeconds
	if at <= start || at >= end {
		at = start + (end-start)/2
	}
	rise := at - start
	if rise < 0.001 {
		rise = 0.001
	}
	fall := end - at
	if fall < 0.001 {
		fall = 0.001
	}
	env := fmt.Sprintf("if(lt(t\\,%.3f)\\,((t-%.3f)/%.3f)\\,max(0\\,(1-((t-%.3f)/%.3f))))", at, start, rise, at, fall)
	return fmt.Sprintf("if(%s\\,%.3f*(%s)*sin(6.283185*%.3f*(t-%.3f)+%.3f)\\,0)", betweenExpression(start, end), amp, env, freq, start, phase)
}

func glitchOffsetTerm(effect Effect, axis string) string {
	amp := effectAmplitude(effect, 10)
	rate := 30.0
	if axis == "y" {
		rate = 20
		amp *= 0.6
	}
	return fmt.Sprintf(
		"if(%s\\,%.3f*if(eq(mod(floor((t-%.3f)*%.3f)\\,2)\\,0)\\,1\\,-1)\\,0)",
		betweenExpression(effect.StartSeconds, effect.EndSeconds),
		amp,
		effect.StartSeconds,
		rate,
	)
}

func motionOffsetExpression(effects []Effect, axis string) string {
	var terms []string
	for _, effect := range effects {
		switch effect.Type {
		case EffectShake:
			terms = append(terms, shakeOffsetTerm(effect, axis))
		case EffectGlitch:
			terms = append(terms, glitchOffsetTerm(effect, axis))
		}
	}
	if len(terms) == 0 {
		return ""
	}
	combined := terms[0]
	for _, term := range terms[1:] {
		combined = fmt.Sprintf("(%s+%s)", combined, term)
	}
	return combined
}

// motionClampedOffset keeps the historical shake/glitch clamp: the window pans
// within [0, 2*pad], centred on pad, so a neutral offset sits exactly in the
// middle of the range.
func motionClampedOffset(pad int, offset string) string {
	return fmt.Sprintf("max(0\\,min(%d\\,%d+(%s)))", 2*pad, pad, offset)
}

// appendMotionCropFilters adds the shake/glitch pan chain after grading.
//
// singleCrop is the no-zoom path where VideoFilter already scaled the frame to
// height+2*pad (see motionSingleCropHeight): one dynamic crop then selects the
// final output window directly, skipping the second resample that the legacy
// chain paid for sharpness. The window base is centred — ((iw-ow)/2, (ih-oh)/2)
// with the pad offset folded in, which collapses to 0 on the axis that carries
// exactly the 2*pad margin — and the outer clamp keeps the crop inside the
// frame on axes where the source aspect leaves less margin (e.g. an already
// portrait source). Amplitude and pan range are the same as before; only the
// geometry differs.
func appendMotionCropFilters(filters []string, effects []Effect, short ShortEdit, singleCrop bool) []string {
	pad := motionPadAmplitude(effects)
	if pad <= 0 {
		return filters
	}
	offsetX := motionOffsetExpression(effects, "x")
	offsetY := motionOffsetExpression(effects, "y")
	if offsetX == "" || offsetY == "" {
		return filters
	}
	width, height := outputDimensions(short)
	x := motionClampedOffset(pad, offsetX)
	y := motionClampedOffset(pad, offsetY)
	if singleCrop {
		baseX := fmt.Sprintf("((iw-ow)/2)-%d+(%s)", pad, x)
		baseY := fmt.Sprintf("((ih-oh)/2)-%d+(%s)", pad, y)
		cropX := fmt.Sprintf("max(0\\,min(iw-%d\\,%s))", width, baseX)
		cropY := fmt.Sprintf("max(0\\,min(ih-%d\\,%s))", height, baseY)
		return append(filters, fmt.Sprintf("crop=%d:%d:x='%s':y='%s'", width, height, cropX, cropY))
	}
	// Legacy zoom+motion (or post-concat) chain: the frame is the final output
	// size, so the shake/glitch window must crop inside it and resample back
	// to the output geometry. A dynamic zoom scale cannot carry a fixed
	// margin, and a compiled short's concat output is already conformed.
	return append(filters,
		fmt.Sprintf("crop=w=iw-%d:h=ih-%d:x='%s':y='%s'", 2*pad, 2*pad, x, y),
		fmt.Sprintf("scale=%d:%d:flags=lanczos", width, height),
	)
}

func chromaShiftFilters(effects []Effect) []string {
	filters := []string{}
	for _, effect := range effects {
		shift := 0
		switch effect.Type {
		case EffectChroma:
			shift = int(math.Round(effectAmplitude(effect, 8)))
		case EffectGlitch:
			shift = int(math.Round(effectAmplitude(effect, 10)))
		default:
			continue
		}
		if shift < 1 {
			shift = 1
		}
		filters = append(filters, fmt.Sprintf(
			"chromashift=cbh=%d:crh=%d:enable='%s'",
			shift,
			-shift,
			betweenExpression(effect.StartSeconds, effect.EndSeconds),
		))
	}
	return filters
}

func gradeFilters(effects []Effect) []string {
	filters := []string{}
	for _, effect := range effects {
		if effect.Type != EffectGrade {
			continue
		}
		contrast := effect.Contrast
		if contrast == 0 {
			contrast = 1
		}
		saturation := effect.Saturation
		if saturation == 0 {
			saturation = 1
		}
		gamma := effect.Gamma
		if gamma == 0 {
			gamma = 1
		}
		filters = append(filters, fmt.Sprintf("eq=contrast=%.3f:saturation=%.3f:gamma=%.3f", contrast, saturation, gamma))
	}
	return filters
}

// drawTextEffect renders a text effect as a drawtext filter. BoxColor "none"
// disables the backing box entirely; a non-empty ShadowColor adds a drop
// shadow at the ShadowX/ShadowY offsets.
func drawTextEffect(effect Effect) string {
	fontOption := ""
	fontFile := strings.TrimSpace(effect.FontFile)
	if fontFile == "" {
		if effect.Bold {
			fontFile = boldDrawtextFontFile()
		} else {
			fontFile = drawtextFontFile()
		}
	}
	if fontFile != "" {
		fontOption = fmt.Sprintf(":fontfile='%s'", escapeDrawtextOption(filepath.ToSlash(fontFile)))
	}
	boxOption := "box=0"
	if effect.BoxColor != "none" {
		boxOption = fmt.Sprintf("box=1:boxcolor=%s:boxborderw=%d", effect.BoxColor, effect.BoxBorder)
	}
	shadowOption := ""
	if effect.ShadowColor != "" {
		shadowOption = fmt.Sprintf(":shadowcolor=%s:shadowx=%d:shadowy=%d", effect.ShadowColor, effect.ShadowX, effect.ShadowY)
	}
	borderOption := ""
	if effect.BorderWidth > 0 {
		borderColor := strings.TrimSpace(effect.BorderColor)
		if borderColor == "" {
			borderColor = "black@0.9"
		}
		borderOption = fmt.Sprintf(":borderw=%d:bordercolor=%s", effect.BorderWidth, borderColor)
	}
	alphaOption := ""
	if alpha := textAlphaExpression(effect.StartSeconds, effect.EndSeconds, effect.FadeInSeconds, effect.FadeOutSeconds); alpha != "" {
		alphaOption = fmt.Sprintf(":alpha='%s'", alpha)
	}
	return fmt.Sprintf(
		"drawtext=text='%s'%s:x=%s:y=%s:fontsize=%d:fontcolor=%s:%s%s%s%s:enable='%s'",
		escapeDrawtextText(effect.Value),
		fontOption,
		effect.X,
		effect.Y,
		effect.Size,
		effect.FontColor,
		boxOption,
		shadowOption,
		borderOption,
		alphaOption,
		betweenExpression(effect.StartSeconds, effect.EndSeconds),
	)
}

func textAlphaExpression(start, end, fadeIn, fadeOut float64) string {
	effect := Effect{
		StartSeconds:   start,
		EndSeconds:     end,
		FadeInSeconds:  fadeIn,
		FadeOutSeconds: fadeOut,
	}
	fadeIn, fadeOut = normalizedFadeDurations(effect)
	if fadeIn <= 0 && fadeOut <= 0 {
		return ""
	}
	expr := "1"
	if fadeIn > 0 {
		expr = fmt.Sprintf("min(%s\\,((t-%.3f)/%.3f))", expr, start, fadeIn)
	}
	if fadeOut > 0 {
		expr = fmt.Sprintf("min(%s\\,((%.3f-t)/%.3f))", expr, end, fadeOut)
	}
	return expr
}

func ffmpegColor(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "#") && len(raw) == 7 {
		return "0x" + raw[1:]
	}
	if strings.HasPrefix(raw, "0x") && len(raw) == 8 {
		return raw
	}
	switch strings.ToLower(raw) {
	case "black":
		return "0x000000"
	case "white":
		return "0xffffff"
	case "green":
		return "0x00ff00"
	case "magenta":
		return "0xff00ff"
	case "cyan":
		return "0x00ffff"
	default:
		return raw
	}
}

// drawtextFontFile resolves the embedded media font once per process. If cache
// materialization fails, defaultDrawtextFontFile retains the prior system-font
// fallback without repeating filesystem probes for every drawtext clause.
var drawtextFontFile = sync.OnceValue(defaultDrawtextFontFile)

func defaultDrawtextFontFile() string {
	if fontPath, err := mediafont.Materialize(); err == nil {
		return fontPath
	}
	if runtime.GOOS != "windows" {
		return ""
	}
	for _, candidate := range []string{
		filepath.Join(`C:\Windows`, "Fonts", "arial.ttf"),
		filepath.Join(`C:\Windows`, "Fonts", "segoeui.ttf"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// boldDrawtextFontFile resolves the same deterministic font for viral-style
// titles. It degrades gracefully to the prior bold system candidates if the
// embedded font cannot be materialized.
var boldDrawtextFontFile = sync.OnceValue(defaultBoldDrawtextFontFile)

func defaultBoldDrawtextFontFile() string {
	if fontPath, err := mediafont.Materialize(); err == nil {
		return fontPath
	}
	if runtime.GOOS != "windows" {
		return drawtextFontFile()
	}
	for _, candidate := range []string{
		filepath.Join(`C:\Windows`, "Fonts", "ariblk.ttf"),  // Arial Black
		filepath.Join(`C:\Windows`, "Fonts", "arialbd.ttf"), // Arial Bold
		filepath.Join(`C:\Windows`, "Fonts", "seguisb.ttf"), // Segoe UI Semibold
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return drawtextFontFile()
}

func betweenExpression(start, end float64) string {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	return fmt.Sprintf("between(t\\,%.3f\\,%.3f)", start, end)
}

func escapeDrawtextText(text string) string {
	return escapeDrawtextOption(text)
}

func escapeDrawtextOption(text string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`'`, `\'`,
		`:`, `\:`,
		`,`, `\,`,
		`[`, `\[`,
		`]`, `\]`,
		`%`, `\%`,
	)
	return replacer.Replace(text)
}

func outputDimensions(short ShortEdit) (int, int) {
	if isLandscapeOutput(short) {
		return 1920, 1080
	}
	return 1080, 1920
}

func isLandscapeOutput(short ShortEdit) bool {
	return short.OutputFormat == OutputFormatLandscape16x9
}

func validateEffectFontFile(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if strings.ContainsAny(value, "\r\n;") {
		return fmt.Errorf("fontfile contains unsupported characters")
	}
	return nil
}
