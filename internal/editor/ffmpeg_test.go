package editor

import (
	"os"
	"strings"
	"testing"
)

func TestCommandWithFilterComplexScriptKeepsShortFilterInline(t *testing.T) {
	command := []string{"ffmpeg", "-filter_complex", "scale=1080:1920", "out.mp4"}

	got, cleanup, err := commandWithFilterComplexScript(command)
	if err != nil {
		t.Fatalf("commandWithFilterComplexScript() error = %v", err)
	}
	defer cleanup()

	if strings.Join(got, "\x00") != strings.Join(command, "\x00") {
		t.Fatalf("commandWithFilterComplexScript() = %#v, want original command", got)
	}
}

func TestCommandWithFilterComplexScriptSpillsLongFilter(t *testing.T) {
	filter := strings.Repeat("scale=1080:1920,", filterComplexScriptThreshold)
	command := []string{"ffmpeg", "-filter_complex", filter, "-map", "[v]", "out.mp4"}

	got, cleanup, err := commandWithFilterComplexScript(command)
	if err != nil {
		t.Fatalf("commandWithFilterComplexScript() error = %v", err)
	}

	if got[1] != "-filter_complex_script" {
		t.Fatalf("filter flag = %q, want -filter_complex_script", got[1])
	}
	if got[2] == filter {
		t.Fatalf("filter script path still contains inline filter")
	}
	b, err := os.ReadFile(got[2])
	if err != nil {
		t.Fatalf("read filter script: %v", err)
	}
	if string(b) != filter {
		t.Fatalf("filter script contents changed")
	}

	cleanup()
	if _, err := os.Stat(got[2]); !os.IsNotExist(err) {
		t.Fatalf("filter script still exists after cleanup: %v", err)
	}
}

func singleClipKillfeedShort() ShortEdit {
	return ShortEdit{
		Preset:          PresetViral60Clean,
		Input:           "in.mp4",
		Output:          "out.mp4",
		DurationSeconds: 6.078,
		TailTrimSeconds: 1.5,
		Kills:           []KillCue{{TimeSeconds: 4.578}},
		Effects: []Effect{{
			Type:         EffectKillfeed,
			StartSeconds: 4.228,
			EndSeconds:   6.078,
			AtSeconds:    4.578,
			X:            "W-w-18",
			Y:            "300",
			Width:        430,
			CropX:        1558,
			CropY:        64,
			CropWidth:    360,
			CropHeight:   110,
			Source:       "edit-request",
		}},
	}
}

func TestBuildFFmpegCommandKillfeedUsesFilterComplex(t *testing.T) {
	short := singleClipKillfeedShort()
	command := strings.Join(BuildFFmpegCommand("ffmpeg", short), " ")
	if strings.Contains(command, "-vf") {
		t.Fatalf("command = %q, want no -vf when killfeed overlays are present", command)
	}
	for _, want := range []string{
		"-filter_complex",
		"split=2[main][kfsrc0]",
		"overlay=x=W-w-18:y=300",
		"format=yuv420p[v]",
		"-map [v] -map 0:a?",
		"-t 6.078",
	} {
		if !strings.Contains(command, want) {
			t.Fatalf("command = %q, want it to contain %q", command, want)
		}
	}
}

func TestBuildFFmpegCommandWithoutKillfeedKeepsVfPath(t *testing.T) {
	short := singleClipKillfeedShort()
	short.Effects = nil
	short.TailTrimSeconds = 0
	command := BuildFFmpegCommand("ffmpeg", short)
	joined := strings.Join(command, " ")
	if !strings.Contains(joined, "-vf") || strings.Contains(joined, "-filter_complex") {
		t.Fatalf("command = %q, want the historical -vf path", joined)
	}
	for _, arg := range command {
		if arg == "-t" {
			t.Fatalf("command = %q, want no -t without tail trim", joined)
		}
	}
}

