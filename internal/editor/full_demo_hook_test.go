package editor

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/radarmap"
	"github.com/rechedev9/cliphub/internal/radaroverlay"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

const hookFixtureSeconds = 10

func hookFixtureShort(dir string, introSeconds, hookRound int, hookOffset int64, items []storyFixtureItem) ShortEdit {
	short := storyFixtureShort(dir, items)
	d := short.FullDemo.Effective
	var timeline []recapplan.TimelineItem
	cursor := int64(introSeconds) * recapplan.OutputFPS
	if introSeconds > 0 {
		timeline = append(timeline, recapplan.TimelineItem{Role: "bumper", SourceRef: "intro", EndFrame: cursor, Reason: recapplan.BumperRoleIntro})
	}
	hookEnd := cursor + hookFixtureSeconds*recapplan.OutputFPS
	timeline = append(timeline, recapplan.TimelineItem{Role: "round", SourceRef: fmt.Sprintf("round-%03d", hookRound), SourceOffsetFrames: hookOffset, StartFrame: cursor, EndFrame: hookEnd, Reason: recapplan.HookReason})
	for _, item := range d.Timeline {
		item.StartFrame, item.EndFrame = item.StartFrame+hookEnd, item.EndFrame+hookEnd
		timeline = append(timeline, item)
	}
	d.Timeline = timeline
	short.FullDemo.Effective = d
	short.DurationSeconds += float64(hookEnd) / recapplan.OutputFPS
	short.Effects = generatedFullDemoOverlayEffects(storyFixtureWithStills(short, dir))
	return short
}

func TestIntroRosterOpensTheFirstRealRoundAfterTheHook(t *testing.T) {
	dir := t.TempDir()
	for intro, want := range map[int]float64{0: 10, 5: 15} {
		short := hookFixtureShort(dir, intro, 3, 600, storyFixtureMatch(4))
		start, end := fullDemoGameplaySpanSeconds(short)
		if start != want || end != want+240 {
			t.Fatalf("gameplay span with a %d s intro clip = %.2f-%.2f, want it to start at round 1 (%.0f)", intro, start, end, want)
		}
		var roster, scoreboard *Effect
		for i := range short.Effects {
			switch short.Effects[i].Source {
			case "full-demo-intro":
				roster = &short.Effects[i]
			case "full-demo-outro":
				scoreboard = &short.Effects[i]
			}
		}
		if roster == nil || roster.StartSeconds != want+demooverlay.IntroOverlayStart() || roster.EndSeconds != want+demooverlay.IntroOverlayEnd() {
			t.Fatalf("the roster must open round 1, not the hook: %+v", roster)
		}
		if scoreboard == nil || scoreboard.EndSeconds != want+240 {
			t.Fatalf("the scoreboard must still close the last round: %+v", scoreboard)
		}
	}
}

func TestHookCarriesItsLabelAndNoOtherStoryOverlay(t *testing.T) {
	dir := t.TempDir()
	short := hookFixtureShort(dir, 5, 13, 0, storyFixtureMatch(24))
	hook, ok := fullDemoHookItem(short.FullDemo.Effective)
	if !ok || hook.StartFrame != 300 {
		t.Fatalf("hook item = %+v %t", hook, ok)
	}
	label := fullDemoHookEffect(filepath.Join(dir, "hook.png"), hook)
	if label.Type != EffectImage || label.Source != fullDemoHookSource || label.StartSeconds != 5 || label.EndSeconds != float64(hook.EndFrame-1)/recapplan.OutputFPS || label.Width != 1920 || label.Height != 1080 {
		t.Fatalf("hook label = %+v", label)
	}
	if !strings.Contains(label.X, strconv.Itoa(-demooverlay.HookSlidePixels)) || label.Y != "0" {
		t.Fatalf("the label must slide in from the left edge: %s", label.X)
	}
	short.Effects = append(short.Effects, label)
	if !fullDemoItemOverlayEligible(short) {
		t.Fatal("the hook label brought back the whole-program pass")
	}
	timeline := short.FullDemo.Effective.Timeline
	if fullDemoItemOverlaysInactive(short, hook) || !fullDemoItemOverlaysInactive(short, timeline[4]) {
		t.Fatal("the label must be composed on the hook item only")
	}
	if first := float64(timeline[2].StartFrame) / recapplan.OutputFPS; label.EndSeconds >= first {
		t.Fatalf("the label leaks into round 1: %.3f >= %.3f", label.EndSeconds, first)
	}

	window, ok := fullDemoHalftimeWindow(short, 13)
	if !ok || window.start != 15+12*60 {
		t.Fatalf("the halftime card must open the real round 13, not its hook: %+v %t", window, ok)
	}
	busy := fullDemoStoryBusyWindows(short.Effects)
	for _, target := range []float64{0, 6, 14} {
		window, ok := fullDemoSubscribeWindowNear(timeline, busy, target)
		if !ok || window.start < 15 {
			t.Fatalf("the subscribe button landed on the hook: %+v %t", window, ok)
		}
	}
	for _, window := range fullDemoSubscribeWindows(short, busy) {
		if window.start < 15 {
			t.Fatalf("the subscribe button landed on the hook: %+v", window)
		}
	}
}

