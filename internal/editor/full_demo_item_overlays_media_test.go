package editor

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/mediafont"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

// fullDemoMediaOverlayShort builds an eligible Full Demo short for one program
// duration, with the production overlay windows for that duration.
func fullDemoMediaOverlayShort(dir string, duration float64) ShortEdit {
	introStart, introEnd, outroStart, outroEnd := demooverlay.OverlayWindows(duration)
	return fullDemoMediaOverlayShortWithWindows(dir, duration, introStart, introEnd, outroStart, outroEnd)
}

func fullDemoMediaOverlayShortWithWindows(dir string, duration, introStart, introEnd, outroStart, outroEnd float64) ShortEdit {
	short := fullDemoItemOverlayFixtureShort(dir)
	short.DurationSeconds = duration
	short.Effects = nil
	if introEnd > introStart {
		short.Effects = append(short.Effects, Effect{
			Type: EffectImage, Path: filepath.Join(dir, "intro.png"), Source: "full-demo-intro",
			X: "0", Y: "0", Width: demooverlay.FrameWidth, Height: demooverlay.FrameHeight,
			StartSeconds: introStart, EndSeconds: introEnd,
			FadeInSeconds: demooverlay.IntroOverlaySlideSeconds, FadeOutSeconds: 0.35,
		})
	}
	if outroEnd > outroStart {
		short.Effects = append(short.Effects, Effect{
			Type: EffectImage, Path: filepath.Join(dir, "outro.png"), Source: "full-demo-outro",
			X: "0", Y: "0", Width: demooverlay.FrameWidth, Height: demooverlay.FrameHeight,
			StartSeconds: outroStart, EndSeconds: outroEnd,
			FadeInSeconds: demooverlay.IntroOverlaySlideSeconds,
		})
	}
	return short
}

// fullDemoOverlayStills writes deterministic intro/outro RGBA stills so both
// compositions composite the exact same pixels.
func fullDemoOverlayStills(t *testing.T, ctx context.Context, ffmpeg, dir string) {
	t.Helper()
	for _, still := range []struct {
		name  string
		vegas string
	}{
		{"intro.png", "testsrc2=s=1920x1080"},
		{"outro.png", "smptebars=s=1920x1080"},
	} {
		command := []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", still.vegas, "-frames:v", "1", "-pix_fmt", "rgba", filepath.Join(dir, still.name)}
		if _, err := runFFmpegOutput(ctx, command, "overlay still"); err != nil {
			t.Fatal(err)
		}
	}
}

// fullDemoMediaBase writes a lossless 1920x1080 60 fps high-motion fixture with
// baked HUD-like text, on an exact 1/60 frame timebase and an exact frame count,
// so both compositions see identical frame clocks.
func fullDemoMediaBase(t *testing.T, ctx context.Context, ffmpeg, dir string, seconds float64) string {
	t.Helper()
	if _, err := mediafont.Materialize(); err != nil {
		t.Fatal(err)
	}
	frames := int(math.Round(seconds * 60))
	path := filepath.Join(dir, "base.mp4")
	text := drawTextEffect(Effect{
		Type: EffectText, Value: "HUD 12 34", X: "24", Y: "24", Size: 56,
		FontColor: "white", BoxColor: "black@0.5", BoxBorder: 12,
		StartSeconds: 0, EndSeconds: seconds,
	})
	command := []string{ffmpeg, "-y", "-v", "error",
		"-f", "lavfi", "-i", fmt.Sprintf("testsrc2=s=1920x1080:r=60:d=%.6f", seconds),
		"-vf", text, "-frames:v", strconv.Itoa(frames),
		"-c:v", "ffv1", "-level", "1", "-pix_fmt", "yuv420p",
		"-video_track_timescale", "60", path}
	if _, err := runFFmpegOutput(ctx, command, "lossless base"); err != nil {
		t.Fatal(err)
	}
	return path
}

// fullDemoFrameHashes returns one hash per decoded/filtered frame. Both sides
// must emit the same pixel format for the hashes to be comparable.
func fullDemoFrameHashes(t *testing.T, ctx context.Context, ffmpeg string, args []string) []string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "frames.md5")
	command := append([]string{ffmpeg, "-y", "-v", "error"}, args...)
	command = append(command, "-f", "framemd5", path)
	if _, err := runFFmpegOutput(ctx, command, "frame hashes"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var hashes []string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ",")
		hashes = append(hashes, strings.TrimSpace(fields[len(fields)-1]))
	}
	return hashes
}

