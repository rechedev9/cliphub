package demosafe

import (
	"errors"
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

func TestParseToEndKeepsTheParserError(t *testing.T) {
	err := ParseToEnd(fakeParser{parse: func() error { return demoinfocs.ErrCancelled }})
	if !errors.Is(err, demoinfocs.ErrCancelled) {
		t.Fatalf("err = %v, want ErrCancelled", err)
	}
}