func TestHookKillsAreNotCountedTwice(t *testing.T) {
	const rate = 64
	rounds := []recapplan.Round{
		{ID: "round-001", Number: 1, RequestedStartTick: 1000, Kills: []killplan.Kill{{Tick: 1000 + 5*rate}}},
		{ID: "round-002", Number: 2, RequestedStartTick: 10000, Kills: []killplan.Kill{{Tick: 10000 + 8*rate}, {Tick: 10000 + 9*rate}}},
	}
	d := recapplan.Document{Clock: recapplan.Clock{TickRate: rate}, Rounds: rounds, Timeline: []recapplan.TimelineItem{
		{Role: "round", SourceRef: "round-002", SourceStartTick: 10000, SourceEndTick: 10000 + 30*rate, SourceOffsetFrames: 300, StartFrame: 0, EndFrame: 600, Reason: recapplan.HookReason},
		{Role: "round", SourceRef: "round-001", SourceStartTick: 1000, SourceEndTick: 1000 + 20*rate, StartFrame: 600, EndFrame: 1800},
		{Role: "round", SourceRef: "round-002", SourceStartTick: 10000, SourceEndTick: 10000 + 30*rate, StartFrame: 1800, EndFrame: 3600},
	}}
	short := ShortEdit{
		DurationSeconds: 60,
		Parts:           []ShortPart{{SegmentID: "round-001", Input: "r1.nut"}, {SegmentID: "round-002", Input: "r2.nut"}},
		fullDemo: &fullDemoRenderContext{recording: recording.RecordingResult{
			Plan:             recording.RecordingPlan{Segments: []recording.RecordingSegment{{ID: "round-001", TickStart: 1000}, {ID: "round-002", TickStart: 10000}}},
			FullDemoEvidence: &recording.FullDemoCaptureEvidence{CertifiedEnds: map[string]int{"round-001": 1000 + 20*rate, "round-002": 10000 + 30*rate}},
		}},
	}
	if err := projectFullDemoTimeline(&short, d); err != nil {
		t.Fatal(err)
	}
	if short.KillCount != 3 || len(short.Kills) != 3 {
		t.Fatalf("kills = %d (%+v), want the three kills of the match once", short.KillCount, short.Kills)
	}
	if len(short.Parts) != 2 || short.Parts[0].SegmentID != "round-001" || short.Parts[0].TimelineStartSeconds != 10 || short.Parts[1].TimelineStartSeconds != 30 || len(short.Parts[1].Kills) != 2 {
		t.Fatalf("the hook must not become a part of the match: %+v", short.Parts)
	}
	if short.Kills[0].TimeSeconds != 15 || short.Kills[1].TimeSeconds != 38 || short.Kills[2].TimeSeconds != 39 {
		t.Fatalf("kill cues must sit on their own round after the hook: %+v", short.Kills)
	}
	if short.CoverTimeSeconds < 10 {
		t.Fatalf("the cover frame came from the hook: %.2f", short.CoverTimeSeconds)
	}
}

