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

func TestFullDemoVoiceMixFollowsCaptureWindow(t *testing.T) {
	voice := 0.85
	tests := []struct {
		name      string
		short     ShortEdit
		want      []string
		wantNot   []string
		wantAtrim []string
	}{
		{
			name: "uses capture ticks not editorial freeze-end",
			short: ShortEdit{
				Preset:        PresetGameplayPOV60,
				Output:        "out.mp4",
				VoiceTracks:   []string{"pov.ogg", "mate.ogg"},
				VoiceTickrate: 64,
				VoiceVolume:   &voice,
				Parts: []ShortPart{{
					SegmentID:        "seg-001",
					Input:            "r1.mp4",
					DurationSeconds:  41.328,
					TickStart:        5487,
					TickEnd:          8260,
					CaptureTickStart: 5615,
					CaptureTickEnd:   8260,
				}},
			},
			want:    []string{"-i pov.ogg", "-i mate.ogg", "volume=0.85", "[pav0]"},
			wantNot: []string{"atrim=start=85.734375"},
			wantAtrim: []string{
				atrimWindow(5615, 8260, 64),
			},
		},
		{
			name: "two live rounds keep independent capture windows",
			short: ShortEdit{
				Preset:        PresetGameplayPOV60,
				Output:        "out.mp4",
				VoiceTracks:   []string{"pov.ogg"},
				VoiceTickrate: 64,
				Parts: []ShortPart{
					{
						SegmentID: "seg-001", Input: "r1.mp4", DurationSeconds: 2,
						TickStart: 14029, TickEnd: 14770, CaptureTickStart: 14157, CaptureTickEnd: 14770,
					},
					{
						SegmentID: "seg-002", Input: "r2.mp4", DurationSeconds: 2,
						TickStart: 22086, TickEnd: 22406, CaptureTickStart: 22186, CaptureTickEnd: 22406,
					},
				},
			},
			wantAtrim: []string{
				atrimWindow(14157, 14770, 64),
				atrimWindow(22186, 22406, 64),
			},
			wantNot: []string{
				atrimWindow(14029, 14770, 64),
				atrimWindow(22086, 22406, 64),
			},
		},
		{
			name: "native POV without music still mixes comms onto game audio",
			short: ShortEdit{
				Preset:        PresetGameplayPOV60,
				Output:        "out.mp4",
				VoiceTracks:   []string{"pov.ogg"},
				VoiceTickrate: 64,
				Parts: []ShortPart{{
					SegmentID: "seg-001", Input: "r1.mp4", DurationSeconds: 2,
					TickStart: 640, TickEnd: 1280, CaptureTickStart: 768, CaptureTickEnd: 1280,
				}},
			},
			want:    []string{"volume=0.85", "[pa0][vmix0]amix=inputs=2:duration=first", atrimWindow(768, 1280, 64)},
			wantNot: []string{"-stream_loop", "[music]"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := strings.Join(BuildCompilationFFmpegCommand("ffmpeg", tt.short), " ")
			for _, want := range append(append([]string{}, tt.want...), tt.wantAtrim...) {
				if !strings.Contains(command, want) {
					t.Fatalf("command missing %q\n%s", want, command)
				}
			}
			for _, ban := range tt.wantNot {
				if ban != "" && strings.Contains(command, ban) {
					t.Fatalf("command unexpectedly contains %q\n%s", ban, command)
				}
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
		recSeg := recording.RecordingSegment{
			ID: part.SegmentID, TickStart: part.TickStart, TickEnd: part.TickEnd,
			Kills: killsForTestSegment(result, part.SegmentID),
		}
		start := recording.EffectiveRecordStartTick(recSeg, result.Plan.Tickrate)
		end := recording.EffectiveRecordEndTick(recSeg, result.Plan)
		want := atrimWindow(start, end, result.Plan.Tickrate)
		if !strings.Contains(command, want) {
			t.Fatalf("missing capture-aligned atrim %q for %s\n%s", want, part.SegmentID, command)
		}
		editorial := atrimWindow(part.TickStart, part.TickEnd, result.Plan.Tickrate)
		if editorial != want && strings.Contains(command, editorial) && start != part.TickStart {
			t.Fatalf("ffmpeg used editorial ticks %q instead of capture window %q", editorial, want)
		}
	}
}

func atrimWindow(startTick, endTick, tickrate int) string {
	return fmt.Sprintf(
		"atrim=start=%.6f:end=%.6f",
		float64(startTick)/float64(tickrate),
		float64(endTick)/float64(tickrate),
	)
}

func killsForTestSegment(result recording.RecordingResult, id string) []killplan.Kill {
	for _, segment := range result.Plan.Segments {
		if segment.ID == id {
			return segment.Kills
		}
	}
	return nil
}
