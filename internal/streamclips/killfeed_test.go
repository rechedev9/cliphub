package streamclips

import (
	"slices"
	"strings"
	"testing"
)

func killFeedPlan(variant string) (EditPlan, ClipRange) {
	plan := DefaultEditPlan()
	plan.Variant = variant
	plan.KillFeed.Centered = true
	clip := ClipRange{ID: "c1", StartSeconds: 10, EndSeconds: 30}
	plan.Clips = []ClipRange{clip}
	return plan, clip
}

func TestKillFeedCutsTheLiveFeedAndCentersItBelowTheHUD(t *testing.T) {
	plan, clip := killFeedPlan(VariantStreamer4060)
	overlay := NewKillFeedOverlay([]KillFeedMask{
		{Area: CropRect{X: 0.75, Y: 0.125, Width: 0.24, Height: 0.0625}, From: -2, To: 6},
		{Area: CropRect{X: 0.8, Y: 0.1875, Width: 0.19, Height: 0.0625}, From: 6.5, To: 9},
		{Area: CropRect{X: 0.8, Y: 0.125, Width: 0.19, Height: 0.0625}, From: 14, To: 25},
	}, 1920, 1080, 20)
	if overlay == nil || len(overlay.Windows) != 2 {
		t.Fatalf("overlay = %+v, want two windows: notices half a second apart share one", overlay)
	}
	if w := overlay.Windows; w[0].From != 0 || w[0].To != 9 || w[1].From != 14 || w[1].To != 20 {
		t.Fatalf("windows = %+v, want 0-9 and 14-20, clamped to the clip", w)
	}
	args, err := BuildFFmpegArgs(FFmpegInputs{
		SourcePath: "in.mp4", OutputPath: "out.mp4", KillFeed: overlay, SourceWidth: 1920, SourceHeight: 1080,
	}, plan, clip)
	if err != nil {
		t.Fatal(err)
	}
	graph := args[slices.Index(args, "-filter_complex")+1]
	for _, want := range []string{
		// The cut reads the source before the panel hides the original notices.
		"[0:v]split=2[kfmain][kfsrc];[kfmain]split=2[facein][gamein]",
		"[kfsrc]crop=488:162:1426:121,scale=520:172:",
		// The feathered mask keeps the cut slightly see-through.
		"color=c=0xEBEBEB:",
		"[kfcut][kfmask]alphamerge,split=2[kfa0][kfa1]",
		// Already on screen when the clip starts: no fade in; the last one outlasts the clip.
		"[kfa0]null,fade=t=out:st=8.800000:d=0.200000:alpha=1[kfw0]",
		"[kfa1]null,fade=t=in:st=14.000000:d=0.200000:alpha=1[kfw1]",
		`[content][kfw0]overlay=x=280:y=933:enable='between(t\,0.000000\,9.000000)'[kfo0]`,
		"[killfeed]fps=60",
		`[gamein]delogo=x=1440:y=135:w=461:h=68:enable='between(t\,-2.200000\,6.200000)',`,
	} {
		if !strings.Contains(graph, want) {
			t.Fatalf("graph lacks %q:\n%s", want, graph)
		}
	}
	if strings.Count(graph, "delogo") != 3 {
		t.Fatalf("want one mask per notice row:\n%s", graph)
	}
}

func TestKillFeedIsSkippedWithoutANoticeInTheClip(t *testing.T) {
	if o := NewKillFeedOverlay([]KillFeedMask{{Area: CropRect{X: 0.8, Y: 0.1, Width: 0.1, Height: 0.05}, From: -6, To: -1}}, 1920, 1080, 10); o != nil {
		t.Fatalf("a notice gone before the clip gave %+v", o)
	}
	plan, clip := killFeedPlan(VariantStreamerFullframeNoCam)
	args, err := BuildFFmpegArgs(FFmpegInputs{SourcePath: "in.mp4", OutputPath: "out.mp4"}, plan, clip)
	if err != nil || strings.Contains(args[slices.Index(args, "-filter_complex")+1], "kfsrc") {
		t.Fatalf("no overlay must render without the cut: %v %v", err, args)
	}
}

func TestKillFeedCutFitsTheOutputWidth(t *testing.T) {
	plan, _ := killFeedPlan(VariantStreamerFullframeNoCam)
	overlay := NewKillFeedOverlay([]KillFeedMask{{Area: CropRect{X: 0.5, Y: 0.1, Width: 0.5, Height: 0.05}, To: 5}}, 1920, 1080, 20)
	layout, _ := VariantByName(plan.Variant)
	kf := newKillFeedGraph(layout, plan, FFmpegInputs{KillFeed: overlay, SourceWidth: 1920, SourceHeight: 1080}, 20)
	if kf.width > layout.OutputWidth-2*killFeedSideMargin || kf.width < 900 {
		t.Fatalf("cut width = %d, want it narrowed to fit %d with margins", kf.width, layout.OutputWidth)
	}
	if kf.cropX+kf.cropW > 1920 || kf.width%2 != 0 || kf.height%2 != 0 {
		t.Fatalf("cut %+v leaves the frame or has an odd size", kf)
	}
}

func TestKillFeedFollowsThePanelAndThePlanPosition(t *testing.T) {
	plan, _ := killFeedPlan(VariantStreamerFullframeNoCam)
	layout, _ := VariantByName(plan.Variant)
	if top := killFeedTop(layout, plan, 1920, 1080); top != 276 {
		t.Fatalf("full frame top = %d, want 276", top)
	}
	y := 0.5
	plan.KillFeed.PositionY = &y
	if top := killFeedTop(layout, plan, 1920, 1080); top != 960 {
		t.Fatalf("plan top = %d, want 960", top)
	}
}

func TestKillFeedIsOnlyForVerticalLayoutsAndStaysInTheFrame(t *testing.T) {
	plan, _ := killFeedPlan(VariantStreamerLandscape16x9)
	if err := plan.Validate(); err != nil || plan.KillFeedEnabled() {
		t.Fatalf("landscape plan: err %v, enabled %v; want valid and off", err, plan.KillFeedEnabled())
	}
	plan, _ = killFeedPlan(VariantStreamer4060)
	bad := 1.2
	plan.KillFeed.PositionY = &bad
	if err := plan.Validate(); err == nil {
		t.Fatal("kill feed position outside the frame validated")
	}
}