func TestHookItemReadsItsStretchOfTheRoundCapture(t *testing.T) {
	options := recapplan.DefaultOptions()
	options.Capture.HUDProfile = "native-clean-spectator"
	options.Overlays.HUDTheme = ""
	hook := recapplan.TimelineItem{Role: "round", SourceRef: "round-002", SourceStartTick: 192, SourceEndTick: 192 + 64*60, SourceOffsetFrames: 600, EndFrame: 480, EndSample: 480 * 800, Reason: recapplan.HookReason}
	first := recapplan.TimelineItem{Role: "round", SourceRef: "round-001", SourceStartTick: 64, SourceEndTick: 128, StartFrame: 480, EndFrame: 540, StartSample: 480 * 800, EndSample: 540 * 800}
	own := recapplan.TimelineItem{Role: "round", SourceRef: "round-002", SourceStartTick: 192, SourceEndTick: 192 + 64*60, StartFrame: 480, EndFrame: 4080, StartSample: 480 * 800, EndSample: 4080 * 800}
	for name, next := range map[string]recapplan.TimelineItem{"another round": first, "its own round": own} {
		d := recapplan.Document{Clock: recapplan.Clock{TickRate: 64}, Options: options, Timeline: []recapplan.TimelineItem{hook, next}}
		short := ShortEdit{
			Parts:    []ShortPart{{SegmentID: "round-001", Input: "r1.nut"}, {SegmentID: "round-002", Input: "r2.nut"}},
			FullDemo: &FullDemoRenderEvidence{Effective: d},
			fullDemo: &fullDemoRenderContext{
				ffmpeg:     "ffmpeg",
				voicePaths: []string{"voice.wav"},
				recording:  recording.RecordingResult{Plan: recording.RecordingPlan{Segments: []recording.RecordingSegment{{ID: "round-001", TickStart: 64}, {ID: "round-002", TickStart: 128}}}},
			},
		}
		boundaries := d.TransitionBoundaries()
		if len(boundaries) != 1 || boundaries[0].OutgoingIndex != 0 || boundaries[0].IncomingIndex != 1 {
			t.Fatalf("%s: the hook must cut into the first round like any round: %+v", name, boundaries)
		}
		command, err := fullDemoItemCommand(short, hook, "hook.nut")
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(command, " ")
		if seconds, ok := firstInputSeek(command); !ok || seconds != 9 || !strings.Contains(joined, "-i r2.nut") {
			t.Fatalf("%s: the hook must seek its own round capture to 2 s before the teaser: %v", name, command)
		}
		if !strings.Contains(joined, "trim=start_frame=120:end_frame=600") || !strings.Contains(joined, "atrim=start_sample=96000:end_sample=480000") {
			t.Fatalf("%s: the hook must keep 8 s of picture and game audio: %v", name, command)
		}
		voiceSeek := ""
		for i, arg := range command {
			if arg == "voice.wav" && i >= 3 && command[i-3] == "-ss" {
				voiceSeek = command[i-2]
			}
		}
		if seconds, err := strconv.ParseFloat(voiceSeek, 64); err != nil || seconds != 13 {
			t.Fatalf("%s: team voice must start where the teaser starts in the demo (13 s): %q in %v", name, voiceSeek, command)
		}
		edges := fullDemoEdges(short, next)
		if edges.in == nil {
			t.Fatalf("%s: the round after the hook has no incoming transition", name)
		}
		if start, count := fullDemoCommsTail(d, edges.in); count != 0 {
			t.Fatalf("%s: the hook's voices ran on over the first round: %d samples from %d", name, count, start)
		}
		following, err := fullDemoItemCommand(short, next, "next.nut")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(strings.Join(following, " "), "voice.wav") != 1 {
			t.Fatalf("%s: the round after the hook mixed a voice tail: %v", name, following)
		}
	}
}

func TestHookRadarLevelComesFromItsOwnStretch(t *testing.T) {
	lowerMax := -100.0
	track := radaroverlay.Track{TickRate: 64, Calibration: radarmap.Calibration{Map: "de_nuke", Scale: 7, Size: 1024, LowerAltitudeMax: &lowerMax}}
	segment := radaroverlay.Segment{ID: "round-002", POV: 3}
	for tick := 0; tick < 6400; tick += 64 {
		z := 0.0
		if tick >= 5120 {
			z = -400
		}
		segment.Frames = append(segment.Frames, radaroverlay.Frame{Tick: tick, Players: []radaroverlay.Player{{Slot: 3, Side: "CT", Z: z}}})
	}
	track.Segments = []radaroverlay.Segment{segment}
	round := recapplan.TimelineItem{Role: "round", SourceRef: "round-002", EndFrame: 6000}
	if level := fullDemoRadarLevel(round, track); level != radarmap.LevelDefault {
		t.Fatalf("the round is mostly upstairs: %s", level)
	}
	hook := recapplan.TimelineItem{Role: "round", Reason: recapplan.HookReason, SourceRef: "round-002", SourceOffsetFrames: 4800, EndFrame: 1200}
	if level := fullDemoRadarLevel(hook, track); level != radarmap.LevelLower {
		t.Fatalf("the hook plays downstairs, so it draws the lower overview: %s", level)
	}
}
