package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// Playlist file format. Rounds carry their demo's hash, so a teammate's library
// finds the same match under its own id; playlist ids make a reimport idempotent.
const (
	playlistFormat       = "cliphub-playlists"
	playlistVersion      = 1
	maxPlaylistFileBytes = 8 << 20
	codePlaylistFormat   = "playlist_format"
)

// playlistFile is exported playlists with the matches their rounds point at.
type playlistFile struct {
	Format     string              `json:"format"`
	Version    int                 `json:"version"`
	ExportedAt time.Time           `json:"exported_at"`
	Playlists  []library.Playlist  `json:"playlists"`
	Matches    []playlistFileMatch `json:"matches"`
}

// playlistFileMatch names a match so whoever lacks it knows which demo to import.
type playlistFileMatch struct {
	MatchID    uuid.UUID  `json:"match_id"`
	DemoSHA256 string     `json:"demo_sha256,omitempty"`
	Map        string     `json:"map"`
	PlayedAt   *time.Time `json:"played_at,omitempty"`
	Teams      []string   `json:"teams"`
	FileName   string     `json:"file_name,omitempty"`
}

// ExportLibraryPlaylists handles GET /api/library/playlists/export: the playlists
// named by repeated ?id, or else the ones the list filter selects.
func (h *Handlers) ExportLibraryPlaylists(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	q := r.URL.Query()
	var playlists []library.Playlist
	if raw := q["id"]; len(raw) > 0 {
		if len(raw) > library.MaxPlaylistList {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("export at most %d playlists", library.MaxPlaylistList))
			return
		}
		for _, s := range raw {
			id, err := uuid.Parse(s)
			if err != nil {
				writeError(w, http.StatusBadRequest, "id must be a valid UUID")
				return
			}
			p, err := h.libraryRepo.GetPlaylist(r.Context(), id)
			if err != nil {
				writeLibraryError(w, "export playlists", err)
				return
			}
			playlists = append(playlists, p)
		}
	} else {
		filter, err := playlistFilterQuery(q)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		filter.Limit, filter.Offset = library.MaxPlaylistList, 0
		if playlists, err = h.libraryRepo.ListPlaylists(r.Context(), filter); err != nil {
			internalError(w, "export playlists", err)
			return
		}
	}
	matches, err := h.playlistFileMatches(r.Context(), playlists)
	if err != nil {
		internalError(w, "export playlists", err)
		return
	}
	writeJSON(w, http.StatusOK, playlistFile{Format: playlistFormat, Version: playlistVersion,
		ExportedAt: time.Now().UTC().Truncate(time.Millisecond), Playlists: playlists, Matches: matches})
}

// playlistFileMatches describes each match the playlists' rounds find in this library.
func (h *Handlers) playlistFileMatches(ctx context.Context, playlists []library.Playlist) ([]playlistFileMatch, error) {
	out := []playlistFileMatch{}
	seen := map[uuid.UUID]bool{}
	for _, p := range playlists {
		for _, round := range p.Rounds {
			if round.Missing || seen[round.MatchID] {
				continue
			}
			seen[round.MatchID] = true
			m, err := h.libraryRepo.GetMatch(ctx, round.MatchID)
			if errors.Is(err, library.ErrNotFound) {
				continue
			} else if err != nil {
				return nil, err
			}
			fm := playlistFileMatch{MatchID: m.ID, Map: m.Map, PlayedAt: &m.PlayedAt, Teams: []string{}}
			for _, t := range m.Teams {
				fm.Teams = append(fm.Teams, matchTeamName(t))
			}
			if demo, err := h.libraryRepo.GetDemo(ctx, m.DemoID); err == nil {
				fm.DemoSHA256, fm.FileName = demo.SHA256, demo.FileName
			} else if !errors.Is(err, library.ErrNotFound) {
				return nil, err
			}
			out = append(out, fm)
		}
	}
	return out, nil
}

func matchTeamName(t library.MatchTeam) string {
	if t.TeamName != "" {
		return t.TeamName
	}
	return t.ClanName
}

// playlistRejected is a playlist of an imported file that was not stored.
type playlistRejected struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
	Error string `json:"error"`
}

type playlistImportResponse struct {
	Imported   int                `json:"imported"`
	Duplicates int                `json:"duplicates"`
	Rejected   []playlistRejected `json:"rejected"`
	Playlists  []library.Playlist `json:"playlists"`
	// MissingMatches are the matches of the imported rounds this library does not have.
	MissingMatches []playlistFileMatch `json:"missing_matches"`
}

// ImportLibraryPlaylists handles POST /api/library/playlists/import with a playlist
// file. Each playlist is stored on its own: an id already stored is a duplicate, a
// bad one is rejected. Rounds whose match is not here are kept and read as missing.
func (h *Handlers) ImportLibraryPlaylists(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	var file playlistFile
	if err := decodeJSONBody(w, r, &file, false, maxPlaylistFileBytes); err != nil {
		writeCodedError(w, http.StatusBadRequest, codePlaylistFormat, "the file is not a ClipHub playlist file")
		return
	}
	if file.Format != playlistFormat || file.Version != playlistVersion {
		writeCodedError(w, http.StatusBadRequest, codePlaylistFormat, fmt.Sprintf("expected format %q version %d", playlistFormat, playlistVersion))
		return
	}
	if len(file.Playlists) > library.MaxPlaylistList {
		writeCodedError(w, http.StatusBadRequest, codePlaylistFormat, fmt.Sprintf("a file holds at most %d playlists", library.MaxPlaylistList))
		return
	}
	resp := playlistImportResponse{Rejected: []playlistRejected{}, Playlists: []library.Playlist{}, MissingMatches: []playlistFileMatch{}}
	for i, in := range file.Playlists {
		p, err := library.CleanPlaylist(library.Playlist{ID: in.ID, Name: in.Name, Folder: in.Folder, Favorite: in.Favorite, Author: in.Author, Rounds: in.Rounds})
		if err != nil {
			resp.Rejected = append(resp.Rejected, playlistRejected{Index: i, Name: in.Name, Error: err.Error()})
			continue
		}
		switch err := h.libraryRepo.CreatePlaylist(r.Context(), &p); {
		case errors.Is(err, library.ErrConflict):
			resp.Duplicates++
		case err != nil:
			internalError(w, "import playlists", err)
			return
		default:
			resp.Imported++
			resp.Playlists = append(resp.Playlists, p)
		}
	}
	resp.MissingMatches = missingPlaylistMatches(resp.Playlists, file.Matches)
	writeJSON(w, http.StatusOK, resp)
}

// missingPlaylistMatches lists each missing round's match once, as the file named it.
func missingPlaylistMatches(playlists []library.Playlist, named []playlistFileMatch) []playlistFileMatch {
	byID := map[uuid.UUID]playlistFileMatch{}
	for _, m := range named {
		byID[m.MatchID] = m
	}
	out := []playlistFileMatch{}
	seen := map[uuid.UUID]bool{}
	for _, p := range playlists {
		for _, round := range p.Rounds {
			if !round.Missing || seen[round.MatchID] {
				continue
			}
			seen[round.MatchID] = true
			m, ok := byID[round.MatchID]
			if !ok {
				m = playlistFileMatch{MatchID: round.MatchID, DemoSHA256: round.DemoSHA256, Map: round.Map, Teams: []string{}}
			}
			out = append(out, m)
		}
	}
	return out
}
