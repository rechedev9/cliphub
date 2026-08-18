package voicecomms

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rechedev9/cliphub/internal/voiceprofile"
)

func TestWriteTracksKeepsOnlyTeamOpus(t *testing.T) {
	dir := t.TempDir()
	report := Report{
		Demo:         "match.dem",
		Tickrate:     64,
		SampleRateHz: 48000,
		Target:       PlayerVoice{SteamID64: "76561198000000001", Name: "pov", Team: "T"},
		Teammates:    []PlayerVoice{{SteamID64: "76561198000000002", Name: "mate", Team: "T"}},
	}
	packets := []Packet{
		{XUID: 76561198000000001, Tick: 64, Format: FormatOpus, Data: []byte{0xF8, 0xFF, 0xFE}, SampleRate: 48000},
		{XUID: 76561198000000002, Tick: 128, Format: FormatOpus, Data: []byte{0xF8, 0xFF, 0xFE}, SampleRate: 48000},
		{XUID: 76561198000000009, Tick: 192, Format: FormatOpus, Data: []byte{0xF8, 0xFF, 0xFE}, SampleRate: 48000},
		{XUID: 76561198000000001, Tick: 256, Format: FormatSteam, Data: []byte{1, 2, 3}},
	}
	index, err := WriteTracks(dir, report, packets, []Sighting{
		{SteamID64: "76561198000000001", Name: "pov", Team: "T", Tick: 0},
		{SteamID64: "76561198000000002", Name: "mate", Team: "T", Tick: 0},
		{SteamID64: "76561198000000009", Name: "enemy", Team: "CT", Tick: 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(index.Tracks) != 2 {
		t.Fatalf("tracks = %#v, want pov+mate", index.Tracks)
	}
	for _, track := range index.Tracks {
		f, err := os.Open(track.Path)
		if err != nil {
			t.Fatal(err)
		}
		_, err = voiceprofile.ValidateAudio(f)
		_ = f.Close()
		if err != nil {
			t.Fatalf("track %s: %v", track.SteamID64, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "voice-index.json")); err != nil {
		t.Fatal(err)
	}
}