func fullDemoLegacyOverlayFilter(short ShortEdit) string {
	return strings.Join(appendCompilationProgramVideo(nil, short, "0:v", 1), ";")
}

type fullDemoMediaItem struct {
	startFrame int64
	endFrame   int64
	role       string
}

func fullDemoItemBaseClause(item fullDemoMediaItem) string {
	return fmt.Sprintf("[0:v]trim=start_frame=%d:end_frame=%d,setpts=PTS-STARTPTS,format=yuv420p", item.startFrame, item.endFrame)
}

// fullDemoPerItemOverlayArgs builds the production item video graph for one
// item: the lossless base plus the ordered overlay stills.
func fullDemoPerItemOverlayArgs(short ShortEdit, base string, item fullDemoMediaItem) []string {
	images := imageEffects(short.Effects)
	args := []string{"-i", base}
	for _, effect := range images {
		args = append(args, "-i", effect.Path)
	}
	clauses, output := fullDemoItemVideoClauses(short, recapplan.TimelineItem{StartFrame: item.startFrame, EndFrame: item.endFrame}, fullDemoItemBaseClause(item), images, 1)
	args = append(args, "-filter_complex", strings.Join(clauses, ";"), "-map", output, "-fps_mode", "passthrough")
	return args
}

// TestFullDemoItemOverlaysMatchLegacyPreEncodePixels is the core equivalence
// proof: whole-program legacy composition and per-item composition must produce
// byte-identical pre-encode frames across joins, non-millisecond item starts,
// slides, dimming, fades, inactive windows, overlapping windows and inclusive
// endpoints.
func TestFullDemoItemOverlaysMatchLegacyPreEncodePixels(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	scenarios := []struct {
		name    string
		seconds float64
		intro   [2]float64
		outro   [2]float64
		items   []fullDemoMediaItem
	}{
		{
			name:    "non-millisecond starts and inclusive intro end",
			seconds: 6,
			intro:   [2]float64{0, 5},
			outro:   [2]float64{5, 6},
			items: []fullDemoMediaItem{
				{0, 137, "round"},   // 2.28333s
				{137, 181, "round"}, // 3.01667s inside the intro slide-out
				{181, 300, "round"},
				{300, 360, "round"}, // first frame at t=5.0 has intro (inclusive) and outro
			},
		},
		{
			name:    "explicit overlapping intro and outro windows",
			seconds: 9,
			intro:   [2]float64{0, 5},
			outro:   [2]float64{4, 9}, // overlap 4s-5s
			items: []fullDemoMediaItem{
				{0, 137, "round"},
				{137, 300, "sponsor"}, // inactive for part, then overlap
				{300, 540, "round"},
			},
		},
		{
			name:    "fractional outro start",
			seconds: 13 + 1.0/60,
			intro:   [2]float64{0, 5},
			outro:   [2]float64{13 + 1.0/60 - 8, 13 + 1.0/60}, // 5.0166667
			items: []fullDemoMediaItem{
				{0, 137, "round"},
				{137, 300, "round"},
				{300, 540, "round"},
				{540, 781, "round"},
			},
		},
		{
			// 123 and 245 are the starts that reproduced the float-shift
			// off-by-one (first PTS 122 / 244). The windows are placed so each
			// of those items actually contains an animating slide or fade.
			name:    "integer clock at starts 123 245 246 during animation",
			seconds: 9,
			intro:   [2]float64{1.8, 3.0},
			outro:   [2]float64{3.8, 9},
			items: []fullDemoMediaItem{
				{0, 123, "round"},
				{123, 245, "round"},
				{245, 246, "round"},
				{246, 540, "round"},
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			dir := t.TempDir()
			short := fullDemoMediaOverlayShortWithWindows(dir, scenario.seconds, scenario.intro[0], scenario.intro[1], scenario.outro[0], scenario.outro[1])
			fullDemoOverlayStills(t, ctx, ffmpeg, dir)
			base := fullDemoMediaBase(t, ctx, ffmpeg, dir, scenario.seconds)

			legacyInputs := []string{"-i", base}
			for _, effect := range imageEffects(short.Effects) {
				legacyInputs = append(legacyInputs, "-i", effect.Path)
			}
			legacy := fullDemoFrameHashes(t, ctx, ffmpeg, append(legacyInputs, "-filter_complex", fullDemoLegacyOverlayFilter(short), "-map", "[v]", "-fps_mode", "passthrough"))

			var perItem []string
			for _, item := range scenario.items {
				perItem = append(perItem, fullDemoFrameHashes(t, ctx, ffmpeg, fullDemoPerItemOverlayArgs(short, base, item))...)
			}

			if len(legacy) != len(perItem) {
				t.Fatalf("legacy frames = %d, per-item frames = %d", len(legacy), len(perItem))
			}
			var mismatches []int
			for i := range legacy {
				if legacy[i] != perItem[i] {
					mismatches = append(mismatches, i)
				}
			}
			if len(mismatches) > 0 {
				limit := min(8, len(mismatches))
				t.Fatalf("%d/%d frames differ between legacy global and per-item composition, first at %v", len(mismatches), len(legacy), mismatches[:limit])
			}
		})
	}
}

