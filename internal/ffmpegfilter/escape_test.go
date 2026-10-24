package ffmpegfilter

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
)

var printedValue = regexp.MustCompile(`\] k=(.*?)\r?\n`)

// The metadata filter prints the value FFmpeg parsed, so each case checks the real
// parser returns the input unchanged and no injected filter wrote a file.
func TestValueSurvivesFFmpegParsing(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	values := []string{
		"plain",
		"it's ok",
		"',metadata=print:file=planted:key='",
		`x'::fontfile=a.ttf;metadata=mode=print:file=planted:key='`,
		`a'\:textfile=planted\:x='`,
		`back\slash\\ and \'`,
		"[in];[out],tail",
		"79% %{pts} %%",
		"  padded  ",
		"'",
		`C:\Users\O'Brien\font.ttf`,
	}
	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			filter := "metadata=mode=add:key=k:value=" + Value(value) + ",metadata=mode=print"
			cmd := exec.Command(ffmpeg, "-hide_banner", "-f", "lavfi", "-i", "color=black:s=16x16:d=0.04",
				"-vf", filter, "-frames:v", "1", "-f", "null", "-")
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("ffmpeg rejected Value(%q): %v\n%s", value, err, out)
			}
			match := printedValue.FindSubmatch(out)
			if match == nil || string(match[1]) != value {
				t.Errorf("Value(%q) parsed as %q", value, match)
			}
			if _, err := os.Stat(filepath.Join(dir, "planted")); err == nil {
				t.Error("an escaped value made FFmpeg write a file")
			}
		})
	}
}

func TestPathUsesForwardSlashes(t *testing.T) {
	if got, want := Path(`C:\Fonts\a.ttf`), `\'C:/Fonts/a.ttf\'`; got != want {
		t.Fatalf("Path = %q, want %q", got, want)
	}
}
