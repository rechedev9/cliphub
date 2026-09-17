package editor

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// The split video and audio item processes must produce exactly the frames and
// samples of the single muxed process they replace, for rounds with voices,
// comms tails, transitions and SFX as well as for the sponsor insert.
func TestFullDemoSplitItemStreamsMatchMuxedItem(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	dir := t.TempDir()
	short, document, _ := fullDemoTransitionCanaryShort(t, ctx, ffmpeg, dir)
	if err := prepareFullDemoTransitions(ctx, &short); err != nil {
		t.Fatal(err)
	}
	if err := prepareFullDemoTracks(ctx, &short, nil); err != nil {
		t.Fatal(err)
	}
	for i, item := range document.Timeline {
		outputs := map[fullDemoItemStreams]string{}
		for streams, name := range map[fullDemoItemStreams]string{fullDemoItemMuxed: "muxed", fullDemoItemVideoOnly: "video", fullDemoItemAudioOnly: "audio"} {
			output := filepath.Join(short.fullDemo.workDir, fmt.Sprintf("%s-%03d.nut", name, i))
			command, err := fullDemoItemStreamCommand(short, item, output, streams)
			if err != nil {
				t.Fatal(err)
			}
			joined := strings.Join(command, " ")
			if streams == fullDemoItemVideoOnly && (strings.Contains(joined, "-c:a") || strings.Contains(joined, ".wav")) {
				t.Fatalf("item %d video command still depends on audio:\n%s", i, joined)
			}
			if streams == fullDemoItemAudioOnly && (strings.Contains(joined, "-c:v") || strings.Contains(joined, "[0:v]")) {
				t.Fatalf("item %d audio command still renders video:\n%s", i, joined)
			}
			if _, err := runFFmpegOutput(ctx, command, "split item "+name); err != nil {
				t.Fatal(err)
			}
			outputs[streams] = output
		}
		wantPCM := fullDemoDecodePCM(t, ctx, ffmpeg, outputs[fullDemoItemMuxed])
		if want := int(item.EndSample-item.StartSample) * 4 * 2; len(wantPCM) != want {
			t.Fatalf("item %d muxed PCM bytes = %d, want %d", i, len(wantPCM), want)
		}
		if string(fullDemoDecodePCM(t, ctx, ffmpeg, outputs[fullDemoItemAudioOnly])) != string(wantPCM) {
			t.Fatalf("item %d audio-only samples differ from the muxed item", i)
		}
		wantFrames := fullDemoFrameHashes(t, ctx, ffmpeg, []string{"-i", outputs[fullDemoItemMuxed], "-map", "0:v:0"})
		gotFrames := fullDemoFrameHashes(t, ctx, ffmpeg, []string{"-i", outputs[fullDemoItemVideoOnly], "-map", "0:v:0"})
		if int64(len(wantFrames)) != item.EndFrame-item.StartFrame || !slices.Equal(gotFrames, wantFrames) {
			t.Fatalf("item %d video-only frames differ from the muxed item: %d vs %d", i, len(gotFrames), len(wantFrames))
		}
	}
}

func TestFullDemoBranchesReturnTheFirstRealFailure(t *testing.T) {
	failure := errors.New("audio_loudness_failed: test")
	cancelled := make(chan struct{})
	err := runFullDemoBranches(context.Background(),
		func(ctx context.Context) error {
			<-ctx.Done()
			close(cancelled)
			return ctx.Err()
		},
		func(context.Context) error { return failure },
	)
	if !errors.Is(err, failure) {
		t.Fatalf("branch error = %v, want the audio failure instead of the cancellation it caused", err)
	}
	select {
	case <-cancelled:
	default:
		t.Fatal("video branch was not cancelled and awaited")
	}
	if err := runFullDemoBranches(context.Background(), func(context.Context) error { return nil }, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestFullDemoBranchProgressIsMonotonicAndShared(t *testing.T) {
	var mu sync.Mutex
	var reported []float64
	branches := newFullDemoBranchProgress(func(_ string, fraction float64) {
		mu.Lock()
		defer mu.Unlock()
		reported = append(reported, fraction)
	})
	video, audio := branches.video(), branches.audio()
	video("v", .5)
	audio("a", 1)
	audio("a", .2)
	video("v", 1)
	if want := []float64{.25, .75, .75, 1}; !slices.Equal(reported, want) {
		t.Fatalf("branch progress = %v, want %v", reported, want)
	}
	if newFullDemoBranchProgress(nil).video() != nil {
		t.Fatal("nil parent progress must stay nil")
	}
}
