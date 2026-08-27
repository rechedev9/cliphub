package voicecomms

import (
	"bytes"
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

func TestAllowedSpeakers(t *testing.T) {
	report := Report{
		Target:    PlayerVoice{SteamID64: "1"},
		Teammates: []PlayerVoice{{SteamID64: "2"}, {SteamID64: "3"}},
	}
	got := allowedSpeakers(report)
	if len(got) != 3 || !got[1] || !got[2] || !got[3] {
		t.Fatalf("allowed = %#v", got)
	}
}
