package voicecomms

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/rechedev9/cliphub/internal/voiceprofile"
)

const (
	fullDemoPOV   = "76561198000000001"
	fullDemoMate  = "76561198000000002"
	fullDemoEnemy = "76561198000000009"
)

var fullDemoOpus = []byte{0xF8, 0xFF, 0xFE}

func TestFullDemoCommsExtractsPOVTeamOnly(t *testing.T) {
	tests := []struct {
		name      string
		packets   []Packet
		sightings []Sighting
		wantIDs   []string
		wantPkts  map[string]int
	}{
		{
			name: "drops enemy on the same tick",
			packets: []Packet{
				opusPkt(fullDemoPOV, 64),
				opusPkt(fullDemoMate, 128),
				opusPkt(fullDemoEnemy, 192),
			},
			sightings: []Sighting{
				{SteamID64: fullDemoPOV, Name: "pov", Team: "T", Tick: 0},
				{SteamID64: fullDemoMate, Name: "mate", Team: "T", Tick: 0},
				{SteamID64: fullDemoEnemy, Name: "enemy", Team: "CT", Tick: 0},
			},
			wantIDs:  []string{fullDemoPOV, fullDemoMate},
			wantPkts: map[string]int{fullDemoPOV: 1, fullDemoMate: 1},
		},
		{
			name: "keeps teammates both halves after side swap",
			packets: []Packet{
				opusPkt(fullDemoPOV, 100),
				opusPkt(fullDemoMate, 100),
				opusPkt(fullDemoEnemy, 100),
				opusPkt(fullDemoPOV, 20000),
				opusPkt(fullDemoMate, 20000),
				opusPkt(fullDemoEnemy, 20000),
			},
			sightings: []Sighting{
				{SteamID64: fullDemoPOV, Name: "pov", Team: "T", Tick: 0},
				{SteamID64: fullDemoMate, Name: "mate", Team: "T", Tick: 0},
				{SteamID64: fullDemoEnemy, Name: "enemy", Team: "CT", Tick: 0},
				{SteamID64: fullDemoPOV, Name: "pov", Team: "CT", Tick: 10000},
				{SteamID64: fullDemoMate, Name: "mate", Team: "CT", Tick: 10000},
				{SteamID64: fullDemoEnemy, Name: "enemy", Team: "T", Tick: 10000},
			},
			wantIDs:  []string{fullDemoPOV, fullDemoMate},
			wantPkts: map[string]int{fullDemoPOV: 2, fullDemoMate: 2},
		},
		{
			name: "drops steam-format packets",
			packets: []Packet{
				opusPkt(fullDemoPOV, 64),
				{XUID: 76561198000000001, Tick: 128, Format: FormatSteam, Data: []byte{1, 2, 3}},
			},
			sightings: []Sighting{
				{SteamID64: fullDemoPOV, Name: "pov", Team: "T", Tick: 0},
			},
			wantIDs:  []string{fullDemoPOV},
			wantPkts: map[string]int{fullDemoPOV: 1},
		},
		{
			name: "drops speaker with no team sighting",
			packets: []Packet{
				opusPkt(fullDemoPOV, 64),
				opusPkt(fullDemoEnemy, 80),
			},
			sightings: []Sighting{
				{SteamID64: fullDemoPOV, Name: "pov", Team: "T", Tick: 0},
				{SteamID64: fullDemoEnemy, Name: "unknown", Team: "", Tick: 0},
			},
			wantIDs:  []string{fullDemoPOV},
			wantPkts: map[string]int{fullDemoPOV: 1},
		},
		{
			name: "drops speaker never sighted",
			packets: []Packet{
				opusPkt(fullDemoPOV, 64),
				opusPkt(fullDemoEnemy, 80),
			},
			sightings: []Sighting{
				{SteamID64: fullDemoPOV, Name: "pov", Team: "T", Tick: 0},
			},
			wantIDs:  []string{fullDemoPOV},
			wantPkts: map[string]int{fullDemoPOV: 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			index, err := WriteTracks(dir, fullDemoReport(), tt.packets, tt.sightings)
			if err != nil {
				t.Fatal(err)
			}
			if len(index.Tracks) != len(tt.wantIDs) {
				t.Fatalf("tracks = %#v, want ids %v", index.Tracks, tt.wantIDs)
			}
			got := map[string]Track{}
			for _, track := range index.Tracks {
				if track.SteamID64 == fullDemoEnemy {
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

func TestFullDemoCommsStampsOverlayOnCaptureClock(t *testing.T) {
	t.Run("packet tick prefers ingame clock", func(t *testing.T) {
		tests := []struct {
			name       string
			protoTick  int
			ingameTick int
			want       int
		}{
			{name: "zero proto uses ingame", protoTick: 0, ingameTick: 5487, want: 5487},
			{name: "nonzero proto still uses ingame", protoTick: 12140, ingameTick: 100, want: 100},
			{name: "ingame zero falls back to proto", protoTick: 12140, ingameTick: 0, want: 12140},
			{name: "both zero", protoTick: 0, ingameTick: 0, want: 0},
			{name: "negative ingame falls back to proto", protoTick: 80, ingameTick: -1, want: 80},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := packetTick(tt.protoTick, tt.ingameTick)
				if got != tt.want {
					t.Fatalf("packetTick(%d, %d) = %d, want %d", tt.protoTick, tt.ingameTick, got, tt.want)
				}
			})
		}
	})
	t.Run("timeline places speech at tick time", func(t *testing.T) {
		tests := []struct {
			name     string
			tick     int
			tickrate int
			wantLead int
		}{
			{name: "one second at 64", tick: 64, tickrate: 64, wantLead: 50},
			{name: "zero tick is immediate", tick: 0, tickrate: 64, wantLead: 0},
			{name: "two seconds at 128", tick: 256, tickrate: 128, wantLead: 100},
			{name: "recap live start at 64", tick: 5615, tickrate: 64, wantLead: 4386},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				frames := timelineFrames([]Packet{{Tick: tt.tick, Data: fullDemoOpus}}, tt.tickrate, 0)
				if len(frames) != tt.wantLead+1 {
					t.Fatalf("len = %d, want %d silence + packet", len(frames), tt.wantLead+1)
				}
				for i := 0; i < tt.wantLead; i++ {
					if !bytes.Equal(frames[i], opusSilence20ms) {
						t.Fatalf("frame %d is not silence", i)
					}
				}
				if !bytes.Equal(frames[tt.wantLead], fullDemoOpus) {
					t.Fatalf("packet was not placed at tick time")
				}
			})
		}
	})
	t.Run("ogg has no encoder pre-skip", func(t *testing.T) {
		var buf bytes.Buffer
		if err := WriteOggOpus(&buf, [][]byte{fullDemoOpus}, 48000, 1); err != nil {
			t.Fatal(err)
		}
		i := bytes.Index(buf.Bytes(), []byte("OpusHead"))
		if i < 0 || i+12 > buf.Len() {
			t.Fatal("missing OpusHead")
		}
		preSkip := binary.LittleEndian.Uint16(buf.Bytes()[i+10 : i+12])
		if preSkip != 0 {
			t.Fatalf("pre-skip = %d samples, want 0 so overlay time matches capture ticks", preSkip)
		}
	})
}

func fullDemoReport() Report {
	return Report{
		Demo:         "match.dem",
		Tickrate:     64,
		SampleRateHz: 48000,
		Target:       PlayerVoice{SteamID64: fullDemoPOV, Name: "pov", Team: "T"},
		Teammates:    []PlayerVoice{{SteamID64: fullDemoMate, Name: "mate", Team: "T"}},
	}
}

func opusPkt(steamid string, tick int) Packet {
	var id uint64
	for _, r := range steamid {
		id = id*10 + uint64(r-'0')
	}
	return Packet{XUID: id, Tick: tick, Format: FormatOpus, Data: fullDemoOpus, SampleRate: 48000}
}
