package store

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// resolvePlaylistRound finds a round's match as SQLite does: by id, else by its demo's hash.
func (r *MemoryLibraryRepository) resolvePlaylistRound(pr library.PlaylistRound) library.PlaylistRound {
	if m, ok := r.matches[pr.MatchID]; ok {
		pr.Map = m.mapName
		return pr
	}
	if pr.DemoSHA256 != "" {
		for _, m := range r.matches {
			if d, ok := r.demos[m.demoID]; ok && d.SHA256 == pr.DemoSHA256 {
				pr.MatchID, pr.Map = m.id, m.mapName
				return pr
			}
		}
	}
	pr.Missing = true
	return pr
}

// playlistView resolves the rounds of a stored playlist into a copy the caller owns.
func (r *MemoryLibraryRepository) playlistView(p library.Playlist) library.Playlist {
	p = playlistCopy(p)
	for i, round := range p.Rounds {
		p.Rounds[i] = r.resolvePlaylistRound(round)
	}
	p.Maps = library.PlaylistMaps(p.Rounds)
	return p
}

func (r *MemoryLibraryRepository) CreatePlaylist(ctx context.Context, p *library.Playlist) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if _, ok := r.playlists[p.ID]; ok {
		return library.ErrConflict
	}
	stored := playlistCopy(*p)
	for i := range stored.Rounds {
		stored.Rounds[i].Missing = false
	}
	now := libraryNow()
	stored.CreatedAt, stored.UpdatedAt = now, now
	r.playlists[stored.ID] = stored
	*p = r.playlistView(stored)
	return nil
}

func (r *MemoryLibraryRepository) GetPlaylist(ctx context.Context, id uuid.UUID) (library.Playlist, error) {
	if err := ctx.Err(); err != nil {
		return library.Playlist{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.playlists[id]
	if !ok {
		return library.Playlist{}, library.ErrNotFound
	}
	return r.playlistView(p), nil
}

func (r *MemoryLibraryRepository) ListPlaylists(ctx context.Context, filter library.PlaylistFilter) ([]library.Playlist, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []library.Playlist{}
	for _, p := range r.playlists {
		if playlistMatches(p, filter) {
			out = append(out, r.playlistView(p))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].ID.String() < out[j].ID.String()
	})
	offset := max(filter.Offset, 0)
	if offset >= len(out) {
		return []library.Playlist{}, nil
	}
	return out[offset:min(offset+clampPlaylistLimit(filter.Limit), len(out))], nil
}

func (r *MemoryLibraryRepository) UpdatePlaylist(ctx context.Context, id uuid.UUID, patch library.PlaylistPatch) (library.Playlist, error) {
	if err := ctx.Err(); err != nil {
		return library.Playlist{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.playlists[id]
	if !ok {
		return library.Playlist{}, library.ErrNotFound
	}
	p = library.ApplyPlaylistPatch(p, patch)
	for i := range p.Rounds {
		p.Rounds[i].Missing = false
	}
	if patch.Edits() {
		p.UpdatedAt = libraryNow()
	}
	r.playlists[id] = p
	return r.playlistView(p), nil
}

func (r *MemoryLibraryRepository) DeletePlaylist(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.playlists[id]; !ok {
		return library.ErrNotFound
	}
	delete(r.playlists, id)
	return nil
}
