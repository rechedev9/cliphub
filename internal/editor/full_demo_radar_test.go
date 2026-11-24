package editor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/radarmap"
	"github.com/rechedev9/cliphub/internal/radaroverlay"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

type fullDemoRadarFixture struct {
	document recapplan.Document
	local    FullDemoLocalRadar
}

func writeFullDemoRadarFile(t *testing.T, path string, body []byte) FullDemoLocalHUD {
	t.Helper()
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	return FullDemoLocalHUD{Path: path, SHA256: hex.EncodeToString(sum[:])}
}

func newFullDemoRadarFixture(t *testing.T, dir string) fullDemoRadarFixture {
	t.Helper()
	options := recapplan.DefaultOptions()
	options.Audio.Voice.Enabled = false
	options.Overlays.HUDTheme = "focus"
	options.Capture.ClipHubRadar = true
	options.Transitions.Enabled = false
	item := recapplan.TimelineItem{Role: "round", SourceRef: "a", EndFrame: 30, EndSample: 24000}
	d := recapplan.Document{Input: recapplan.Input{DemoSHA256: strings.Repeat("a", 64)}, Clock: recapplan.Clock{TickRate: 60}, Options: options, Timeline: []recapplan.TimelineItem{item}}
	cal, _ := radarmap.Lookup("de_mirage")
	track := radaroverlay.Track{Version: radaroverlay.Version, DemoSHA256: d.Input.DemoSHA256, TickRate: 60, Map: "de_mirage", Calibration: cal, Segments: []radaroverlay.Segment{{
		ID: "a", TickStart: 0, TickEnd: 60, POV: 0,
		Frames: []radaroverlay.Frame{
			{Tick: 0, Players: []radaroverlay.Player{{Slot: 0, Side: "T", X: -1000, Y: -1000}}},
			{Tick: 30, Players: []radaroverlay.Player{{Slot: 0, Side: "T", X: -900, Y: -1000}}},
		},
	}}}
	body, err := json.Marshal(track)
	if err != nil {
		t.Fatal(err)
	}
	art := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			art.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	var png64 bytes.Buffer
	if err := png.Encode(&png64, art); err != nil {
		t.Fatal(err)
	}
	return fullDemoRadarFixture{document: d, local: FullDemoLocalRadar{
		Rounds: map[string]FullDemoLocalHUD{"a": writeFullDemoRadarFile(t, filepath.Join(dir, "radar-a.json"), body)},
		Upper:  writeFullDemoRadarFile(t, filepath.Join(dir, "radar-upper.png"), png64.Bytes()),
	}}
}