// TestFullDemoItemOverlayFloatShiftChangesPixels reproduces the real clock bug.
// The float shift "+StartFrame/60/TB" evaluates 123/60/(1/60) as
// 122.99999999999999, so the first frames of the item land one frame early;
// adding N only rounds back to the exact integer after a few frames. The item
// start 123 (2.05s) is therefore placed inside the intro slide-out (intro ends
// at 2.3s, slide-out 2.0-2.3s) so those first frames are animating. The
// canonical integer clock matches the legacy program exactly.
func TestFullDemoItemOverlayFloatShiftChangesPixels(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	dir := t.TempDir()
	const seconds = 6.0
	// Intro ends at 2.3s so start 123 = 2.05s is inside the slide-out. Outro
	// stays a plain window; only the intro timing is narrowed.
	short := fullDemoMediaOverlayShortWithWindows(dir, seconds, 0, 2.3, 5, 6)
	fullDemoOverlayStills(t, ctx, ffmpeg, dir)
	base := fullDemoMediaBase(t, ctx, ffmpeg, dir, seconds)

	legacyInputs := []string{"-i", base}
	for _, effect := range imageEffects(short.Effects) {
		legacyInputs = append(legacyInputs, "-i", effect.Path)
	}
	legacy := fullDemoFrameHashes(t, ctx, ffmpeg, append(legacyInputs, "-filter_complex", fullDemoLegacyOverlayFilter(short), "-map", "[v]", "-fps_mode", "passthrough"))

	// start 123 = 2.05s; the float shift emits first PTS 122.
	start := int64(123)
	end := int64(300)
	exact := fullDemoPerItemOverlayArgs(short, base, fullDemoMediaItem{startFrame: start, endFrame: end})
	exactHashes := fullDemoFrameHashes(t, ctx, ffmpeg, exact)
	if len(exactHashes) != int(end-start) {
		t.Fatalf("exact item frames = %d, want %d", len(exactHashes), end-start)
	}
	for i := range exactHashes {
		if exactHashes[i] != legacy[int(start)+i] {
			t.Fatalf("integer frame clock diverged at global frame %d", int(start)+i)
		}
	}

	images := imageEffects(short.Effects)
	clauses, output := fullDemoItemVideoClauses(short, recapplan.TimelineItem{StartFrame: start, EndFrame: end}, fullDemoItemBaseClause(fullDemoMediaItem{startFrame: start, endFrame: end}), images, 1)
	newClauses := append([]string{}, clauses...)
	// Keep the canonical 1/60 base; only the shift arithmetic is replaced with
	// the buggy float form.
	newClauses[1] = fmt.Sprintf("[vlocal]settb=expr=1/60,setpts=PTS-STARTPTS+%d/60/TB[vg0]", start)
	if strings.Join(newClauses, ";") == strings.Join(clauses, ";") {
		t.Fatal("test did not replace the integer shift with the float shift")
	}
	args := []string{"-i", base}
	for _, effect := range images {
		args = append(args, "-i", effect.Path)
	}
	args = append(args, "-filter_complex", strings.Join(newClauses, ";"), "-map", output, "-fps_mode", "passthrough")
	floatHashes := fullDemoFrameHashes(t, ctx, ffmpeg, args)
	mismatch, first := 0, -1
	for i := range floatHashes {
		if int(start)+i >= len(legacy) || floatHashes[i] != legacy[int(start)+i] {
			mismatch++
			if first < 0 {
				first = i
			}
		}
	}
	t.Logf("float sensitivity: mismatches=%d first=%d", mismatch, first)
	if mismatch == 0 {
		t.Fatal("float-shifted global clock matched the legacy program; the fixture is not sensitive to the off-by-one clock bug")
	}
}

