package editor

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

func TestFullDemoTransitionVisualChoices(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	dir := t.TempDir()
	source := filepath.Join(dir, "source.nut")
	if _, err := runFFmpegOutput(ctx, []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "testsrc2=s=320x180:r=60:d=0.5", "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo:d=0.5", "-c:v", "libx264", "-preset", "ultrafast", "-bf", "0", "-c:a", "pcm_f32le", source}, "transition visual fixture"); err != nil {
		t.Fatal(err)
	}
	base := recapplan.Document{Clock: recapplan.Clock{TickRate: 60}, Options: recapplan.DefaultOptions(), Rounds: []recapplan.Round{{ID: "a", RoundEndTick: 20, Kills: []killplan.Kill{{Tick: 15}}}}, Timeline: []recapplan.TimelineItem{
		{Role: "round", SourceRef: "a", SourceStartTick: 0, SourceEndTick: 30, StartFrame: 0, EndFrame: 30, StartSample: 0, EndSample: 24000},
		{Role: "round", SourceRef: "b", SourceStartTick: 60, SourceEndTick: 90, StartFrame: 30, EndFrame: 60, StartSample: 24000, EndSample: 48000},
	}}
	base.Options.Audio.Music.Enabled = false
	pixels := func(path string, frame int) []byte {
		t.Helper()
		data, err := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-i", path, "-vf", "select=eq(n\\,"+strconv.Itoa(frame)+"),scale=320:180", "-frames:v", "1", "-pix_fmt", "rgb24", "-f", "rawvideo", "pipe:1").Output()
		if err != nil || len(data) != 320*180*3 {
			t.Fatalf("decode effect frame: %v (%d bytes)", err, len(data))
		}
		return data
	}
	difference := func(a, b []byte) float64 {
		total := 0
		for i := range a {
			total += absInt(int(a[i]) - int(b[i]))
		}
		return float64(total) / float64(len(a))
	}
	var clean8, clean15, clean28, clean29 []byte
	for _, style := range []string{"off", "left", "right", "up", "down", "zoom-cut", "zoom-event", "flash", "flash-4", "rgb"} {
		t.Run(style, func(t *testing.T) {
			o := recapplan.DefaultTransitions()
			o.Enabled = style != "off"
			o.Whip = false
			o.Zoom = false
			o.Whoosh = false
			o.Impact = false
			o.GameFadeMS = 0
			switch style {
			case "left", "right", "up", "down":
				o.Whip = true
				o.Direction = style
			case "zoom-cut":
				o.Zoom = true
			case "zoom-event":
				o.Zoom = true
				o.ZoomAnchor = "last-kill"
				o.Whoosh = true
			case "flash":
				o.Flash = true
			case "flash-4":
				o.Flash, o.FlashFrames = true, 4
			case "rgb":
				o.RGBSplit = true
				o.RGBPixels = 6
			}
			d := base
			d.Options.Transitions = &o
			path := filepath.Join(dir, style+".nut")
			short := ShortEdit{Preset: PresetGameplayPOV60, OutputFormat: OutputFormatLandscape16x9, OutputFPS: 60, VideoCRF: 18, VideoPreset: "ultrafast", Threads: 2, Parts: []ShortPart{{SegmentID: "a", Input: source}}, FullDemo: &FullDemoRenderEvidence{Effective: d}, fullDemo: &fullDemoRenderContext{ffmpeg: ffmpeg, recording: recording.RecordingResult{Plan: recording.RecordingPlan{Segments: []recording.RecordingSegment{{ID: "a", TickStart: 0}}}}}}
			command, err := fullDemoItemCommand(short, d.Timeline[0], 0, path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runFFmpegOutput(ctx, command, "individual transition"); err != nil {
				t.Fatal(err)
			}
			frame8, frame15, frame28, frame29 := pixels(path, 8), pixels(path, 15), pixels(path, 28), pixels(path, 29)
			if style == "off" {
				clean8, clean15, clean28, clean29 = frame8, frame15, frame28, frame29
				return
			}
			if style == "flash-4" && difference(frame28, clean28) < 2 {
				t.Fatal("four-frame flash did not cover both outgoing frames")
			}
			if style == "flash" && difference(frame28, clean28) > 2 {
				t.Fatal("two-frame flash leaked outside its single outgoing frame")
			}
			outside := difference(frame8, clean8)
			if outside > 2 {
				t.Fatalf("effect changed frames outside its window: %g", outside)
			}
			changed := difference(frame29, clean29)
			if style == "zoom-event" {
				changed = difference(frame15, clean15)
				pcm := fullDemoReadAudio(t, ctx, ffmpeg, path, .22, .06)
				var energy float64
				for i := 0; i < len(pcm); i += 4 {
					v := float64(math.Float32frombits(binary.LittleEndian.Uint32(pcm[i : i+4])))
					energy += v * v
				}
				if energy == 0 {
					t.Fatal("event zoom has no paired whoosh")
				}
			}
			if changed <= outside+.05 {
				t.Fatalf("effect produced no visible difference: cut/event=%g outside=%g", changed, outside)
			}
		})
	}
}

