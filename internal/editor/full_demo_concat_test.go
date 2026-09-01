package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/composition"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

func TestBuildFFmpegCommandFullDemoUsesConcatDemuxer(t *testing.T) {
	dir := t.TempDir()
	parts := make([]ShortPart, 20)
	for i := range parts {
		parts[i] = ShortPart{
			SegmentID:        fmt.Sprintf("seg-%03d", i+1),
			Input:            filepath.Join(dir, fmt.Sprintf("seg-%03d.mp4", i+1)),
			DurationSeconds:  90,
			TickStart:        i * 64 * 90,
			TickEnd:          (i + 1) * 64 * 90,
			CaptureTickStart: i * 64 * 90,
			CaptureTickEnd:   (i + 1) * 64 * 90,
		}
	}
	short := ShortEdit{
		Preset:          PresetGameplayPOV60,
		OutputFormat:    OutputFormatLandscape16x9,
		Output:          filepath.Join(dir, "out.mp4"),
		OutputFPS:       60,
		HQFilters:       true,
		AudioNormalize:  true,
		DurationSeconds: 1800,
		Parts:           parts,
		Effects: []Effect{
			{Type: EffectImage, Path: filepath.Join(dir, "intro.png"), Source: "full-demo-intro", StartSeconds: 5, EndSeconds: 14, FadeInSeconds: 0.45, Width: demooverlay.FrameWidth, Height: demooverlay.FrameHeight},
			{Type: EffectImage, Path: filepath.Join(dir, "outro.png"), Source: "full-demo-outro", StartSeconds: 1792, EndSeconds: 1800, Width: demooverlay.FrameWidth, Height: demooverlay.FrameHeight},
		},
		VoiceTracks:   []string{filepath.Join(dir, "pov.ogg"), filepath.Join(dir, "mate.ogg")},
		VoiceTickrate: 64,
		Tickrate:      64,
	}

	command := BuildFFmpegCommand("ffmpeg", short)
	joined := strings.Join(command, " ")
	if argAfter(command, "-f") != "concat" {
		t.Fatalf("command = %q, want -f concat demuxer", joined)
	}
	if strings.Contains(joined, "concat=n=20:v=1") {
		t.Fatalf("command opened 20 decoded video streams:\n%s", joined)
	}
	if got := countDashI(command); got != 5 {
		t.Fatalf("-i count = %d, want 5 (concat list, intro, outro, two voice tracks)", got)
	}
	if argAfter(command, "-i") != fullDemoConcatListPath(short) {
		t.Fatalf("first -i = %q, want concat list %q", argAfter(command, "-i"), fullDemoConcatListPath(short))
	}
	for _, want := range []string{
		filepath.Join(dir, "intro.png"),
		filepath.Join(dir, "outro.png"),
		filepath.Join(dir, "pov.ogg"),
		"fade=t=in:st=0",
		"gblur=sigma=",
		"pow(1-(t-",
		"[0:v]",
		"atrim=start=",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("command missing %q:\n%s", want, joined)
		}
	}
	for _, part := range short.Parts {
		if containsArg(command, part.Input) {
			t.Fatalf("command passed round %s as a parallel -i", part.Input)
		}
	}

	list, err := fullDemoConcatList(short)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(list, "ffconcat version 1.0\n") {
		t.Fatalf("concat list missing ffconcat header:\n%s", list)
	}
	if strings.Count(list, "file '") != 20 || strings.Count(list, "outpoint ") != 20 {
		t.Fatalf("concat list = %q, want 20 file+outpoint entries", list)
	}
	for _, part := range short.Parts {
		want := composition.ConcatFileLine(part.Input)
		if !strings.Contains(list, strings.TrimSuffix(want, "\n")) {
			t.Fatalf("concat list missing %q:\n%s", want, list)
		}
	}
}

func TestFullDemoConcatListEscapesQuotesAndBackslashes(t *testing.T) {
	short := ShortEdit{
		Preset:       PresetGameplayPOV60,
		OutputFormat: OutputFormatLandscape16x9,
		OutputFPS:    60,
		Parts: []ShortPart{{
			Input:           `C:\tmp\clip's\seg-001.mp4`,
			DurationSeconds: 2,
		}},
	}
	got, err := fullDemoConcatList(short)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, strings.TrimSuffix(composition.ConcatFileLine(short.Parts[0].Input), "\n")) {
		t.Fatalf("concat list = %q, want composition.ConcatFileLine escaping", got)
	}
}

