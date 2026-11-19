// Package demosafe runs demoinfocs parses so a crafted demo cannot panic the process.
package demosafe

import (
	"errors"
	"fmt"
	"io"

	demoinfocs "github.com/rechedev9/demo-parser-rsn/pkg/demoinfocs"
)

// ErrPanicked marks a parse that panicked instead of returning an error.
var ErrPanicked = errors.New("parser panicked")

// ErrDemoTooShort is a demo that ends during the parser's first read, so every retry fails the same way.
var ErrDemoTooShort = errors.New("truncated_demo: the demo file is empty or cut short")

// ParseToEnd is p.ParseToEnd with a parser panic returned as an ErrPanicked error.
func ParseToEnd(p demoinfocs.Parser) error {
	return Recover(p.ParseToEnd)
}

// NewParser builds a demoinfocs parser, which reads its first buffer at once and panics on a read error.
func NewParser(r io.Reader) (demoinfocs.Parser, error) {
	var p demoinfocs.Parser
	err := Recover(func() error {
		p = demoinfocs.NewParser(r)
		return nil
	})
	switch {
	case err == nil:
		return p, nil
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return nil, fmt.Errorf("open demo parser: %w", ErrDemoTooShort)
	default:
		return nil, fmt.Errorf("open demo parser: %w", err)
	}
}

// Recover runs fn and returns a panic on this goroutine as an ErrPanicked error.
func Recover(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			// An error value stays on the chain, so a read error the bit reader panics with stays visible.
			if e, ok := r.(error); ok {
				err = fmt.Errorf("%w: %w", ErrPanicked, e)
				return
			}
			err = fmt.Errorf("%w: %v", ErrPanicked, r)
		}
	}()
	return fn()
}
