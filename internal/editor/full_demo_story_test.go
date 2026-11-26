package editor

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/brandkit"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

type storyFixtureItem struct {
	round   int
	seconds int
}

func storyFixtureShort(dir string, items []storyFixtureItem) ShortEdit {
	short := fullDemoItemOverlayFixtureShort(dir)
	var document recapplan.Document
	document.Clock = recapplan.Clock{TickRate: 64}
	document.Options = short.FullDemo.Effective.Options
	frame := int64(0)
	for _, item := range items {
		end := frame + int64(item.seconds)*recapplan.OutputFPS
		if item.round == 0 {
			document.Timeline = append(document.Timeline, recapplan.TimelineItem{Role: "bumper", SourceRef: "sponsor", StartFrame: frame, EndFrame: end})
		} else {
			id := fmt.Sprintf("round-%03d", item.round)
			document.Rounds = append(document.Rounds, recapplan.Round{ID: id, Number: item.round})
			document.Timeline = append(document.Timeline, recapplan.TimelineItem{Role: "round", SourceRef: id, StartFrame: frame, EndFrame: end})
		}
		frame = end
	}
	short.FullDemo.Effective = document
	short.DurationSeconds = float64(frame) / recapplan.OutputFPS
	short.Effects = generatedFullDemoOverlayEffects(storyFixtureWithStills(short, dir))
	return short
}

func storyFixtureWithStills(short ShortEdit, dir string) ShortEdit {
	short.FullDemoIntroImagePath, short.FullDemoOutroImagePath = filepath.Join(dir, "intro.png"), filepath.Join(dir, "outro.png")
	return short
}

func storyFixtureMatch(rounds int) []storyFixtureItem {
	var items []storyFixtureItem
	for number := 1; number <= rounds; number++ {
		items = append(items, storyFixtureItem{round: number, seconds: 60})
	}
	return items
}

func TestHalftimeCardOpensTheFirstRoundOfTheSecondHalf(t *testing.T) {
	dir := t.TempDir()
	short := storyFixtureShort(dir, storyFixtureMatch(24))
	window, ok := fullDemoHalftimeWindow(short, 13)
	if !ok || window.start != 720 || window.end != 720+demooverlay.HalftimeSeconds {
		t.Fatalf("halftime window = %+v %t, want the first four seconds of round 13", window, ok)
	}
	effect := fullDemoHalftimeEffect(short, filepath.Join(dir, "halftime.png"), window)
	if effect.Type != EffectImage || effect.Source != fullDemoHalftimeSource || effect.Width != 1920 || effect.Height != 1080 || effect.FadeInSeconds <= 0 || effect.FadeOutSeconds <= 0 {
		t.Fatalf("halftime effect = %+v", effect)
	}
	if !strings.Contains(effect.Y, "-48") || !strings.Contains(effect.Y, "720.400") || !strings.Contains(effect.Y, "723.700") {
		t.Fatalf("halftime card does not slide in and out of its window: %s", effect.Y)
	}

	if _, ok := fullDemoHalftimeWindow(storyFixtureShort(dir, storyFixtureMatch(12)), 13); ok {
		t.Fatal("a video without round 13 drew the halftime card")
	}
	selected := storyFixtureShort(dir, []storyFixtureItem{{round: 3, seconds: 60}, {round: 14, seconds: 60}, {round: 20, seconds: 60}})
	if _, ok := fullDemoHalftimeWindow(selected, 13); ok {
		t.Fatal("a round selection without the switch round drew the halftime card")
	}

	opening := storyFixtureShort(dir, []storyFixtureItem{{round: 13, seconds: 60}, {round: 14, seconds: 60}})
	window, ok = fullDemoHalftimeWindow(opening, 13)
	if !ok || window.start != demooverlay.IntroOverlayEnd()+fullDemoStoryGapSeconds || window.end != window.start+demooverlay.HalftimeSeconds {
		t.Fatalf("halftime over the opening round = %+v %t, want it after the intro roster", window, ok)
	}

	closing := storyFixtureShort(dir, []storyFixtureItem{{round: 12, seconds: 60}, {round: 13, seconds: 6}})
	if window, ok := fullDemoHalftimeWindow(closing, 13); ok {
		t.Fatalf("the halftime card landed on the final scoreboard: %+v", window)
	}

	split := storyFixtureShort(dir, []storyFixtureItem{{round: 12, seconds: 60}, {round: 13, seconds: 30}, {seconds: 20}, {round: 13, seconds: 30}, {round: 14, seconds: 60}})
	split.FullDemo.Effective.Rounds = []recapplan.Round{{ID: "round-012", Number: 12}, {ID: "round-013", Number: 13}, {ID: "round-014", Number: 14}}
	split.FullDemo.Effective.Timeline[3].SourceOffsetFrames = 30 * recapplan.OutputFPS
	if window, ok := fullDemoHalftimeWindow(split, 13); !ok || window.start != 60 {
		t.Fatalf("a round split by the sponsor shows the card once, at its start: %+v %t", window, ok)
	}
}

