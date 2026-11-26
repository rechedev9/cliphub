package editor

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

const cameraFixtureRate = 64

func cameraFixtureWindow(round, startSecond int) recapplan.CameraWindow {
	roundID := fmt.Sprintf("round-%03d", round)
	kill := (startSecond + 5) * cameraFixtureRate
	return recapplan.CameraWindow{
		ID: recapplan.CameraWindowID(roundID, kill), RoundID: roundID, Round: round, KillTick: kill,
		StartTick: startSecond * cameraFixtureRate, EndTick: kill + cameraFixtureRate, EnemySteamID64: "76561198000000002", EnemyName: "enemy",
	}
}

func cameraFixtureShort(t *testing.T, dir string, windows ...recapplan.CameraWindow) ShortEdit {
	t.Helper()
	short := hookFixtureShort(dir, 5, 2, 600, storyFixtureMatch(3))
	short.Parts, short.fullDemo.recording.Plan.Segments = nil, nil
	for round := 1; round <= 3; round++ {
		id := fmt.Sprintf("round-%03d", round)
		short.Parts = append(short.Parts, ShortPart{SegmentID: id, Input: filepath.Join(dir, id+".nut")})
		short.fullDemo.recording.Plan.Segments = append(short.fullDemo.recording.Plan.Segments, recording.RecordingSegment{ID: id})
	}
	for _, w := range windows {
		short.fullDemo.recording.Artifacts = append(short.fullDemo.recording.Artifacts, recording.RecordingArtifact{Role: "segment", Type: "video", SegmentID: w.ID, Path: w.ID + ".mp4"})
	}
	if len(windows) > 0 {
		short.FullDemo.Effective.SecondCamera = &recapplan.SecondCamera{Windows: windows}
	}
	if err := attachFullDemoSecondCamera(&short, short.FullDemo.Effective.SecondCamera, dir); err != nil {
		t.Fatal(err)
	}
	return short
}

func cameraFixtureItem(t *testing.T, short ShortEdit, round int) recapplan.TimelineItem {
	t.Helper()
	for _, item := range short.FullDemo.Effective.Timeline {
		if item.Role == "round" && !item.IsHook() && item.SourceRef == fmt.Sprintf("round-%03d", round) {
			return item
		}
	}
	t.Fatalf("no round %d", round)
	return recapplan.TimelineItem{}
}

func cameraEvidence(t *testing.T, short ShortEdit, id string) FullDemoCameraEvidence {
	t.Helper()
	i := slices.IndexFunc(short.FullDemo.SecondCamera, func(e FullDemoCameraEvidence) bool { return e.ID == id })
	if i < 0 {
		t.Fatalf("no evidence for %s: %+v", id, short.FullDemo.SecondCamera)
	}
	return short.FullDemo.SecondCamera[i]
}

func TestSecondCameraDrawsTheEnemyViewOverItsRoundOnly(t *testing.T) {
	dir := t.TempDir()
	w := cameraFixtureWindow(2, 20)
	short := cameraFixtureShort(t, dir, w)
	if e := cameraEvidence(t, short, w.ID); !e.Drawn || e.Reason != "" || e.RoundID != "round-002" {
		t.Fatalf("evidence = %+v", e)
	}
	clip := filepath.Join(dir, w.ID+".mp4")
	command, err := fullDemoItemVideoCommand(short, cameraFixtureItem(t, short, 2), filepath.Join(dir, "r2.nut"))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(command, " ")
	for _, want := range []string{
		"-i " + clip, "alphamerge", "overlay=x=40:y=225:eof_action=pass", "overlay=x=40:y=509:eof_action=pass",
		"enable='between(n,1200,1559)'", "setpts=N+1200", "fade=t=in:st=20.000000:d=0.250000:alpha=1", "fade=t=out:st=25.750000:d=0.250000:alpha=1",
		"scale=504:284", "trim=start_frame=0:end_frame=360",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("camera command lacks %q: %v", want, command)
		}
	}
	camera := slices.Index(command, clip)
	if command[camera-2] == "-ss" {
		t.Fatalf("a window shown from its first frame needs no seek: %v", command[camera-3:camera+1])
	}
	index := fullDemoInputCount(command[:camera])
	if strings.Contains(joined, fmt.Sprintf("[%d:a]", index)) || slices.Contains(command, fmt.Sprintf("%d:a", index)) {
		t.Fatalf("the camera's game audio must not reach the mix: %v", command)
	}
	muxed, err := fullDemoItemCommand(short, cameraFixtureItem(t, short, 2), filepath.Join(dir, "r2-muxed.nut"))
	if err != nil {
		t.Fatal(err)
	}
	if mapped := strings.Join(muxed, " "); !strings.Contains(mapped, "-i "+clip) || strings.Count(mapped, "-map ") != 2 {
		t.Fatalf("the muxed item must map one video and one audio stream: %v", muxed)
	}
	for _, item := range short.FullDemo.Effective.Timeline {
		if item.Role != "round" || (item.SourceRef == "round-002" && !item.IsHook()) {
			continue
		}
		other, err := fullDemoItemVideoCommand(short, item, filepath.Join(dir, "other.nut"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.Join(other, " "), clip) {
			t.Fatalf("the camera reached item %s (hook %t)", item.SourceRef, item.IsHook())
		}
	}
}

