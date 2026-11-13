package httpapi

import (
	"context"
	"log"
	"path/filepath"

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
	h.StartProFollowSync(ctx)
	if deps.Dir != "" {
		go community.RetireRankings(ctx, deps.Dir, deps.Client)
	}
}

// StartProFollowSync imports the followed pro teams' new maps in the background
// until ctx ends. Call it before the handlers serve requests.
func (h *Handlers) StartProFollowSync(ctx context.Context) {
	if h.references.community.Client != nil && h.libraryRepo != nil {
		wake := make(chan struct{}, 1)
		h.references.pro.wake = wake
		go h.runProFollowSync(ctx, wake)
	}
}

// CommunityOff as the base URL keeps the server pro data offline.
const CommunityOff = "off"

// OpenCommunity opens the community caches under dataDir/community. baseURL is
// empty for the default server, CommunityOff to stay offline, or another https URL.
func OpenCommunity(dataDir, baseURL string) CommunityDeps {
	dir := filepath.Join(dataDir, "community")
	var client *community.Client
	switch baseURL {
	case CommunityOff:
		log.Printf("community: the community server is disabled")
	default:
		if baseURL == "" {
			baseURL = community.DefaultBaseURL
		}
		c, err := community.NewClient(baseURL)
		if err != nil {
			log.Printf("community: %v; the community server is disabled", err)
		} else {
			client = c
		}
	}
	return CommunityDeps{
		Client:    client,
		Reference: community.OpenProReferenceCache(dir, client),
		Lineups:   community.OpenLineupsCache(dir, client),
		Dir:       dir,
	}
}
