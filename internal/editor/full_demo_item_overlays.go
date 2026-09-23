package editor

import (
	"fmt"
	"math"
	"strings"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

// supportedFullDemoOverlaySources is the complete set of global effects the
// per-item composition can project. Anything else keeps the legacy post-concat
// re-encode path (see fullDemoProgramUsesItemOverlays).
var supportedFullDemoOverlaySources = map[string]bool{
	"full-demo-intro": true,
	"full-demo-outro": true,
}

// fullDemoItemOverlayEligible reports whether item encodes can carry the global
// overlays. It is a pure function of the approved short, so the program command
// and every item command make the same decision even though the program command
// is built first.
//
// The per-item graph reuses appendCompilationProgramVideo unchanged (see
// fullDemoItemVideoClauses), so effect order and the legacy window clamping are
// preserved exactly. This gate only rejects combinations that graph cannot
// express per item: non-image effects, the program-global cover freeze, a
// missing still, and non-finite windows.
func fullDemoItemOverlayEligible(short ShortEdit) bool {
	if short.FullDemo == nil || !isFullDemoNative(short.Preset, short.OutputFormat, len(short.Parts) > 0) {
		return false
	}
	// The cover first-frame freeze is an n-based clause over the whole program;
	// it has no item-local form, so it must keep the legacy global pass.
	if short.CoverFirstFrame {
		return false
	}
	for i := range short.Effects {
		effect := short.Effects[i]
		if effect.Type != EffectImage || !supportedFullDemoOverlaySources[effect.Source] {
			return false
		}
		if strings.TrimSpace(effect.Path) == "" {
			return false
		}
		if math.IsNaN(effect.StartSeconds) || math.IsNaN(effect.EndSeconds) ||
			math.IsInf(effect.StartSeconds, 0) || math.IsInf(effect.EndSeconds, 0) {
			return false
		}
	}
	return true
}

// fullDemoProgramUsesItemOverlays is the copy-path gate. It additionally
// requires prepared items: the concat list written before preparation still
// lists raw parts, and copying those would silently drop the overlays.
func fullDemoProgramUsesItemOverlays(short ShortEdit) bool {
	if short.fullDemo == nil || len(short.fullDemo.preparedInputs) == 0 {
		return false
	}
	return fullDemoItemOverlayEligible(short)
}

// Most rounds lie entirely between the intro and outro. They do not need to
// decode and animate those stills again. Keep a conservative millisecond around
// each inclusive window: betweenExpression rounds its bounds to milliseconds,
// while item PTS are exact 1/60 frame indices.
func fullDemoItemOverlaysInactive(short ShortEdit, item recapplan.TimelineItem) bool {
	if !fullDemoItemOverlayEligible(short) || len(short.Effects) == 0 || item.StartFrame < 0 || item.EndFrame <= item.StartFrame {
		return false
	}
	first := float64(item.StartFrame) / recapplan.OutputFPS
	last := float64(item.EndFrame-1) / recapplan.OutputFPS
	for _, effect := range short.Effects {
		start := max(0, effect.StartSeconds)
		end := max(start, effect.EndSeconds)
		if start-.001 <= last && end+.001 >= first {
			return false
		}
	}
	return true
}

// fullDemoItemVideoClauses builds the item's video filter clauses.
//
// videoBase is the unlabeled item chain after transitions and custom HUD, in
// item-local time. When the short is eligible and has image effects, the base is
// shifted onto the global frame clock, the original appendCompilationProgramVideo
// graph runs unchanged (identical expressions to the legacy whole-program pass),
// and the output is shifted back so the encoder and concat still see item-local
// PTS. images must be imageEffects(short.Effects) so the inputs the caller adds
// match the indices the graph expects; imageInputStart is the FFmpeg input index
// of the first image.
func fullDemoItemVideoClauses(short ShortEdit, item recapplan.TimelineItem, videoBase string, images []Effect, imageInputStart int) ([]string, string) {
	if fullDemoItemOverlaysInactive(short, item) {
		// overlay=format=auto negotiates RGBA even while every overlay is
		// disabled. Preserve that round trip, including its chroma rounding and
		// color range; simply removing the overlays changes inactive pixels.
		return []string{videoBase + ",format=rgba,format=yuv420p,settb=expr=1/60,setpts=N[vout]"}, "[vout]"
	}
	if !fullDemoItemOverlayEligible(short) || len(images) == 0 {
		return []string{videoBase + "[v]"}, "[v]"
	}
	clauses := []string{videoBase + "[vlocal]"}
	// Canonical integer clock. settb=expr=1/60 makes every PTS an exact integer
	// frame count, and setpts=N+StartFrame places item frame N at its global
	// frame index. The float form "+StartFrame/60/TB" is NOT exact: FFmpeg
	// evaluates it through TB and rounds down at some starts (start 123 emits
	// first PTS 122, start 245 -> 244), which shifts every animated overlay by
	// one frame. The local reset after the graph is the same canonical clock,
	// not a float STARTPTS subtraction.
	clauses = append(clauses, fmt.Sprintf("[vlocal]settb=expr=1/60,setpts=N+%d[vg0]", item.StartFrame))
	clauses = appendCompilationProgramVideo(clauses, short, "vg0", imageInputStart)
	clauses = append(clauses, "[v]settb=expr=1/60,setpts=N[vout]")
	return clauses, "[vout]"
}

// fullDemoInputCount counts the -i inputs already appended to an item command,
// so overlay image inputs can be referenced by their real FFmpeg indices.
func fullDemoInputCount(command []string) int {
	count := 0
	for _, arg := range command {
		if arg == "-i" {
			count++
		}
	}
	return count
}