func TestHalftimeCardMovesUnderTheSponsorBanner(t *testing.T) {
	dir := t.TempDir()
	short := storyFixtureShort(dir, storyFixtureMatch(24))
	window, _ := fullDemoHalftimeWindow(short, 13)
	short.FullDemo.Effective.Options.Overlays.Brand.Banner = &brandkit.Banner{StartSeconds: 30, EndSeconds: 45}
	if effect := fullDemoHalftimeEffect(short, "halftime.png", window); !strings.Contains(effect.Y, "\\,0\\,") {
		t.Fatalf("a banner outside the window moved the card: %s", effect.Y)
	}
	short.FullDemo.Effective.Options.Overlays.Brand.Banner = &brandkit.Banner{StartSeconds: 700}
	rest := fullDemoBannerTop + fullDemoBannerMaxHeight + fullDemoStoryMarginPixels - demooverlay.HalftimeCardTop
	if effect := fullDemoHalftimeEffect(short, "halftime.png", window); !strings.Contains(effect.Y, fmt.Sprintf("\\,%d\\,", rest)) || rest <= 0 {
		t.Fatalf("the card stays over the sponsor banner: %s", effect.Y)
	}
	effect := fullDemoHalftimeEffect(short, "halftime.png", window)
	if !strings.Contains(effect.Y, strconv.Itoa(rest+demooverlay.HalftimeSlidePixels)) || strings.Contains(effect.Y, strconv.Itoa(rest-demooverlay.HalftimeSlidePixels)) {
		t.Fatalf("under the banner the card slides in from below, never across it: %s", effect.Y)
	}
}

func storyWindowsOverlap(a, b fullDemoStoryWindow) bool {
	return a.start < b.end && b.start < a.end
}

