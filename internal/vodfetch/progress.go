package vodfetch

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// ProgressFunc reports downloaded and total bytes from yt-dlp stderr.
type ProgressFunc func(done, total int64)

var ytdlpProgressRe = regexp.MustCompile(`(?i)\[download\]\s+([0-9.]+)%\s+of\s+~?\s*([0-9.]+)\s*(KiB|MiB|GiB|TiB|B)\b`)

// ParseYtdlpProgressLine reads one `--newline` progress line into bytes.
func ParseYtdlpProgressLine(line string) (done, total int64, ok bool) {
	m := ytdlpProgressRe.FindStringSubmatch(strings.TrimSpace(line))
	if len(m) != 4 {
		return 0, 0, false
	}
	pct, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, 0, false
	}
	size, err := strconv.ParseFloat(m[2], 64)
	if err != nil || size <= 0 {
		return 0, 0, false
	}
	total = scaleYtdlpSize(size, m[3])
	if total <= 0 {
		return 0, 0, false
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	done = int64(pct/100*float64(total) + 0.5)
	if done > total {
		done = total
	}
	return done, total, true
}

func scaleYtdlpSize(n float64, unit string) int64 {
	mult := 1.0
	switch strings.ToLower(unit) {
	case "kib":
		mult = 1024
	case "mib":
		mult = 1024 * 1024
	case "gib":
		mult = 1024 * 1024 * 1024
	case "tib":
		mult = 1024 * 1024 * 1024 * 1024
	}
	if n <= 0 {
		return 0
	}
	return int64(n*mult + 0.5)
}

func runWithProgress(ctx context.Context, dir, name string, args []string, onProgress ProgressFunc) (string, string, error) {
	// #nosec G204 -- vodfetch executes a configured local yt-dlp binary with an argument slice, not a shell string.
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", "", err
	}
	cmd.Stdout = &stdout
	if err := cmd.Start(); err != nil {
		return "", "", err
	}
	scanner := bufio.NewScanner(io.TeeReader(stderrPipe, &stderr))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if done, total, ok := ParseYtdlpProgressLine(scanner.Text()); ok && onProgress != nil {
			onProgress(done, total)
		}
	}
	waitErr := cmd.Wait()
	return stdout.String(), stderr.String(), waitErr
}