// TestFullDemoItemOverlayFirstPTSUsesIntegerClock pins the exact clock: the
// canonical shift emits first PTS == StartFrame for 123/245/246, and the float
// shift demonstrably emits StartFrame-1 for the same starts.
func TestFullDemoItemOverlayFirstPTSUsesIntegerClock(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	dir := t.TempDir()
	short := fullDemoMediaOverlayShort(dir, 6)
	fullDemoOverlayStills(t, ctx, ffmpeg, dir)
	base := fullDemoMediaBase(t, ctx, ffmpeg, dir, 6)
	images := imageEffects(short.Effects)

	for _, start := range []int64{123, 245, 246} {
		item := fullDemoMediaItem{startFrame: start, endFrame: start + 120}
		clauses, _ := fullDemoItemVideoClauses(short, recapplan.TimelineItem{StartFrame: start, EndFrame: start + 120}, fullDemoItemBaseClause(item), images, 1)
		production := clauses[0] + ";" + strings.Replace(clauses[1], "[vg0]", "[vpts]", 1)
		if got := fullDemoFirstVideoPTS(t, ctx, ffmpeg, base, production); got != start {
			t.Fatalf("integer clock at start %d: first PTS = %d, want %d", start, got, start)
		}
		floatShift := fmt.Sprintf("[vlocal]settb=expr=1/60,setpts=PTS-STARTPTS+%d/60/TB[vpts]", start)
		if got := fullDemoFirstVideoPTS(t, ctx, ffmpeg, base, clauses[0]+";"+floatShift); got != start-1 {
			t.Fatalf("float shift at start %d: first PTS = %d, want the known off-by-one %d", start, got, start-1)
		}
	}
}

// fullDemoFirstVideoPTS returns the PTS of the first frame a filtergraph maps to
// [vpts].
func fullDemoFirstVideoPTS(t *testing.T, ctx context.Context, ffmpeg, base, graph string) int64 {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pts.md5")
	if _, err := runFFmpegOutput(ctx, []string{ffmpeg, "-y", "-v", "error", "-i", base, "-filter_complex", graph, "-map", "[vpts]", "-f", "framemd5", path}, "first pts"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 3 {
			t.Fatalf("unexpected framemd5 line %q", line)
		}
		pts, err := strconv.ParseInt(strings.TrimSpace(fields[2]), 10, 64)
		if err != nil {
			t.Fatalf("parse pts %q: %v", fields[2], err)
		}
		return pts
	}
	t.Fatal("framemd5 produced no frames")
	return 0
}