func TestFullDemoConcatListRejectsRhythmGapsAndMusic(t *testing.T) {
	base := ShortEdit{
		Preset:       PresetGameplayPOV60,
		OutputFormat: OutputFormatLandscape16x9,
		OutputFPS:    60,
		Parts:        []ShortPart{{SegmentID: "seg-001", Input: "a.mp4", DurationSeconds: 2}},
	}
	tests := []struct {
		name  string
		short ShortEdit
		want  string
	}{
		{
			name: "rhythm gap",
			short: func() ShortEdit {
				s := base
				s.Parts = []ShortPart{{SegmentID: "seg-002", Input: "b.mp4", DurationSeconds: 2, GapBeforeSeconds: 0.5}}
				return s
			}(),
			want: "rhythm gaps",
		},
		{
			name: "music bed",
			short: func() ShortEdit {
				s := base
				s.MusicPath = "bed.mp3"
				return s
			}(),
			want: "music bed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := fullDemoConcatList(tt.short)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestBuildFFmpegCommandShortsCompilationStillUsesFilterConcat(t *testing.T) {
	short := ShortEdit{
		Preset:       PresetViral60Clean,
		OutputFormat: OutputFormatShort9x16,
		Output:       "compiled.mp4",
		Parts: []ShortPart{
			{SegmentID: "seg-001", Input: "seg-001.mp4", DurationSeconds: 4},
			{SegmentID: "seg-002", Input: "seg-002.mp4", DurationSeconds: 3},
		},
	}
	command := BuildFFmpegCommand("ffmpeg", short)
	joined := strings.Join(command, " ")
	if strings.Contains(joined, "-f concat") {
		t.Fatalf("shorts compilation used concat demuxer:\n%s", joined)
	}
	if !strings.Contains(joined, "concat=n=2:v=1:a=1") {
		t.Fatalf("shorts compilation lost filtergraph concat:\n%s", joined)
	}
	if !containsArg(command, "seg-001.mp4") || !containsArg(command, "seg-002.mp4") {
		t.Fatalf("shorts compilation missing part inputs: %#v", command)
	}
}

func TestWriteFullDemoConcatListWritesBesideOutput(t *testing.T) {
	dir := t.TempDir()
	short := ShortEdit{
		Preset:       PresetGameplayPOV60,
		OutputFormat: OutputFormatLandscape16x9,
		Output:       filepath.Join(dir, "out.mp4"),
		OutputFPS:    60,
		Tickrate:     64,
		Parts: []ShortPart{
			{SegmentID: "seg-001", Input: filepath.Join(dir, "a.mp4"), DurationSeconds: 2, TickStart: 0, TickEnd: 128},
			{SegmentID: "seg-002", Input: filepath.Join(dir, "b.mp4"), DurationSeconds: 3, TickStart: 128, TickEnd: 320},
		},
	}
	if err := writeFullDemoConcatList(short); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(fullDemoConcatListPath(short))
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	if strings.Count(got, "file '") != 2 || strings.Count(got, "outpoint ") != 2 {
		t.Fatalf("list = %q, want two file+outpoint entries", got)
	}
}

func TestWriteFullDemoConcatListNoopsForShorts(t *testing.T) {
	dir := t.TempDir()
	short := ShortEdit{
		Preset:       PresetViral60Clean,
		OutputFormat: OutputFormatShort9x16,
		Output:       filepath.Join(dir, "out.mp4"),
		Parts:        []ShortPart{{Input: "a.mp4", DurationSeconds: 2}},
	}
	if err := writeFullDemoConcatList(short); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fullDemoConcatListPath(short)); !os.IsNotExist(err) {
		t.Fatalf("shorts wrote a concat list: %v", err)
	}
}

func TestFullDemoCompilationFilterKeepsOverlays(t *testing.T) {
	short := ShortEdit{
		Preset:          PresetGameplayPOV60,
		OutputFormat:    OutputFormatLandscape16x9,
		DurationSeconds: 24,
		OutputFPS:       60,
		Parts:           []ShortPart{{Input: "p1.mp4", DurationSeconds: 24, TickStart: 1000, TickEnd: 2536}},
		Tickrate:        64,
		Effects: []Effect{
			{
				Type:           EffectImage,
				Path:           "intro.png",
				Source:         "full-demo-intro",
				StartSeconds:   5,
				EndSeconds:     14,
				FadeInSeconds:  demooverlay.IntroOverlaySlideSeconds,
				FadeOutSeconds: 0.35,
				Width:          demooverlay.FrameWidth,
				Height:         demooverlay.FrameHeight,
			},
			{
				Type:         EffectImage,
				Path:         "outro.png",
				Source:       "full-demo-outro",
				StartSeconds: 16,
				EndSeconds:   24,
				Width:        demooverlay.FrameWidth,
				Height:       demooverlay.FrameHeight,
			},
		},
	}
	got := fullDemoCompilationFilter(short)
	wantDim := fmt.Sprintf(
		"[vtail]trim=start=16.000:end=24.000,gblur=sigma=%.3f,eq=brightness=%.3f[vblurred]",
		demooverlay.OutroBlurSigma, demooverlay.OutroEQBrightness,
	)
	for _, want := range []string{
		"[0:v]",
		wantDim,
		"split=2[img0srcL][img0srcR]",
		"crop=",
		"pow(1-(t-",
		"fade=t=in:st=0",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("fullDemoCompilationFilter missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "concat=n=") && strings.Contains(got, ":v=1") {
		t.Fatalf("full demo filter still concat-decoded parts:\n%s", got)
	}
}

func TestFullDemoVoiceMixConcatsOneWindowPerPart(t *testing.T) {
	short := ShortEdit{
		Preset:        PresetGameplayPOV60,
		OutputFormat:  OutputFormatLandscape16x9,
		OutputFPS:     60,
		VoiceTracks:   []string{"pov.ogg"},
		VoiceTickrate: 64,
		Tickrate:      64,
		Parts: []ShortPart{
			{Input: "a.mp4", DurationSeconds: 2, TickStart: 0, TickEnd: 128, CaptureTickStart: 0, CaptureTickEnd: 128},
			{Input: "b.mp4", DurationSeconds: 3, TickStart: 128, TickEnd: 320, CaptureTickStart: 128, CaptureTickEnd: 320},
		},
	}
	got := fullDemoVoiceMixFilter(short)
	if !strings.Contains(got, "[vt0_0]") || !strings.Contains(got, "[vt0_1]") {
		t.Fatalf("voice mix dropped a part window:\n%s", got)
	}
	if !strings.Contains(got, "concat=n=2:v=0:a=1") {
		t.Fatalf("voice mix did not concat both part windows:\n%s", got)
	}
}

func countDashI(command []string) int {
	n := 0
	for _, arg := range command {
		if arg == "-i" {
			n++
		}
	}
	return n
}

func TestFullDemoConcatsTwoFixtureRounds(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	dir := t.TempDir()
	const partDur = 12.0
	p1 := filepath.Join(dir, "recording", "segments", "seg-001.mp4")
	p2 := filepath.Join(dir, "recording", "segments", "seg-002.mp4")
	writeLavfiCapture(t, ffmpeg, p1, 1920, 1080, partDur)
	writeLavfiCapture(t, ffmpeg, p2, 1920, 1080, partDur)

	result := testRecordingResult(dir)
	result.Plan.Segments = []recording.RecordingSegment{
		{ID: "seg-001", Round: 1, TickStart: 1000, TickEnd: 1000 + int(partDur*64), Kills: []killplan.Kill{{Tick: 1400, Weapon: "ak47"}}},
		{ID: "seg-002", Round: 2, TickStart: 2000, TickEnd: 2000 + int(partDur*64), Kills: []killplan.Kill{{Tick: 2400, Weapon: "ak47"}}},
	}
	result.Artifacts = []recording.RecordingArtifact{
		{SegmentID: "seg-001", Role: "segment", Type: "video", Path: p1, SizeBytes: 1, DurationSeconds: partDur, Codec: "h264", Width: 1920, Height: 1080, FrameRate: "60/1"},
		{SegmentID: "seg-002", Role: "segment", Type: "video", Path: p2, SizeBytes: 1, DurationSeconds: partDur, Codec: "h264", Width: 1920, Height: 1080, FrameRate: "60/1"},
	}

	doc := demooverlay.Build(demooverlay.Roster{
		TargetSteamID64: "76561198148986856",
		Map:             "de_mirage",
		ScoreCT:         13,
		ScoreT:          8,
		Players: []demooverlay.RosterPlayer{
			{SteamID64: "76561198148986856", Name: "donk666", Team: "CT", Kills: 23, Deaths: 14, Assists: 4},
			{SteamID64: "9", Name: "KingwayO", Team: "T", Kills: 18, Deaths: 16},
		},
	}, nil)
	opts := testManifestOptions(dir, nil)
	opts.Preset = PresetGameplayPOV60
	opts.OutputFormat = OutputFormatLandscape16x9
	opts.CompileSegments = true
	opts.KillEffect = KillEffectClean
	opts.Transition = TransitionCut
	opts.VideoPreset = "ultrafast"
	opts.VideoCRF = 28
	opts.CoversEnabled = false
	opts.FullDemoOverlay = &doc

	manifest := mustBuildManifest(t, result, opts)
	if len(manifest.Shorts) != 1 {
		t.Fatalf("compiled shorts = %d", len(manifest.Shorts))
	}
	short := manifest.Shorts[0]
	if got := countDashI(short.FFmpegCommand); got != 3 {
		t.Fatalf("-i count = %d, want 3 (list + intro + outro)", got)
	}
	if _, err := os.Stat(fullDemoConcatListPath(short)); err != nil {
		t.Fatalf("manifest did not write concat list: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(short.Output), 0o750); err != nil {
		t.Fatal(err)
	}
	runFFmpegCommand(t, short.FFmpegCommand)
	w, h := probeVideoSize(t, ffmpeg, short.Output)
	if w != 1920 || h != 1080 {
		t.Fatalf("compiled Full Demo = %dx%d, want 1920x1080", w, h)
	}
}
