package httpapi

import "github.com/rechedev9/cliphub/internal/storage"

// gameFiles is where map and equipment assets come from when the handlers
// have no record tools to find CS2, as on the ClipMind server.
type gameFiles struct {
	cs2Exe string
	cache  storage.Storage
}

// WithGameFiles reads CS2's assets from the install of cs2Exe and caches them
// in cache, which several Handlers may share: they depend on the game build only.
func WithGameFiles(cs2Exe string, cache storage.Storage) Option {
	return func(h *Handlers) {
		h.references.game = gameFiles{cs2Exe: cs2Exe, cache: cache}
	}
}

func (h *Handlers) gameCache() storage.Storage {
	if h.references.game.cache != nil {
		return h.references.game.cache
	}
	return h.storage
}
