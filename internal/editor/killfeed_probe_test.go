package editor

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"testing"
)

// killfeedTestFrame draws a CS2-style highlighted kill notice in the
// top-right quadrant of a 1920x1080 frame: a saturated red border with a 1px
// dimmer anti-aliased ring just outside it, the way the game renders it.
func killfeedTestFrame(t *testing.T, notice image.Rectangle) image.Image {
	t.Helper()
	frame := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	drawKillfeedNotice(frame, notice)
	return frame
}

// drawKillfeedNotice paints a single CS2-style highlighted kill notice (a 2px
// saturated-red border ring around a dimmer anti-aliased fill) onto frame.
func drawKillfeedNotice(frame *image.RGBA, notice image.Rectangle) {
	dim := color.RGBA{R: 130, G: 45, B: 45, A: 255}
	for y := notice.Min.Y; y < notice.Max.Y; y++ {
		for x := notice.Min.X; x < notice.Max.X; x++ {
			frame.Set(x, y, dim)
		}
	}
	red := color.RGBA{R: 200, G: 30, B: 30, A: 255}
	inner := notice.Inset(1)
	for x := inner.Min.X; x < inner.Max.X; x++ {
		for d := 0; d < 2; d++ {
			frame.Set(x, inner.Min.Y+d, red)
			frame.Set(x, inner.Max.Y-1-d, red)
		}
	}
	for y := inner.Min.Y; y < inner.Max.Y; y++ {
		for d := 0; d < 2; d++ {
			frame.Set(inner.Min.X+d, y, red)
			frame.Set(inner.Max.X-1-d, y, red)
		}
	}
}

// fillSolidRed paints a solid saturated-red block, simulating red scene
// geometry (a wall or container) that passes the strict red threshold but is
// not a thin notice ring.
func fillSolidRed(frame *image.RGBA, block image.Rectangle) {
	wall := color.RGBA{R: 200, G: 30, B: 30, A: 255}
	for y := block.Min.Y; y < block.Max.Y; y++ {
		for x := block.Min.X; x < block.Max.X; x++ {
			frame.Set(x, y, wall)
		}
	}
}

func TestDetectKillfeedHighlight(t *testing.T) {
	notice := image.Rect(1700, 115, 1910, 152)
	rect, ok := detectKillfeedHighlight(killfeedTestFrame(t, notice))
	if !ok {
		t.Fatal("detectKillfeedHighlight ok = false, want true")
	}
	if rect.Min.X > notice.Min.X || rect.Min.Y > notice.Min.Y || rect.Max.X < notice.Max.X || rect.Max.Y < notice.Max.Y {
		t.Fatalf("rect = %v, want it to cover the full anti-aliased notice %v", rect, notice)
	}
	if rect.Min.X < notice.Min.X-2 || rect.Min.Y < notice.Min.Y-2 || rect.Max.X > notice.Max.X+2 || rect.Max.Y > notice.Max.Y+2 {
		t.Fatalf("rect = %v, want at most %dpx beyond notice %v", rect, killfeedHighlightMargin, notice)
	}

	if _, ok := detectKillfeedHighlight(image.NewRGBA(image.Rect(0, 0, 1920, 1080))); ok {
		t.Fatal("detectKillfeedHighlight on empty frame ok = true, want false")
	}
}

func TestDetectKillfeedHighlightIgnoresDistantDimRed(t *testing.T) {
	notice := image.Rect(1700, 70, 1910, 106)
	frame := killfeedTestFrame(t, notice).(*image.RGBA)
	// dim red scene noise (an explosion glow) far below the notice must not
	// stretch the detected box; only the local anti-aliased ring counts
	dim := color.RGBA{R: 130, G: 45, B: 45, A: 255}
	for y := 160; y < 200; y++ {
		for x := 1600; x < 1700; x++ {
			frame.Set(x, y, dim)
		}
	}
	rect, ok := detectKillfeedHighlight(frame)
	if !ok {
		t.Fatal("detectKillfeedHighlight ok = false, want true")
	}
	if rect.Max.Y > notice.Max.Y+2 {
		t.Fatalf("rect = %v, want it to ignore dim red noise below notice %v", rect, notice)
	}
}

