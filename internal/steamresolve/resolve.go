// Package steamresolve turns a CS2 share code into match identifiers and,
// when a Steam Game Coordinator transport is available and configured, the
// match's downloadable demo URL. Decoding is pure offline arithmetic and
// always works; resolving the demo URL is best effort on top of it.
//
// Credentials come from the environment only (ZV_STEAM_USERNAME,
// ZV_STEAM_PASSWORD, ZV_STEAM_GUARD), exactly like every other external tool
// in this product. Nothing here persists a password.
package steamresolve

import (
	"context"
	"errors"
	"fmt"

	"github.com/rechedev9/cliphub/internal/sharecode"
	"github.com/rechedev9/cliphub/internal/steamgc"
)

// ErrInvalidCode marks a share code that could not be decoded. The HTTP layer
// maps it to a 400 with errors.Is.
var ErrInvalidCode = errors.New("invalid share code")

// Environment variables holding the Steam session credentials.
const (
	EnvUsername = "ZV_STEAM_USERNAME"
	// #nosec G101 -- environment variable names, never credential values.
	EnvPassword = "ZV_STEAM_PASSWORD"
	EnvGuard    = "ZV_STEAM_GUARD"
)

// Transport asks the CS2 Game Coordinator for the matches identified by req.
type Transport interface {
	RequestMatch(ctx context.Context, req steamgc.Request) ([]steamgc.Match, error)
}

// Result is a resolved share code. The identifiers are always present because
// decoding needs no Steam session; DemoURL is set and Resolved is true only
// when the Game Coordinator returned this match with a downloadable demo.
type Result struct {
	MatchID   uint64
	OutcomeID uint64
	TokenID   uint32
	DemoURL   string
	Resolved  bool
}

// Service resolves share codes, optionally through a Game Coordinator
// transport. A nil transport degrades to decode-only, never to an error.
type Service struct {
	transport Transport
}

// NewService builds a Service. t may be nil, in which case Resolve returns
// decoded identifiers with Resolved false.
func NewService(t Transport) *Service {
	return &Service{transport: t}
}

// Resolve decodes code and, when a transport is wired, asks the Game
// Coordinator for the match's demo URL.
//
// Decoding succeeds independently of Steam: with no transport the decoded
// identifiers come back with Resolved false and no error — "we know which
// match this is, we just cannot fetch it" is a real answer, not a failure. An
// expired demo (the GC returns no matching match, or the match without a URL)
// is likewise Resolved false with no error. Only a bad code (ErrInvalidCode)
// or a transport failure is an error. The caller decides whether to wire a
// transport; this method does not re-read the environment.
func (s *Service) Resolve(ctx context.Context, code string) (Result, error) {
	m, err := sharecode.Decode(code)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrInvalidCode, err)
	}
	res := Result{MatchID: m.MatchID, OutcomeID: m.OutcomeID, TokenID: m.TokenID}
	if s == nil || s.transport == nil {
		return res, nil
	}
	matches, err := s.transport.RequestMatch(ctx, steamgc.Request{
		MatchID:   m.MatchID,
		OutcomeID: m.OutcomeID,
		Token:     m.TokenID,
	})
	if err != nil {
		// Deliberately omits the share code and credentials from the message.
		return Result{}, fmt.Errorf("steamresolve: game coordinator request failed: %w", err)
	}
	for _, match := range matches {
		if match.MatchID == m.MatchID && match.DemoURL != "" {
			res.DemoURL = match.DemoURL
			res.Resolved = true
			return res, nil
		}
	}
	return res, nil
}
