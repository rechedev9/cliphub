package recording

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMuxSegmentPairsBoundsWorkAndPreservesOrder(t *testing.T) {
	pairs := []segmentMediaPair{{segmentID: "c"}, {segmentID: "a"}, {segmentID: "b"}, {segmentID: "d"}}
	started := make(chan string, len(pairs))
	release := make(chan struct{})
	done := make(chan []RecordingArtifact, 1)
	go func() {
		done <- muxSegmentPairs(pairs, 2, func(pair segmentMediaPair) RecordingArtifact {
			started <- pair.segmentID
			<-release
			return RecordingArtifact{SegmentID: pair.segmentID}
		})
	}()
	defer close(release)
	for range 2 {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("independent segments did not start concurrently")
		}
	}
	select {
	case id := <-started:
		t.Fatalf("segment %s exceeded the two-worker budget", id)
	default:
	}
	// Release one work item at a time; completion order cannot reorder results.
	for range pairs {
		release <- struct{}{}
	}
	select {
	case got := <-done:
		for i, pair := range pairs {
			if got[i].SegmentID != pair.segmentID {
				t.Fatalf("result %d = %s, want %s", i, got[i].SegmentID, pair.segmentID)
			}
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pool did not finish")
	}
}

func TestMuxSegmentPairsSerializesTakesSharingDestination(t *testing.T) {
	pairs := []segmentMediaPair{
		{segmentID: "a", takeID: "take0000"},
		{segmentID: "b", takeID: "take0001"},
		{segmentID: "A", takeID: "take0002"},
		{segmentID: "b", takeID: "take0003"},
	}
	var active [2]atomic.Int32
	var mu sync.Mutex
	observed := map[string][]string{}
	started := make(chan struct{}, len(pairs))
	release := make(chan struct{})
	done := make(chan []RecordingArtifact, 1)
	go func() {
		done <- muxSegmentPairs(pairs, 4, func(pair segmentMediaPair) RecordingArtifact {
			key := strings.ToLower(pair.segmentID)
			index := int(key[0] - 'a')
			if active[index].Add(1) != 1 {
				t.Error("concurrent writers to the same segment destination")
			}
			defer active[index].Add(-1)
			started <- struct{}{}
			<-release
			mu.Lock()
			observed[key] = append(observed[key], pair.takeID)
			mu.Unlock()
			return RecordingArtifact{SegmentID: pair.segmentID, TakeID: pair.takeID, ProbeError: "retained per-take error"}
		})
	}()
	for range 2 {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			close(release)
			t.Fatal("independent destinations did not start")
		}
	}
	select {
	case <-started:
		close(release)
		t.Fatal("two takes started against the same destination")
	default:
	}
	close(release)
	var got []RecordingArtifact
	select {
	case got = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("duplicate destination group did not finish")
	}
	want := map[string][]string{"a": {"take0000", "take0002"}, "b": {"take0001", "take0003"}}
	if !reflect.DeepEqual(observed, want) {
		t.Fatalf("take order = %v, want %v", observed, want)
	}
	for i, pair := range pairs {
		if got[i].TakeID != pair.takeID || got[i].ProbeError != "retained per-take error" {
			t.Fatalf("lost result for pair %d: %+v", i, got[i])
		}
	}
}

// Exercise actual AAC encoding, reordered video packets, metadata collection
// and atomic publication, comparing the pool with the former serial commands.
func TestConcurrentMuxMatchesSerialCaptureMedia(t *testing.T) {
	ffmpeg, ffprobe := FindFFmpeg(), FindFFprobe()
	if ffmpeg == "" || ffprobe == "" {
		t.Fatal("FFmpeg and ffprobe are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	video, audio := filepath.Join(dir, "video.mp4"), filepath.Join(dir, "audio.wav")
	for _, args := range [][]string{
		{"-y", "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=160x90:rate=60", "-frames:v", "122", "-c:v", "libx264", "-preset", "medium", "-bf", "3", "-threads", "2", video},
		{"-y", "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100", "-t", "2.026304", "-ac", "2", audio},
	} {
		if output, err := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput(); err != nil {
			t.Fatalf("fixture: %v\n%s", err, output)
		}
	}
	serial := filepath.Join(dir, "serial.mp4")
	if err := muxPair(ctx, ffmpeg, video, audio, serial); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(serial)
	if err != nil {
		t.Fatal(err)
	}
	plan := RecordingPlan{OutputDir: filepath.Join(dir, "parallel")}
	var inputs []RecordingArtifact
	for i := range 5 {
		id, take := fmt.Sprintf("segment-%d", i), fmt.Sprintf("take%04d", i)
		plan.Segments = append(plan.Segments, RecordingSegment{ID: id})
		inputs = append(inputs, RecordingArtifact{SegmentID: id, TakeID: take, Type: "video", Path: video}, RecordingArtifact{SegmentID: id, TakeID: take, Type: "audio", Path: audio})
	}
	for pass := range 2 { // The second pass must reuse the committed clips.
		got := MuxSegmentClips(ctx, plan, inputs, ffmpeg, ffprobe)
		if len(got) != len(plan.Segments) {
			t.Fatalf("pass %d: %d artifacts", pass, len(got))
		}
		for i, artifact := range got {
			if artifact.SegmentID != plan.Segments[i].ID || artifact.ProbeError != "" || artifact.FrameCount != 122 {
				t.Fatalf("pass %d: unexpected artifact %+v", pass, artifact)
			}
			body, err := os.ReadFile(artifact.Path)
			if err != nil || !bytes.Equal(body, want) {
				t.Fatalf("pass %d: concurrent mux differs from serial: %v", pass, err)
			}
		}
	}
	plan.OutputDir = filepath.Join(dir, "cancelled")
	cancelled, stop := context.WithCancel(ctx)
	stop()
	for _, artifact := range MuxSegmentClips(cancelled, plan, inputs, ffmpeg, ffprobe) {
		if artifact.ProbeError == "" {
			t.Fatalf("cancelled mux reported success: %+v", artifact)
		}
		for _, path := range []string{artifact.Path, artifact.Path + ".part"} {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("cancelled mux left an output at %s: %v", path, err)
			}
		}
	}
	plan.OutputDir = filepath.Join(dir, "one-failure")
	inputs[0].Path = filepath.Join(dir, "missing.mp4")
	for i, artifact := range MuxSegmentClips(ctx, plan, inputs, ffmpeg, ffprobe) {
		if (artifact.ProbeError != "") != (i == 0) {
			t.Fatalf("one failed take affected another destination: %+v", artifact)
		}
	}
}