// TestFullDemoItemOverlaysConcatCopiesExactFrames proves the prepared NUT items
// concatenate with -c:v copy without duplicating, dropping or retiming frames,
// and that PCM audio keeps exact samples, content and monotonic timestamps.
func TestFullDemoItemOverlaysConcatCopiesExactFrames(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	dir := t.TempDir()
	const itemFrames = 90
	const itemSamples = itemFrames * 800
	items := make([]string, 3)
	for i := range items {
		items[i] = filepath.Join(dir, fmt.Sprintf("item-%03d.nut", i))
		command := []string{ffmpeg, "-y", "-v", "error",
			"-f", "lavfi", "-i", fmt.Sprintf("testsrc2=s=1920x1080:r=60:d=%.3f", float64(itemFrames)/60),
			"-f", "lavfi", "-i", fmt.Sprintf("sine=f=%d:r=48000:d=%.3f", 300+i*100, float64(itemFrames)/60),
			"-map", "0:v:0", "-map", "1:a:0",
			"-c:v", "libx264", "-preset", "ultrafast", "-bf", "0", "-pix_fmt", "yuv420p",
			"-c:a", "pcm_f32le", "-ar", "48000", "-ac", "2", items[i]}
		if _, err := runFFmpegOutput(ctx, command, "item nut"); err != nil {
			t.Fatal(err)
		}
	}

	short := fullDemoMediaOverlayShort(dir, 4.5)
	fullDemoOverlayStills(t, ctx, ffmpeg, dir)
	short.fullDemo.preparedInputs = append([]string{}, items...)
	list := "ffconcat version 1.0\n"
	for _, path := range items {
		list += "file '" + filepath.ToSlash(path) + "'\n"
		list += fmt.Sprintf("duration %.9f\n", float64(itemFrames)/60)
	}
	if err := os.WriteFile(fullDemoConcatListPath(short), []byte(list), 0o600); err != nil {
		t.Fatal(err)
	}
	command := buildFullDemoProgramCommand(ffmpeg, short)
	if !containsArg(command, "copy") {
		t.Fatalf("prepared program did not copy compatible H264:\n%v", command)
	}
	program := fullDemoProgramPath(short)
	if _, err := runFFmpegOutput(ctx, command, "concat copy"); err != nil {
		t.Fatal(err)
	}

	wantFrames := int64(itemFrames * len(items))
	gotFrames := fullDemoCountFrames(t, ctx, ffmpeg, program)
	if gotFrames != wantFrames {
		t.Fatalf("copied program frames = %d, want %d", gotFrames, wantFrames)
	}

	// The copied program must decode to exactly the same frames as the items.
	var wantHashes []string
	for _, path := range items {
		wantHashes = append(wantHashes, fullDemoFrameHashes(t, ctx, ffmpeg, []string{"-i", path, "-map", "0:v:0"})...)
	}
	gotHashes := fullDemoFrameHashes(t, ctx, ffmpeg, []string{"-i", program, "-map", "0:v:0"})
	if len(gotHashes) != len(wantHashes) {
		t.Fatalf("program hashes = %d, want %d", len(gotHashes), len(wantHashes))
	}
	for i := range wantHashes {
		if gotHashes[i] != wantHashes[i] {
			t.Fatalf("copied frame %d differs from the prepared item", i)
		}
	}

	// PCM: exact sample count, exact content and contiguous monotonic PTS.
	programPCM := fullDemoDecodePCM(t, ctx, ffmpeg, program)
	if len(programPCM) != itemSamples*4*2*len(items) {
		t.Fatalf("program PCM bytes = %d, want %d", len(programPCM), itemSamples*4*2*len(items))
	}
	var wantPCM []byte
	for _, path := range items {
		wantPCM = append(wantPCM, fullDemoDecodePCM(t, ctx, ffmpeg, path)...)
	}
	if string(programPCM) != string(wantPCM) {
		t.Fatal("copied program PCM differs from the prepared item samples")
	}
	fullDemoAssertContiguousAudioPTS(t, ctx, ffmpeg, program, itemSamples*len(items))
}

// fullDemoDecodePCM returns the decoded float PCM (interleaved stereo f32le).
func fullDemoDecodePCM(t *testing.T, ctx context.Context, ffmpeg, path string) []byte {
	t.Helper()
	out, err := runFFmpegOutput(ctx, []string{ffmpeg, "-v", "error", "-i", path, "-map", "0:a:0", "-f", "f32le", "-ac", "2", "pipe:1"}, "decode pcm")
	if err != nil {
		t.Fatal(err)
	}
	return []byte(out)
}

// fullDemoAssertContiguousAudioPTS checks that every audio packet starts where
// the previous one ended, so a concat join never drops or duplicates samples.
func fullDemoAssertContiguousAudioPTS(t *testing.T, ctx context.Context, ffmpeg, path string, wantSamples int) {
	t.Helper()
	out, err := runFFmpegOutput(ctx, []string{fullDemoFFprobe(ffmpeg), "-v", "error", "-select_streams", "a:0", "-show_entries", "packet=pts,duration", "-of", "json", path}, "audio packets")
	if err != nil {
		t.Fatal(err)
	}
	var packets struct {
		Packets []struct {
			PTS      int64 `json:"pts"`
			Duration int64 `json:"duration"`
		}
	}
	if err := json.Unmarshal([]byte(out), &packets); err != nil {
		t.Fatal(err)
	}
	var position int64
	for i, packet := range packets.Packets {
		if packet.PTS != position || packet.Duration <= 0 {
			t.Fatalf("audio packet %d has a gap or invalid duration at %d: %+v", i, position, packet)
		}
		position += packet.Duration
	}
	if position != int64(wantSamples) {
		t.Fatalf("audio PTS span = %d samples, want %d", position, wantSamples)
	}
}

