package voicecomms

import (
	"os"
	"testing"

	"github.com/rechedev9/cliphub/internal/voiceprofile"
)

func TestPacketSpillRoundTrip(t *testing.T) {
	dir := t.TempDir()
	spill, err := newPacketSpill(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer spill.Close()

	opus := []byte{0xF8, 0xFF, 0xFE}
	offsets := []uint32{0}
	if err := spill.write(0, 76561198000000001, 64, opus, offsets); err != nil {
		t.Fatal(err)
	}
	got, gotOffsets, err := spill.payload(0)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(opus) {
		t.Fatalf("payload = %v, want %v", got, opus)
	}
	if len(gotOffsets) != 1 || gotOffsets[0] != 0 {
		t.Fatalf("offsets = %v, want [0]", gotOffsets)
	}
}

func TestWriteTracksWithSpillMatchesInMemory(t *testing.T) {
	const (
		pov  = "76561198000000001"
		mate = "76561198000000002"
	)
	opus := []byte{0xF8, 0xFF, 0xFE}
	packets := []Packet{
		{XUID: 76561198000000001, Tick: 64, Format: FormatOpus, Bytes: len(opus), SampleRate: 48000},
		{XUID: 76561198000000002, Tick: 128, Format: FormatOpus, Bytes: len(opus), SampleRate: 48000},
	}
	sightings := []Sighting{
		{SteamID64: pov, Name: "pov", Team: "T", Tick: 0},
		{SteamID64: mate, Name: "mate", Team: "T", Tick: 0},
	}
	report := Report{
		Demo:         "match.dem",
		Tickrate:     64,
		SampleRateHz: 48000,
		Target:       PlayerVoice{SteamID64: pov, Name: "pov", Team: "T"},
	}

	spillDir := t.TempDir()
	spill, err := newPacketSpill(spillDir)
	if err != nil {
		t.Fatal(err)
	}
	for i, pkt := range packets {
		if err := spill.write(i, pkt.XUID, pkt.Tick, opus, nil); err != nil {
			t.Fatal(err)
		}
	}

	memDir := t.TempDir()
	memPackets := []Packet{
		{XUID: packets[0].XUID, Tick: packets[0].Tick, Format: FormatOpus, Data: append([]byte(nil), opus...), Bytes: len(opus), SampleRate: 48000},
		{XUID: packets[1].XUID, Tick: packets[1].Tick, Format: FormatOpus, Data: append([]byte(nil), opus...), Bytes: len(opus), SampleRate: 48000},
	}
	memIndex, err := WriteTracks(memDir, report, memPackets, sightings)
	if err != nil {
		t.Fatal(err)
	}

	spillOutDir := t.TempDir()
	spillIndex, err := writeTracksWithSpill(spillOutDir, report, packets, sightings, spill)
	if err != nil {
		t.Fatal(err)
	}
	if err := spill.Close(); err != nil {
		t.Fatal(err)
	}
	if len(spillIndex.Tracks) != len(memIndex.Tracks) {
		t.Fatalf("spill tracks = %d, mem tracks = %d", len(spillIndex.Tracks), len(memIndex.Tracks))
	}
	for _, track := range spillIndex.Tracks {
		f, err := os.Open(track.Path)
		if err != nil {
			t.Fatalf("open spill track %s: %v", track.SteamID64, err)
		}
		_, err = voiceprofile.ValidateAudio(f)
		_ = f.Close()
		if err != nil {
			t.Fatalf("validate spill track %s: %v", track.SteamID64, err)
		}
	}
}
