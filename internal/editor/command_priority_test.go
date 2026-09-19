package editor

import (
	"context"
	"os/exec"
	"testing"
)

// Only the video-only item pool runs off the audio critical path. The
// audio-only pool is a link of that path and the muxed form carries the same
// audio, so neither may ever be demoted.
func TestFullDemoItemPoolBackgroundCoversVideoItemsOnly(t *testing.T) {
	for _, tc := range []struct {
		name    string
		streams fullDemoItemStreams
		want    bool
	}{
		{"video only", fullDemoItemVideoOnly, true},
		{"muxed", fullDemoItemMuxed, false},
		{"audio only", fullDemoItemAudioOnly, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := fullDemoItemPoolBackground(tc.streams); got != tc.want {
				t.Fatalf("fullDemoItemPoolBackground(%v) = %v, want %v", tc.streams, got, tc.want)
			}
		})
	}
}

// The mark is carried by the context, exactly like the timing stage, and it
// never leaks into a sibling context.
func TestBackgroundProcessPriorityIsScopedToItsContext(t *testing.T) {
	root := context.Background()
	if backgroundProcessPriority(root) {
		t.Fatal("an unmarked context is background priority")
	}
	marked := withBackgroundProcessPriority(root)
	if !backgroundProcessPriority(marked) {
		t.Fatal("the marked context lost its priority")
	}
	child, cancel := context.WithCancel(marked)
	defer cancel()
	if !backgroundProcessPriority(child) {
		t.Fatal("a derived context lost its priority")
	}
	if backgroundProcessPriority(root) {
		t.Fatal("the mark escaped into its parent")
	}
}

// An unmarked command must be started exactly as it was built, on every
// platform.
func TestApplyBackgroundProcessPriorityLeavesUnmarkedCommandsAlone(t *testing.T) {
	cmd := exec.Command("ffmpeg", "-version")
	applyBackgroundProcessPriority(context.Background(), cmd)
	if cmd.SysProcAttr != nil {
		t.Fatalf("unmarked command was rewritten: %+v", cmd.SysProcAttr)
	}
}