// fullDemoEncodeTopology encodes one topology at production settings:
//
//	legacy: item base encode -> decode -> whole-program overlay -> re-encode
//	new:    item overlay encode -> concat copy
//
// Both are compared against the same lossless composited reference.
func fullDemoEncodeTopology(t *testing.T, ctx context.Context, ffmpeg, dir, base, output string, short ShortEdit, items []fullDemoMediaItem, encoder string, legacy bool) {
	t.Helper()
	encodeArgs := appendVideoEncodeArgs(nil, ShortEdit{VideoEncoder: encoder, VideoCRF: StandardVideoCRF, VideoPreset: StandardVideoPreset})
	images := imageEffects(short.Effects)

	parts := make([]string, 0, len(items))
	for i, item := range items {
		part := filepath.Join(dir, fmt.Sprintf("%s-part-%03d.nut", filepath.Base(output), i))
		args := []string{ffmpeg, "-y", "-v", "error", "-i", base}
		var clauses []string
		var out string
		itemClause := fullDemoItemBaseClause(item)
		timelineItem := recapplan.TimelineItem{StartFrame: item.startFrame, EndFrame: item.endFrame}
		if legacy {
			// First generation is the plain item base; the overlays are added
			// by the whole-program second generation below.
			clauses, out = []string{itemClause + "[v]"}, "[v]"
		} else {
			for _, effect := range images {
				args = append(args, "-i", effect.Path)
			}
			clauses, out = fullDemoItemVideoClauses(short, timelineItem, itemClause, images, 1)
		}
		args = append(args, "-filter_complex", strings.Join(clauses, ";"), "-map", out)
		args = append(args, encodeArgs...)
		// Production item encodes pin -bf 0; the legacy whole-program re-encode
		// does not.
		args = append(args, "-bf", "0", "-an", "-fps_mode", "passthrough", part)
		if _, err := runFFmpegOutput(ctx, args, "topology item encode"); err != nil {
			t.Fatal(err)
		}
		parts = append(parts, part)
	}
	list := "ffconcat version 1.0\n"
	for _, part := range parts {
		list += "file '" + filepath.ToSlash(part) + "'\n"
	}
	listPath := filepath.Join(dir, filepath.Base(output)+"-concat.txt")
	if err := os.WriteFile(listPath, []byte(list), 0o600); err != nil {
		t.Fatal(err)
	}

	if !legacy {
		if _, err := runFFmpegOutput(ctx, []string{ffmpeg, "-y", "-v", "error", "-f", "concat", "-safe", "0", "-i", listPath, "-c", "copy", output}, "topology concat copy"); err != nil {
			t.Fatal(err)
		}
		return
	}

	// Legacy second generation: decode the encoded items, apply the original
	// whole-program graph, and encode again at the same settings.
	args := []string{ffmpeg, "-y", "-v", "error", "-f", "concat", "-safe", "0", "-i", listPath}
	for _, effect := range images {
		args = append(args, "-i", effect.Path)
	}
	args = append(args, "-filter_complex", fullDemoLegacyOverlayFilter(short), "-map", "[v]")
	args = append(args, encodeArgs...)
	args = append(args, "-an", "-fps_mode", "passthrough", output)
	if _, err := runFFmpegOutput(ctx, args, "topology legacy re-encode"); err != nil {
		t.Fatal(err)
	}
}