func TestDetectKillfeedHighlightIgnoresSolidRedScene(t *testing.T) {
	notice := image.Rect(1700, 115, 1910, 152)
	frame := killfeedTestFrame(t, notice).(*image.RGBA)
	// a solid saturated-red wall inside the scan region but away from the
	// notice must not be unioned into the crop: it is a tall solid blob, not a
	// thin notice ring.
	fillSolidRed(frame, image.Rect(1250, 0, 1500, 324))
	rect, ok := detectKillfeedHighlight(frame)
	if !ok {
		t.Fatal("detectKillfeedHighlight ok = false, want true")
	}
	if rect.Min.X > notice.Min.X || rect.Min.Y > notice.Min.Y || rect.Max.X < notice.Max.X || rect.Max.Y < notice.Max.Y {
		t.Fatalf("rect = %v, want it to cover the full notice %v", rect, notice)
	}
	if rect.Min.X < 1690 {
		t.Fatalf("rect = %v, want it not to stretch into the red wall (Min.X >= 1690)", rect)
	}
	if rect.Min.X < notice.Min.X-2 || rect.Min.Y < notice.Min.Y-2 || rect.Max.X > notice.Max.X+2 || rect.Max.Y > notice.Max.Y+2 {
		t.Fatalf("rect = %v, want at most %dpx beyond notice %v", rect, killfeedHighlightMargin, notice)
	}
}

func TestDetectKillfeedHighlightRejectsSceneOnlyRed(t *testing.T) {
	frame := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	// only a solid red wall in the scan region, no kill notice at all.
	fillSolidRed(frame, image.Rect(1250, 0, 1500, 324))
	if _, ok := detectKillfeedHighlight(frame); ok {
		t.Fatal("detectKillfeedHighlight on scene-only red ok = true, want false")
	}
}

func TestDetectKillfeedHighlightCoversStackedNotices(t *testing.T) {
	top := image.Rect(1700, 70, 1910, 106)
	bottom := image.Rect(1690, 115, 1910, 151)
	frame := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	drawKillfeedNotice(frame, top)
	drawKillfeedNotice(frame, bottom)
	rect, ok := detectKillfeedHighlight(frame)
	if !ok {
		t.Fatal("detectKillfeedHighlight ok = false, want true")
	}
	both := top.Union(bottom)
	if rect.Min.X > both.Min.X || rect.Min.Y > both.Min.Y || rect.Max.X < both.Max.X || rect.Max.Y < both.Max.Y {
		t.Fatalf("rect = %v, want it to cover both stacked notices %v", rect, both)
	}
}

