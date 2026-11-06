package vodfetch

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestParseProgressLine(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		wantPercent   float64
		wantNoPercent bool
		want          Progress
	}{
		{
			// Real yt-dlp 2026.10 line for a YouTube video track.
			name:        "byte total",
			line:        "cliphub-progress av01.0.00M.08 64512 223779 NA NA NA 0",
			wantPercent: 64512.0 / 223779 * 100,
			want:        Progress{DownloadedBytes: 64512, TotalBytes: 223779},
		},
		{
			name:        "audio track",
			line:        "cliphub-progress none 309288 309288 NA NA NA NA",
			wantPercent: 100,
			want:        Progress{AudioOnly: true, DownloadedBytes: 309288, TotalBytes: 309288},
		},
		{
			// Real Twitch VOD (HLS) line: fragments are exact, the size is an estimate.
			name:        "hls fragments",
			line:        "cliphub-progress avc1.4D400C 160391060 NA 1359970186.6666667 434 3683 177.97",
			wantPercent: 434.0 / 3683 * 100,
			want:        Progress{DownloadedBytes: 160391060, TotalBytes: 1359970186, TotalEstimated: true, ETASeconds: 177},
		},
		{
			name:          "live without length",
			line:          "cliphub-progress avc1 5000 NA NA 12 NA NA",
			wantNoPercent: true,
			want:          Progress{DownloadedBytes: 5000},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseProgressLine(tt.line)
			if !ok {
				t.Fatalf("ParseProgressLine(%q) not ok", tt.line)
			}
			if tt.wantNoPercent != (got.Percent == nil) {
				t.Fatalf("Percent = %v, want nil %v", got.Percent, tt.wantNoPercent)
			}
			if got.Percent != nil && (*got.Percent < tt.wantPercent-0.001 || *got.Percent > tt.wantPercent+0.001) {
				t.Errorf("Percent = %v, want %v", *got.Percent, tt.wantPercent)
			}
			got.Percent = nil
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseProgressLineRejectsOtherOutput(t *testing.T) {
	for _, line := range []string{"Me at the zoo", "", "cliphub-progress avc1 NA NA NA NA NA NA", "cliphub-progress a b"} {
		if _, ok := ParseProgressLine(line); ok {
			t.Errorf("ParseProgressLine(%q) ok, want false", line)
		}
	}
}

func TestDownloadTitleSkipsProgressLines(t *testing.T) {
	stdout := "cliphub-progress none 1 2 NA NA NA 0\nMe at the zoo\ncliphub-progress none 2 2 NA NA NA NA\n"
	if got := downloadTitle(stdout); got != "Me at the zoo" {
		t.Errorf("downloadTitle() = %q", got)
	}
}

func TestLineWriterSplitsChunks(t *testing.T) {
	var lines []string
	w := &lineWriter{onLine: func(l string) { lines = append(lines, l) }}
	for _, chunk := range []string{"cliphub-pro", "gress a\r\nsecond\nthi", "rd"} {
		if _, err := w.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	if len(lines) != 2 || lines[0] != "cliphub-progress a" || lines[1] != "second" {
		t.Errorf("lines = %q", lines)
	}
	if w.String() != "cliphub-progress a\r\nsecond\nthird" {
		t.Errorf("String() = %q", w.String())
	}
}

// TestExecRunnerReportsProgress drives the default runner through a real
// process, so the stdout wiring is covered without yt-dlp or the network.
func TestExecRunnerReportsProgress(t *testing.T) {
	shell, flag := "sh", "-c"
	if runtime.GOOS == "windows" {
		shell, flag = "cmd", "/c"
	}
	if _, err := exec.LookPath(shell); err != nil {
		t.Skip(shell + " not available")
	}
	var got []Progress
	f := Fetcher{OnProgress: func(p Progress) { got = append(got, p) }}
	_, _, err := f.runner().Run(context.Background(), filepath.Clean(t.TempDir()), shell, flag, "echo cliphub-progress none 5 10 NA NA NA 3")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].DownloadedBytes != 5 || got[0].Percent == nil || *got[0].Percent != 50 {
		t.Errorf("progress = %+v", got)
	}
}
