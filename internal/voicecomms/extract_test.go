package voicecomms

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rechedev9/cliphub/internal/voiceprofile"
)

func TestWriteTracksKeepsOnlyTeamOpus(t *testing.T) {
	const (
		pov   = "76561198000000001"
		mate  = "76561198000000002"
		enemy = "76561198000000009"
	)
	opus := []byte{0xF8, 0xFF, 0xFE}
	tests := []struct {
		name      string
		packets   []Packet
		sightings []Sighting
		wantIDs   []string
		wantPkts  map[string]int
	}{
		{
			name: "drops enemy and steam-format packets",
			packets: []Packet{
				{XUID: 76561198000000001, Tick: 64, Format: FormatOpus, Data: opus, SampleRate: 48000},
				{XUID: 76561198000000002, Tick: 128, Format: FormatOpus, Data: opus, SampleRate: 48000},
				{XUID: 76561198000000009, Tick: 192, Format: FormatOpus, Data: opus, SampleRate: 48000},
				{XUID: 76561198000000001, Tick: 256, Format: FormatSteam, Data: []byte{1, 2, 3}},
			},
			sightings: []Sighting{
				{SteamID64: pov, Name: "pov", Team: "T", Tick: 0},
				{SteamID64: mate, Name: "mate", Team: "T", Tick: 0},
				{SteamID64: enemy, Name: "enemy", Team: "CT", Tick: 0},
			},
			wantIDs:  []string{pov, mate},
			wantPkts: map[string]int{pov: 1, mate: 1},
		},
		{
			name: "keeps teammates both halves after side swap",
			packets: []Packet{
				{XUID: 76561198000000001, Tick: 100, Format: FormatOpus, Data: opus},
				{XUID: 76561198000000002, Tick: 100, Format: FormatOpus, Data: opus},
				{XUID: 76561198000000009, Tick: 100, Format: FormatOpus, Data: opus},
				{XUID: 76561198000000001, Tick: 20000, Format: FormatOpus, Data: opus},
				{XUID: 76561198000000002, Tick: 20000, Format: FormatOpus, Data: opus},
				{XUID: 76561198000000009, Tick: 20000, Format: FormatOpus, Data: opus},
			},
			sightings: []Sighting{
				{SteamID64: pov, Name: "pov", Team: "T", Tick: 0},
				{SteamID64: mate, Name: "mate", Team: "T", Tick: 0},
				{SteamID64: enemy, Name: "enemy", Team: "CT", Tick: 0},
				{SteamID64: pov, Name: "pov", Team: "CT", Tick: 10000},
				{SteamID64: mate, Name: "mate", Team: "CT", Tick: 10000},
				{SteamID64: enemy, Name: "enemy", Team: "T", Tick: 10000},
			},
			wantIDs:  []string{pov, mate},
			wantPkts: map[string]int{pov: 2, mate: 2},
		},
		{
			name: "drops speaker with empty team",
			packets: []Packet{
				{XUID: 76561198000000001, Tick: 64, Format: FormatOpus, Data: opus},
				{XUID: 76561198000000009, Tick: 80, Format: FormatOpus, Data: opus},
			},
			sightings: []Sighting{
				{SteamID64: pov, Name: "pov", Team: "T", Tick: 0},
				{SteamID64: enemy, Name: "unknown", Team: "", Tick: 0},
			},
			wantIDs:  []string{pov},
			wantPkts: map[string]int{pov: 1},
		},
		{
			name: "drops speaker never sighted",
			packets: []Packet{
				{XUID: 76561198000000001, Tick: 64, Format: FormatOpus, Data: opus},
				{XUID: 76561198000000009, Tick: 80, Format: FormatOpus, Data: opus},
			},
			sightings: []Sighting{
				{SteamID64: pov, Name: "pov", Team: "T", Tick: 0},
			},
			wantIDs:  []string{pov},
			wantPkts: map[string]int{pov: 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			index, err := WriteTracks(dir, Report{
				Demo:         "match.dem",
				Tickrate:     64,
				SampleRateHz: 48000,
				Target:       PlayerVoice{SteamID64: pov, Name: "pov", Team: "T"},
				Teammates:    []PlayerVoice{{SteamID64: mate, Name: "mate", Team: "T"}},
			}, tt.packets, tt.sightings)
			if err != nil {
				t.Fatal(err)
			}
			if len(index.Tracks) != len(tt.wantIDs) {
				t.Fatalf("tracks = %#v, want ids %v", index.Tracks, tt.wantIDs)
			}
			got := map[string]Track{}
			for _, track := range index.Tracks {
				if track.SteamID64 == enemy {
					t.Fatalf("enemy track leaked: %#v", index.Tracks)
				}
				got[track.SteamID64] = track
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
			for _, id := range tt.wantIDs {
				track, ok := got[id]
				if !ok {
					t.Fatalf("missing track %s in %#v", id, index.Tracks)
				}
				if want := tt.wantPkts[id]; track.Packets != want {
					t.Fatalf("track %s packets = %d, want %d", id, track.Packets, want)
				}
			}
			if _, err := os.Stat(filepath.Join(dir, "voice-index.json")); err != nil {
				t.Fatal(err)
			}
		})
	}
}