func TestSubscribeButtonShowsTwiceOverTheRounds(t *testing.T) {
	dir := t.TempDir()
	short := storyFixtureShort(dir, storyFixtureMatch(24))
	windows := fullDemoSubscribeWindows(short, fullDemoStoryBusyWindows(short.Effects))
	if len(windows) != 2 || windows[0] != (fullDemoStoryWindow{360, 366}) || windows[1] != (fullDemoStoryWindow{1080, 1086}) {
		t.Fatalf("subscribe windows = %+v, want six seconds of one round near 25%% and 75%% of the match", windows)
	}

	halftime := fullDemoStoryWindow{360, 364}
	moved := fullDemoSubscribeWindows(short, append(fullDemoStoryBusyWindows(short.Effects), halftime))
	if len(moved) != 2 || storyWindowsOverlap(moved[0], halftime) || moved[0].end > halftime.start-fullDemoStoryGapSeconds {
		t.Fatalf("the button overlaps the halftime card: %+v", moved)
	}

	sponsored := storyFixtureShort(dir, []storyFixtureItem{{round: 1, seconds: 115}, {seconds: 30}, {round: 2, seconds: 115}, {round: 3, seconds: 120}, {round: 4, seconds: 120}})
	windows = fullDemoSubscribeWindows(sponsored, fullDemoStoryBusyWindows(sponsored.Effects))
	if len(windows) != 2 {
		t.Fatalf("subscribe windows with a sponsor = %+v", windows)
	}
	for _, window := range windows {
		inRound := false
		for _, item := range sponsored.FullDemo.Effective.Timeline {
			first, last := fullDemoRoundItemSeconds(item)
			inRound = inRound || (item.Role == "round" && window.start >= first && window.end <= last)
		}
		if !inRound || math.Abs(window.end-window.start-demooverlay.SubscribeSeconds) > 1e-9 {
			t.Fatalf("the button is not wholly over one round: %+v", window)
		}
	}
	if windows[0].end > 115 {
		t.Fatalf("the first button should stay on the round before the sponsor: %+v", windows[0])
	}
}

func TestSubscribeButtonSkipsWhatAShortVideoCannotFit(t *testing.T) {
	dir := t.TempDir()
	for seconds, want := range map[int]int{20: 0, 40: 1, 240: 2} {
		short := storyFixtureShort(dir, []storyFixtureItem{{round: 1, seconds: seconds}})
		busy := fullDemoStoryBusyWindows(short.Effects)
		windows := fullDemoSubscribeWindows(short, busy)
		if len(windows) != want {
			t.Fatalf("a %d s video shows the button %d times, want %d: %+v", seconds, len(windows), want, windows)
		}
		for _, window := range windows {
			for _, taken := range busy {
				if storyWindowsOverlap(window, taken) {
					t.Fatalf("a %d s video draws the button over the roster or scoreboard: %+v over %+v", seconds, window, taken)
				}
			}
		}
	}
}

func TestSubscribeButtonSitsAboveABottomRightWatermark(t *testing.T) {
	for _, size := range [][2]int{{400, 200}, {64, 64}, {900, 120}} {
		_, _, _, top := watermarkPlacement(brandkit.CornerBottomRight, size[0], size[1], demooverlay.FrameWidth, demooverlay.FrameHeight)
		lift := subscribeLiftOverWatermark(size[0], size[1])
		if bottom := demooverlay.SubscribeBottomEdge - lift; lift <= 0 || bottom > top-fullDemoStoryMarginPixels {
			t.Fatalf("a %dx%d logo at y=%d leaves the button's bottom edge at %d", size[0], size[1], top, bottom)
		}
	}
	short := storyFixtureShort(t.TempDir(), storyFixtureMatch(2))
	short.FullDemo.Effective.Options.Overlays.Brand.Watermark = &brandkit.Watermark{Corner: brandkit.CornerTopLeft, Opacity: 1}
	if lift, err := fullDemoSubscribeLift(short); err != nil || lift != 0 {
		t.Fatalf("a watermark in another corner moved the button: %d %v", lift, err)
	}
	effect := fullDemoSubscribeEffect("subscribe.png", fullDemoStoryWindow{100, 106}, 101)
	if effect.Y != "-101" || effect.Source != fullDemoSubscribeSource || !strings.Contains(effect.X, "480") || !strings.Contains(effect.X, "100.400") || !strings.Contains(effect.X, "105.700") {
		t.Fatalf("subscribe effect = %+v", effect)
	}
}