func TestTransitionMotionEstimatesAndStaticFallback(t *testing.T) {
	const w, h = transitionMotionWidth, transitionMotionHeight
	previous := make([]byte, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			previous[y*w+x] = byte((x*13 + y*23 + x*y) % 251)
		}
	}
	for _, tc := range []struct {
		name   string
		dx, dy int
	}{{"right", 3, 0}, {"left", -3, 0}, {"up", 0, -2}, {"down", 0, 2}} {
		current := make([]byte, w*h)
		for y := 7; y < h-7; y++ {
			for x := 7; x < w-7; x++ {
				current[y*w+x] = previous[(y-tc.dy)*w+x-tc.dx]
			}
		}
		got, ok := estimateTransitionDirection(previous, current)
		if !ok || got != tc.name {
			t.Fatalf("%s: %s %v", tc.name, got, ok)
		}
	}
	if _, ok := estimateTransitionDirection(previous, previous); ok {
		t.Fatal("static source invented motion")
	}
}

func TestTransitionCommsBoundsAndEventAnchors(t *testing.T) {
	o := recapplan.DefaultTransitions()
	o.Enabled = true
	o.CommsTailSeconds = 1.5
	d := recapplan.Document{Clock: recapplan.Clock{TickRate: 60}, Options: recapplan.DefaultOptions(), Timeline: []recapplan.TimelineItem{
		{Role: "round", SourceRef: "a", SourceStartTick: 0, SourceEndTick: 120, StartFrame: 0, EndFrame: 120, StartSample: 0, EndSample: 96000},
		{Role: "round", SourceRef: "b", SourceStartTick: 150, SourceEndTick: 270, StartFrame: 120, EndFrame: 240, StartSample: 96000, EndSample: 192000},
	}}
	d.Options.Transitions = &o
	boundaries := transitionDirections(d)
	evidence := FullDemoRenderEvidence{Effective: d, Transitions: boundaries}
	if err := evidence.validateTransitions(); err != nil {
		t.Fatal(err)
	}
	evidence.Transitions = nil
	if err := evidence.validateTransitions(); err == nil {
		t.Fatal("missing transition evidence accepted")
	}
	evidence.Transitions = append([]FullDemoTransitionEvidence{}, boundaries...)
	evidence.Transitions[0].Frame++
	if err := evidence.validateTransitions(); err == nil {
		t.Fatal("shifted transition evidence accepted")
	}
	start, count := fullDemoCommsTail(d, &boundaries[0])
	if start != 96000 || count != 24000 {
		t.Fatalf("tail overlaps incoming source or repeats outgoing audio: %d %d", start, count)
	}
	d.Options.Audio.Voice.Enabled = false
	if _, count := fullDemoCommsTail(d, &boundaries[0]); count != 0 {
		t.Fatal("disabled voice retained a tail")
	}
	d.Rounds = []recapplan.Round{{ID: "a", RoundEndTick: 110, Kills: []killplan.Kill{{Tick: 35}, {Tick: 90}}}}
	o.ZoomAnchor = "last-kill"
	if frame, ok := zoomEventFrame(d, d.Timeline[0]); !ok || frame != 90 {
		t.Fatalf("last kill anchor: %d %v", frame, ok)
	}
	o.ZoomAnchor = "round-end"
	if frame, ok := zoomEventFrame(d, d.Timeline[0]); !ok || frame != 110 {
		t.Fatalf("round end anchor: %d %v", frame, ok)
	}
	d.Timeline[0].SourceOffsetFrames = 115
	if _, ok := zoomEventFrame(d, d.Timeline[0]); ok {
		t.Fatal("event outside a split piece did not fall back to the cut")
	}
}

