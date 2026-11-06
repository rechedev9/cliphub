package gameassets

import (
	"encoding/binary"
	"strings"
	"testing"
)

// bitWriter packs fields least significant bit first, the way BC7 stores them.
type bitWriter struct {
	lo, hi uint64
	pos    int
}

func (w *bitWriter) put(v, n int) {
	for i := 0; i < n; i++ {
		bit := uint64(v >> i & 1)
		if p := w.pos + i; p < 64 {
			w.lo |= bit << p
		} else {
			w.hi |= bit << (p - 64)
		}
	}
	w.pos += n
}

func (w *bitWriter) block(t *testing.T) []byte {
	t.Helper()
	if w.pos != 128 {
		t.Fatalf("block has %d bits, want 128", w.pos)
	}
	out := make([]byte, 16)
	binary.LittleEndian.PutUint64(out, w.lo)
	binary.LittleEndian.PutUint64(out[8:], w.hi)
	return out
}

func pixel(px *[64]byte, i int) [4]byte {
	return [4]byte{px[i*4], px[i*4+1], px[i*4+2], px[i*4+3]}
}

func TestBC1FourAndThreeColorModes(t *testing.T) {
	var px [64]byte
	block := []byte{0x00, 0xf8, 0x1f, 0x00, 0, 0, 0, 0} // red > blue: four colors
	binary.LittleEndian.PutUint32(block[4:], 0b11100100_11100100_11100100_11100100)
	decodeBC1(block, &px)
	want := [4][4]byte{{255, 0, 0, 255}, {0, 0, 255, 255}, {170, 0, 85, 255}, {85, 0, 170, 255}}
	for i := 0; i < 16; i++ {
		if got := pixel(&px, i); got != want[i%4] {
			t.Errorf("four-color pixel %d = %v, want %v", i, got, want[i%4])
		}
	}

	block = []byte{0x1f, 0x00, 0x00, 0xf8, 0, 0, 0, 0} // blue < red: three colors and transparent
	binary.LittleEndian.PutUint32(block[4:], 0b11100100)
	decodeBC1(block, &px)
	want = [4][4]byte{{0, 0, 255, 255}, {255, 0, 0, 255}, {127, 0, 127, 255}, {0, 0, 0, 0}}
	for i := 0; i < 4; i++ {
		if got := pixel(&px, i); got != want[i] {
			t.Errorf("three-color pixel %d = %v, want %v", i, got, want[i])
		}
	}
}

func TestBC3AlphaModes(t *testing.T) {
	var px [64]byte
	block := make([]byte, 16)
	copy(block[8:], []byte{0xff, 0xff, 0x00, 0x00, 0, 0, 0, 0}) // white color block
	bits := uint64(2) | uint64(7)<<3 | uint64(6)<<45
	for i := 0; i < 6; i++ {
		block[2+i] = byte(bits >> (8 * i))
	}

	block[0], block[1] = 255, 0 // eight interpolated values
	decodeBC3(block, &px)
	for i, want := range map[int]byte{0: 218, 1: 36, 15: 72, 2: 255} {
		if got := px[i*4+3]; got != want {
			t.Errorf("8-value alpha of pixel %d = %d, want %d", i, got, want)
		}
	}
	if got := pixel(&px, 0); got[0] != 255 || got[1] != 255 || got[2] != 255 {
		t.Errorf("color = %v, want white", got)
	}

	block[0], block[1] = 0, 255 // six values plus 0 and 255
	decodeBC3(block, &px)
	for i, want := range map[int]byte{0: 51, 1: 255, 15: 0} {
		if got := px[i*4+3]; got != want {
			t.Errorf("6-value alpha of pixel %d = %d, want %d", i, got, want)
		}
	}
}

func TestBC7Mode6(t *testing.T) {
	var w bitWriter
	w.put(1<<6, 7)
	for _, pair := range [][2]int{{10, 100}, {20, 50}, {30, 0}, {127, 64}} { // R, G, B, A
		w.put(pair[0], 7)
		w.put(pair[1], 7)
	}
	w.put(1, 1) // endpoint 0 p-bit
	w.put(0, 1) // endpoint 1 p-bit
	for i := 0; i < 16; i++ {
		n := 4
		if i == 0 {
			n = 3
		}
		w.put(i, n)
	}
	var px [64]byte
	decodeBC7(w.block(t), &px)
	for i, want := range map[int][4]byte{0: {21, 41, 61, 255}, 8: {116, 72, 29, 188}, 15: {200, 100, 0, 128}} {
		if got := pixel(&px, i); got != want {
			t.Errorf("pixel %d = %v, want %v", i, got, want)
		}
	}
}