func TestRefineKillfeedEffectsMeasuresCropPerKill(t *testing.T) {
	notice := image.Rect(1690, 196, 1910, 232)
	var gotInput string
	var gotAt float64
	probe := func(input string, atSeconds float64) (image.Image, error) {
		gotInput = input
		gotAt = atSeconds
		return killfeedTestFrame(t, notice), nil
	}

	short := ShortEdit{
		DurationSeconds: 12,
		Effects: []Effect{
			{
				Type:         EffectKillfeed,
				StartSeconds: 9.5,
				EndSeconds:   12,
				AtSeconds:    9.55,
				Width:        430,
				CropX:        1558,
				CropY:        64,
				CropWidth:    360,
				CropHeight:   110,
			},
		},
		Parts: []ShortPart{
			{SegmentID: "seg-001", Input: "seg-001.mp4", DurationSeconds: 6, TimelineStartSeconds: 0},
			{SegmentID: "seg-002", Input: "seg-002.mp4", DurationSeconds: 6, TimelineStartSeconds: 6},
		},
	}

	warnings := refineKillfeedEffects(&short, probe)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	if gotInput != "seg-002.mp4" {
		t.Fatalf("probe input = %q, want seg-002.mp4", gotInput)
	}
	if want := 9.55 - 6 + killfeedSampleDelaySeconds; math.Abs(gotAt-want) > 1e-9 {
		t.Fatalf("probe at = %.3f, want %.3f", gotAt, want)
	}
	effect := short.Effects[0]
	crop := image.Rect(effect.CropX, effect.CropY, effect.CropX+effect.CropWidth, effect.CropY+effect.CropHeight)
	if crop.Min.X > notice.Min.X || crop.Min.Y > notice.Min.Y || crop.Max.X < notice.Max.X || crop.Max.Y < notice.Max.Y {
		t.Fatalf("crop = %v, want it to cover notice %v", crop, notice)
	}
	if effect.CropHeight > notice.Dy()+16 {
		t.Fatalf("crop height = %d, want tight fit around %d", effect.CropHeight, notice.Dy())
	}
	wantWidth := int(float64(effect.CropWidth)*killfeedOverlayScale + 0.5)
	if effect.Width != wantWidth {
		t.Fatalf("overlay width = %d, want %d (crop width scaled)", effect.Width, wantWidth)
	}
}

func TestRefineKillfeedEffectsKeepsDefaultsOnFailure(t *testing.T) {
	tests := []struct {
		name  string
		probe func(input string, atSeconds float64) (image.Image, error)
		want  string
	}{
		{
			name: "probe error",
			probe: func(string, float64) (image.Image, error) {
				return nil, fmt.Errorf("boom")
			},
			want: "boom",
		},
		{
			name: "no highlight detected",
			probe: func(string, float64) (image.Image, error) {
				return image.NewRGBA(image.Rect(0, 0, 1920, 1080)), nil
			},
			want: "no highlighted kill notice",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			short := ShortEdit{
				DurationSeconds: 12,
				Effects: []Effect{
					{
						Type:         EffectKillfeed,
						StartSeconds: 1,
						EndSeconds:   4,
						AtSeconds:    1.05,
						Width:        430,
						CropX:        1558,
						CropY:        64,
						CropWidth:    360,
						CropHeight:   110,
					},
				},
				Parts: []ShortPart{
					{SegmentID: "seg-001", Input: "seg-001.mp4", DurationSeconds: 6, TimelineStartSeconds: 0},
				},
			}

			warnings := refineKillfeedEffects(&short, tt.probe)
			if len(warnings) != 1 || !strings.Contains(warnings[0], tt.want) {
				t.Fatalf("warnings = %v, want one containing %q", warnings, tt.want)
			}
			effect := short.Effects[0]
			if effect.CropX != 1558 || effect.CropY != 64 || effect.CropWidth != 360 || effect.CropHeight != 110 || effect.Width != 430 {
				t.Fatalf("crop changed on failure: %#v", effect)
			}
		})
	}
}

func TestRefineKillfeedEffectsUsesShortInputWithoutParts(t *testing.T) {
	notice := image.Rect(1700, 70, 1910, 106)
	var gotInput string
	var gotAt float64
	probe := func(input string, atSeconds float64) (image.Image, error) {
		gotInput = input
		gotAt = atSeconds
		return killfeedTestFrame(t, notice), nil
	}

	short := ShortEdit{
		Input:           "seg-001.mp4",
		DurationSeconds: 6,
		Effects: []Effect{
			{
				Type:         EffectKillfeed,
				StartSeconds: 2,
				EndSeconds:   5,
				AtSeconds:    2.05,
				Width:        430,
				CropWidth:    360,
				CropHeight:   110,
				CropX:        1558,
				CropY:        64,
			},
		},
	}

	if warnings := refineKillfeedEffects(&short, probe); len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	if gotInput != "seg-001.mp4" {
		t.Fatalf("probe input = %q, want seg-001.mp4", gotInput)
	}
	if want := 2.05 + killfeedSampleDelaySeconds; gotAt != want {
		t.Fatalf("probe at = %.3f, want %.3f", gotAt, want)
	}
}

