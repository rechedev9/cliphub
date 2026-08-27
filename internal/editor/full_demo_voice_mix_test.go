package editor

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/voicecomms"
)

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
		recSeg := recording.RecordingSegment{
			ID: part.SegmentID, TickStart: part.TickStart, TickEnd: part.TickEnd,
			Kills: killsForTestSegment(result, part.SegmentID),
		}
		start := recording.EffectiveRecordStartTick(recSeg, result.Plan.Tickrate)
		end := recording.EffectiveRecordEndTick(recSeg, result.Plan)
		want := fmt.Sprintf("atrim=start=%.6f:end=%.6f", float64(start)/float64(result.Plan.Tickrate), float64(end)/float64(result.Plan.Tickrate))
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