func TestFullDemoClipHubRadarDrawsTheMapTopLeftOnEveryFrame(t *testing.T) {
	mediaTest(t)
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	dir := t.TempDir()
	source := filepath.Join(dir, "source.nut")
	if _, err := runFFmpegOutput(ctx, []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "color=black:s=320x180:r=60:d=0.5", "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo:d=0.5", "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "pcm_f32le", source}, "radar fixture"); err != nil {
		t.Fatal(err)
	}
	fixture := newFullDemoRadarFixture(t, dir)
	radar, err := loadFullDemoRadar(fixture.local, fixture.document)
	if err != nil {
		t.Fatal(err)
	}
	hud := customhud.Timeline{Version: customhud.TelemetryVersion, DemoSHA256: fixture.document.Input.DemoSHA256, TargetSteamID: customhud.ExampleTarget, TickRate: 60, EndTick: 60, Snapshots: []customhud.Snapshot{customhud.Example()}}
	short := ShortEdit{Preset: PresetGameplayPOV60, OutputFormat: OutputFormatLandscape16x9, OutputFPS: 60, VideoCRF: 18, VideoPreset: "ultrafast", Threads: 2, Parts: []ShortPart{{SegmentID: "a", Input: source}}, FullDemo: &FullDemoRenderEvidence{Effective: fixture.document}, fullDemo: &fullDemoRenderContext{hud: &hud, radar: radar, ffmpeg: ffmpeg, recording: recording.RecordingResult{Plan: recording.RecordingPlan{Segments: []recording.RecordingSegment{{ID: "a", TickStart: 0}}}}}}
	box, err := fullDemoRadarBox(short)
	if err != nil {
		t.Fatal(err)
	}
	if box.X != 43 || box.Y != 43 || box.W != 280 {
		t.Fatalf("radar box = %+v, want the small top left corner of a 1080p frame", box)
	}
	item := fixture.document.Timeline[0]
	for _, videoOnly := range []bool{false, true} {
		output := filepath.Join(dir, "output.nut")
		var command []string
		if videoOnly {
			command, err = fullDemoItemVideoCommand(short, item, output)
		} else {
			command, err = fullDemoItemCommand(short, item, output)
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err := runFFmpegOutput(ctx, command, "radar composition"); err != nil {
			t.Fatal(err)
		}
		if script, err := os.ReadFile(output + ".radar.ass"); err != nil || !strings.Contains(string(script), `\move(`) {
			t.Fatalf("radar script lacks the moving player: %v", err)
		}
		for _, probe := range []struct {
			x, y  int
			onMap bool
		}{{box.X + box.W/2, box.Y + 8, true}, {box.X + box.W + 40, box.Y + box.H/2, false}} {
			crop := "1:1:" + strconv.Itoa(probe.x) + ":" + strconv.Itoa(probe.y)
			data, err := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-i", output, "-vf", "format=rgb24,crop="+crop, "-pix_fmt", "rgb24", "-f", "rawvideo", "pipe:1").Output()
			if err != nil || len(data) != 30*3 {
				t.Fatalf("radar frame count: %d %v", len(data)/3, err)
			}
			for i := 0; i < len(data); i += 3 {
				if probe.onMap && (data[i] < 180 || data[i+1] > 40) {
					t.Fatalf("map plate missing at %s: %v", crop, data[i:i+3])
				}
				if !probe.onMap && (data[i] > 15 || data[i+1] > 15 || data[i+2] > 15) {
					t.Fatalf("radar drawn outside its box at %s", crop)
				}
			}
		}
	}
	short.fullDemo.radar = nil
	if _, err := fullDemoItemCommand(short, item, filepath.Join(dir, "missing.nut")); err == nil {
		t.Fatal("missing ClipHub radar silently omitted")
	}
}

func TestFullDemoClipHubRadarRefusesTracksThatDoNotMatchThePlan(t *testing.T) {
	dir := t.TempDir()
	fixture := newFullDemoRadarFixture(t, dir)
	execution := func(change func(*FullDemoExecution)) FullDemoExecution {
		local := fixture.local
		local.Rounds = map[string]FullDemoLocalHUD{"a": fixture.local.Rounds["a"]}
		e := FullDemoExecution{Approved: recapplan.Snapshot{Document: fixture.document}, Radar: &local}
		change(&e)
		return e
	}
	inputs, err := verifyFullDemoRadar(context.Background(), execution(func(*FullDemoExecution) {}))
	if err != nil || len(inputs) != 2 {
		t.Fatalf("matching radar tracks: inputs=%v err=%v", inputs, err)
	}
	for name, change := range map[string]func(*FullDemoExecution){
		"tracks without the option": func(e *FullDemoExecution) { e.Approved.Document.Options.Capture.ClipHubRadar = false },
		"option without tracks":     func(e *FullDemoExecution) { e.Radar = nil },
		"another round": func(e *FullDemoExecution) {
			e.Radar.Rounds = map[string]FullDemoLocalHUD{"b": fixture.local.Rounds["a"]}
		},
		"a missing round": func(e *FullDemoExecution) {
			e.Approved.Document.Timeline = append(e.Approved.Document.Timeline, recapplan.TimelineItem{Role: "round", SourceRef: "b"})
		},
		"altered art": func(e *FullDemoExecution) { e.Radar.Upper.SHA256 = strings.Repeat("0", 64) },
	} {
		if _, err := verifyFullDemoRadar(context.Background(), execution(change)); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	if _, err := verifyFullDemoRadar(context.Background(), FullDemoExecution{Approved: recapplan.Snapshot{Document: recapplan.Document{}}}); err != nil {
		t.Fatalf("a plan without the radar needs no tracks: %v", err)
	}
	other := fixture.document
	other.Input.DemoSHA256 = strings.Repeat("f", 64)
	if _, err := loadFullDemoRadar(fixture.local, other); err == nil {
		t.Fatal("a track of another demo was accepted")
	}
	renamed := fixture.local
	renamed.Rounds = map[string]FullDemoLocalHUD{"b": fixture.local.Rounds["a"]}
	if _, err := loadFullDemoRadar(renamed, fixture.document); err == nil {
		t.Fatal("a track of another round was accepted")
	}
}
