package editor

import (
	"slices"
	"sync"
	"testing"
)

// Preparation can rebuild short.FFmpegCommand (Full Demo overlay consolidation
// switches program assembly to a video copy path), but the result shorts are
// cloned before preparation. Without refreshing the result clone, the persisted
// shorts-result.json and the reuse contract would describe a legacy re-encode
// that never ran, even when assembly later fails.
func TestRenderShortCopiesPreparedCommandIntoResult(t *testing.T) {
	prepared := []string{
		"ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", "concat-list.txt",
		"-map", "0:v:0", "-map", "0:a:0", "-c:v", "copy", "-c:a", "pcm_f32le",
		"full-demo-program.nut",
	}
	renderer := &shortPackRenderer{
		manifest: &Manifest{Shorts: []ShortEdit{{FFmpegCommand: append([]string(nil), prepared...)}}},
		result: &Result{Shorts: []ShortResult{{
			FFmpegCommand: []string{"ffmpeg", "-i", "round-001.mp4", "-c:v", "h264_nvenc", "-preset", "p5", "final.mp4"},
		}}},
		shortMu: make([]sync.Mutex, 1),
	}
	renderer.copyPreparedCommand(0, &renderer.manifest.Shorts[0])
	got := renderer.result.Shorts[0].FFmpegCommand
	if !slices.Equal(got, prepared) {
		t.Fatalf("result command = %v, want the prepared copy-path command %v", got, prepared)
	}
	if fullDemoCommandHas(got, "h264_nvenc") {
		t.Fatalf("result still reports the legacy re-encode: %v", got)
	}
	// The result must own its slice: mutating the prepared short afterwards
	// cannot rewrite already-recorded evidence.
	renderer.manifest.Shorts[0].FFmpegCommand[0] = "mutated"
	if renderer.result.Shorts[0].FFmpegCommand[0] != "ffmpeg" {
		t.Fatalf("result command aliases the prepared short: %v", renderer.result.Shorts[0].FFmpegCommand)
	}
	// A nil short is a no-op, so a skipped/failed preparation cannot erase the
	// previously cloned evidence.
	before := append([]string(nil), renderer.result.Shorts[0].FFmpegCommand...)
	renderer.copyPreparedCommand(0, nil)
	if !slices.Equal(renderer.result.Shorts[0].FFmpegCommand, before) {
		t.Fatalf("nil short changed the result: %v", renderer.result.Shorts[0].FFmpegCommand)
	}
}