func TestStoryOverlaysKeepThePerItemComposition(t *testing.T) {
	dir := t.TempDir()
	short := storyFixtureShort(dir, storyFixtureMatch(24))
	halftime, _ := fullDemoHalftimeWindow(short, 13)
	short.Effects = append(short.Effects, fullDemoHalftimeEffect(short, filepath.Join(dir, "halftime.png"), halftime))
	for _, window := range fullDemoSubscribeWindows(short, fullDemoStoryBusyWindows(short.Effects)) {
		short.Effects = append(short.Effects, fullDemoSubscribeEffect(filepath.Join(dir, "subscribe.png"), window, 0))
	}
	if len(short.Effects) != 5 {
		t.Fatalf("effects = %d, want the roster, the scoreboard, the halftime card and two buttons", len(short.Effects))
	}
	if !fullDemoItemOverlayEligible(short) {
		t.Fatal("the story overlays forced the legacy whole-program pass")
	}
	timeline := short.FullDemo.Effective.Timeline
	active := map[int]bool{0: true, 6: true, 12: true, 18: true, 23: true}
	for i, item := range timeline {
		if got := !fullDemoItemOverlaysInactive(short, item); got != active[i] {
			t.Fatalf("round %d composes overlays = %t, want %t", i+1, got, active[i])
		}
	}
	clauses, output := fullDemoItemVideoClauses(short, timeline[12], "[0:v]format=yuv420p", imageEffects(short.Effects), 1)
	filter := strings.Join(clauses, ";")
	if output != "[vout]" {
		t.Fatalf("output label = %q", output)
	}
	for _, want := range []string{"setpts=N+43200[vg0]", "between(t\\,720.000\\,724.000)", "between(t\\,360.000\\,366.000)", "fade=t=in:st=0:d=0.400:alpha=1", "setpts=PTS-STARTPTS+720.000/TB", "overlay=x=0:y='if(lt(t\\,720.400)"} {
		if !strings.Contains(filter, want) {
			t.Fatalf("item graph lacks %q:\n%s", want, filter)
		}
	}
	short.fullDemo.preparedInputs = []string{filepath.Join(dir, "item-000.nut")}
	if joined := strings.Join(BuildFFmpegCommand("ffmpeg", short), " "); !strings.Contains(joined, "-c:v copy") || strings.Contains(joined, "-filter_complex") {
		t.Fatalf("the program no longer copies the composed items:\n%s", joined)
	}
}

func TestStoryOverlaysDrawNothingWithoutAChoiceOrAScore(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZV_OVERLAY_RENDERER_PATH", "")
	manifest := &Manifest{OutputDir: dir, fullDemoOverlay: &demooverlay.Document{}}
	short := storyFixtureShort(dir, storyFixtureMatch(24))
	if effects, err := fullDemoStoryOverlayEffects(manifest, short); err != nil || effects != nil {
		t.Fatalf("a plan without story overlays drew %+v (%v)", effects, err)
	}
	short.FullDemo.Effective.Options.Overlays.Halftime = true
	if effects, err := fullDemoStoryOverlayEffects(manifest, short); err != nil || len(effects) != 0 {
		t.Fatalf("an unknown halftime score must skip the card without failing: %+v (%v)", effects, err)
	}
	manifest.fullDemoOverlay.Halftime = &demooverlay.Halftime{Round: 31}
	if effects, err := fullDemoStoryOverlayEffects(manifest, short); err != nil || len(effects) != 0 {
		t.Fatalf("a switch round outside the video must skip the card without failing: %+v (%v)", effects, err)
	}
	tiny := storyFixtureShort(dir, []storyFixtureItem{{round: 1, seconds: 20}})
	tiny.FullDemo.Effective.Options.Overlays.Subscribe = true
	if effects, err := fullDemoStoryOverlayEffects(manifest, tiny); err != nil || len(effects) != 0 {
		t.Fatalf("a video too short for the button must skip it without failing: %+v (%v)", effects, err)
	}
	short.FullDemo.Effective.Options.Overlays.Subscribe = true
	if _, err := fullDemoStoryOverlayEffects(manifest, short); err == nil {
		t.Fatal("a chosen button that cannot be drawn must fail the render like the roster does")
	}
}