// TestFullDemoItemOverlaysEncodedTopologyMatchesReference compares the actual
// encode topologies (legacy two-generation vs new one-generation) against a
// common lossless composited reference at production software CRF16/slow and
// native p5/cq16.
func TestFullDemoItemOverlaysEncodedTopologyMatchesReference(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	dir := t.TempDir()
	short := fullDemoMediaOverlayShort(dir, 6)
	fullDemoOverlayStills(t, ctx, ffmpeg, dir)
	base := fullDemoMediaBase(t, ctx, ffmpeg, dir, 6)
	// Non-millisecond item start inside the intro slide.
	items := []fullDemoMediaItem{{0, 137, "round"}, {137, 360, "round"}}
	images := imageEffects(short.Effects)

	// Common lossless composited reference: the original whole-program graph
	// encoded losslessly, so both topologies are measured against the same ideal.
	reference := filepath.Join(dir, "reference.mkv")
	refArgs := []string{ffmpeg, "-y", "-v", "error", "-i", base}
	for _, effect := range images {
		refArgs = append(refArgs, "-i", effect.Path)
	}
	refArgs = append(refArgs, "-filter_complex", fullDemoLegacyOverlayFilter(short), "-map", "[v]", "-c:v", "ffv1", "-level", "1", "-pix_fmt", "yuv420p", "-an", "-fps_mode", "passthrough", reference)
	if _, err := runFFmpegOutput(ctx, refArgs, "lossless reference"); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name    string
		encoder string
		margin  float64
	}{
		{"software", "", 0.5},
		{"nvenc", VideoEncoderNVENC, 1.0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.encoder == VideoEncoderNVENC && !fullDemoHasNVENC(ctx, ffmpeg) {
				t.Skip("h264_nvenc unavailable")
			}
			legacy := filepath.Join(dir, "legacy-"+tc.name+".mp4")
			newOutput := filepath.Join(dir, "new-"+tc.name+".mp4")
			fullDemoEncodeTopology(t, ctx, ffmpeg, dir, base, legacy, short, items, tc.encoder, true)
			fullDemoEncodeTopology(t, ctx, ffmpeg, dir, base, newOutput, short, items, tc.encoder, false)

			legacyPSNR := fullDemoPSNR(t, ctx, ffmpeg, reference, legacy)
			newPSNR := fullDemoPSNR(t, ctx, ffmpeg, reference, newOutput)
			t.Logf("%s topology vs lossless reference: legacy(2-gen)=%.2f dB new(1-gen)=%.2f dB direct=%.2f dB frames ref=%d legacy=%d new=%d",
				tc.name, legacyPSNR, newPSNR, fullDemoPSNR(t, ctx, ffmpeg, legacy, newOutput),
				fullDemoCountFrames(t, ctx, ffmpeg, reference), fullDemoCountFrames(t, ctx, ffmpeg, legacy), fullDemoCountFrames(t, ctx, ffmpeg, newOutput))
			if legacyPSNR <= 0 || newPSNR <= 0 {
				t.Fatalf("invalid PSNR: legacy=%.2f new=%.2f", legacyPSNR, newPSNR)
			}
			if newPSNR < legacyPSNR-tc.margin {
				t.Fatalf("one-generation topology is worse than two-generation: new=%.2f legacy=%.2f", newPSNR, legacyPSNR)
			}
		})
	}
}

func fullDemoPSNR(t *testing.T, ctx context.Context, ffmpeg, reference, candidate string) float64 {
	t.Helper()
	out, err := runFFmpegOutput(ctx, []string{ffmpeg, "-v", "info", "-i", reference, "-i", candidate, "-lavfi", "[0:v][1:v]psnr", "-f", "null", "-"}, "psnr")
	if err != nil {
		t.Fatal(err)
	}
	idx := strings.LastIndex(out, "average:")
	if idx < 0 {
		t.Fatalf("psnr output missing average:\n%s", out)
	}
	value := strings.TrimSpace(strings.SplitN(out[idx+len("average:"):], " ", 2)[0])
	psnr, err := strconv.ParseFloat(value, 64)
	if err != nil {
		t.Fatalf("parse psnr %q: %v", value, err)
	}
	return psnr
}

