package parser

import (
	"encoding/binary"
	"errors"
	"testing"
)

func demoHeader(fileInfo uint32) []byte {
	h := make([]byte, DemoHeaderLen)
	copy(h, "PBDEMS2\x00")
	binary.LittleEndian.PutUint32(h[8:12], fileInfo)
	binary.LittleEndian.PutUint32(h[12:16], fileInfo-5)
	return h
}

// The sizes come from real files: 5kMirage.dem is 230188962 bytes with its
// file info at 230188944, and the copy that failed was its first 4 MB.
func TestCheckDemoLength(t *testing.T) {
	cases := []struct {
		name   string
		header []byte
		size   int64
		want   error
	}{
		{name: "complete", header: demoHeader(230188944), size: 230188962},
		{name: "cut copy", header: demoHeader(230188944), size: 4000000, want: ErrTruncatedDemo},
		{name: "cut at the file info", header: demoHeader(230188944), size: 230188944, want: ErrTruncatedDemo},
		{name: "recording never closed", header: demoHeader(0), size: 4000000},
		{name: "short header", header: demoHeader(230188944)[:8], size: 4000000},
		{name: "offset no admitted demo reaches", header: []byte("PBDEMS2\x00rest-of-demo"), size: 20},
		{name: "not a CS2 demo",header: append([]byte("HL2DEMO\x00"), demoHeader(230188944)[8:]...), size: 4000000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := CheckDemoLength(tc.header, tc.size); !errors.Is(err, tc.want) {
				t.Fatalf("CheckDemoLength error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestDemoRecordingUnclosed(t *testing.T) {
	if !DemoRecordingUnclosed(demoHeader(0)) {
		t.Error("a CS2 header without its file info offset must read as unclosed")
	}
	if DemoRecordingUnclosed(demoHeader(230188944)) {
		t.Error("a closed recording must not read as unclosed")
	}
	if DemoRecordingUnclosed(make([]byte, DemoHeaderLen)) {
		t.Error("bytes without the CS2 magic must not read as unclosed")
	}
}
