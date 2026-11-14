//go:build js && wasm

package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/rechedev9/cliphub/internal/demozstd"
	"github.com/rechedev9/cliphub/internal/parser"
)

// maxDemoBytes is Studio's import cap (maxDemoBytes in internal/httpapi),
// counted on the decompressed demo.
const maxDemoBytes = 700 << 20

var (
	gzipMagic = []byte{0x1f, 0x8b}

	errNotADemo   = errors.New("este archivo no es una demo de CS2")
	errCSGODemo   = errors.New("es una demo de CS:GO; solo se admiten demos de CS2")
	errTooLarge   = errors.New("la demo descomprimida pasa de 700 MB")
	errTruncated  = errors.New("la demo está incompleta: vuelve a descargarla")
	errBadArchive = errors.New("el archivo comprimido está dañado")
)

// demoStream is the decompressed .dem whatever the file held: .dem, .dem.zst
// or .dem.gz. Its bytes are what Studio hashes and stores, so the SHA matches.
type demoStream struct {
	src    io.Reader
	closer io.Closer
	header []byte
	n      int64
}

// openDemo detects the format by its magic bytes and checks the CS2 header
// before the scan starts, so a wrong file fails at once with a clear reason.
func openDemo(src io.Reader) (*demoStream, error) {
	buffered := bufio.NewReader(src)
	magic, err := buffered.Peek(len(gzipMagic))
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("no se pudo leer el archivo: %w", err)
	}
	var decoded io.ReadCloser
	if bytes.Equal(magic, gzipMagic) {
		gz, err := gzip.NewReader(buffered)
		if err != nil {
			return nil, errBadArchive
		}
		decoded = gz
	} else {
		// demozstd is Studio's own import path: zstd when the magic says so, the bytes as they are otherwise.
		decoded, _, err = demozstd.Open(buffered, "", maxDemoBytes)
		if err != nil {
			return nil, errBadArchive
		}
	}
	header := make([]byte, parser.DemoHeaderLen)
	n, err := io.ReadFull(decoded, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		_ = decoded.Close()
		return nil, errBadArchive
	}
	header = header[:n]
	switch {
	case bytes.HasPrefix(header, []byte("PBDEMS2")):
	case bytes.HasPrefix(header, []byte("HL2DEMO")):
		_ = decoded.Close()
		return nil, errCSGODemo
	default:
		_ = decoded.Close()
		return nil, errNotADemo
	}
	return &demoStream{src: io.MultiReader(bytes.NewReader(header), decoded), closer: decoded, header: header}, nil
}

func (d *demoStream) Read(p []byte) (int, error) {
	n, err := d.src.Read(p)
	d.n += int64(n)
	if d.n > maxDemoBytes {
		return n, errTooLarge
	}
	return n, err
}

func (d *demoStream) Close() error {
	return d.closer.Close()
}

// finish checks the whole demo after the scan. A failed scan stops early, so the
// rest is read first: a cap or a cut download explains that failure better.
func (d *demoStream) finish(scanErr error) error {
	if scanErr != nil {
		_, _ = io.Copy(io.Discard, d)
	}
	if d.n > maxDemoBytes {
		return errTooLarge
	}
	if parser.CheckDemoLength(d.header, d.n) != nil {
		return errTruncated
	}
	return scanErr
}

// uploadName is the name Studio stores: demozstd.DisplayName drops .zst, and .gz goes the same way.
func uploadName(fileName string) string {
	name := demozstd.DisplayName(fileName)
	if len(name) >= 3 && strings.EqualFold(name[len(name)-3:], ".gz") {
		return strings.TrimSpace(name[:len(name)-3])
	}
	return name
}
