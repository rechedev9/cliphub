package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/rechedev9/cliphub/internal/winproc"
)

// A playing capture writes the console or its take files every few seconds, even through
// a fast-forwarded gap, so five quiet minutes mean CS2 never started or froze mid-capture.
const (
	captureStallLimit = 5 * time.Minute
	captureStallProbe = 5 * time.Second
)

// captureStallError ends a wait whose cs2.exe runs but never makes progress.
type captureStallError struct {
	idle time.Duration
}

func (e *captureStallError) Error() string {
	return fmt.Sprintf("capture_stalled: cs2.exe kept running but made no capture progress for %s", e.idle.Round(time.Second))
}

// captureProgress fingerprints what a playing capture changes; any difference is activity.
type captureProgress struct {
	consoleBytes int64
	files        int
	fileBytes    int64
	newest       int64
}

// captureStallWatch tracks the last time a capture made progress.
type captureStallWatch struct {
	limit      time.Duration
	every      time.Duration
	consoleLog string
	probe      func() captureProgress
	titles     func() []string
	last       captureProgress
	changed    time.Time
}

func newCaptureStallWatch(consoleLog, outputDir string) *captureStallWatch {
	return &captureStallWatch{
		limit:      captureStallLimit,
		every:      captureStallProbe,
		consoleLog: consoleLog,
		probe:      func() captureProgress { return probeCaptureProgress(consoleLog, outputDir) },
		titles:     cs2WindowTitles,
	}
}

func (s *captureStallWatch) start(now time.Time) {
	s.last, s.changed = s.probe(), now
}

// observe probes the capture at now and returns how long it has been idle.
func (s *captureStallWatch) observe(now time.Time) time.Duration {
	if current := s.probe(); current != s.last {
		s.last, s.changed = current, now
	}
	return now.Sub(s.changed)
}

// report writes what remote diagnostics need to explain a wait that was cut.
func (s *captureStallWatch) report(now time.Time) {
	if s == nil {
		return
	}
	logConsoleTail(s.consoleLog)
	if s.titles != nil {
		log.Printf("cs2 windows: %q", s.titles())
	}
	log.Printf("cs2 stall: no activity for %ds", int(now.Sub(s.changed).Seconds()))
}

func probeCaptureProgress(consoleLog, outputDir string) captureProgress {
	var p captureProgress
	if info, err := os.Stat(consoleLog); err == nil {
		p.consoleBytes = info.Size()
	}
	_ = filepath.WalkDir(outputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		// Stat the file itself: a Windows directory listing can lag a file another process still writes.
		info, err := os.Stat(path)
		if err != nil {
			return nil
		}
		p.files++
		p.fileBytes += info.Size()
		p.newest = max(p.newest, info.ModTime().UnixNano())
		return nil
	})
	return p
}

func cs2WindowTitles() []string {
	_, titles, _ := winproc.WindowTitles("cs2.exe")
	return titles
}
