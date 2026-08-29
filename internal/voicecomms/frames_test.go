package voicecomms

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/voiceprofile"
)

func TestSplitVoiceFrames(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		offsets []uint32
		want    [][]byte
	}{
		{name: "empty", want: nil},
		{name: "single blob", data: []byte{1, 2, 3}, want: [][]byte{{1, 2, 3}}},
		{
			name:    "start offsets",
			data:    []byte{10, 11, 12, 20, 21, 30},
			offsets: []uint32{0, 3, 5},
			want:    [][]byte{{10, 11, 12}, {20, 21}, {30}},
		},
		{
			name:    "sizes",
			data:    []byte{1, 2, 3, 4, 5},
			offsets: []uint32{2, 3},
			want:    [][]byte{{1, 2}, {3, 4, 5}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitVoiceFrames(tt.data, tt.offsets)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d (%v)", len(got), len(tt.want), got)
			}
			for i := range got {
				if !bytes.Equal(got[i], tt.want[i]) {
					t.Fatalf("frame %d = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestWriteOggOpusIsValidContainer(t *testing.T) {
	var buf bytes.Buffer
	frames := [][]byte{
		{0xF8, 0xFF, 0xFE},
		{0xF8, 0xFF, 0xFE},
	}
	if err := WriteOggOpus(&buf, frames, 48000, 1); err != nil {
		t.Fatal(err)
	}
	body := buf.Bytes()
	if !bytes.HasPrefix(body, []byte("OggS")) || !bytes.Contains(body, []byte("OpusHead")) || !bytes.Contains(body, []byte("OpusTags")) {
		t.Fatalf("missing ogg/opus headers: %q", body[:min(80, len(body))])
	}
	if _, err := voiceprofile.ValidateAudio(bytes.NewReader(body)); err != nil {
		t.Fatalf("ValidateAudio: %v", err)
	}
}

func TestOpusHeadHasNoEncoderPreSkip(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteOggOpus(&buf, [][]byte{{0xF8, 0xFF, 0xFE}}, 48000, 1); err != nil {
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
}

func TestTimelineFramesAlignsPacketToTickTime(t *testing.T) {
	pkt := []byte{0xF8, 0xFF, 0xFE}
	tests := []struct {
		name     string
		tick     int
		tickrate int
		wantLead int
	}{
		{name: "one second at 64", tick: 64, tickrate: 64, wantLead: 50},
		{name: "zero tick is immediate", tick: 0, tickrate: 64, wantLead: 0},
		{name: "two seconds at 128", tick: 256, tickrate: 128, wantLead: 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frames := timelineFrames([]Packet{{Tick: tt.tick, Data: pkt}}, tt.tickrate, 0)
			if len(frames) != tt.wantLead+1 {
				t.Fatalf("len = %d, want %d silence + packet", len(frames), tt.wantLead+1)
			}
			for i := 0; i < tt.wantLead; i++ {
				if !bytes.Equal(frames[i], opusSilence20ms) {
					t.Fatalf("frame %d is not silence", i)
				}
			}
			if !bytes.Equal(frames[tt.wantLead], pkt) {
				t.Fatalf("packet was not placed at tick time")
			}
		})
	}
}

func TestWriteOggPageRejectsNegativeGranule(t *testing.T) {
	tests := []struct {
		name    string
		granule int64
		wantErr string
	}{
		{name: "negative one", granule: -1, wantErr: "ogg page granule must be non-negative"},
		{name: "min int64", granule: math.MinInt64, wantErr: "ogg page granule must be non-negative"},
		{name: "zero", granule: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := writeOggPage(&buf, 0, tt.granule, 1, 0, [][]byte{{0xF8, 0xFF, 0xFE}})
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("writeOggPage(granule=%d) = %v, want nil", tt.granule, err)
				}
				if buf.Len() == 0 {
					t.Fatal("writeOggPage wrote nothing")
				}
				return
			}
			if err == nil {
				t.Fatalf("writeOggPage(granule=%d) succeeded, want error containing %q", tt.granule, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("writeOggPage(granule=%d) error = %q, want it to contain %q", tt.granule, err, tt.wantErr)
			}
			if buf.Len() != 0 {
				t.Fatalf("writeOggPage wrote %d bytes on error", buf.Len())
			}
		})
	}
}