func TestRefineKillfeedEffectsDropsGeneratedOverlayWithoutHighlight(t *testing.T) {
	short := ShortEdit{
		Input:           "seg-001.mp4",
		DurationSeconds: 12,
		Effects: []Effect{
			{Type: EffectZoom, StartSeconds: 0.8, EndSeconds: 1.4, Scale: 1.08},
			{Type: EffectKillfeed, StartSeconds: 1, EndSeconds: 4, AtSeconds: 1.05, CropX: 1558, CropY: 64, CropWidth: 360, CropHeight: 110, Width: 430, Source: "edit-request"},
			{Type: EffectKillfeed, StartSeconds: 5, EndSeconds: 8, AtSeconds: 5.05, CropX: 1558, CropY: 64, CropWidth: 360, CropHeight: 110, Width: 430, Source: "kill"},
		},
	}
	probe := func(string, float64) (image.Image, error) {
		return image.NewRGBA(image.Rect(0, 0, 1920, 1080)), nil
	}

	warnings := refineKillfeedEffects(&short, probe)
	if len(warnings) != 2 {
		t.Fatalf("warnings = %v, want one per killfeed effect", warnings)
	}
	if !strings.Contains(warnings[0], "dropping overlay") || !strings.Contains(warnings[1], "keeping default crop") {
		t.Fatalf("warnings = %v, want generated dropped and scripted kept", warnings)
	}
	if len(short.Effects) != 2 {
		t.Fatalf("effects = %#v, want the generated overlay removed", short.Effects)
	}
	if short.Effects[0].Type != EffectZoom {
		t.Fatalf("effects[0] = %#v, want the zoom untouched", short.Effects[0])
	}
	kept := short.Effects[1]
	if kept.Type != EffectKillfeed || kept.Source != "kill" || kept.CropX != 1558 {
		t.Fatalf("effects[1] = %#v, want the scripted killfeed with default crop", kept)
	}
}

// TestKillfeedSampleTimesCoversDemoTickLag documents the candidate window used
// when CS2 paints the death notice later than the tick-derived kill time (real
// captures observed ~0.6–0.8s lag). Mutation of the window constants must still
// keep the legacy first sample and reach past that lag.
func TestKillfeedSampleTimesCoversDemoTickLag(t *testing.T) {
	tests := []struct {
		name        string
		short       ShortEdit
		effect      Effect
		wantInput   string
		wantFirst   float64
		mustInclude []float64
		mustStayLE  float64
	}{
		{
			name: "compilation part clamps to part duration",
			short: ShortEdit{
				Parts: []ShortPart{
					{SegmentID: "seg-001", Input: "seg-001.mp4", DurationSeconds: 2.5, TimelineStartSeconds: 0},
				},
			},
			effect:      Effect{Type: EffectKillfeed, AtSeconds: 1.0, StartSeconds: 0.65},
			wantInput:   "seg-001.mp4",
			wantFirst:   1.0 + killfeedSampleDelaySeconds,
			mustInclude: []float64{1.0 + killfeedSampleDelaySeconds, 1.75},
			mustStayLE:  2.5,
		},
		{
			name: "later part offsets from timeline start",
			short: ShortEdit{
				Parts: []ShortPart{
					{SegmentID: "seg-001", Input: "a.mp4", DurationSeconds: 2.5, TimelineStartSeconds: 0},
					{SegmentID: "seg-002", Input: "b.mp4", DurationSeconds: 2.5, TimelineStartSeconds: 2.5},
				},
			},
			effect:      Effect{Type: EffectKillfeed, AtSeconds: 3.5, StartSeconds: 3.15},
			wantInput:   "b.mp4",
			wantFirst:   1.0 + killfeedSampleDelaySeconds,
			mustInclude: []float64{1.0 + killfeedSampleDelaySeconds, 1.75},
			mustStayLE:  2.5,
		},
		{
			name: "single-clip short uses duration as bound",
			short: ShortEdit{
				Input:           "only.mp4",
				DurationSeconds: 6,
			},
			effect:      Effect{Type: EffectKillfeed, AtSeconds: 1.0, StartSeconds: 0.65},
			wantInput:   "only.mp4",
			wantFirst:   1.0 + killfeedSampleDelaySeconds,
			mustInclude: []float64{1.0 + killfeedSampleDelaySeconds, 1.0 + killfeedSampleDelaySeconds + 0.8},
			mustStayLE:  6,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, times := killfeedSampleTimes(&tt.short, tt.effect)
			if input != tt.wantInput {
				t.Fatalf("input = %q, want %q", input, tt.wantInput)
			}
			if len(times) == 0 {
				t.Fatal("sample times empty")
			}
			if math.Abs(times[0]-tt.wantFirst) > 1e-9 {
				t.Fatalf("first sample = %.3f, want legacy delay %.3f", times[0], tt.wantFirst)
			}
			for i := 1; i < len(times); i++ {
				if times[i] <= times[i-1] {
					t.Fatalf("times not strictly increasing: %v", times)
				}
			}
			for _, need := range tt.mustInclude {
				if !sampleTimesContain(times, need) {
					t.Fatalf("times = %v, want to include %.3f (covers demo/tick lag)", times, need)
				}
			}
			for _, at := range times {
				if at > tt.mustStayLE+1e-9 {
					t.Fatalf("sample %.3f exceeds bound %.3f: %v", at, tt.mustStayLE, times)
				}
				if at < 0 {
					t.Fatalf("negative sample %.3f in %v", at, times)
				}
			}
		})
	}
}

