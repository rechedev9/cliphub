package demosafe

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
)

type fakeParser struct {
	demoinfocs.Parser
	parse func() error
}

func (f fakeParser) ParseToEnd() error { return f.parse() }

func TestParseToEndTurnsAPanicIntoAnError(t *testing.T) {
	err := ParseToEnd(fakeParser{parse: func() error { panic("index out of range") }})
	if !errors.Is(err, ErrPanicked) || !strings.Contains(err.Error(), "parser panicked: index out of range") {
		t.Fatalf("err = %v, want the panic as an error", err)
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

// A read error while the parser fills its first buffer comes back as an error, not a panic.
func TestNewParserReturnsAReadError(t *testing.T) {
	readErr := &fs.PathError{Op: "read", Path: "demo.dem", Err: errors.New("the process cannot access the file")}
	p, err := NewParser(failingReader{readErr})
	if p != nil || !errors.Is(err, readErr) || !strings.HasPrefix(err.Error(), "open demo parser: parser panicked: ") {
		t.Fatalf("NewParser = %v, %v; want no parser and the read error", p, err)
	}
}

func TestParseToEndKeepsTheParserError(t *testing.T) {
	err := ParseToEnd(fakeParser{parse: func() error { return demoinfocs.ErrCancelled }})
	if !errors.Is(err, demoinfocs.ErrCancelled) {
		t.Fatalf("err = %v, want ErrCancelled", err)
	}
}
