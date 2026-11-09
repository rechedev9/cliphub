package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// libraryPlaylistRepository is the saved playlists part of LibraryRepository.
type libraryPlaylistRepository interface {
	CreatePlaylist(ctx context.Context, p *library.Playlist) error
	GetPlaylist(ctx context.Context, id uuid.UUID) (library.Playlist, error)
	ListPlaylists(ctx context.Context, filter library.PlaylistFilter) ([]library.Playlist, error)
	UpdatePlaylist(ctx context.Context, id uuid.UUID, patch library.PlaylistPatch) (library.Playlist, error)
	DeletePlaylist(ctx context.Context, id uuid.UUID) error
}

// badPlaylistInput is a client mistake in a playlist request, answered with 400.
type badPlaylistInput struct{ code, msg string }

func (e badPlaylistInput) Error() string { return e.msg }

func writePlaylistError(w http.ResponseWriter, op string, err error) {
	var bad badPlaylistInput
	if errors.As(err, &bad) {
		writeCodedError(w, http.StatusBadRequest, bad.code, bad.msg)
		return
	}
	writeLibraryError(w, op, err)
}

// ListLibraryPlaylists handles GET /api/library/playlists.
func (h *Handlers) ListLibraryPlaylists(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	filter, err := playlistFilterQuery(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	playlists, err := h.libraryRepo.ListPlaylists(r.Context(), filter)
	if err != nil {
		internalError(w, "list playlists", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"playlists": playlists})
}

func playlistFilterQuery(q url.Values) (library.PlaylistFilter, error) {
	f := library.PlaylistFilter{Query: library.CleanName(q.Get("q")), Map: strings.ToLower(strings.TrimSpace(q.Get("map"))), Folder: library.CleanName(q.Get("folder"))}
	switch {
	case len([]rune(f.Query)) > library.MaxPlaylistNameRunes:
		return f, fmt.Errorf("q takes at most %d characters", library.MaxPlaylistNameRunes)
	case len([]rune(f.Folder)) > library.MaxPlaylistFolderRunes:
		return f, fmt.Errorf("folder takes at most %d characters", library.MaxPlaylistFolderRunes)
	case f.Map != "" && !mapNamePattern.MatchString(f.Map):
		return f, errors.New("map must be a map name such as de_mirage")
	}
	for _, b := range []struct {
		name string
		dst  *bool
	}{{"no_folder", &f.NoFolder}, {"favorite", &f.Favorite}} {
		switch q.Get(b.name) {
		case "", "false":
		case "true":
			*b.dst = true
		default:
			return f, fmt.Errorf("%s must be true or false", b.name)
		}
	}
	for _, p := range []struct {
		name string
		dst  *int
		min  int
	}{{"limit", &f.Limit, 1}, {"offset", &f.Offset, 0}} {
		if raw := q.Get(p.name); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < p.min || (p.name == "limit" && n > library.MaxPlaylistList) {
				return f, fmt.Errorf("%s is out of range", p.name)
			}
			*p.dst = n
		}
	}
	return f, nil
}

// playlistRoundRequest is a round as a client names it: the server fills its map and demo.
type playlistRoundRequest struct {
	MatchID uuid.UUID `json:"match_id"`
	Round   int       `json:"round"`
	Comment string    `json:"comment"`
}

// playlistRequest is a playlist as a client sends it; nil fields are not set.
type playlistRequest struct {
	Name     *string                 `json:"name"`
	Folder   *string                 `json:"folder"`
	Author   *string                 `json:"author"`
	Favorite *bool                   `json:"favorite"`
	Rounds   *[]playlistRoundRequest `json:"rounds"`
}

// patch turns the request into a playlist patch over current, resolving new rounds.
func (h *Handlers) playlistPatch(ctx context.Context, current []library.PlaylistRound, req playlistRequest) (library.PlaylistPatch, error) {
	patch := library.PlaylistPatch{Name: req.Name, Folder: req.Folder, Author: req.Author, Favorite: req.Favorite}
	if req.Rounds != nil {
		rounds, err := h.playlistRounds(ctx, current, *req.Rounds)
		if err != nil {
			return patch, err
		}
		patch.Rounds = &rounds
	}
	return patch, nil
}