func TestBuildMusicFFmpegCommandKillfeedAndTailTrim(t *testing.T) {
	short := singleClipKillfeedShort()
	short.MusicPath = "music.mp3"
	command := BuildFFmpegCommand("ffmpeg", short)
	joined := strings.Join(command, " ")
	for _, want := range []string{
		"split=2[main][kfsrc0]",
		"[0:a]volume=0.20[game]",
		"[1:a]volume=1.00[music]",
		"amix=inputs=2:duration=first:dropout_transition=0:normalize=0",
		"-t 6.078",
		"-shortest",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("command = %q, want it to contain %q", joined, want)
		}
	}
	if command[len(command)-1] != "out.mp4" || command[len(command)-2] != "-shortest" {
		t.Fatalf("command tail = %v, want ... -shortest out.mp4", command[len(command)-3:])
	}

	custom := singleClipKillfeedShort()
	custom.MusicPath = "music.mp3"
	custom.MusicVolume = 0.35
	customJoined := strings.Join(BuildFFmpegCommand("ffmpeg", custom), " ")
	if !strings.Contains(customJoined, "[1:a]volume=0.35[music]") {
		t.Fatalf("command = %q, want custom music volume 0.35", customJoined)
	}
	if strings.Contains(customJoined, "[1:a]volume=1.00[music]") {
		t.Fatalf("command = %q, want no default 1.00 music volume with a custom volume", customJoined)
	}
}

func TestBuildFFmpegCommandCoverFirstFrameFreezesCoverFrame(t *testing.T) {
	short := singleClipKillfeedShort()
	short.Effects = nil
	short.CoverFirstFrame = true
	short.CoverTimeSeconds = 4.458
	command := strings.Join(BuildFFmpegCommand("ffmpeg", short), " ")
	if strings.Contains(command, "-vf") {
		t.Fatalf("command = %q, want no -vf with the cover first-frame freeze", command)
	}
	for _, want := range []string{
		"-filter_complex",
		"split=2[cfmain][cfsrc]",
		"[cfsrc]trim=start=4.458:duration=0.050,loop=loop=-1:size=1:start=0,setpts=N/60/TB,trim=end_frame=2[cffreeze]",
		"[cfmain][cffreeze]overlay=eof_action=pass:enable='lt(n,2)'[cfcover]",
		"[cfcover]format=yuv420p[v]",
		"-map [v] -map 0:a?",
	} {
		if !strings.Contains(command, want) {
			t.Fatalf("command = %q, want it to contain %q", command, want)
		}
	}
}

func TestBuildFFmpegCommandCoverFirstFrameClampsSampleNearClipEnd(t *testing.T) {
	short := singleClipKillfeedShort()
	short.Effects = nil
	short.CoverFirstFrame = true
	short.CoverTimeSeconds = short.DurationSeconds
	command := strings.Join(BuildFFmpegCommand("ffmpeg", short), " ")
	if !strings.Contains(command, "[cfsrc]trim=start=5.978:duration=0.050") {
		t.Fatalf("command = %q, want the freeze sample clamped 0.1s before the clip end", command)
	}
}

func TestBuildFFmpegCommandCoverFirstFrameChainsAfterKillfeed(t *testing.T) {
	short := singleClipKillfeedShort()
	short.CoverFirstFrame = true
	short.CoverTimeSeconds = 4.458
	command := strings.Join(BuildFFmpegCommand("ffmpeg", short), " ")
	killfeedAt := strings.Index(command, "[kfsrc0]")
	coverAt := strings.Index(command, "split=2[cfmain][cfsrc]")
	if killfeedAt < 0 || coverAt < 0 || coverAt < killfeedAt {
		t.Fatalf("command = %q, want the cover freeze after the killfeed overlay chain", command)
	}
	if !strings.Contains(command, "[vkf0]split=2[cfmain][cfsrc]") {
		t.Fatalf("command = %q, want the cover freeze fed by the killfeed output", command)
	}
}

func TestBuildMusicFFmpegCommandCoverFirstFrame(t *testing.T) {
	short := singleClipKillfeedShort()
	short.Effects = nil
	short.MusicPath = "music.mp3"
	short.CoverFirstFrame = true
	short.CoverTimeSeconds = 4.458
	command := strings.Join(BuildFFmpegCommand("ffmpeg", short), " ")
	for _, want := range []string{
		"split=2[cfmain][cfsrc]",
		"[cfcover]format=yuv420p[v]",
		"amix=inputs=2:duration=first:dropout_transition=0:normalize=0",
	} {
		if !strings.Contains(command, want) {
			t.Fatalf("command = %q, want it to contain %q", command, want)
		}
	}
}

