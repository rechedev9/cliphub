package demozstd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/klauspost/compress/zstd"
)

var magic = []byte{0x28, 0xb5, 0x2f, 0xfd}

type reader struct {
	src    io.Reader
	closer io.Closer
}

func (r *reader) Read(p []byte) (int, error) {
	return r.src.Read(p)
}

func (r *reader) Close() error {
	if r.closer == nil {
		return nil
	}
	return r.closer.Close()
}

func Open(src io.Reader, fileName string, maxBytes int64) (io.ReadCloser, string, error) {
	if maxBytes < 1 {
		return nil, "", fmt.Errorf("demo size limit must be positive")
	}
	buffered := bufio.NewReader(src)
	head, err := buffered.Peek(len(magic))
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, "", fmt.Errorf("read upload header: %w", err)
	}
	name := DisplayName(fileName)
	if !bytes.Equal(head, magic) {
		return &reader{src: io.LimitReader(buffered, maxBytes+1)}, name, nil
	}
	dec, err := zstd.NewReader(buffered, zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(uint64(maxBytes)+1<<20))
	if err != nil {
		return nil, "", fmt.Errorf("open zstd demo: %w", err)
	}
	return &reader{src: io.LimitReader(dec, maxBytes+1), closer: closerFunc(dec.Close)}, name, nil
}

func DisplayName(fileName string) string {
	trimmed := strings.TrimSpace(fileName)
	if len(trimmed) >= 4 && strings.EqualFold(trimmed[len(trimmed)-4:], ".zst") {
		return strings.TrimSpace(trimmed[:len(trimmed)-4])
	}
	return trimmed
}

type closerFunc func()

func (fn closerFunc) Close() error {
	fn()
	return nil
}