// playlistRounds keeps what the playlist knows of a round it already holds, so
// a missing round can still move, and reads every new round's map and demo from its match.
func (h *Handlers) playlistRounds(ctx context.Context, current []library.PlaylistRound, in []playlistRoundRequest) ([]library.PlaylistRound, error) {
	if len(in) > library.MaxPlaylistRounds {
		return nil, badPlaylistInput{"invalid_request", fmt.Sprintf("a playlist holds at most %d rounds", library.MaxPlaylistRounds)}
	}
	matches := map[uuid.UUID]library.Match{}
	shas := map[uuid.UUID]string{}
	out := make([]library.PlaylistRound, 0, len(in))
	for _, req := range in {
		want := library.PlaylistRound{MatchID: req.MatchID, Round: req.Round}
		if i := slices.IndexFunc(current, func(r library.PlaylistRound) bool { return library.SamePlaylistRound(r, want) }); i >= 0 {
			kept := current[i]
			kept.Comment = req.Comment
			out = append(out, kept)
			continue
		}
		m, ok := matches[req.MatchID]
		if !ok {
			var err error
			if m, err = h.libraryRepo.GetMatch(ctx, req.MatchID); errors.Is(err, library.ErrNotFound) {
				return nil, badPlaylistInput{codeUnknownMatch, fmt.Sprintf("match %s is not in the library", req.MatchID)}
			} else if err != nil {
				return nil, err
			}
			demo, err := h.libraryRepo.GetDemo(ctx, m.DemoID)
			if err != nil && !errors.Is(err, library.ErrNotFound) {
				return nil, err
			}
			matches[req.MatchID], shas[req.MatchID] = m, demo.SHA256
		}
		if req.Round < 1 || req.Round > m.RoundCount {
			return nil, badPlaylistInput{"invalid_request", fmt.Sprintf("match %s has no round %d", req.MatchID, req.Round)}
		}
		out = append(out, library.PlaylistRound{MatchID: m.ID, Round: req.Round, Map: m.Map, DemoSHA256: shas[req.MatchID], Comment: req.Comment})
	}
	return out, nil
}

