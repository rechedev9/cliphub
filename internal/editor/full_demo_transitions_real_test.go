package editor

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

// Opt-in review loops use existing real captures and the production rendering
// stages. Excerpts are explicitly previews, not newly certified Full Demo jobs.
func TestFullDemoTransitionsRealCapturePreview(t *testing.T) {
	root := os.Getenv("FULL_DEMO_TRANSITION_JOB_DIR")
	if root == "" {
		t.Skip("set FULL_DEMO_TRANSITION_JOB_DIR to review existing real captures")
	}
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	read := func(path string, out any) {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatal(err)
		}
	}
	var result recording.RecordingResult
	read(filepath.Join(root, "recording", "recording-result.json"), &result)
	if !result.CaptureVerified || result.CaptureMode != recording.CaptureModeReal || len(result.Plan.Segments) < 2 {
		t.Fatal("preview requires existing real capture evidence")
	}
	if demo := os.Getenv("TEST_DEMO_PATH"); demo != "" {
		hash, err := mediaassets.FileDigest(ctx, demo, 8<<30)
		if err != nil {
			t.Fatal(err)
		}
		if hash != result.Plan.DemoSHA256 {
			t.Fatal("preview capture does not match the supplied demo")
		}
	}
	var facts recapplan.Facts
	read(filepath.Join(root, "full-demo", "facts.json"), &facts)
	if facts.DemoSHA256 != result.Plan.DemoSHA256 || facts.TargetSteamID64 != result.Plan.TargetSteamID64 {
		t.Fatal("preview facts differ from capture")
	}
	dir := t.TempDir()
	options := recapplan.DefaultOptions()
	options.Audio.Music.Enabled, options.Audio.Voice.Enabled, options.Sponsor.Enabled = false, false, false
	base := recapplan.Document{Clock: recapplan.Clock{TickRate: result.Plan.Tickrate, FPS: 60, SampleRate: 48000}, Options: options}
	var parts []ShortPart
	// Show the last four seconds of R1 and the first four seconds of R2.
	for i, segment := range result.Plan.Segments[:2] {
		path := filepath.Join(root, "recording", "revisions", result.CaptureRevision, "segments", segment.ID+".mp4")
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
		endTick := segment.TickEnd
		if result.FullDemoEvidence != nil {
			if end, ok := result.FullDemoEvidence.CertifiedEnds[segment.ID]; ok {
				endTick = min(endTick, end)
			}
		}
		count, err := recapplan.TickFrames(endTick-segment.TickStart, result.Plan.Tickrate)
		if err != nil {
			t.Fatal(err)
		}
		// Older captured files can end a frame before their tick-based window.
		// Review only available frames; keep the preview's 480-frame delivery
		// assertion strict and never claim this repairs the original capture.
		ffprobe, err := exec.LookPath("ffprobe")
		if err != nil {
			t.Fatal(err)
		}
		probe, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-count_frames", "-select_streams", "v:0", "-show_entries", "stream=nb_read_frames", "-of", "json", path).Output()
		if err != nil {
			t.Fatal(err)
		}
		var streams struct {
			Streams []struct {
				Frames string `json:"nb_read_frames"`
			} `json:"streams"`
		}
		if err := json.Unmarshal(probe, &streams); err != nil || len(streams.Streams) != 1 {
			t.Fatal("invalid real source frame probe")
		}
		available, err := strconv.ParseInt(streams.Streams[0].Frames, 10, 64)
		if err != nil || available < 240 {
			t.Fatal("insufficient real source frames")
		}
		if available < count {
			t.Logf("%s: preview bounded to %d available frames; capture's tick window requested %d", segment.ID, available, count)
		}
		count = min(count, available)
		length := min(int64(240), count)
		offset := int64(0)
		if i == 0 {
			offset = count - length
		}
		start := int64(i) * 240
		base.Timeline = append(base.Timeline, recapplan.TimelineItem{Role: "round", SourceRef: segment.ID, SourceStartTick: segment.TickStart, SourceEndTick: endTick, SourceOffsetFrames: offset, StartFrame: start, EndFrame: start + length, StartSample: start * 800, EndSample: (start + length) * 800})
		parts = append(parts, ShortPart{SegmentID: segment.ID, Input: path})
		for _, r := range facts.Rounds {
			if r.ID == segment.ID {
				base.Rounds = append(base.Rounds, recapplan.Round{ID: r.ID, RoundEndTick: r.RoundEndTick, Kills: r.Kills})
			}
		}
	}
	if base.Timeline[0].EndFrame != 240 || base.Timeline[1].EndFrame != 480 {
		t.Fatal("preview requires four seconds of each source")
	}
	for _, name := range []string{"clean", "subtle", "kinetic", "vertical", "event-zoom", "audio"} {
		t.Run(name, func(t *testing.T) {
			work := filepath.Join(dir, name)
			if err := os.MkdirAll(work, 0700); err != nil {
				t.Fatal(err)
			}
			d := base
			o := recapplan.DefaultTransitions()
			o.Enabled = name != "clean"
			switch name {
			case "subtle":
				o.Whip = false
				o.ZoomPercent = 5
				o.WhooshGainDB = -26
				o.Impact = false
			case "kinetic":
				o.Direction = "follow-motion"
				o.Flash = true
				o.RGBSplit = true
			case "vertical":
				o.Direction = "up"
				o.Flash = true
				o.RGBSplit = true
			case "event-zoom":
				o.Whip = false
				o.ZoomAnchor = "last-kill"
			case "audio":
				o.Whip = false
				o.Zoom = false
				o.GameTailLowpassHz = 2400
			}
			d.Options.Transitions = &o
			short := ShortEdit{Preset: PresetGameplayPOV60, OutputFormat: OutputFormatLandscape16x9, OutputFPS: 60, Tickrate: result.Plan.Tickrate, VideoCRF: 18, VideoPreset: "fast", VideoEncoder: VideoEncoderNVENC, Threads: 4, Output: filepath.Join(work, "preview.mp4"), DurationSeconds: 8, Parts: parts,
				FullDemo: &FullDemoRenderEvidence{SchemaVersion: "1.0", Effective: d}, fullDemo: &fullDemoRenderContext{recording: result, ffmpeg: ffmpeg, workDir: filepath.Join(work, "prepared")}}
			if err := prepareFullDemoCompilation(ctx, &short, nil); err != nil {
				t.Fatal(err)
			}
			if _, err := runFFmpegOutput(ctx, buildFullDemoCompilationCommand(ffmpeg, short), "real transition preview"); err != nil {
				t.Fatal(err)
			}
			e, err := masterFullDemoProgram(ctx, ffmpeg, fullDemoProgramPath(short), short.Output, filepath.Join(work, "logs"), options.Audio.Loudness, false, 8, nil)
			if err != nil {
				t.Fatal(err)
			}
			ffprobe, err := exec.LookPath("ffprobe")
			if err != nil {
				t.Fatal(err)
			}
			delivery, err := verifyFullDemoDelivery(ctx, ffmpeg, ffprobe, short.Output, 480, nil)
			if err != nil {
				t.Fatal(err)
			}
			if destination := os.Getenv("FULL_DEMO_EVIDENCE_DIR"); destination != "" {
				if err := os.MkdirAll(destination, 0700); err != nil {
					t.Fatal(err)
				}
				b, err := os.ReadFile(short.Output)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(destination, "donk-"+name+"-preview.mp4"), b, 0600); err != nil {
					t.Fatal(err)
				}
				b, err = json.MarshalIndent(struct {
					Kind, DemoSHA256, Player string
					Transitions              []FullDemoTransitionEvidence
					Delivery                 *FullDemoDeliveryEvidence
					Loudness                 ProgramLoudnessEvidence
				}{"preview-from-existing-real-capture", facts.DemoSHA256, facts.TargetSteamID64, short.FullDemo.Transitions, delivery, e}, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(destination, "donk-"+name+"-preview.json"), b, 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
