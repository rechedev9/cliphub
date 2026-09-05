package editor

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/voicecomms"
)

func TestFullDemoVoiceSeekUsesDocumentFrameClock(t *testing.T) {
	for _, tc := range []struct {
		name        string
		legacyRate  int
		splitFrames int64
		wantSample  int64
	}{
		{"fractional tick boundary", 64, 0, 96800},
		{"different legacy tickrate", 128, 0, 96800},
		{"unset legacy tickrate", 0, 0, 96800},
		{"resumed after sponsor split", 128, 17, 110400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			short := ShortEdit{
				Tickrate: tc.legacyRate,
				Parts:    []ShortPart{{SegmentID: "round-001", Input: "game.nut"}},
				FullDemo: &FullDemoRenderEvidence{Effective: recapplan.Document{Clock: recapplan.Clock{TickRate: 64}, Options: recapplan.DefaultOptions()}},
				fullDemo: &fullDemoRenderContext{
					ffmpeg: "ffmpeg", voicePaths: []string{"pov.wav", "teammate.wav"},
					recording: recording.RecordingResult{Plan: recording.RecordingPlan{Segments: []recording.RecordingSegment{{ID: "round-001", TickStart: 64}}}},
				},
			}
			item := recapplan.TimelineItem{Role: "round", SourceRef: "round-001", SourceStartTick: 129, SourceOffsetFrames: tc.splitFrames, EndFrame: 60, EndSample: 48000}
			command, err := fullDemoItemCommand(short, item, 0, "round.nut")
			if err != nil {
				t.Fatal(err)
			}
			seeks := 0
			for i, arg := range command {
				if arg != "-ss" {
					continue
				}
				seconds, err := strconv.ParseFloat(command[i+1], 64)
				if err != nil || int64(math.Round(seconds*48000)) != tc.wantSample {
					t.Fatalf("voice seek %q = %.0f samples, want %d: %v", command[i+1], seconds*48000, tc.wantSample, err)
				}
				seeks++
			}
			if seeks != 2 {
				t.Fatalf("voice seeks = %d, want both team tracks", seeks)
			}
			// The game input starts at tick 64. Tick 129 is 61 frames into
			// that capture, on the same frame/sample grid as the voice seek.
			gameStart := (61 + tc.splitFrames) * 800
			if !strings.Contains(strings.Join(command, " "), fmt.Sprintf("atrim=start_sample=%d:end_sample=%d", gameStart, gameStart+48000)) {
				t.Fatalf("game audio lost its capture-relative frame window: %v", command)
			}
		})
	}
}

func TestFullDemoVoiceMixManifestDropsEnemyTracks(t *testing.T) {
	dir := t.TempDir()
	voiceDir := filepath.Join(dir, "voice")
	opus := []byte{0xF8, 0xFF, 0xFE}
	const (
		pov   = "76561198000000001"
		mate  = "76561198000000002"
		enemy = "76561198000000009"
	)
	_, err := voicecomms.WriteTracks(voiceDir, voicecomms.Report{
		Demo:         "match.dem",
		Tickrate:     64,
		SampleRateHz: 48000,
		Target:       voicecomms.PlayerVoice{SteamID64: pov, Name: "pov", Team: "T"},
		Teammates:    []voicecomms.PlayerVoice{{SteamID64: mate, Name: "mate", Team: "T"}},
	}, []voicecomms.Packet{
		{XUID: 76561198000000001, Tick: 64, Format: voicecomms.FormatOpus, Data: opus},
		{XUID: 76561198000000002, Tick: 128, Format: voicecomms.FormatOpus, Data: opus},
		{XUID: 76561198000000009, Tick: 192, Format: voicecomms.FormatOpus, Data: opus},
	}, []voicecomms.Sighting{
		{SteamID64: pov, Name: "pov", Team: "T", Tick: 0},
		{SteamID64: mate, Name: "mate", Team: "T", Tick: 0},
		{SteamID64: enemy, Name: "enemy", Team: "CT", Tick: 0},
	})
	if err != nil {
		t.Fatal(err)
	}

	result := testRecordingResult(dir)
	opts := testManifestOptions(dir, nil)
	opts.Preset = PresetGameplayPOV60
	opts.OutputFormat = OutputFormatLandscape16x9
	opts.CompileSegments = true
	opts.KillEffect = KillEffectClean
	opts.VoiceDir = voiceDir
	voice := 0.85
	opts.VoiceVolume = &voice

	manifest := mustBuildManifest(t, result, opts)
	if len(manifest.Shorts) != 1 {
		t.Fatalf("shorts = %d, want 1 compiled recap", len(manifest.Shorts))
	}
	short := manifest.Shorts[0]
	if len(short.VoiceTracks) != 2 {
		t.Fatalf("voice tracks = %#v, want pov+mate", short.VoiceTracks)
	}
	for _, path := range short.VoiceTracks {
		if strings.Contains(path, enemy) {
			t.Fatalf("enemy track reached the mix: %#v", short.VoiceTracks)
		}
	}
	if short.VoiceTickrate != 64 {
		t.Fatalf("voice tickrate = %d, want 64", short.VoiceTickrate)
	}

	command := strings.Join(BuildCompilationFFmpegCommand("ffmpeg", short), " ")
	if strings.Contains(command, enemy+".ogg") {
		t.Fatalf("ffmpeg mixes enemy comms:\n%s", command)
	}
	if !strings.Contains(command, pov+".ogg") || !strings.Contains(command, mate+".ogg") {
		t.Fatalf("ffmpeg missing POV-team tracks:\n%s", command)
	}
	for _, part := range short.Parts {
		startTick, _ := captureTicks(part)
		sync := partSyncDuration(part, short.VoiceTickrate)
		frames := max(1, int(math.Round(sync*float64(outputFPS(short)))))
		dur := float64(frames) / float64(outputFPS(short))
		start := float64(startTick) / float64(short.VoiceTickrate)
		want := fmt.Sprintf("atrim=start=%.6f:end=%.6f", start, start+dur)
		if !strings.Contains(command, want) {
			t.Fatalf("missing capture-aligned atrim %q for %s\n%s", want, part.SegmentID, command)
		}
	}
}

func killsForTestSegment(result recording.RecordingResult, id string) []killplan.Kill {
	for _, segment := range result.Plan.Segments {
		if segment.ID == id {
			return segment.Kills
		}
	}
	return nil
}