func TestSecondCameraStaysOffTheIntroRoster(t *testing.T) {
	dir := t.TempDir()
	clipped, dropped := cameraFixtureWindow(1, 3), cameraFixtureWindow(3, 0)
	dropped.RoundID, dropped.Round = "round-001", 1
	short := cameraFixtureShort(t, dir, clipped)
	item := cameraFixtureItem(t, short, 1)
	shot, ok := short.fullDemo.cameras[item.StartFrame]
	rosterEnd := float64(item.StartFrame)/recapplan.OutputFPS + demooverlay.IntroOverlayEnd()
	if !ok || shot.v0 != int64(rosterEnd*recapplan.OutputFPS)+1-item.StartFrame || shot.v1 != 9*recapplan.OutputFPS {
		t.Fatalf("a window under the roster must start once the roster is gone: %+v %t", shot, ok)
	}
	if shot.v1-shot.v0 < fullDemoSecondCameraMinSeconds*recapplan.OutputFPS {
		t.Fatalf("the clipped window is shorter than the minimum: %+v", shot)
	}
	command, err := fullDemoItemVideoCommand(short, item, filepath.Join(dir, "r1.nut"))
	if err != nil {
		t.Fatal(err)
	}
	clip := filepath.Join(dir, clipped.ID+".mp4")
	if joined := strings.Join(command, " "); !strings.Contains(joined, "-ss 0.016667 -i "+clip) || !strings.Contains(joined, "trim=start_frame=120:end_frame=359") {
		t.Fatalf("the clipped camera must start at its first frame after the roster: %v", command)
	}
	dropped.ID = recapplan.CameraWindowID(dropped.RoundID, dropped.KillTick)
	short = cameraFixtureShort(t, dir, dropped)
	if e := cameraEvidence(t, short, dropped.ID); e.Drawn || e.Reason != "covered_by_overlay" {
		t.Fatalf("a window with under 3 s left must be dropped: %+v", e)
	}
	if len(short.fullDemo.cameras) != 0 {
		t.Fatalf("a dropped window was still drawn: %+v", short.fullDemo.cameras)
	}
}

func TestSecondCameraReportsAWindowTheCaptureOmitted(t *testing.T) {
	dir := t.TempDir()
	kept, lost := cameraFixtureWindow(2, 20), cameraFixtureWindow(3, 20)
	short := cameraFixtureShort(t, dir, kept)
	short.FullDemo.Effective.Warnings = append(short.FullDemo.Effective.Warnings, recapplan.Notice{Code: recapplan.WarnSecondCameraOmitted, RoundID: lost.RoundID})
	short.FullDemo.SecondCamera = nil
	if err := attachFullDemoSecondCamera(&short, &recapplan.SecondCamera{Windows: []recapplan.CameraWindow{kept, lost}}, dir); err != nil {
		t.Fatal(err)
	}
	if e := cameraEvidence(t, short, lost.ID); e.Drawn || e.Reason != "capture_omitted" {
		t.Fatalf("omitted window evidence = %+v", e)
	}
	if e := cameraEvidence(t, short, kept.ID); !e.Drawn {
		t.Fatalf("kept window evidence = %+v", e)
	}
	short.FullDemo.Effective.SecondCamera = nil
	short.FullDemo.SecondCamera = nil
	if err := attachFullDemoSecondCamera(&short, nil, dir); err != nil || short.FullDemo.SecondCamera != nil {
		t.Fatalf("with the option off there is no camera evidence: %v %+v", err, short.FullDemo.SecondCamera)
	}
	short.fullDemo.recording.Artifacts = nil
	short.FullDemo.Effective.SecondCamera = &recapplan.SecondCamera{Windows: []recapplan.CameraWindow{kept}}
	if err := attachFullDemoSecondCamera(&short, short.FullDemo.Effective.SecondCamera, dir); err == nil {
		t.Fatal("a usable window without its clip must fail the render")
	}
}

