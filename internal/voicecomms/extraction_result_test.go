package voicecomms

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestExtractionAvailabilityAndTemporalMembership(t *testing.T) {
	const target = "76561198000000001"
	const mate = "76561198000000002"
	report := Report{Tickrate: 100, Target: PlayerVoice{SteamID64: target}}
	sightings := []Sighting{{SteamID64: target, Team: "T", Tick: 0}, {SteamID64: mate, Team: "T", Tick: 0}, {SteamID64: mate, Team: "", Tick: 200}, {SteamID64: mate, Team: "CT", Tick: 300}, {SteamID64: target, Team: "CT", Tick: 400}}
	for _, tc := range []struct {
		name                string
		tick                int
		format, clock, want string
		packets             bool
	}{
		{"no content", 100, FormatOpus, "ingame_tick", NoPackets, false},
		{"teammate", 100, FormatOpus, "ingame_tick", Available, true},
		{"disconnected", 250, FormatOpus, "ingame_tick", NoTeamPackets, true},
		{"reconnect enemy side", 350, FormatOpus, "ingame_tick", NoTeamPackets, true},
		{"overtime side change", 450, FormatOpus, "ingame_tick", Available, true},
		{"unsupported explicit", 100, FormatSteam, "ingame_tick", UnsupportedCodec, true},
		{"clock discontinuity", 100, FormatOpus, "discontinuous", InvalidTimeline, true},
		{"unmapped proto clock", 100, FormatOpus, "voice_data_tick", InvalidTimeline, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var packets []Packet
			if tc.packets {
				packets = []Packet{{XUID: 76561198000000002, Tick: tc.tick, Format: tc.format, ClockKind: tc.clock, Data: opusSilence20ms}}
			}
			r := classifyExtraction(report, packets, sightings)
			if r.Availability != tc.want {
				t.Fatalf("availability=%s want %s", r.Availability, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name string
		rows []Sighting
		tick int
		team string
	}{
		{"out of order", []Sighting{{SteamID64: mate, Team: "CT", Tick: 300}, {SteamID64: mate, Team: "T", Tick: 0}}, 350, "CT"},
		{"unknown clock cannot borrow future", []Sighting{{SteamID64: mate, Team: "T", Tick: 10}}, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, team := sightingAt(tc.rows, mate, tc.tick)
			if team != tc.team {
				t.Fatalf("team=%q want %q", team, tc.team)
			}
		})
	}
}

type granuleWriter struct {
	last  uint64
	pages int
}

func (w *granuleWriter) Write(b []byte) (int, error) {
	if len(b) >= 27 {
		w.last = binary.LittleEndian.Uint64(b[6:14])
		w.pages++
	}
	return len(b), nil
}

func TestStreamedVoiceClockKeepsLongGapsAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name          string
		seconds, rate int
	}{{"31 minute gap", 31 * 60, 100}, {"40 minute gap", 40 * 60, 128}, {"64 tick clock", 2, 64}} {
		t.Run(tc.name, func(t *testing.T) {
			w := &granuleWriter{}
			p := []indexedPacket{{packet: Packet{Tick: tc.seconds * tc.rate, Data: opusSilence20ms}}}
			if err := writeTimelineOgg(context.Background(), w, p, nil, tc.rate, 48000, 1); err != nil {
				t.Fatal(err)
			}
			want := uint64(tc.seconds)*48000 + 960
			if w.last != want {
				t.Fatalf("last sample=%d want %d (long silence was shortened)", w.last, want)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := writeTimelineOgg(ctx, io.Discard, []indexedPacket{{packet: Packet{Tick: 240000, Data: opusSilence20ms}}}, nil, 100, 48000, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel=%v", err)
	}
}

func TestOpusPacketDurations(t *testing.T) {
	for _, tc := range []struct {
		name    string
		packet  []byte
		samples int
	}{
		{"20ms CELT", []byte{0xf8, 0xff, 0xfe}, 960},
		{"two CELT frames", []byte{0xf9, 0}, 1920},
		{"three CELT frames", []byte{0xfb, 3, 0}, 2880},
		{"hybrid config14", []byte{14 << 3, 0}, 480},
		{"hybrid config15", []byte{15 << 3, 0}, 960},
		{"code3 missing count", []byte{0xfb}, 0},
		{"duration exceeds120ms", []byte{0xfb, 7}, 0},
		{"empty", nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := opusFrameSamples(tc.packet); got != tc.samples {
				t.Fatalf("samples=%d want %d", got, tc.samples)
			}
		})
	}
}

func TestSpillClosedAndOversizedPacketsFail(t *testing.T) {
	for _, tc := range []struct {
		name  string
		close bool
		size  int
	}{{"closed", true, 3}, {"oversized", false, 65536}} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := newPacketSpill(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if tc.close {
				s.Close()
			}
			if err := s.write(0, 1, 100, make([]byte, tc.size), nil); err == nil {
				t.Fatal("silently accepted unwritable packet")
			}
		})
	}
}

func TestDecodedVoicePeakDistinguishesSilence(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("FFmpeg is required for decoded voice evidence")
	}
	for _, tc := range []struct {
		name, source string
		audible      bool
	}{{"tone", "sine=frequency=880:sample_rate=48000:duration=0.25", true}, {"silence", "anullsrc=r=48000:cl=mono", false}} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "voice.ogg")
			out, err := exec.Command(ffmpeg, "-nostdin", "-v", "error", "-f", "lavfi", "-i", tc.source, "-t", "0.25", "-c:a", "libopus", path).CombinedOutput()
			if err != nil {
				t.Fatalf("fixture: %v %s", err, out)
			}
			peak, err := decodedTrackPeak(context.Background(), ffmpeg, path)
			if err != nil {
				t.Fatal(err)
			}
			if (peak > -100) != tc.audible {
				t.Fatalf("peak=%g audible=%v", peak, tc.audible)
			}
		})
	}
	bad := filepath.Join(t.TempDir(), "corrupt.ogg")
	if err := os.WriteFile(bad, bytes.Repeat([]byte("invalid"), 10), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := decodedTrackPeak(context.Background(), ffmpeg, bad); err == nil {
		t.Fatal("corrupt audio was reported as silence")
	}
}
