package recording

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEncoderFFmpegCodec(t *testing.T) {
	tests := []struct {
		encoder string
		want    string
	}{
		{encoder: "", want: "libx264"},
		{encoder: EncoderLibx264, want: "libx264"},
		{encoder: EncoderNVENC, want: "h264_nvenc"},
		{encoder: EncoderAMF, want: "h264_amf"},
		{encoder: EncoderQSV, want: "h264_qsv"},
	}
	for _, tt := range tests {
		if got := EncoderFFmpegCodec(tt.encoder); got != tt.want {
			t.Errorf("EncoderFFmpegCodec(%q) = %q, want %q", tt.encoder, got, tt.want)
		}
	}
}

func TestHLAEStreamFFmpeg(t *testing.T) {
	writeFile := func(t *testing.T, path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name       string
		configured string
		setup      func(t *testing.T, hlaeDir string) string // returns want; "" = FindFFmpeg fallback
	}{
		{name: "bundled bin wins over ini", setup: func(t *testing.T, dir string) string {
			bundled := filepath.Join(dir, "ffmpeg", "bin", "ffmpeg.exe")
			writeFile(t, bundled, "fake")
			writeFile(t, filepath.Join(dir, "ffmpeg", "ffmpeg.ini"), "[Ffmpeg]\r\nPath=C:\\other\\ffmpeg.exe\r\n")
			return bundled
		}},
		{name: "ini path with CRLF", setup: func(t *testing.T, dir string) string {
			writeFile(t, filepath.Join(dir, "ffmpeg", "ffmpeg.ini"), "[Ffmpeg]\r\nPath=C:\\tools\\ffmpeg.exe\r\n")
			return `C:\tools\ffmpeg.exe`
		}},
		// zv-recorder rewrites the ini with the ffmpeg Studio provisions, so the probe tests that binary.
		{name: "configured ffmpeg wins over an older ini", configured: `C:\studio\ffmpeg.exe`, setup: func(t *testing.T, dir string) string {
			writeFile(t, filepath.Join(dir, "ffmpeg", "ffmpeg.ini"), "[Ffmpeg]\r\nPath=C:\\winget\\ffmpeg.exe\r\n")
			return `C:\studio\ffmpeg.exe`
		}},
		{name: "bundled bin wins over configured ffmpeg", configured: `C:\studio\ffmpeg.exe`, setup: func(t *testing.T, dir string) string {
			bundled := filepath.Join(dir, "ffmpeg", "bin", "ffmpeg.exe")
			writeFile(t, bundled, "fake")
			return bundled
		}},
		// Without an ffmpeg folder zv-recorder writes no ini, so HLAE never sees the configured one.
		{name: "configured ffmpeg ignored without an ffmpeg folder", configured: `C:\studiofmpeg.exe`, setup: func(t *testing.T, dir string) string {
			return ""
		}},
		{name: "no config falls back to PATH ffmpeg", setup: func(t *testing.T, dir string) string {
			return ""
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			want := tt.setup(t, dir)
			if want == "" {
				want = FindFFmpeg()
			}
			if got := HLAEStreamFFmpeg(filepath.Join(dir, "HLAE.exe"), tt.configured); got != want {
				t.Fatalf("HLAEStreamFFmpeg = %q, want %q", got, want)
			}
		})
	}
}

func TestCheckEncoderSupported(t *testing.T) {
	t.Run("software encoders need no probe", func(t *testing.T) {
		for _, encoder := range []string{"", EncoderLibx264} {
			if err := CheckEncoderSupported("", encoder); err != nil {
				t.Errorf("CheckEncoderSupported(%q) = %v, want nil", encoder, err)
			}
		}
	})

	t.Run("unknown encoder rejected", func(t *testing.T) {
		if err := CheckEncoderSupported("ffmpeg.exe", "bogus-264"); err == nil {
			t.Fatal("CheckEncoderSupported(bogus) succeeded, want error")
		}
	})

	t.Run("hardware encoder with no ffmpeg fails clearly", func(t *testing.T) {
		err := CheckEncoderSupported("", EncoderNVENC)
		if err == nil || !strings.Contains(err.Error(), "ffmpeg not found") {
			t.Fatalf("CheckEncoderSupported(\"\", nvenc) = %v, want ffmpeg-not-found error", err)
		}
	})

	t.Run("resolved ffmpeg provides the encoder", func(t *testing.T) {
		ffmpeg, err := exec.LookPath("ffmpeg")
		if err != nil {
			t.Skip("ffmpeg not on PATH; skipping encoder presence check")
		}
		// Independently confirm which hardware H.264 encoders the resolved binary
		// reports, then assert CheckEncoderSupported agrees on every one.
		out, err := exec.Command(ffmpeg, "-hide_banner", "-encoders").Output()
		if err != nil {
			t.Fatalf("ffmpeg -encoders failed: %v", err)
		}
		catalog := string(out)
		for _, encoder := range []string{EncoderNVENC, EncoderAMF, EncoderQSV} {
			want := EncoderFFmpegCodec(encoder)
			got := CheckEncoderSupported(ffmpeg, encoder)
			if strings.Contains(catalog, want) && got != nil {
				t.Errorf("CheckEncoderSupported(%q) = %v, but %s advertises %s", encoder, got, ffmpeg, want)
			}
			if !strings.Contains(catalog, want) && got == nil {
				t.Errorf("CheckEncoderSupported(%q) = nil, but %s does not advertise %s", encoder, ffmpeg, want)
			}
		}
	})
}

// fakeEncoderFFmpeg lists h264_nvenc and ends every other run with exitCode.
func fakeEncoderFFmpeg(t *testing.T, exitCode string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ffmpeg")
	body := "#!/bin/sh\nif [ \"$2\" = \"-encoders\" ]; then echo ' V....D h264_nvenc'; exit 0; fi\n" +
		"echo 'Driver does not support the required nvenc API version' >&2\nexit " + exitCode + "\n"
	if runtime.GOOS == "windows" {
		path = filepath.Join(dir, "ffmpeg.cmd")
		body = "@echo off\r\nif \"%~2\"==\"-encoders\" (\r\necho  V....D h264_nvenc\r\nexit /b 0\r\n)\r\n" +
			"echo Driver does not support the required nvenc API version 1>&2\r\nexit /b " + exitCode + "\r\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// A build lists NVENC on any machine; an old GPU driver only refuses it when
// the encoder opens, which used to fail every render after a good capture.
func TestCheckEncoderUsableNeedsTheEncoderToOpen(t *testing.T) {
	if err := CheckEncoderUsable(fakeEncoderFFmpeg(t, "0"), EncoderNVENC); err != nil {
		t.Fatalf("an encoder that opens = %v, want nil", err)
	}
	refused := fakeEncoderFFmpeg(t, "1")
	if err := CheckEncoderSupported(refused, EncoderNVENC); err != nil {
		t.Fatalf("the listing check = %v, want nil", err)
	}
	err := CheckEncoderUsable(refused, EncoderNVENC)
	if err == nil || !strings.Contains(err.Error(), "nvenc API version") {
		t.Fatalf("an encoder the driver refuses = %v, want the driver's reason", err)
	}
	if err := CheckEncoderUsable("", EncoderLibx264); err != nil {
		t.Fatalf("software encoding = %v, want nil", err)
	}
}