func TestBuildCompilationFFmpegCommandCoverFirstFrame(t *testing.T) {
	short := ShortEdit{
		Preset:           PresetViral60Clean,
		Output:           "out.mp4",
		DurationSeconds:  11.078,
		CoverFirstFrame:  true,
		CoverTimeSeconds: 4.458,
		Parts: []ShortPart{
			{SegmentID: "seg-001", Input: "p1.mp4", DurationSeconds: 6.078, Kills: []KillCue{{TimeSeconds: 4.578}}},
			{SegmentID: "seg-002", Input: "p2.mp4", DurationSeconds: 5},
		},
	}
	command := strings.Join(BuildCompilationFFmpegCommand("ffmpeg", short), " ")
	for _, want := range []string{
		"[vbase]split=2[cfmain][cfsrc]",
		"[cfsrc]trim=start=4.458:duration=0.050",
		"[cfcover]format=yuv420p[v]",
	} {
		if !strings.Contains(command, want) {
			t.Fatalf("command = %q, want it to contain %q", command, want)
		}
	}
	if strings.Contains(command, "[catv]format=yuv420p[v]") {
		t.Fatalf("command = %q, want the base filter routed through [vbase]", command)
	}
}

func TestBuildCompilationFFmpegCommandWithoutCoverFirstFrameUnchanged(t *testing.T) {
	short := ShortEdit{
		Preset:          PresetViral60Clean,
		Output:          "out.mp4",
		DurationSeconds: 11.078,
		Parts: []ShortPart{
			{SegmentID: "seg-001", Input: "p1.mp4", DurationSeconds: 6.078},
		},
	}
	command := strings.Join(BuildCompilationFFmpegCommand("ffmpeg", short), " ")
	if strings.Contains(command, "cfmain") || strings.Contains(command, "[vbase]") {
		t.Fatalf("command = %q, want the historical single-clause video path", command)
	}
}

func TestBuildCompilationFFmpegCommandTailTrimsPartInputs(t *testing.T) {
	short := ShortEdit{
		Preset:          PresetViral60Clean,
		Output:          "out.mp4",
		DurationSeconds: 11.078,
		TailTrimSeconds: 1.5,
		Parts: []ShortPart{
			{SegmentID: "seg-001", Input: "p1.mp4", DurationSeconds: 6.078, Kills: []KillCue{{TimeSeconds: 4.578}}},
			{SegmentID: "seg-002", Input: "p2.mp4", DurationSeconds: 5},
		},
	}
	command := strings.Join(BuildCompilationFFmpegCommand("ffmpeg", short), " ")
	if !strings.Contains(command, "-t 6.078 -i p1.mp4") {
		t.Fatalf("command = %q, want the kill part trimmed at input level", command)
	}
	if strings.Contains(command, "-t 5.000") {
		t.Fatalf("command = %q, want no trim on the kill-less part", command)
	}
}

func TestCompilationPostConcatFilter(t *testing.T) {
	base := ShortEdit{
		Preset:          PresetViral60Clean,
		OutputFPS:       60,
		DurationSeconds: 10,
	}
	tests := []struct {
		name        string
		effects     []Effect
		wantContain []string
		// wantFullChain asserts the geometry/fps chain from VideoFilter runs
		// again post-concat (only expected when a zoom effect is present).
		wantFullChain bool
	}{
		{
			name:        "no effects skips the geometry chain already applied per part",
			wantContain: []string{"format=yuv420p"},
		},
		{
			name: "grade effect applies without re-scaling/re-cropping",
			effects: []Effect{
				{Type: EffectGrade, Contrast: 1.1, Saturation: 1.05, Gamma: 1},
			},
			wantContain: []string{"eq=contrast=1.100:saturation=1.050:gamma=1.000", "format=yuv420p"},
		},
		{
			name: "zoom effect needs the dynamic scale, keeps the full chain",
			effects: []Effect{
				{Type: EffectZoom, StartSeconds: 1, EndSeconds: 2, AtSeconds: 1.5, Scale: 1.2},
			},
			wantFullChain: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			short := base
			short.Effects = tt.effects
			got := compilationPostConcatFilter(short)
			if tt.wantFullChain {
				if want := VideoFilter(short); got != want {
					t.Fatalf("compilationPostConcatFilter() = %q, want the full VideoFilter chain %q", got, want)
				}
				return
			}
			if strings.Contains(got, "scale=") || strings.Contains(got, "crop=") || strings.Contains(got, "fps=") {
				t.Fatalf("compilationPostConcatFilter() = %q, want no re-applied geometry/fps chain", got)
			}
			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Fatalf("compilationPostConcatFilter() = %q, want it to contain %q", got, want)
				}
			}
		})
	}
}

