package demozstd

import (
	"bytes"
	"io"
	"math/bits"
	"slices"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestOpenPassesThroughPlainDemos(t *testing.T) {
	t.Parallel()
	plain := []byte("PBDEMS2\x00rest")
	rc, name, err := Open(bytes.NewReader(plain), "match.dem", 1024)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rc.Close() })
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) || name != "match.dem" {
		t.Fatalf("got %q name=%q", got, name)
	}
}

func TestOpenDecompressesZstdDemos(t *testing.T) {
	t.Parallel()
	plain := []byte("PBDEMS2\x00rest-of-demo")
	compressed := zstdBytes(t, plain)
	rc, name, err := Open(bytes.NewReader(compressed), "1-abcd.dem.zst", 1024)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rc.Close() })
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("decompressed = %q, want %q", got, plain)
	}
	if name != "1-abcd.dem" {
		t.Fatalf("name = %q, want 1-abcd.dem", name)
	}
}

func TestDisplayNameTable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want string
	}{
		{in: "match.dem", want: "match.dem"},
		{in: "match.dem.zst", want: "match.dem"},
		{in: "match.DEM.ZST", want: "match.DEM"},
		{in: "match.zst", want: "match"},
		{in: "", want: ""},
	}
	for _, test := range tests {
		if got := DisplayName(test.in); got != test.want {
			t.Errorf("DisplayName(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

func TestOpenCapsDecompressedSize(t *testing.T) {
	t.Parallel()
	plain := bytes.Repeat([]byte("PBDEMS2\x00payload"), 64)
	compressed := zstdBytes(t, plain)
	rc, _, err := Open(bytes.NewReader(compressed), "big.dem.zst", 32)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rc.Close() })
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(got)) > 33 {
		t.Fatalf("read %d bytes, want capped at 33", len(got))
	}
}

func TestOpenRejectsFramesWithAHugeWindow(t *testing.T) {
	t.Parallel()
	plain := []byte("PBDEMS2\x00rest-of-demo")
	for _, test := range []struct {
		window int
		ok     bool
	}{{window: maxWindow, ok: true}, {window: maxWindow * 2, ok: false}} {
		// A hand-built frame: window descriptor 2^(10+exponent), then one raw last block.
		exponent := byte(bits.Len(uint(test.window)) - 11)
		blockHeader := len(plain)<<3 | 1
		frame := append([]byte{0x28, 0xb5, 0x2f, 0xfd, 0x00, exponent << 3,
			byte(blockHeader), byte(blockHeader >> 8), byte(blockHeader >> 16)}, plain...)
		rc, _, err := Open(bytes.NewReader(frame), "w.dem.zst", 1<<30)
		if err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(rc)
		_ = rc.Close()
		if test.ok && (err != nil || !bytes.Equal(got, plain)) {
			t.Errorf("window %d: got %q, %v; want the demo", test.window, got, err)
		}
		if !test.ok && err == nil {
			t.Errorf("window %d: decoded %d bytes, want an error", test.window, len(got))
		}
	}
}

// TestOpenMalformedTwoBlockFrameReturnsError feeds the two-block frame from
// GHSA-fpmh-frq8-7rj8 (CVE-2026-116591). On klauspost/compress <= 1.20.1 the
// streaming decoder panics in fse.nextFast; 1.20.2 returns a decode error.
func TestOpenMalformedTwoBlockFrameReturnsError(t *testing.T) {
	t.Parallel()
	rc, _, err := Open(bytes.NewReader(twoBlockFSEPanicFrame()), "crafted.dem.zst", 8<<20)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rc.Close() })
	got, err := io.ReadAll(rc)
	if err == nil {
		t.Fatalf("decoded %d bytes from a malformed two-block frame", len(got))
	}
	if strings.Contains(err.Error(), "decompress zstd demo:") {
		t.Fatalf("decoder panicked: %v", err)
	}
}

// twoBlockFSEPanicFrame is the GHSA-fpmh-frq8-7rj8 input, built here instead of
// checked in as a blob. Magic 28 b5 2f fd, frame header descriptor 0x00, window
// descriptor 0x38 (128 KiB, under maxWindow). Block 1 leaves FSE scratch state;
// block 2's Huffman-weight table carries a normalized count of 32768, which
// readNCount stores in an int16 as -32768. The repeated tails are that payload.
func twoBlockFSEPanicFrame() []byte {
	return slices.Concat(
		[]byte{0x28, 0xb5, 0x2f, 0xfd, 0x00, 0x38, 0x74, 0x06, 0x00, 0x42, 0xb8, 0x32, 0x0d, 0x1a, 0x00, 0x00, 0x00, 0x90, 0xaf, 0xfa, 0xff, 0x03, 0x00, 0x80, 0xff, 0x7f},
		bytes.Repeat([]byte{0xa5, 0x94, 0x52, 0x4a, 0x29}, 37),
		[]byte{0xa5, 0x94, 0x12, 0x00, 0xad, 0x56, 0x03, 0xee, 0xff, 0xdf, 0xb3, 0x1a, 0x1c, 0x1a, 0x00, 0xe0, 0xff, 0x1f, 0x0f, 0xb1, 0x9c, 0x5a, 0xcf, 0x4d, 0xe4, 0x1d, 0x7b, 0xab, 0x99, 0x5e, 0x01, 0x51, 0xa4, 0x73, 0xd8, 0xb1, 0xf3, 0x39, 0x4b, 0x67, 0xf6, 0xab, 0x1a, 0xab, 0x1a, 0xab, 0x1a},
		bytes.Repeat([]byte{0x8c, 0x31, 0xc6, 0x18, 0x63}, 1365),
		[]byte{0x8c, 0x31},
		bytes.Repeat([]byte{0x63, 0x8c, 0x31, 0xc6, 0x18}, 1365),
		[]byte{0x63, 0x2c},
		bytes.Repeat([]byte{0x31, 0xc6, 0x18, 0x63, 0x8c}, 1365),
		[]byte{0x31, 0x46},
		bytes.Repeat([]byte{0x31, 0xc6, 0x18, 0x63, 0x8c}, 1365),
		[]byte{0x31, 0x06, 0x00},
	)
}

func TestDecompressPanicBecomesError(t *testing.T) {
	t.Parallel()
	r := &reader{src: panicRead{}, decompress: true}
	n, err := r.Read(make([]byte, 8))
	if n != 0 || err == nil || !strings.Contains(err.Error(), "decompress zstd demo: zstd reader canary") {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestPlainReadDoesNotRecover(t *testing.T) {
	t.Parallel()
	r := &reader{src: panicRead{}}
	defer func() {
		rec := recover()
		if rec != "zstd reader canary" {
			t.Fatalf("plain read recovered %#v", rec)
		}
	}()
	if _, err := r.Read(make([]byte, 4)); err != nil {
		t.Fatalf("plain read returned %v", err)
	}
	t.Fatal("plain read returned")
}

type panicRead struct{}

func (panicRead) Read([]byte) (int, error) {
	panic("zstd reader canary")
}

func zstdBytes(t *testing.T, plain []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := enc.Write(plain); err != nil {
		t.Fatal(err)
	}
	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
