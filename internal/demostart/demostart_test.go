package demostart

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type testRecord struct {
	kind    byte
	tick    []byte
	payload []byte
}

var noTick = []byte{0xff, 0xff, 0xff, 0xff, 0x0f}

// demoBytes lays records out the way CS2 writes them: the 16-byte header,
// then per record the command, tick and size varints and the payload.
func demoBytes(magic string, records ...testRecord) []byte {
	b := append([]byte(magic), 0, 0, 0, 0, 0, 0, 0, 0)
	for _, r := range records {
		b = append(b, r.kind)
		b = append(b, r.tick...)
		b = append(b, byte(len(r.payload)))
		b = append(b, r.payload...)
	}
	return b
}

func writeDemo(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "demo.dem")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func cs2Opening() []testRecord {
	return []testRecord{
		{kind: 1, tick: noTick, payload: []byte("header")},
		{kind: 8 | demCompressed, tick: noTick, payload: []byte("signon")},
		{kind: 3, tick: noTick},
		{kind: demPacket | demCompressed, tick: []byte{1}, payload: []byte("packet")},
		{kind: demFullPacket | demCompressed, tick: []byte{1}, payload: []byte("full")},
		{kind: demPacket, tick: []byte{2}, payload: []byte("next")},
	}
}

func TestWriteTickZeroCopyMovesOnlyTheOpeningTicks(t *testing.T) {
	t.Parallel()
	original := demoBytes(magic, cs2Opening()...)
	src := writeDemo(t, original)
	dst := filepath.Join(t.TempDir(), "copy.dem")
	if err := WriteTickZeroCopy(src, dst); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	opening := cs2Opening()
	opening[3].tick, opening[4].tick = []byte{0}, []byte{0}
	if want := demoBytes(magic, opening...); !bytes.Equal(got, want) {
		t.Fatalf("copy = %x, want %x", got, want)
	}
	if after, _ := os.ReadFile(src); !bytes.Equal(after, original) {
		t.Fatal("the source demo changed")
	}
	// The copy no longer starts at tick 1, so it is never patched twice.
	if _, err := TickOneOffsets(dst); !errors.Is(err, ErrLayout) {
		t.Fatalf("TickOneOffsets(copy) = %v, want ErrLayout", err)
	}
}

func TestTickOneOffsetsRejectsOtherOpenings(t *testing.T) {
	t.Parallel()
	opening := cs2Opening()
	laterStart := cs2Opening()
	laterStart[3].tick, laterStart[4].tick = []byte{0x80, 0x02}, []byte{0x80, 0x02}
	longTick := cs2Opening()
	longTick[4].tick = []byte{0x81, 0x00}
	noFullPacket := cs2Opening()
	noFullPacket[4] = testRecord{kind: demPacket, tick: []byte{1}}
	tickedSignon := cs2Opening()
	tickedSignon[1].tick = []byte{0}
	cases := map[string][]byte{
		"source 1 demo":            demoBytes("HL2DEMO\x00", opening...),
		"first snapshot later":     demoBytes(magic, laterStart...),
		"two-byte tick":            demoBytes(magic, longTick...),
		"packet without snapshot":  demoBytes(magic, noFullPacket...),
		"signon record with tick":  demoBytes(magic, tickedSignon...),
		"truncated before packets": demoBytes(magic, opening[:2]...),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src := writeDemo(t, data)
			if _, err := TickOneOffsets(src); !errors.Is(err, ErrLayout) {
				t.Fatalf("TickOneOffsets() = %v, want ErrLayout", err)
			}
			dst := filepath.Join(t.TempDir(), "copy.dem")
			if err := WriteTickZeroCopy(src, dst); !errors.Is(err, ErrLayout) {
				t.Fatalf("WriteTickZeroCopy() = %v, want ErrLayout", err)
			}
			if _, err := os.Stat(dst); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("a refused copy left %s behind: %v", dst, err)
			}
		})
	}
}