// TestKillfeedSampleTimesBackfillsShortPostRoll is B5b: a kill with little
// post-roll left in its part/short (typically the last kill in a clip) has
// every forward offset clamp to the same last-available frame, collapsing the
// window to one sample. killfeedSampleTimes must then also probe backward
// from that last frame so the window still reaches several distinct frames
// instead of repeating the same one, without ever probing before the kill
// itself or changing the first (legacy, late-offset-preferred) sample for
// clips where the forward window already fits.
func TestKillfeedSampleTimesBackfillsShortPostRoll(t *testing.T) {
	tests := []struct {
		name          string
		short         ShortEdit
		effect        Effect
		wantInput     string
		wantFirst     float64
		wantMinTimes  int
		wantFloorAtLE float64
	}{
		{
			name: "single-clip short with little post-roll backfills backward",
			short: ShortEdit{
				Input:           "only.mp4",
				DurationSeconds: 5.3,
			},
			effect:        Effect{Type: EffectKillfeed, AtSeconds: 5.0, StartSeconds: 4.65},
			wantInput:     "only.mp4",
			wantFirst:     5.3,
			wantMinTimes:  2,
			wantFloorAtLE: 5.0,
		},
		{
			name: "compilation part with little post-roll backfills backward",
			short: ShortEdit{
				Parts: []ShortPart{
					{SegmentID: "seg-001", Input: "a.mp4", DurationSeconds: 2.5, TimelineStartSeconds: 0},
					{SegmentID: "seg-002", Input: "b.mp4", DurationSeconds: 0.3, TimelineStartSeconds: 2.5},
				},
			},
			effect:        Effect{Type: EffectKillfeed, AtSeconds: 2.5, StartSeconds: 2.15},
			wantInput:     "b.mp4",
			wantFirst:     0.3,
			wantMinTimes:  2,
			wantFloorAtLE: 0.0,
		},
		{
			name: "zero post-roll cannot backfill beyond the single last frame",
			short: ShortEdit{
				Input:           "only.mp4",
				DurationSeconds: 5.0,
			},
			effect:        Effect{Type: EffectKillfeed, AtSeconds: 5.0, StartSeconds: 4.65},
			wantInput:     "only.mp4",
			wantFirst:     5.0,
			wantMinTimes:  1,
			wantFloorAtLE: 5.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, times := killfeedSampleTimes(&tt.short, tt.effect)
			if input != tt.wantInput {
				t.Fatalf("input = %q, want %q", input, tt.wantInput)
			}
			if len(times) == 0 {
				t.Fatal("sample times empty")
			}
			if math.Abs(times[0]-tt.wantFirst) > 1e-9 {
				t.Fatalf("first sample = %.3f, want legacy late-offset preference %.3f (unchanged)", times[0], tt.wantFirst)
			}
			if len(times) < tt.wantMinTimes {
				t.Fatalf("times = %v, want at least %d distinct samples", times, tt.wantMinTimes)
			}
			seen := make(map[string]bool, len(times))
			for _, at := range times {
				key := fmt.Sprintf("%.6f", at)
				if seen[key] {
					t.Fatalf("times = %v, want every sample distinct", times)
				}
				seen[key] = true
				if at < tt.wantFloorAtLE-1e-9 {
					t.Fatalf("sample %.3f probes before the kill itself (floor %.3f): %v", at, tt.wantFloorAtLE, times)
				}
			}
		})
	}
}

