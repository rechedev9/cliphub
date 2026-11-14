//go:build js && wasm

package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// fakeDemo is a CS2 header whose file info offset sits inside the payload, so the length check passes.
func fakeDemo(size int) []byte {
	demo := make([]byte, size)
	copy(demo, "PBDEMS2\x00")
	binary.LittleEndian.PutUint32(demo[8:12], uint32(size-1))
	for i := 16; i < size; i++ {
		demo[i] = byte(i * 31)
	}
	return demo
}

func zstdBytes(t *testing.T, plain []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	enc, err := zstd.NewWriter(&out)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := enc.Write(plain); err != nil {
		t.Fatal(err)
	}
	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func gzipBytes(t *testing.T, plain []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	enc := gzip.NewWriter(&out)
	if _, err := enc.Write(plain); err != nil {
		t.Fatal(err)
	}
	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestOpenDemoReadsTheSameDemoFromEveryFormat(t *testing.T) {
	plain := fakeDemo(256 << 10)
	want := sha256.Sum256(plain)
	for name, file := range map[string][]byte{"dem": plain, "zst": zstdBytes(t, plain), "gz": gzipBytes(t, plain)} {
		stream, err := openDemo(bytes.NewReader(file))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got, err := io.ReadAll(stream)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if sha256.Sum256(got) != want || stream.n != int64(len(plain)) {
			t.Fatalf("%s: read %d bytes with another checksum, want the %d plain bytes", name, stream.n, len(plain))
		}
		if err := stream.finish(nil); err != nil {
			t.Fatalf("%s: finish: %v", name, err)
		}
		_ = stream.Close()
	}
}

func TestOpenDemoRefusesWhatIsNotACS2Demo(t *testing.T) {
	csgo := append([]byte("HL2DEMO\x00"), make([]byte, 64)...)
	for name, test := range map[string]struct {
		file []byte
		want error
	}{
		"csgo":         {file: csgo, want: errCSGODemo},
		"csgo in gzip": {file: gzipBytes(t, csgo), want: errCSGODemo},
		"text":         {file: []byte("hello, this is not a demo"), want: errNotADemo},
		"empty":        {file: nil, want: errNotADemo},
		"broken gzip":  {file: []byte{0x1f, 0x8b, 0x00}, want: errBadArchive},
	} {
		if _, err := openDemo(bytes.NewReader(test.file)); !errors.Is(err, test.want) {
			t.Errorf("%s: err = %v, want %v", name, err, test.want)
		}
	}
}

func TestFinishRefusesACutDownload(t *testing.T) {
	cut := fakeDemo(64 << 10)
	binary.LittleEndian.PutUint32(cut[8:12], 1<<20)
	stream, err := openDemo(bytes.NewReader(zstdBytes(t, cut)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(stream); err != nil {
		t.Fatal(err)
	}
	if err := stream.finish(nil); !errors.Is(err, errTruncated) {
		t.Fatalf("finish = %v, want errTruncated", err)
	}
}

func TestFinishReadsTheRestAfterAFailedScan(t *testing.T) {
	plain := fakeDemo(128 << 10)
	stream, err := openDemo(bytes.NewReader(gzipBytes(t, plain)))
	if err != nil {
		t.Fatal(err)
	}
	scanErr := errors.New("parsing demo: bad packet")
	if err := stream.finish(scanErr); !errors.Is(err, scanErr) {
		t.Fatalf("finish = %v, want the scan error, not a cut download", err)
	}
}

func TestUploadNameMatchesStudio(t *testing.T) {
	for in, want := range map[string]string{
		"1-abcd-1-2.dem.zst": "1-abcd-1-2.dem",
		"match.dem.gz":       "match.dem",
		"match.DEM.GZ":       "match.DEM",
		"match.dem":          "match.dem",
		"":                   "",
	} {
		if got := uploadName(in); got != want {
			t.Errorf("uploadName(%q) = %q, want %q", in, got, want)
		}
	}
}
