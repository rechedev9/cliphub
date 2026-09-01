package editor

import (
	"strings"
	"testing"
)

func TestAppendVideoEncodeArgsNVENC(t *testing.T) {
	args := appendVideoEncodeArgs(nil, ShortEdit{VideoEncoder: VideoEncoderNVENC, VideoCRF: 16})
	got := strings.Join(args, " ")
	for _, want := range []string{"-c:v h264_nvenc", "-preset p5", "-rc vbr", "-cq 16", "-pix_fmt yuv420p"} {
		if !strings.Contains(got, want) {
			t.Fatalf("appendVideoEncodeArgs() = %q, missing %q", got, want)
		}
	}
	if strings.Contains(got, "libx264") {
		t.Fatalf("appendVideoEncodeArgs() = %q, must not contain libx264", got)
	}
}

func TestAppendVideoEncodeArgsLibx264Default(t *testing.T) {
	args := appendVideoEncodeArgs(nil, ShortEdit{VideoPreset: "slow", VideoCRF: 16})
	got := strings.Join(args, " ")
	for _, want := range []string{"-c:v libx264", "-preset slow", "-crf 16"} {
		if !strings.Contains(got, want) {
			t.Fatalf("appendVideoEncodeArgs() = %q, missing %q", got, want)
		}
	}
}

func TestFullFrameVideoFilterSkipsScaleWhenSourceMatches(t *testing.T) {
	short := ShortEdit{
		Preset:       PresetGameplayPOV60,
		OutputFormat: OutputFormatLandscape16x9,
		SourceWidth:  1920,
		SourceHeight: 1080,
		SourceFPS:    60,
		OutputFPS:    60,
		Parts:        []ShortPart{{Input: "p1.mp4"}},
	}
	got := FullFrameVideoFilter(short)
	for _, forbidden := range []string{"scale=", "crop=", "fps=60"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("FullFrameVideoFilter() = %q, must skip %q when source matches output", got, forbidden)
		}
	}
	if !strings.Contains(got, "setsar=1") {
		t.Fatalf("FullFrameVideoFilter() = %q, want setsar=1", got)
	}
}

func TestFullFrameVideoFilterKeepsScaleWhenSourceDiffers(t *testing.T) {
	short := ShortEdit{
		Preset:       PresetGameplayPOV60,
		OutputFormat: OutputFormatLandscape16x9,
		SourceWidth:  1280,
		SourceHeight: 720,
		SourceFPS:    30,
		OutputFPS:    60,
	}
	got := FullFrameVideoFilter(short)
	for _, want := range []string{"scale=", "crop=", "fps=60"} {
		if !strings.Contains(got, want) {
			t.Fatalf("FullFrameVideoFilter() = %q, missing %q", got, want)
		}
	}
}
