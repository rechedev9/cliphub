package httpapi

import (
	"context"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

const (
	codeCommunityOff  = "community_disabled"
	codeCommunityDown = "community_unavailable"
)

// CommunityDeps are the community server pieces; a nil Client means the server is disabled.
type CommunityDeps struct {
	Client    *community.Client
	Reference *community.ProReferenceCache
	Lineups   *community.LineupsCache
	// Dir holds the community files; a leftover rankings opt-in there is retired on start.
	Dir string
}

// WithCommunity wires the server pro reference and the pro lineups.
func WithCommunity(deps CommunityDeps) Option {
	return func(h *Handlers) {
		h.references.community = deps
	}
}

// proReference is the server reference when Studio has one, else the bundled one.
func (h *Handlers) proReference() (tacticalplan.ProReference, string) {
	if c := h.references.community.Reference; c != nil {
		if ref, server := c.Current(); server {
			return ref, referenceSourceServer
		}
	}
	return tacticalplan.DefaultProReference(), referenceSourceBundled
}

// StartCommunity refreshes the pro reference and imports the followed pro teams'
// matches in the background until ctx ends, and asks the server to forget an install that shared rankings.
func (h *Handlers) StartCommunity(ctx context.Context) {
	deps := h.references.community
	if deps.Reference != nil {
		go deps.Reference.Run(ctx)
	}
	if deps.Client != nil && h.libraryRepo != nil {
		wake := make(chan struct{}, 1)
		h.references.pro.wake = wake
		go h.runProFollowSync(ctx, wake)
	}
	if deps.Dir != "" {
		go community.RetireRankings(ctx, deps.Dir, deps.Client)
	}
}