// TestRefineKillfeedEffectsKeepsOverlayNearClipEnd is the end-to-end B5b
// regression: a generated ("edit-request") killfeed overlay for the last kill
// in a clip, where post-roll leaves the forward sample window almost no room.
// Before the backward-fill fix, killfeedSampleTimes would hand back a single
// sample (the clip's last frame) and, since the highlight is not painted on
// that exact frame, refineKillfeedEffects would silently drop the overlay.
// With the fix, the backward samples reach the frame where the highlight is
// actually painted and the overlay survives with its measured crop.
func TestRefineKillfeedEffectsKeepsOverlayNearClipEnd(t *testing.T) {
	notice := image.Rect(1700, 90, 1910, 126)
	const paintedAt = 5.1 // only reachable by probing backward from the clip end

	var probed []float64
	probe := func(input string, atSeconds float64) (image.Image, error) {
		if input != "only.mp4" {
			t.Fatalf("probe input = %q, want only.mp4", input)
		}
		probed = append(probed, atSeconds)
		if math.Abs(atSeconds-paintedAt) > 1e-9 {
			return image.NewRGBA(image.Rect(0, 0, 1920, 1080)), nil
		}
		return killfeedTestFrame(t, notice), nil
	}

	short := ShortEdit{
		Input:           "only.mp4",
		DurationSeconds: 5.3,
		Effects: []Effect{
			{
				Type:         EffectKillfeed,
				StartSeconds: 4.65,
				EndSeconds:   5.3,
				AtSeconds:    5.0,
				Width:        430,
				CropX:        1558,
				CropY:        64,
				CropWidth:    360,
				CropHeight:   110,
				Source:       "edit-request",
			},
		},
	}

	warnings := refineKillfeedEffects(&short, probe)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none: the backward sample should find the notice", warnings)
	}
	if len(short.Effects) != 1 {
		t.Fatalf("effects = %#v, want the generated overlay kept, not dropped", short.Effects)
	}
	if len(probed) < 2 {
		t.Fatalf("probe calls = %v, want more than the single collapsed forward sample", probed)
	}
	if !sampleTimesContain(probed, paintedAt) {
		t.Fatalf("probe calls = %v, never reached the backward sample at %.3f", probed, paintedAt)
	}

	effect := short.Effects[0]
	crop := image.Rect(effect.CropX, effect.CropY, effect.CropX+effect.CropWidth, effect.CropY+effect.CropHeight)
	if crop.Min.X > notice.Min.X || crop.Min.Y > notice.Min.Y || crop.Max.X < notice.Max.X || crop.Max.Y < notice.Max.Y {
		t.Fatalf("crop = %v, want it to cover the notice %v", crop, notice)
	}
}

func sampleTimesContain(times []float64, want float64) bool {
	for _, at := range times {
		if math.Abs(at-want) <= 1e-9 {
			return true
		}
	}
	return false
}