// CreateLibraryPlaylist handles POST /api/library/playlists.
func (h *Handlers) CreateLibraryPlaylist(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	var req playlistRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid playlist JSON")
		return
	}
	patch, err := h.playlistPatch(r.Context(), nil, req)
	if err != nil {
		writePlaylistError(w, "create playlist", err)
		return
	}
	p, err := library.CleanPlaylist(library.ApplyPlaylistPatch(library.Playlist{}, patch))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.libraryRepo.CreatePlaylist(r.Context(), &p); err != nil {
		writeLibraryError(w, "create playlist", err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *Handlers) loadLibraryPlaylist(w http.ResponseWriter, r *http.Request) (library.Playlist, bool) {
	if !h.libraryReady(w) {
		return library.Playlist{}, false
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return library.Playlist{}, false
	}
	p, err := h.libraryRepo.GetPlaylist(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get playlist", err)
		return library.Playlist{}, false
	}
	return p, true
}

// GetLibraryPlaylist handles GET /api/library/playlists/{id}.
func (h *Handlers) GetLibraryPlaylist(w http.ResponseWriter, r *http.Request) {
	if p, ok := h.loadLibraryPlaylist(w, r); ok {
		writeJSON(w, http.StatusOK, p)
	}
}

// editPlaylist loads the playlist the path names, cleans the edit over it and
// stores it, answering with the playlist as saved.
func (h *Handlers) editPlaylist(w http.ResponseWriter, r *http.Request, edit func(current library.Playlist) (library.PlaylistPatch, error)) {
	current, ok := h.loadLibraryPlaylist(w, r)
	if !ok {
		return
	}
	patch, err := edit(current)
	if err != nil {
		writePlaylistError(w, "update playlist", err)
		return
	}
	next, err := library.CleanPlaylist(library.ApplyPlaylistPatch(current, patch))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	clean := library.PlaylistPatch{Favorite: patch.Favorite}
	if patch.Name != nil {
		clean.Name = &next.Name
	}
	if patch.Folder != nil {
		clean.Folder = &next.Folder
	}
	if patch.Author != nil {
		clean.Author = &next.Author
	}
	if patch.Rounds != nil {
		clean.Rounds = &next.Rounds
	}
	updated, err := h.libraryRepo.UpdatePlaylist(r.Context(), current.ID, clean)
	if err != nil {
		writeLibraryError(w, "update playlist", err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// PatchLibraryPlaylist handles PATCH /api/library/playlists/{id}; rounds replaces the list, in order.
func (h *Handlers) PatchLibraryPlaylist(w http.ResponseWriter, r *http.Request) {
	h.editPlaylist(w, r, func(current library.Playlist) (library.PlaylistPatch, error) {
		var req playlistRequest
		if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
			return library.PlaylistPatch{}, badPlaylistInput{"invalid_request", "invalid playlist JSON"}
		}
		return h.playlistPatch(r.Context(), current.Rounds, req)
	})
}

// DeleteLibraryPlaylist handles DELETE /api/library/playlists/{id}.
func (h *Handlers) DeleteLibraryPlaylist(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.libraryRepo.DeletePlaylist(r.Context(), id); err != nil {
		writeLibraryError(w, "delete playlist", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// addPlaylistRoundsRequest adds rounds at position (0 is the start), or at the end without one.
type addPlaylistRoundsRequest struct {
	Rounds   []playlistRoundRequest `json:"rounds"`
	Position *int                   `json:"position"`
}

// AddLibraryPlaylistRounds handles POST /api/library/playlists/{id}/rounds. A
// round the playlist already holds stays where it is, with its comment.
func (h *Handlers) AddLibraryPlaylistRounds(w http.ResponseWriter, r *http.Request) {
	h.editPlaylist(w, r, func(current library.Playlist) (library.PlaylistPatch, error) {
		var req addPlaylistRoundsRequest
		if err := decodeSingleJSONBody(w, r, &req, true); err != nil || len(req.Rounds) == 0 {
			return library.PlaylistPatch{}, badPlaylistInput{"invalid_request", "send the rounds to add"}
		}
		fresh := []playlistRoundRequest{}
		for _, in := range req.Rounds {
			want := library.PlaylistRound{MatchID: in.MatchID, Round: in.Round}
			held := slices.ContainsFunc(current.Rounds, func(r library.PlaylistRound) bool { return library.SamePlaylistRound(r, want) })
			if !held && !slices.ContainsFunc(fresh, func(f playlistRoundRequest) bool { return f.MatchID == in.MatchID && f.Round == in.Round }) {
				fresh = append(fresh, in)
			}
		}
		added, err := h.playlistRounds(r.Context(), nil, fresh)
		if err != nil {
			return library.PlaylistPatch{}, err
		}
		at := len(current.Rounds)
		if req.Position != nil {
			at = min(max(*req.Position, 0), at)
		}
		rounds := slices.Concat(current.Rounds[:at], added, current.Rounds[at:])
		return library.PlaylistPatch{Rounds: &rounds}, nil
	})
}

// playlistRoundIndex finds the round the path names in the playlist.
func playlistRoundIndex(r *http.Request, p library.Playlist) (int, error) {
	matchID, err := uuid.Parse(chi.URLParam(r, "match"))
	if err != nil {
		return 0, badPlaylistInput{"invalid_request", "match must be a valid UUID"}
	}
	round, err := strconv.Atoi(chi.URLParam(r, "round"))
	if err != nil {
		return 0, badPlaylistInput{"invalid_request", "round must be a number"}
	}
	want := library.PlaylistRound{MatchID: matchID, Round: round}
	i := slices.IndexFunc(p.Rounds, func(r library.PlaylistRound) bool { return library.SamePlaylistRound(r, want) })
	if i < 0 {
		return 0, fmt.Errorf("playlist round %s/%d: %w", matchID, round, library.ErrNotFound)
	}
	return i, nil
}

// playlistRoundPatch edits one round: its comment, or where it goes (0 is the start).
type playlistRoundPatch struct {
	Comment  *string `json:"comment"`
	Position *int    `json:"position"`
}

// PatchLibraryPlaylistRound handles PATCH /api/library/playlists/{id}/rounds/{match}/{round}.
func (h *Handlers) PatchLibraryPlaylistRound(w http.ResponseWriter, r *http.Request) {
	h.editPlaylist(w, r, func(current library.Playlist) (library.PlaylistPatch, error) {
		i, err := playlistRoundIndex(r, current)
		if err != nil {
			return library.PlaylistPatch{}, err
		}
		var req playlistRoundPatch
		if err := decodeSingleJSONBody(w, r, &req, true); err != nil || (req.Comment == nil && req.Position == nil) {
			return library.PlaylistPatch{}, badPlaylistInput{"invalid_request", "send a comment or a position"}
		}
		round := current.Rounds[i]
		if req.Comment != nil {
			round.Comment = *req.Comment
		}
		rounds := slices.Delete(slices.Clone(current.Rounds), i, i+1)
		at := i
		if req.Position != nil {
			at = min(max(*req.Position, 0), len(rounds))
		}
		rounds = slices.Insert(rounds, at, round)
		return library.PlaylistPatch{Rounds: &rounds}, nil
	})
}

// DeleteLibraryPlaylistRound handles DELETE /api/library/playlists/{id}/rounds/{match}/{round}
// and answers with the playlist left.
func (h *Handlers) DeleteLibraryPlaylistRound(w http.ResponseWriter, r *http.Request) {
	h.editPlaylist(w, r, func(current library.Playlist) (library.PlaylistPatch, error) {
		i, err := playlistRoundIndex(r, current)
		if err != nil {
			return library.PlaylistPatch{}, err
		}
		rounds := slices.Delete(slices.Clone(current.Rounds), i, i+1)
		return library.PlaylistPatch{Rounds: &rounds}, nil
	})
}
