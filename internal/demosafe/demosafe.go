// Package demosafe runs demoinfocs parses so a crafted demo cannot panic the process.
package demosafe

import (
	"errors"
	"fmt"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
)

// ErrPanicked marks a parse that panicked instead of returning an error.
var ErrPanicked = errors.New("parser panicked")

// ParseToEnd is p.ParseToEnd with a parser panic returned as an ErrPanicked error.
func ParseToEnd(p demoinfocs.Parser) error {
	return Recover(p.ParseToEnd)
}

// Recover runs fn and returns a panic on this goroutine as an ErrPanicked error.
func Recover(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: %v", ErrPanicked, r)
		}
	}()
	return fn()
}
