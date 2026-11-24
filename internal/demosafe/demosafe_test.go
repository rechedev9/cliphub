package demosafe

import (
	"bytes"
	"errors"
	"io/fs"
	"strings"
	"testing"

	demoinfocs "github.com/rechedev9/demo-parser-rsn/pkg/demoinfocs"
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
	if p != nil || !errors.Is(err, readErr) || !errors.Is(err, ErrPanicked) {
		t.Fatalf("NewParser = %v, %v; want no parser and the read error", p, err)
	}
}

// An empty demo is a cut file, not a read error a retry may clear.
func TestNewParserRefusesAnEmptyDemo(t *testing.T) {
	p, err := NewParser(bytes.NewReader(nil))
	if p != nil || !errors.Is(err, ErrDemoTooShort) || errors.Is(err, ErrPanicked) {
		t.Fatalf("NewParser = %v, %v; want no parser and ErrDemoTooShort", p, err)
	}
}

func TestParseToEndKeepsTheParserError(t *testing.T) {
	err := ParseToEnd(fakeParser{parse: func() error { return demoinfocs.ErrCancelled }})
	if !errors.Is(err, demoinfocs.ErrCancelled) {
		t.Fatalf("err = %v, want ErrCancelled", err)
	}
}

type openedDemo struct {
	*bytes.Reader
	closes int
}

func (d *openedDemo) Close() error {
	d.closes++
	return nil
}

func TestParserLeavesTheDemoToWhoeverOpenedIt(t *testing.T) {
	demo := &openedDemo{Reader: bytes.NewReader(make([]byte, 64<<10))}
	p, err := NewParser(demo)
	if err != nil {
		t.Fatalf("NewParser: %v", err)
	}
	if err := p.Close(); err != nil || demo.closes != 0 {
		t.Fatalf("parser Close = %v and closed the demo %d times, want the demo left open for its opener", err, demo.closes)
	}
}
