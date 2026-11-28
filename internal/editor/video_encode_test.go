package editor

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeFakeHardwareFFmpeg(t *testing.T, advertised []string, usableCodec string, counterFile string) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, "ffmpeg.cmd")
		var lines []string
		lines = append(lines, "@echo off")
		if counterFile != "" {
			lines = append(lines, "echo x>>\""+counterFile+"\"")
		}
		lines = append(lines, "if \"%~2\"==\"-encoders\" (")
		for _, enc := range advertised {
			lines = append(lines, "echo  V....D "+enc)
		}
		lines = append(lines, "exit /b 0")
		lines = append(lines, ")")
		if usableCodec != "" {
			lines = append(lines, "echo %* | findstr /C:\""+usableCodec+"\" >nul")
			lines = append(lines, "if not errorlevel 1 exit /b 0")
		}
		lines = append(lines, "echo encoder unavailable 1>&2")
		lines = append(lines, "exit /b 1")
		if err := os.WriteFile(path, []byte(strings.Join(lines, "\r\n")+"\r\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	path := filepath.Join(dir, "ffmpeg")
	var lines []string
	lines = append(lines, "#!/bin/sh")
	if counterFile != "" {
		lines = append(lines, "echo x >> \""+counterFile+"\"")
	}
	lines = append(lines, "if [ \"$2\" = \"-encoders\" ]; then")
	for _, enc := range advertised {
		lines = append(lines, "  echo ' V....D "+enc+"'")
	}
	lines = append(lines, "  exit 0")
	lines = append(lines, "fi")
	if usableCodec != "" {
		lines = append(lines, "case \"$*\" in *\""+usableCodec+"\"*) exit 0 ;; esac")
	}
	lines = append(lines, "echo 'encoder unavailable' >&2")
	lines = append(lines, "exit 1")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestValidateVideoEncoder(t *testing.T) {
	for _, valid := range []string{"", VideoEncoderAuto, VideoEncoderNVENC, VideoEncoderAMF, VideoEncoderQSV} {
		if err := validateVideoEncoder(valid); err != nil {
			t.Errorf("validateVideoEncoder(%q) = %v, want nil", valid, err)
		}
	}
	if err := validateVideoEncoder("unknown-codec"); err == nil {
		t.Fatal("validateVideoEncoder(unknown) = nil, want error")
	}
}

func TestAppendVideoEncodeArgsAMFAndQSV(t *testing.T) {
	amf := strings.Join(appendVideoEncodeArgs(nil, ShortEdit{VideoEncoder: VideoEncoderAMF, VideoCRF: 16}), " ")
	for _, want := range []string{"-c:v h264_amf", "-quality balanced", "-rc cqp", "-qp_i 20", "-qp_p 20", "-pix_fmt yuv420p"} {
		if !strings.Contains(amf, want) {
			t.Fatalf("AMF args = %q, missing %q", amf, want)
		}
	}

	qsv := strings.Join(appendVideoEncodeArgs(nil, ShortEdit{VideoEncoder: VideoEncoderQSV, VideoCRF: 18}), " ")
	for _, want := range []string{"-c:v h264_qsv", "-preset medium", "-global_quality 22", "-pix_fmt yuv420p"} {
		if !strings.Contains(qsv, want) {
			t.Fatalf("QSV args = %q, missing %q", qsv, want)
		}
	}
}

func TestDetectHardwareEncoderCascadeAndCaching(t *testing.T) {
	all := []string{"h264_nvenc", "h264_amf", "h264_qsv"}

	t.Run("prefers nvenc first", func(t *testing.T) {
		ffmpeg := writeFakeHardwareFFmpeg(t, all, "h264_nvenc", "")
		if got := DetectHardwareEncoder(ffmpeg); got != VideoEncoderNVENC {
			t.Fatalf("DetectHardwareEncoder() = %q, want %q", got, VideoEncoderNVENC)
		}
	})

	t.Run("falls back to amf when nvenc fails", func(t *testing.T) {
		ffmpeg := writeFakeHardwareFFmpeg(t, all, "h264_amf", "")
		if got := DetectHardwareEncoder(ffmpeg); got != VideoEncoderAMF {
			t.Fatalf("DetectHardwareEncoder() = %q, want %q", got, VideoEncoderAMF)
		}
	})

	t.Run("falls back to qsv when nvenc and amf fail", func(t *testing.T) {
		ffmpeg := writeFakeHardwareFFmpeg(t, all, "h264_qsv", "")
		if got := DetectHardwareEncoder(ffmpeg); got != VideoEncoderQSV {
			t.Fatalf("DetectHardwareEncoder() = %q, want %q", got, VideoEncoderQSV)
		}
	})

	t.Run("falls back to empty libx264 when none work", func(t *testing.T) {
		ffmpeg := writeFakeHardwareFFmpeg(t, all, "", "")
		if got := DetectHardwareEncoder(ffmpeg); got != "" {
			t.Fatalf("DetectHardwareEncoder() = %q, want empty", got)
		}
	})

	t.Run("caches result per ffmpeg path", func(t *testing.T) {
		counter := filepath.Join(t.TempDir(), "calls.txt")
		ffmpeg := writeFakeHardwareFFmpeg(t, all, "h264_nvenc", counter)
		first := DetectHardwareEncoder(ffmpeg)
		before, err := os.ReadFile(counter)
		if err != nil {
			t.Fatalf("read counter: %v", err)
		}
		second := DetectHardwareEncoder(ffmpeg)
		after, err := os.ReadFile(counter)
		if err != nil {
			t.Fatalf("read counter: %v", err)
		}
		if first != VideoEncoderNVENC || second != VideoEncoderNVENC {
			t.Fatalf("results = %q, %q; want %q", first, second, VideoEncoderNVENC)
		}
		if string(before) != string(after) {
			t.Fatalf("second DetectHardwareEncoder call invoked ffmpeg again (%d -> %d bytes)", len(before), len(after))
		}
	})

	t.Run("auto mode uses detected encoder or libx264", func(t *testing.T) {
		amfFFmpeg := writeFakeHardwareFFmpeg(t, all, "h264_amf", "")
		args := appendVideoEncodeArgs([]string{amfFFmpeg}, ShortEdit{VideoEncoder: VideoEncoderAuto, VideoCRF: 18})
		if got := strings.Join(args, " "); !strings.Contains(got, "-c:v h264_amf") {
			t.Fatalf("auto with AMF = %q, want h264_amf", got)
		}

		cpuFFmpeg := writeFakeHardwareFFmpeg(t, all, "", "")
		args = appendVideoEncodeArgs([]string{cpuFFmpeg}, ShortEdit{VideoEncoder: VideoEncoderAuto, VideoCRF: 18, VideoPreset: "fast"})
		if got := strings.Join(args, " "); !strings.Contains(got, "-c:v libx264") {
			t.Fatalf("auto with CPU fallback = %q, want libx264", got)
		}
	})
}

func TestAppendVideoEncodeArgsNVENC(t *testing.T) {
	args := appendVideoEncodeArgs(nil, ShortEdit{VideoEncoder: VideoEncoderNVENC, VideoCRF: 16})
	got := strings.Join(args, " ")
	for _, want := range []string{"-c:v h264_nvenc", "-preset p5", "-rc vbr", "-cq 20", "-pix_fmt yuv420p"} {
		if !strings.Contains(got, want) {
			t.Fatalf("appendVideoEncodeArgs() = %q, missing %q", got, want)
		}
	}
	if strings.Contains(got, "libx264") {
		t.Fatalf("appendVideoEncodeArgs() = %q, must not contain libx264", got)
	}
	if got := strings.Join(appendVideoEncodeArgs(nil, ShortEdit{VideoEncoder: VideoEncoderNVENC, VideoCRF: 50}), " "); !strings.Contains(got, "-cq 51 ") {
		t.Fatalf("appendVideoEncodeArgs(crf 50) = %q, want the CQ capped at 51", got)
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
