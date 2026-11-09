package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/httpapi"
)

// communityOff in ZV_COMMUNITY_URL keeps Studio off the community server.
const communityOff = "off"

// communityDeps reads ZV_COMMUNITY_URL: empty for the default server, "off"
// to stay offline, or another https URL.
func communityDeps(dataDir string) httpapi.CommunityDeps {
	dir := filepath.Join(dataDir, "community")
	raw := os.Getenv("ZV_COMMUNITY_URL")
	var client *community.Client
	switch raw {
	case communityOff:
		log.Printf("community: ZV_COMMUNITY_URL=off, the community server is disabled")
	default:
		if raw == "" {
			raw = community.DefaultBaseURL
		}
		c, err := community.NewClient(raw)
		if err != nil {
			log.Printf("community: %v; the community server is disabled", err)
		} else {
			client = c
		}
	}
	return httpapi.CommunityDeps{
		Client:    client,
		Reference: community.OpenProReferenceCache(dir, client),
		Lineups:   community.OpenLineupsCache(dir, client),
		Dir:       dir,
	}
}
