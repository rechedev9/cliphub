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

func TestKillFeedStacksTheNoticesCenteredBelowTheHUD(t *testing.T) {
	plan, clip := killFeedPlan(VariantStreamer4060)
	notices := []KillFeedNotice{
		{Path: "a.png", Width: 300, Height: 41, AppearSeconds: -2, DisappearSeconds: 6,
			SourceArea: CropRect{X: 0.75, Y: 0.125, Width: 0.125, Height: 0.0625}},
		{Path: "b.png", Width: 340, Height: 41, AppearSeconds: 4, DisappearSeconds: 20},
	}
	args, err := BuildFFmpegArgs(FFmpegInputs{
		SourcePath: "in.mp4", OutputPath: "out.mp4", KillFeed: notices, SourceWidth: 1920, SourceHeight: 1080,
	}, plan, clip)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"a.png", "b.png"} {
		if !slices.Contains(args, path) {
			t.Fatalf("missing sprite input %s in %v", path, args)
		}
	}
	graph := args[slices.Index(args, "-filter_complex")+1]
	for _, want := range []string{
		// Already on screen when the clip starts: no fade in, gone at 6 s.
		"[1:v]format=rgba,fade=t=out:st=5.850000",
		`[content][kf0]overlay=x=390:y='933':format=rgb:enable='between(t\,0.000000\,6.000000)'`,
		// The newer notice sits one row lower until the older one leaves.
		"[2:v]format=rgba,fade=t=in:st=4.000000:d=0.150000:alpha=1[kf1]",
		`overlay=x=370:y='933+37.333333*clip((6.200000-t)/0.200000\,0\,1)'`,
		"[killfeed]fps=60",
		// The streamer's own copy is hidden on the game panel only; b has no area.
		`[gamein]delogo=x=1440:y=135:w=240:h=68:enable='between(t\,-2.200000\,6.200000)',crop=`,
	} {
		if !strings.Contains(graph, want) {
			t.Fatalf("graph lacks %q:\n%s", want, graph)
		}
	}
	if strings.Count(graph, "delogo") != 1 {
		t.Fatalf("want one mask, for the notice with an area:\n%s", graph)
	}
	if strings.Contains(graph, "[2:v]format=rgba,fade=t=in:st=4.000000:d=0.150000:alpha=1,fade=t=out") {
		t.Fatalf("a notice that outlasts the clip must not fade out:\n%s", graph)
	}
}

func TestKillFeedFollowsThePanelAndThePlanPosition(t *testing.T) {
	plan, _ := killFeedPlan(VariantStreamerFullframeNoCam)
	layout, _ := VariantByName(plan.Variant)
	if top := killFeedTop(layout, plan, 1920, 1080); top != 276 {
		t.Fatalf("full frame top = %d, want 276", top)
	}
	if scale, _ := KillFeedScale(plan, 1280, 720); scale < 1.77 || scale > 1.78 {
		t.Fatalf("720p full frame scale = %f, want the 1080p one", scale)
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
