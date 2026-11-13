package clipmind

import (
	"fmt"
	"log"
	"path/filepath"

	lineupdata "github.com/rechedev9/cliphub/data/lineups"
	"github.com/rechedev9/cliphub/internal/faceit"
	"github.com/rechedev9/cliphub/internal/httpapi"
	"github.com/rechedev9/cliphub/internal/lineups"
	"github.com/rechedev9/cliphub/internal/storage"
)

// shared is what every workspace reads the same way Studio does: the game's map
// assets, the pro data and the lineup catalog. It lives outside workspaces/, so
// deleting a workspace never touches it.
type shared struct {
	opts      []httpapi.Option
	community httpapi.CommunityDeps
}

func openShared(cfg Config) (shared, error) {
	cache, err := storage.NewLocal(filepath.Join(cfg.DataDir, "gameassets"))
	if err != nil {
		return shared{}, fmt.Errorf("clipmind: game asset cache: %w", err)
	}
	if cfg.CS2Path == "" {
		log.Printf("clipmind: no CS2 install configured; maps and icons answer cs2_not_found")
	}
	if n, err := httpapi.PruneBundledMapPictures(filepath.Join(cfg.DataDir, "gameassets")); err != nil || n > 0 {
		log.Printf("clipmind: removed %d old map pictures from the game cache (%v)", n, err)
	}
	community := httpapi.OpenCommunity(cfg.DataDir, cfg.CommunityURL)
	opts := []httpapi.Option{httpapi.WithGameFiles(cfg.CS2Path, cache), httpapi.WithCommunity(community)}
	if catalog, err := lineups.LoadFS(lineupdata.Files); err != nil {
		log.Printf("clipmind: lineup catalog: %v", err)
	} else {
		opts = append(opts, httpapi.WithLineupCatalog(catalog))
	}
	if cfg.FaceitAPIKey != "" {
		client, err := faceit.New(faceit.Options{APIKey: cfg.FaceitAPIKey})
		if err != nil {
			return shared{}, fmt.Errorf("clipmind: faceit client: %w", err)
		}
		opts = append(opts, httpapi.WithFaceit(client, nil))
	}
	return shared{opts: opts, community: community}, nil
}
