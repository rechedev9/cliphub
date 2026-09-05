package voicecomms

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestDecodedTeamVoiceAfterLongSilenceAndSideChange(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("FFmpeg is required for the decoded voice clock canary")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	const target, mate, enemy = "76561198000000001", "76561198000000002", "76561198000000003"
	const rate, lateSeconds = 100, 31 * 60
	sightings := []Sighting{
		{SteamID64: target, Team: "CT", Tick: 0}, {SteamID64: mate, Team: "CT", Tick: 0}, {SteamID64: enemy, Team: "T", Tick: 0},
		{SteamID64: target, Team: "T", Tick: 90000}, {SteamID64: mate, Team: "T", Tick: 90000}, {SteamID64: enemy, Team: "CT", Tick: 90000},
	}
	var packets []Packet
	for _, source := range []struct {
		id        string
		frequency int
	}{{target, 880}, {mate, 660}, {enemy, 1320}} {
		ogg, err := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "sine=f="+strconv.Itoa(source.frequency)+":r=48000:d=0.2", "-ac", "1", "-c:a", "libopus", "-frame_duration", "20", "-f", "ogg", "pipe:1").Output()
		if err != nil {
			t.Fatal(err)
		}
		frames := testOpusPackets(t, ogg)
		id, err := strconv.ParseUint(source.id, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		for _, start := range []int{rate, lateSeconds * rate} {
			for i, frame := range frames {
				packets = append(packets, Packet{XUID: id, Tick: start + i*2, Format: FormatOpus, ClockKind: "ingame_tick", Data: frame})
			}
		}
	}
	report, err := Classify(target, packets, sightings, Meta{Tickrate: rate, DurationTicks: lateSeconds*rate + 100})
	if err != nil {
		t.Fatal(err)
	}
	index, err := writeTracksWithSpillContext(ctx, filepath.Join(t.TempDir(), "tracks"), report, packets, sightings, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(index.Tracks) != 2 {
		t.Fatalf("enemy or missing teammate tracks: %+v", index.Tracks)
	}
	for _, track := range index.Tracks {
		frequency := 880.0
		if track.SteamID64 == mate {
			frequency = 660
		} else if track.SteamID64 != target {
			t.Fatal("enemy track escaped filtering")
		}
		for _, second := range []int{1, lateSeconds} {
			pcm, err := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-ss", strconv.FormatFloat(float64(second)-.1, 'f', 3, 64), "-i", track.Path, "-t", "0.3", "-ac", "1", "-ar", "48000", "-f", "f32le", "pipe:1").Output()
			if err != nil || len(pcm) < .3*48000*4 {
				t.Fatalf("decoded timeline: bytes=%d error=%v", len(pcm), err)
			}
			power := func(start, count int, freq float64) float64 {
				var re, im float64
				for i := 0; i < count; i++ {
					v := float64(math.Float32frombits(binary.LittleEndian.Uint32(pcm[(start+i)*4 : (start+i+1)*4])))
					angle := 2 * math.Pi * freq * float64(i) / 48000
					re += v * math.Cos(angle)
					im += v * math.Sin(angle)
				}
				return re*re + im*im
			}
			before, wanted, opponent := power(480, 1920, frequency), power(7680, 4800, frequency), power(7680, 4800, 1320)
			if wanted < 1 || before > wanted*1e-5 || opponent > wanted*1e-4 {
				t.Fatalf("speaker=%s second=%d before=%g wanted=%g enemy=%g", track.SteamID64, second, before, wanted, opponent)
			}
		}
	}
}

// Read only our short generated Ogg fixture, including laced packets. Production
// extraction consumes demo packets directly and does not use this test reader.
func testOpusPackets(t *testing.T, data []byte) [][]byte {
	t.Helper()
	var packets [][]byte
	var pending []byte
	for len(data) > 0 {
		if len(data) < 27 || string(data[:4]) != "OggS" {
			t.Fatal("invalid generated Ogg page")
		}
		n := int(data[26])
		if len(data) < 27+n {
			t.Fatal("invalid Ogg lacing")
		}
		laces, body := data[27:27+n], data[27+n:]
		used := 0
		for _, lace := range laces {
			end := used + int(lace)
			if end > len(body) {
				t.Fatal("truncated Ogg packet")
			}
			pending = append(pending, body[used:end]...)
			used = end
			if lace < 255 {
				if !bytes.HasPrefix(pending, []byte("OpusHead")) && !bytes.HasPrefix(pending, []byte("OpusTags")) {
					packets = append(packets, pending)
				}
				pending = nil
			}
		}
		data = body[used:]
	}
	if len(packets) < 5 || len(pending) > 0 {
		t.Fatal("no complete audible Opus fixture")
	}
	return packets
}