func TestSecondCameraSitsUnderTheClipHubRadar(t *testing.T) {
	short := cameraFixtureShort(t, t.TempDir())
	short.FullDemo.Effective.Options.Capture.ClipHubRadar = true
	if y, err := fullDemoSecondCameraY(short); err != nil || y != 347 {
		t.Fatalf("camera y under the radar = %d %v, want 347", y, err)
	}
}

func TestSecondCameraComposesInFFmpeg(t *testing.T) {
	mediaTest(t)
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	dir := t.TempDir()
	short := fullDemoItemOverlayFixtureShort(dir)
	short.Effects = nil
	short.VideoPreset, short.Threads = "ultrafast", 2
	short.fullDemo.ffmpeg = ffmpeg
	options := short.FullDemo.Effective.Options
	options.Transitions.Enabled = false
	short.FullDemo.Effective.Options = options
	game, camera := filepath.Join(dir, "game.nut"), filepath.Join(dir, "camera.mp4")
	for path, colour := range map[string]string{game: "black", camera: "white"} {
		if _, err := runFFmpegOutput(ctx, []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "color=" + colour + ":s=320x180:r=60:d=5", "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo:d=5", "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "pcm_f32le", path}, "camera fixture"); err != nil {
			t.Fatal(err)
		}
	}
	item := recapplan.TimelineItem{Role: "round", SourceRef: "round-001", SourceStartTick: 64, SourceEndTick: 64 + 64*5, EndFrame: 240, EndSample: 240 * recapplan.SamplesPerFrame}
	short.FullDemo.Effective.Timeline = []recapplan.TimelineItem{item}
	short.fullDemo.cameras = map[int64]fullDemoCameraShot{0: {window: recapplan.CameraWindow{RoundID: "round-001", EnemyName: "enemy"}, clip: camera, f0: 0, v0: 30, v1: 210}}
	output := filepath.Join(dir, "output.nut")
	command, err := fullDemoItemVideoCommand(short, item, output)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runFFmpegOutput(ctx, command, "camera composition"); err != nil {
		t.Fatal(err)
	}
	luma := func(x, y int) []byte {
		data, err := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-i", output, "-vf", fmt.Sprintf("format=gray,crop=1:1:%d:%d", x, y), "-f", "rawvideo", "pipe:1").Output()
		if err != nil || len(data) != 240 {
			t.Fatalf("frames at %d,%d: %d %v", x, y, len(data), err)
		}
		return data
	}
	inside := luma(demooverlay.SecondCameraX+demooverlay.SecondCameraWidth/2, demooverlay.SecondCameraY+demooverlay.SecondCameraHeight/2)
	corner := luma(demooverlay.SecondCameraX, demooverlay.SecondCameraY)
	for frame := range 240 {
		shown := frame >= 60 && frame < 180
		if shown && inside[frame] < 200 {
			t.Fatalf("frame %d: the camera is not drawn (%d)", frame, inside[frame])
		}
		if (frame < 30 || frame >= 210) && inside[frame] > 30 {
			t.Fatalf("frame %d: the camera is drawn outside its window (%d)", frame, inside[frame])
		}
		if corner[frame] > 30 {
			t.Fatalf("frame %d: the rounded corner covers the game (%d)", frame, corner[frame])
		}
	}
}