// TestFullDemoItemOverlaysPipelineIntegration runs the production preparation
// and program assembly for a supported overlay plan and proves the delivered
// program actually carries the intro/outro composition on the copy path.
func TestFullDemoItemOverlaysPipelineIntegration(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	dir := t.TempDir()

	const duration = 20.0
	source := filepath.Join(dir, "source.nut")
	command := []string{ffmpeg, "-y", "-v", "error",
		"-f", "lavfi", "-i", "testsrc2=s=1920x1080:r=60:d=20",
		"-f", "lavfi", "-i", "sine=f=440:r=48000:d=20",
		"-map", "0:v:0", "-map", "1:a:0",
		"-c:v", "libx264", "-preset", "ultrafast", "-bf", "0", "-pix_fmt", "yuv420p",
		"-c:a", "pcm_f32le", "-ar", "48000", "-ac", "2", source}
	if _, err := runFFmpegOutput(ctx, command, "integration source"); err != nil {
		t.Fatal(err)
	}

	short := fullDemoMediaOverlayShort(dir, duration)
	fullDemoOverlayStills(t, ctx, ffmpeg, dir)
	short.fullDemo.workDir = filepath.Join(dir, "prepared")
	short.fullDemo.recording = recording.RecordingResult{Plan: recording.RecordingPlan{Tickrate: 64, DemoDurationTicks: 1280, Segments: []recording.RecordingSegment{{ID: "round-001", TickStart: 0}}}}
	options := short.FullDemo.Effective.Options
	options.Transitions = nil
	short.FullDemo.Effective = recapplan.Document{Clock: recapplan.Clock{TickRate: 64}, Options: options}

	const itemFrames = 300
	timeline := make([]recapplan.TimelineItem, 0, 4)
	short.Parts = nil
	for i := 0; i < 4; i++ {
		id := "round-00" + strconv.Itoa(i+1)
		short.Parts = append(short.Parts, ShortPart{SegmentID: id, Input: source, DurationSeconds: duration})
		start := int64(i * itemFrames)
		timeline = append(timeline, recapplan.TimelineItem{
			Role: "round", SourceRef: id,
			SourceStartTick: 0, SourceOffsetFrames: start,
			StartFrame: start, EndFrame: start + itemFrames,
			StartSample: start * recapplan.SamplesPerFrame, EndSample: (start + itemFrames) * recapplan.SamplesPerFrame,
		})
	}
	short.FullDemo.Effective.Timeline = timeline

	if err := prepareFullDemoCompilation(ctx, &short, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(short.FFmpegCommand, " "), "-c:v copy") {
		t.Fatalf("prepared program did not take the copy path:\n%v", short.FFmpegCommand)
	}
	program := fullDemoProgramPath(short)
	if _, err := runFFmpegOutput(ctx, short.FFmpegCommand, "integration program"); err != nil {
		t.Fatal(err)
	}
	if frames := fullDemoCountFrames(t, ctx, ffmpeg, program); frames != 4*itemFrames {
		t.Fatalf("program frames = %d, want %d", frames, 4*itemFrames)
	}

	introFrame := extractFrame(t, ffmpeg, program, 2.5, filepath.Join(dir, "intro-frame.png"))
	bodyFrame := extractFrame(t, ffmpeg, program, 8.0, filepath.Join(dir, "body-frame.png"))
	outroFrame := extractFrame(t, ffmpeg, program, 16.0, filepath.Join(dir, "outro-frame.png"))
	if filesEqual(t, introFrame, bodyFrame) {
		t.Fatal("intro overlay did not land in the delivered program")
	}
	if filesEqual(t, outroFrame, bodyFrame) {
		t.Fatal("outro overlay did not land in the delivered program")
	}
}

func fullDemoCountFrames(t *testing.T, ctx context.Context, ffmpeg, path string) int64 {
	t.Helper()
	out, err := runFFmpegOutput(ctx, []string{fullDemoFFprobe(ffmpeg), "-v", "error", "-count_frames", "-select_streams", "v:0", "-show_entries", "stream=nb_read_frames", "-of", "default=nw=1:nk=1", path}, "count frames")
	if err != nil {
		t.Fatal(err)
	}
	frames, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	if err != nil {
		t.Fatalf("parse frame count %q: %v", out, err)
	}
	return frames
}

// fullDemoFFprobe resolves ffprobe next to the resolved ffmpeg binary.
func fullDemoFFprobe(ffmpeg string) string {
	base := filepath.Base(ffmpeg)
	if base == ffmpeg {
		return "ffprobe"
	}
	return filepath.Join(filepath.Dir(ffmpeg), "ffprobe"+filepath.Ext(base))
}

func fullDemoHasNVENC(ctx context.Context, ffmpeg string) bool {
	out, err := runFFmpegOutput(ctx, []string{ffmpeg, "-hide_banner", "-encoders"}, "encoders")
	return err == nil && strings.Contains(out, "h264_nvenc")
}
