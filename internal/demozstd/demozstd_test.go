package demozstd

import (
	"bytes"
	"io"
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
