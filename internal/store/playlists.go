package store

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// PlaylistRepository stores the saved playlists of rounds. Every read resolves
// a round's match by its id, else by its demo's hash; a round with neither in
// the library reads as missing and keeps its stored map.
type PlaylistRepository interface {
	// CreatePlaylist stores a playlist with its rounds; an id already stored is ErrConflict.
	CreatePlaylist(ctx context.Context, p *library.Playlist) error
	GetPlaylist(ctx context.Context, id uuid.UUID) (library.Playlist, error)
	// ListPlaylists returns playlists by most recent edit; the map filter reads the stored maps.
	ListPlaylists(ctx context.Context, filter library.PlaylistFilter) ([]library.Playlist, error)
	// UpdatePlaylist applies a patch; only a patch that Edits moves updated_at.
	UpdatePlaylist(ctx context.Context, id uuid.UUID, patch library.PlaylistPatch) (library.Playlist, error)
	// DeletePlaylist removes a playlist; a missing id is ErrNotFound.
	DeletePlaylist(ctx context.Context, id uuid.UUID) error
}

func clampPlaylistLimit(limit int) int {
	if limit <= 0 || limit > library.MaxPlaylistList {
		return library.MaxPlaylistList
	}
	return limit
}

// playlistSearchKey is what a query matches, the same text SQLite builds.
func playlistSearchKey(p library.Playlist) string {
	return strings.ToLower(p.Name + "\n" + p.Author + "\n" + p.Folder)
}

// playlistMatches is the list filter over a stored playlist; NoFolder wins over Folder.
func playlistMatches(p library.Playlist, f library.PlaylistFilter) bool {
	switch {
	case f.Favorite && !p.Favorite, f.NoFolder && p.Folder != "",
		!f.NoFolder && f.Folder != "" && strings.ToLower(p.Folder) != strings.ToLower(f.Folder):
		return false
	case f.Map != "" && !slices.ContainsFunc(p.Rounds, func(r library.PlaylistRound) bool { return r.Map == f.Map }):
		return false
	}
	q := strings.ToLower(library.CleanName(f.Query))
	return q == "" || strings.Contains(playlistSearchKey(p), q)
}

// playlistCopy gives a playlist its own round and note lists and its derived
// maps; a round without notes reads as an empty list, as SQLite gives it.
func playlistCopy(p library.Playlist) library.Playlist {
	p.Rounds = append([]library.PlaylistRound{}, p.Rounds...)
	for i, r := range p.Rounds {
		p.Rounds[i].Notes = append([]library.PlaylistNote{}, r.Notes...)
	}
	p.Maps = library.PlaylistMaps(p.Rounds)
	return p
}