func TestBuildCompilationFFmpegCommandMixesVoiceTracks(t *testing.T) {
	short := ShortEdit{
		Preset:        PresetViral60Clean,
		Output:        "out.mp4",
		VoiceTracks:   []string{"a.ogg", "b.ogg"},
		VoiceTickrate: 64,
		Parts: []ShortPart{
			{SegmentID: "seg-001", Input: "p1.mp4", DurationSeconds: 2, TickStart: 640, TickEnd: 1280, CaptureTickStart: 768, CaptureTickEnd: 1280},
		},
	}
	command := strings.Join(BuildCompilationFFmpegCommand("ffmpeg", short), " ")
	for _, want := range []string{"-i a.ogg", "-i b.ogg", "atrim=start=12.000000:end=20.000000", "volume=0.85", "amix=inputs=2:duration=first", "[pav0]"} {
		if !strings.Contains(command, want) {
			t.Fatalf("command = %q, missing %q", command, want)
		}
	}
}

func TestAudioMixVolumes(t *testing.T) {
	gameHalf := 0.50
	gameMute := 0.0
	voiceFull := 1.00
	voiceLow := 0.40
	part := []ShortPart{{SegmentID: "seg-001", Input: "p1.mp4", DurationSeconds: 2, TickStart: 640, TickEnd: 1280, CaptureTickStart: 768, CaptureTickEnd: 1280}}
	tests := []struct {
		name    string
		short   ShortEdit
		want    []string
		notWant []string
	}{
		{
			name: "single-clip custom game volume",
			short: ShortEdit{
				Input: "in.mp4", Output: "out.mp4", MusicPath: "music.mp3",
				GameVolume: &gameHalf, DurationSeconds: 2,
			},
			want:    []string{"[0:a]volume=0.50[game]", "[1:a]volume=1.00[music]"},
			notWant: []string{"[0:a]volume=0.20[game]"},
		},
		{
			name: "single-clip muted game audio",
			short: ShortEdit{
				Input: "in.mp4", Output: "out.mp4", MusicPath: "music.mp3",
				GameVolume: &gameMute, DurationSeconds: 2,
			},
			want: []string{"[0:a]volume=0.00[game]"},
		},
		{
			name: "compilation custom game volume without voice",
			short: ShortEdit{
				Output: "out.mp4", MusicPath: "music.mp3", GameVolume: &gameHalf, Parts: part,
			},
			want:    []string{"[gamea]volume=0.50[game]"},
			notWant: []string{"[gamea]volume=0.20[game]", "[gamea]anull[game]"},
		},
		{
			name: "legacy music plus voice stays coupled",
			short: ShortEdit{
				Output: "out.mp4", MusicPath: "music.mp3",
				VoiceTracks: []string{"a.ogg"}, VoiceTickrate: 64, Parts: part,
			},
			want:    []string{"volume=0.85", "[gamea]volume=0.20[game]", "[pa0][vmix0]amix="},
			notWant: []string{"[gamea]anull[game]", "[pa0]volume="},
		},
		{
			name: "explicit volumes decouple voice from game duck",
			short: ShortEdit{
				Output: "out.mp4", MusicPath: "music.mp3",
				GameVolume: &gameHalf, VoiceVolume: &voiceFull,
				VoiceTracks: []string{"a.ogg"}, VoiceTickrate: 64, Parts: part,
			},
			want:    []string{"volume=1.00[vt0_0]", "[pa0]volume=0.50[pag0]", "[pag0][vmix0]amix=", "[gamea]anull[game]"},
			notWant: []string{"volume=0.85", "[gamea]volume=0.20[game]", "[gamea]volume=0.50[game]"},
		},
		{
			name: "custom voice volume without music",
			short: ShortEdit{
				Output: "out.mp4", VoiceVolume: &voiceLow,
				VoiceTracks: []string{"a.ogg"}, VoiceTickrate: 64, Parts: part,
			},
			want:    []string{"volume=0.40[vt0_0]"},
			notWant: []string{"volume=0.85", "[pa0]volume="},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var command string
			if len(tt.short.Parts) > 0 {
				command = strings.Join(BuildCompilationFFmpegCommand("ffmpeg", tt.short), " ")
			} else {
				command = strings.Join(BuildFFmpegCommand("ffmpeg", tt.short), " ")
			}
			for _, want := range tt.want {
				if !strings.Contains(command, want) {
					t.Fatalf("command = %q, missing %q", command, want)
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(command, notWant) {
					t.Fatalf("command = %q, unexpectedly contains %q", command, notWant)
				}
			}
		})
	}
}