// TestRefineKillfeedEffectsFindsLateDeathNotice is the production failure mode:
// tick-derived AtSeconds + 0.35s has no red ring yet; the notice appears later
// in the same segment (observed ~+0.7s). The probe must walk the window, crop
// the late frame, and retime AtSeconds so the FFmpeg freeze uses that same frame.
func TestRefineKillfeedEffectsFindsLateDeathNotice(t *testing.T) {
	notice := image.Rect(1690, 80, 1910, 116)
	const killAt = 1.0
	const noticeAt = 1.70 // ~0.7s after tick-derived kill, inside the scan window

	var probed []float64
	probe := func(input string, atSeconds float64) (image.Image, error) {
		if input != "seg-001.mp4" {
			t.Fatalf("probe input = %q, want seg-001.mp4", input)
		}
		probed = append(probed, atSeconds)
		if atSeconds+1e-9 < noticeAt {
			return image.NewRGBA(image.Rect(0, 0, 1920, 1080)), nil
		}
		return killfeedTestFrame(t, notice), nil
	}

	short := ShortEdit{
		DurationSeconds: 12,
		Effects: []Effect{
			{
				Type:         EffectKillfeed,
				StartSeconds: killAt - 0.35,
				EndSeconds:   killAt + 2.80,
				AtSeconds:    killAt,
				Width:        430,
				CropX:        1558,
				CropY:        64,
				CropWidth:    360,
				CropHeight:   110,
				Source:       "edit-request",
			},
		},
		Parts: []ShortPart{
			{SegmentID: "seg-001", Input: "seg-001.mp4", DurationSeconds: 2.5, TimelineStartSeconds: 0},
		},
	}

	warnings := refineKillfeedEffects(&short, probe)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want success once the late notice is found", warnings)
	}
	if len(short.Effects) != 1 {
		t.Fatalf("effects = %#v, want the generated overlay kept", short.Effects)
	}
	if len(probed) < 2 {
		t.Fatalf("probe calls = %v, want multiple samples after the empty early frame", probed)
	}
	if math.Abs(probed[0]-(killAt+killfeedSampleDelaySeconds)) > 1e-9 {
		t.Fatalf("first probe = %.3f, want legacy %.3f", probed[0], killAt+killfeedSampleDelaySeconds)
	}
	if !sampleTimesContain(probed, noticeAt) && probed[len(probed)-1]+1e-9 < noticeAt {
		t.Fatalf("probe calls = %v, never reached notice at %.3f", probed, noticeAt)
	}

	effect := short.Effects[0]
	crop := image.Rect(effect.CropX, effect.CropY, effect.CropX+effect.CropWidth, effect.CropY+effect.CropHeight)
	if crop.Min.X > notice.Min.X || crop.Min.Y > notice.Min.Y || crop.Max.X < notice.Max.X || crop.Max.Y < notice.Max.Y {
		t.Fatalf("crop = %v, want it to cover late notice %v", crop, notice)
	}

	// Freeze path must reuse the winning sample, not the original kill+0.35.
	_, freezeAt := killfeedSamplePart(&short, effect)
	if freezeAt+1e-9 < noticeAt {
		t.Fatalf("freeze sample = %.3f, want >= notice frame %.3f (AtSeconds was retimed)", freezeAt, noticeAt)
	}
	if math.Abs(freezeAt-probed[len(probed)-1]) > 1e-9 {
		t.Fatalf("freeze sample = %.3f, want last successful probe %.3f", freezeAt, probed[len(probed)-1])
	}
}