func TestBC7Mode5Rotation(t *testing.T) {
	var w bitWriter
	w.put(1<<5, 6)
	w.put(1, 2) // rotation 1 swaps alpha and red
	for ch := 0; ch < 3; ch++ {
		w.put(0, 7)
		w.put(127, 7)
	}
	w.put(0, 8)
	w.put(255, 8)
	w.put(0, 1) // color index of pixel 0
	for i := 1; i < 16; i++ {
		w.put(3, 2)
	}
	w.put(0, 1) // alpha index of pixel 0
	for i := 1; i < 16; i++ {
		w.put(2, 2)
	}
	var px [64]byte
	decodeBC7(w.block(t), &px)
	if got := pixel(&px, 0); got != [4]byte{0, 0, 0, 0} {
		t.Errorf("pixel 0 = %v, want transparent black", got)
	}
	if got := pixel(&px, 9); got != [4]byte{171, 255, 255, 255} {
		t.Errorf("pixel 9 = %v, want red and alpha swapped to {171 255 255 255}", got)
	}
}

// Mode 1, shape 17: pixels 1, 2, 3 and 7 form subset 1, whose anchor is pixel 2,
// so a wrong anchor bit width shifts every later index.
func TestBC7Mode1PartitionAndSharedPBits(t *testing.T) {
	var w bitWriter
	w.put(1<<1, 2)
	w.put(17, 6)
	subset0, subset1 := [2][3]int{{0, 0, 0}, {63, 63, 63}}, [2][3]int{{0, 0, 0}, {0, 0, 63}}
	for ch := 0; ch < 3; ch++ {
		w.put(subset0[0][ch], 6)
		w.put(subset0[1][ch], 6)
		w.put(subset1[0][ch], 6)
		w.put(subset1[1][ch], 6)
	}
	w.put(1, 1) // shared p-bit of subset 0
	w.put(1, 1) // shared p-bit of subset 1
	for i := 0; i < 16; i++ {
		n := 3
		if i == 0 || i == 2 {
			n = 2
		}
		w.put(i%8, n)
	}
	var px [64]byte
	decodeBC7(w.block(t), &px)
	gray := [8]byte{2, 38, 73, 109, 148, 184, 219, 255}
	for i := 0; i < 16; i++ {
		g := gray[i%8]
		want := [4]byte{g, g, g, 255}
		if i == 1 || i == 2 || i == 3 || i == 7 {
			want = [4]byte{2, 2, g, 255}
		}
		if got := pixel(&px, i); got != want {
			t.Errorf("pixel %d = %v, want %v", i, got, want)
		}
	}
}

func TestBC7ReservedModeIsTransparentBlack(t *testing.T) {
	px := [64]byte{1, 2, 3}
	decodeBC7(make([]byte, 16), &px)
	if px != ([64]byte{}) {
		t.Fatal("a reserved-mode block must decode to transparent black")
	}
}

// The anchor tables must name a pixel of the subset they anchor, and every
// three-subset shape must use all three subsets with pixel 0 in subset 0.
func TestBC7PartitionTablesAreConsistent(t *testing.T) {
	for p := 0; p < 64; p++ {
		if bc7Subset(2, p, 0) != 0 || bc7Subset(2, p, int(bc7Anchor2[p])) != 1 {
			t.Errorf("two-subset shape %d: anchor %d is not in subset 1", p, bc7Anchor2[p])
		}
		shape := bc7Partition3[p]
		if len(shape) != 16 || shape[0] != '0' || !strings.Contains(shape, "1") || !strings.Contains(shape, "2") {
			t.Errorf("three-subset shape %d = %q", p, shape)
		}
		if bc7Subset(3, p, int(bc7Anchor3a[p])) != 1 || bc7Subset(3, p, int(bc7Anchor3b[p])) != 2 {
			t.Errorf("three-subset shape %d: anchors %d, %d not in subsets 1, 2", p, bc7Anchor3a[p], bc7Anchor3b[p])
		}
	}
}