func TestFullDemoTransitionProgramMediaCanary(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	dir := t.TempDir()
	makeMedia := func(name, video, audio string, seconds float64) string {
		t.Helper()
		path := filepath.Join(dir, name+".nut")
		command := []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", video, "-f", "lavfi", "-i", audio, "-t", decimal(seconds), "-map", "0:v", "-map", "1:a", "-c:v", "libx264", "-preset", "ultrafast", "-bf", "0", "-c:a", "pcm_f32le", "-ar", "48000", "-ac", "2", path}
		if _, err := runFFmpegOutput(ctx, command, "transition test source"); err != nil {
			t.Fatal(err)
		}
		return path
	}
	roundVideo := makeMedia("round", "testsrc2=s=160x90:r=60", "sine=f=440:r=48000", 121.0/60)
	sponsor := makeMedia("sponsor", "color=c=lime:s=160x90:r=60", "sine=f=660:r=48000", 1)
	voice := filepath.Join(dir, "voice.wav")
	if _, err := runFFmpegOutput(ctx, []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "aevalsrc='0.1*sin(2*PI*880*t)*(between(t,4,4.7)+between(t,14,14.7))':s=48000:d=20", "-c:a", "pcm_f32le", voice}, "transition voice fixture"); err != nil {
		t.Fatal(err)
	}
	options := recapplan.DefaultOptions()
	options.Capture.Crosshair.AllowCaptureDefault = true
	options.Editorial.RoundTailSeconds = 0
	options.Audio.Voice.Normalization = "none"
	options.Audio.Music.Enabled = false
	options.Sponsor.PlacementPolicy, options.Sponsor.AfterRoundID = "round-boundary", "round-002"
	transitions := recapplan.DefaultTransitions()
	transitions.Enabled, transitions.Flash, transitions.RGBSplit = true, true, true
	transitions.Direction, transitions.GameTailLowpassHz = "follow-motion", 2400
	options.Transitions = &transitions
	hash, err := mediaassets.FileDigest(ctx, sponsor, 1<<30)
	if err != nil {
		t.Fatal(err)
	}
	ref := recapplan.AssetRef{ID: uuid.NewString(), SHA256: hash}
	options.Sponsor.Video = &ref
	assets := []recapplan.AssetEvidence{{Ref: ref, DurationFrames: 60, HasVideo: true, HasAudio: true, Title: "Test sponsor", Creator: "ClipHub tests", Permission: "Original synthetic signal", SourceURL: "https://example.invalid/test"}}
	facts := recapplan.Facts{SchemaVersion: "1.0", DemoSHA256: strings.Repeat("a", 64), TargetSteamID64: "76561198000000001", ClockKind: recapplan.ClockIngame, TickRate: 64, EndTick: 1280, Complete: true}
	var parts []ShortPart
	var segments []recording.RecordingSegment
	for i, id := range []string{"round-001", "round-002", "round-003", "round-004"} {
		start := 320 * i
		facts.Rounds = append(facts.Rounds, recapplan.RoundFacts{ID: id, Number: i + 1, StartTick: start, FreezeEndTick: start + 256, RoundEndTick: start + 256, NextStartTick: start + 320, Evidence: "round-events"})
		parts = append(parts, ShortPart{SegmentID: id, Input: roundVideo})
		segments = append(segments, recording.RecordingSegment{ID: id, TickStart: start + 128})
	}
	voiceEvidence := recapplan.VoiceEvidence{Availability: "available", IndexRef: "test/voice", IndexHash: strings.Repeat("b", 64), ExtractorVersion: "team-packet-clock-v2", ClockKind: recapplan.ClockIngame, SelectedPackets: 2, Activity: []recapplan.TickRange{{Start: 256, End: 301}, {Start: 896, End: 941}}}
	document, err := recapplan.Plan(facts, options, voiceEvidence, assets, "test/facts")
	if err != nil || len(document.Blockers) > 0 {
		t.Fatalf("plan: %v %+v", err, document.Blockers)
	}
	approval := recapplan.Snapshot{Document: document, Approval: recapplan.Approval{PlanHash: document.PlanHash, AllowSafeTailTrim: true, Timestamp: time.Now()}}
	execution := FullDemoExecution{SchemaVersion: "1.0", Approved: approval, Assets: []FullDemoLocalMedia{{Ref: ref, Path: sponsor}}, VoiceTracks: []FullDemoLocalVoice{{Path: voice, SteamID64: facts.TargetSteamID64, StorageKey: "test/voice"}}}
	frames := document.Timeline[len(document.Timeline)-1].EndFrame
	short := ShortEdit{Preset: PresetGameplayPOV60, OutputFormat: OutputFormatLandscape16x9, OutputFPS: 60, Tickrate: 64, VideoCRF: 18, VideoPreset: "ultrafast", Threads: 2, Output: filepath.Join(dir, "transitions.mp4"), DurationSeconds: float64(frames) / 60, Parts: parts,
		FullDemo: &FullDemoRenderEvidence{SchemaVersion: "1.0", Approved: approval, Effective: document},
		fullDemo: &fullDemoRenderContext{execution: execution, recording: recording.RecordingResult{Plan: recording.RecordingPlan{Tickrate: 64, DemoDurationTicks: 1280, Segments: segments}}, ffmpeg: ffmpeg, workDir: filepath.Join(dir, "prepared")},
	}
	if err := prepareFullDemoCompilation(ctx, &short, nil); err != nil {
		t.Fatal(err)
	}
	if len(short.FullDemo.Transitions) != 2 {
		t.Fatalf("transitions on ad boundaries: %+v", short.FullDemo.Transitions)
	}
	if _, err := runFFmpegOutput(ctx, buildFullDemoCompilationCommand(ffmpeg, short), "transition concat"); err != nil {
		t.Fatal(err)
	}
	program := fullDemoProgramPath(short)
	for _, cut := range []float64{121.0 / 60, 423.0 / 60} {
		pcm := fullDemoReadAudio(t, ctx, ffmpeg, program, cut+.12, .1)
		if fullDemoFrequencyPower(pcm, 880) < fullDemoFrequencyPower(pcm, 1320)*100 {
			t.Fatal("original comms after cut did not enter the next round")
		}
		later := fullDemoReadAudio(t, ctx, ffmpeg, program, cut+.8, .1)
		if fullDemoFrequencyPower(later, 880) > fullDemoFrequencyPower(pcm, 880)*.001 {
			t.Fatal("comms exceeded approved tail")
		}
	}
	adAudio := fullDemoReadAudio(t, ctx, ffmpeg, program, 242.0/60+.1, .1)
	if fullDemoFrequencyPower(adAudio, 660) < fullDemoFrequencyPower(adAudio, 880)*100 {
		t.Fatal("comms entered the ad")
	}
	loudness, err := masterFullDemoProgram(ctx, ffmpeg, program, short.Output, filepath.Join(dir, "logs"), options.Audio.Loudness, false, short.DurationSeconds, nil)
	if err != nil {
		t.Fatal(err)
	}
	short.FullDemo.ProgramLoudness = &loudness
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatal(err)
	}
	short.FullDemo.Delivery, err = verifyFullDemoDelivery(ctx, ffmpeg, ffprobe, short.Output, frames, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := short.FullDemo.ValidateCompleted(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(document.Timeline, short.FullDemo.Effective.Timeline) {
		t.Fatal("render changed canonical timeline")
	}
	if root := os.Getenv("FULL_DEMO_EVIDENCE_DIR"); root != "" {
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(short.Output)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "transitions-synthetic-canary.mp4"), body, 0600); err != nil {
			t.Fatal(err)
		}
		body, err = json.MarshalIndent(short.FullDemo, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "transitions-synthetic-canary.json"), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