// TestRefineKillfeedEffectsLateNoticeMutations kills individual behaviors the
// windowed probe relies on: giving up after the first empty frame, forgetting
// to retime AtSeconds, or accepting a probe error as terminal without trying
// later samples in the window.
func TestRefineKillfeedEffectsLateNoticeMutations(t *testing.T) {
	notice := image.Rect(1700, 90, 1910, 126)
	tests := []struct {
		name     string
		probe    func(calls *[]float64) func(string, float64) (image.Image, error)
		wantWarn bool
		wantKeep bool
		check    func(t *testing.T, short ShortEdit, calls []float64)
	}{
		{
			name: "empty early frames then hit",
			probe: func(calls *[]float64) func(string, float64) (image.Image, error) {
				return func(_ string, at float64) (image.Image, error) {
					*calls = append(*calls, at)
					if at < 1.55 {
						return image.NewRGBA(image.Rect(0, 0, 1920, 1080)), nil
					}
					return killfeedTestFrame(t, notice), nil
				}
			},
			wantKeep: true,
			check: func(t *testing.T, short ShortEdit, calls []float64) {
				t.Helper()
				// Mutation: a single-sample probe would only call 1.35 and drop.
				if len(calls) < 2 {
					t.Fatalf("calls = %v, want at least one empty frame then a hit", calls)
				}
				if math.Abs(calls[0]-1.35) > 1e-9 || calls[len(calls)-1] < 1.55 {
					t.Fatalf("calls = %v, want first legacy 1.35 then a sample >= 1.55", calls)
				}
				_, freeze := killfeedSamplePart(&short, short.Effects[0])
				if freeze < 1.55 {
					t.Fatalf("freeze = %.3f, want retimed past 1.55", freeze)
				}
			},
		},
		{
			name: "transient probe error then hit",
			probe: func(calls *[]float64) func(string, float64) (image.Image, error) {
				return func(_ string, at float64) (image.Image, error) {
					*calls = append(*calls, at)
					if at < 1.5 {
						return nil, fmt.Errorf("ffmpeg flake at %.2f", at)
					}
					return killfeedTestFrame(t, notice), nil
				}
			},
			wantKeep: true,
			check: func(t *testing.T, short ShortEdit, calls []float64) {
				t.Helper()
				if len(calls) < 2 {
					t.Fatalf("calls = %v, want retry after probe error", calls)
				}
				if short.Effects[0].CropWidth == 360 && short.Effects[0].CropX == 1558 {
					t.Fatal("crop left at defaults; success path should rewrite measured box")
				}
			},
		},
		{
			name: "window exhausted stays drop for generated",
			probe: func(calls *[]float64) func(string, float64) (image.Image, error) {
				return func(_ string, at float64) (image.Image, error) {
					*calls = append(*calls, at)
					return image.NewRGBA(image.Rect(0, 0, 1920, 1080)), nil
				}
			},
			wantWarn: true,
			wantKeep: false,
			check: func(t *testing.T, short ShortEdit, calls []float64) {
				t.Helper()
				if len(calls) < 4 {
					t.Fatalf("calls = %v, want full window before giving up", calls)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []float64
			short := ShortEdit{
				DurationSeconds: 6,
				Effects: []Effect{{
					Type: EffectKillfeed, StartSeconds: 0.65, EndSeconds: 3.8, AtSeconds: 1.0,
					CropX: 1558, CropY: 64, CropWidth: 360, CropHeight: 110, Width: 430, Source: "edit-request",
				}},
				Parts: []ShortPart{{SegmentID: "seg-001", Input: "seg-001.mp4", DurationSeconds: 2.5, TimelineStartSeconds: 0}},
			}
			warnings := refineKillfeedEffects(&short, tt.probe(&calls))
			if tt.wantWarn && len(warnings) == 0 {
				t.Fatal("warnings empty, want a failure warning")
			}
			if !tt.wantWarn && len(warnings) != 0 {
				t.Fatalf("warnings = %v, want none", warnings)
			}
			kept := false
			for _, e := range short.Effects {
				if e.Type == EffectKillfeed {
					kept = true
				}
			}
			if kept != tt.wantKeep {
				t.Fatalf("killfeed kept = %v, want %v (effects=%#v)", kept, tt.wantKeep, short.Effects)
			}
			if tt.check != nil {
				tt.check(t, short, calls)
			}
		})
	}
}
