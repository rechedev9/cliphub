package editor

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

// This is a real FFmpeg canary over synthetic sources. It invokes the same
// Full Demo preparation/concat/master stages but does not manufacture an HLAE
// attestation or pass synthetic capture through the production-real gate.
func TestFullDemoSponsorAndPlaylistMediaCanary(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	for _, scenario := range []string{"embedded", "replace-narration", "manual-split", "playlist-once", "final-boundary"} {
		t.Run(scenario, func(t *testing.T) {
			audioPolicy := "embedded"
			if scenario == "replace-narration" {
				audioPolicy = scenario
			}
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			dir := t.TempDir()
			makeMedia := func(name, color string, frequency int, seconds float64) string {
				t.Helper()
				path := filepath.Join(dir, name+".nut")
				command := []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "color=c=" + color + ":s=160x90:r=60:d=" + decimal(seconds), "-f", "lavfi", "-i", "sine=f=" + decimal(float64(frequency)) + ":r=48000:d=" + decimal(seconds), "-map", "0:v", "-map", "1:a", "-c:v", "libx264", "-preset", "ultrafast", "-bf", "0", "-c:a", "pcm_f32le", "-ac", "2", path}
				if _, err := runFFmpegOutput(ctx, command, "create synthetic media"); err != nil {
					t.Fatal(err)
				}
				return path
			}
			roundOne := makeMedia("round-one", "red", 440, 2)
			roundTwo := makeMedia("round-two", "blue", 440, 2)
			sponsor := makeMedia("sponsor", "lime", 660, 1)
			narration := makeMedia("narration", "black", 1200, 1)
			musicOne := makeMedia("music-one", "black", 220, .75)
			musicTwo := makeMedia("music-two", "black", 330, .75)
			voice := makeMedia("team-voice", "black", 880, 8)
			options := recapplan.DefaultOptions()
			options.Capture.Crosshair.AllowCaptureDefault = true
			options.Editorial.FreezeSeconds, options.Editorial.RoundTailSeconds = 0, 0
			options.Editorial.KeepFreezeVoice = false
			options.Audio.Voice.Normalization = "none"
			options.Audio.Music.Ducking.Enabled = false
			options.Sponsor.PlacementPolicy, options.Sponsor.AfterRoundID = "round-boundary", "round-001"
			options.Sponsor.AudioPolicy = audioPolicy
			if scenario == "final-boundary" {
				options.Sponsor.AfterRoundID = "round-002"
			}
			if scenario == "manual-split" {
				frame := int64(60)
				options.Sponsor.PlacementPolicy, options.Sponsor.AfterRoundID = "manual-frame", ""
				options.Sponsor.ManualStartFrame, options.Sponsor.AllowSplitRound = &frame, true
			}
			if scenario == "playlist-once" {
				options.Audio.Music.LoopPolicy = "once-pad-silence"
			}
			assets := []recapplan.AssetEvidence{}
			local := []FullDemoLocalMedia{}
			addAsset := func(path string, frames int64) recapplan.AssetRef {
				t.Helper()
				hash, err := mediaassets.FileDigest(ctx, path, 1<<30)
				if err != nil {
					t.Fatal(err)
				}
				ref := recapplan.AssetRef{ID: uuid.NewString(), SHA256: hash}
				assets = append(assets, recapplan.AssetEvidence{Ref: ref, DurationFrames: frames, HasVideo: true, HasAudio: true, Title: "Synthetic test source", Creator: "ClipHub test", Permission: "Original synthetic signal", SourceURL: "https://example.invalid/synthetic-fixture"})
				local = append(local, FullDemoLocalMedia{Ref: ref, Path: path})
				return ref
			}
			options.Audio.Music.Assets = []recapplan.AssetRef{addAsset(musicOne, 45), addAsset(musicTwo, 45)}
			videoRef := addAsset(sponsor, 60)
			options.Sponsor.Video = &videoRef
			if audioPolicy == "replace-narration" {
				ref := addAsset(narration, 60)
				options.Sponsor.Narration = &ref
			}
			facts := recapplan.Facts{SchemaVersion: "1.0", DemoSHA256: strings.Repeat("a", 64), TargetSteamID64: "76561198000000001", ClockKind: recapplan.ClockIngame, TickRate: 64, EndTick: 512, Complete: true, Rounds: []recapplan.RoundFacts{
				{ID: "round-001", Number: 1, StartTick: 128, FreezeEndTick: 128, RoundEndTick: 255, NextStartTick: 320, Evidence: "round-events", Kills: []killplan.Kill{}, Utility: []killplan.UtilityThrow{}},
				{ID: "round-002", Number: 2, StartTick: 320, FreezeEndTick: 320, RoundEndTick: 447, Evidence: "round-events", Kills: []killplan.Kill{}, Utility: []killplan.UtilityThrow{}},
			}}
			voiceEvidence := recapplan.VoiceEvidence{Availability: "available", IndexRef: "synthetic/index.json", IndexHash: strings.Repeat("b", 64), ExtractorVersion: "team-packet-clock-v2", ClockKind: recapplan.ClockIngame, SelectedPackets: 2, Activity: []recapplan.TickRange{{Start: 128, End: 448}}}
			document, err := recapplan.Plan(facts, options, voiceEvidence, assets, "synthetic/facts.json")
			if err != nil || len(document.Blockers) > 0 {
				t.Fatalf("plan: %v; blockers: %+v", err, document.Blockers)
			}
			approval := recapplan.Snapshot{Document: document, Approval: recapplan.Approval{PlanHash: document.PlanHash, AllowSafeTailTrim: true, Timestamp: time.Now().UTC()}}
			execution := FullDemoExecution{SchemaVersion: "1.0", Approved: approval, Assets: local, VoiceTracks: []FullDemoLocalVoice{{SteamID64: facts.TargetSteamID64, StorageKey: "synthetic/team-voice", Path: voice}}}
			short := ShortEdit{Preset: PresetGameplayPOV60, OutputFormat: OutputFormatLandscape16x9, OutputFPS: 60, Tickrate: 64, VideoCRF: 18, VideoPreset: "ultrafast", Threads: 2, Output: filepath.Join(dir, "final.mp4"), DurationSeconds: 5,
				Parts:    []ShortPart{{SegmentID: "round-001", Input: roundOne}, {SegmentID: "round-002", Input: roundTwo}},
				FullDemo: &FullDemoRenderEvidence{SchemaVersion: "1.0", Approved: approval, Effective: document},
				fullDemo: &fullDemoRenderContext{execution: execution, recording: recording.RecordingResult{Plan: recording.RecordingPlan{Segments: []recording.RecordingSegment{{ID: "round-001", TickStart: 128}, {ID: "round-002", TickStart: 320}}}}, ffmpeg: ffmpeg, workDir: filepath.Join(dir, "prepared")},
			}
			if err := prepareFullDemoCompilation(ctx, &short); err != nil {
				t.Fatal(err)
			}
			program := fullDemoProgramPath(short)
			if err := runFFmpegAtomic(ctx, buildFullDemoCompilationCommand(ffmpeg, short), "concat media canary", "", program); err != nil {
				t.Fatal(err)
			}
			loudness, err := masterFullDemoProgram(ctx, ffmpeg, program, short.Output, filepath.Join(dir, "logs"), options.Audio.Loudness, false)
			if err != nil {
				t.Fatal(err)
			}
			short.FullDemo.ProgramLoudness = &loudness
			ffprobe, err := exec.LookPath("ffprobe")
			if err != nil {
				t.Fatal(err)
			}
			short.FullDemo.Delivery, err = verifyFullDemoDelivery(ctx, ffmpeg, ffprobe, short.Output, 300)
			if err != nil {
				t.Fatal(err)
			}
			if err := short.FullDemo.ValidateCompleted(); err != nil {
				t.Fatal(err)
			}
			adStart := float64(document.SponsorPlacement.StartFrame) / 60
			for _, sample := range []struct {
				at      float64
				channel int
			}{{.5, 0}, {adStart + .5, 1}, {3.5, 2}} {
				command := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-ss", decimal(sample.at), "-i", short.Output, "-frames:v", "1", "-vf", "scale=1:1", "-pix_fmt", "rgb24", "-f", "rawvideo", "pipe:1")
				pixel, err := command.Output()
				if err != nil || len(pixel) != 3 || pixel[sample.channel] < 150 {
					t.Fatalf("timeline video at %.2f = %v, %v", sample.at, pixel, err)
				}
			}
			pcm := fullDemoReadAudio(t, ctx, ffmpeg, short.Output, adStart+.4, .1)
			wantFrequency, removedFrequency := 660.0, 1200.0
			if audioPolicy == "replace-narration" {
				wantFrequency, removedFrequency = removedFrequency, wantFrequency
			}
			wanted := fullDemoFrequencyPower(pcm, wantFrequency)
			for _, absent := range []float64{440, 880, 220, 330, removedFrequency} {
				if unwanted := fullDemoFrequencyPower(pcm, absent); wanted < unwanted*100 {
					t.Fatalf("sponsor contains %.0fHz: wanted=%g unwanted=%g", absent, wanted, unwanted)
				}
			}
			pcm = fullDemoReadAudio(t, ctx, ffmpeg, short.Output, 3.45, .1)
			if scenario == "playlist-once" {
				for _, frequency := range []float64{220, 330} {
					if fullDemoFrequencyPower(pcm, frequency) > fullDemoFrequencyPower(pcm, 440)*1e-5 {
						t.Fatal("one-shot playlist continued after EOF")
					}
				}
			} else {
				wantMusic, otherMusic := 330.0, 220.0
				if scenario == "final-boundary" {
					wantMusic, otherMusic = otherMusic, wantMusic
				}
				if fullDemoFrequencyPower(pcm, wantMusic) < fullDemoFrequencyPower(pcm, otherMusic)*20 {
					t.Fatal("playlist did not follow gameplay time across sponsor placement")
				}
			}
			if fullDemoFrequencyPower(pcm, 880) < fullDemoFrequencyPower(pcm, 1320)*100 {
				t.Fatal("team voice is missing or unexpected voice frequency entered the mix")
			}
			if _, err := runFFmpegOutput(ctx, []string{ffmpeg, "-v", "error", "-xerror", "-i", short.Output, "-map", "0:v:0", "-map", "0:a:0", "-f", "null", "-"}, "decode full canary program"); err != nil {
				t.Fatal(err)
			}
			short.CoverPath, short.CoverTimeSeconds, short.HQFilters = filepath.Join(dir, "cover.jpg"), adStart-1.0/60, true
			if _, err := runFFmpegOutput(ctx, BuildCoverFFmpegCommand(ffmpeg, short), "exact gameplay cover"); err != nil {
				t.Fatal(err)
			}
			pixel, err := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-i", short.CoverPath, "-frames:v", "1", "-vf", "scale=1:1", "-pix_fmt", "rgb24", "-f", "rawvideo", "pipe:1").Output()
			coverChannel := 0
			if scenario == "final-boundary" {
				coverChannel = 2
			}
			if err != nil || len(pixel) != 3 || pixel[coverChannel] < 150 || pixel[1] > 80 {
				t.Fatalf("automatic cover entered sponsor: %v %v", pixel, err)
			}
			if root := os.Getenv("FULL_DEMO_EVIDENCE_DIR"); root != "" {
				if err := os.MkdirAll(root, 0700); err != nil {
					t.Fatal(err)
				}
				b, err := os.ReadFile(short.Output)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, scenario+"-synthetic-canary.mp4"), b, 0600); err != nil {
					t.Fatal(err)
				}
				b, err = json.MarshalIndent(short.FullDemo, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, scenario+"-synthetic-canary.json"), b, 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func fullDemoReadAudio(t *testing.T, ctx context.Context, ffmpeg, path string, start, duration float64) []byte {
	t.Helper()
	command := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-ss", decimal(start), "-i", path, "-t", decimal(duration), "-vn", "-ac", "1", "-ar", "48000", "-f", "f32le", "pipe:1")
	pcm, err := command.Output()
	if err != nil || len(pcm) < 4 {
		t.Fatalf("decode canary samples: %v", err)
	}
	return pcm
}

func fullDemoFrequencyPower(pcm []byte, frequency float64) float64 {
	var real, imaginary float64
	for i := 0; i+4 <= len(pcm); i += 4 {
		value := float64(math.Float32frombits(binary.LittleEndian.Uint32(pcm[i : i+4])))
		phase := 2 * math.Pi * frequency * float64(i/4) / 48000
		real += value * math.Cos(phase)
		imaginary += value * math.Sin(phase)
	}
	return real*real + imaginary*imaginary
}
