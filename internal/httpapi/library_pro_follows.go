package httpapi

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/library"
)

const (
	// proFollowFirstDays is how far back following a team imports its matches.
	proFollowFirstDays = 14
	proFollowEvery     = 3 * time.Hour
	proFollowFirstWait = 2 * time.Minute
	maxProFollows      = 50
)

type proFollowRequest struct {
	TeamID int    `json:"team_id"`
	Name   string `json:"name"`
}

// ListLibraryProFollows handles GET /api/library/pro-follows.
func (h *Handlers) ListLibraryProFollows(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	follows, err := h.libraryRepo.ListProFollows(r.Context())
	if err != nil {
		internalError(w, "list pro follows", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"follows": follows})
}

// FollowLibraryProTeam handles POST /api/library/pro-follows: Studio imports the
// team's pro matches of the last two weeks and every new one after that.
func (h *Handlers) FollowLibraryProTeam(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	var req proFollowRequest
	if err := decodeSingleJSONBody(w, r, &req, false); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	name := cleanLibraryText(req.Name)
	if req.TeamID <= 0 || library.NormalizeName(name) == "" || len([]rune(name)) > library.MaxTagRunes*4 {
		writeError(w, http.StatusBadRequest, "team_id must be an HLTV team id and name its name")
		return
	}
	follows, err := h.libraryRepo.ListProFollows(r.Context())
	if err == nil && len(follows) >= maxProFollows {
		writeError(w, http.StatusBadRequest, "Studio follows at most 50 pro teams")
		return
	}
	f := library.ProFollow{TeamID: req.TeamID, Name: name}
	if err == nil {
		err = h.libraryRepo.FollowProTeam(r.Context(), &f)
	}
	if err != nil {
		internalError(w, "follow pro team", err)
		return
	}
	h.wakeProFollowSync()
	writeJSON(w, http.StatusOK, f)
}

// UnfollowLibraryProTeam handles DELETE /api/library/pro-follows/{team}; the maps
// already imported stay in the library.
func (h *Handlers) UnfollowLibraryProTeam(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	teamID, err := strconv.Atoi(chi.URLParam(r, "team"))
	if err != nil || teamID <= 0 {
		writeError(w, http.StatusBadRequest, "team must be an HLTV team id")
		return
	}
	if err := h.libraryRepo.UnfollowProTeam(r.Context(), teamID); err != nil {
		writeLibraryError(w, "unfollow pro team", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) wakeProFollowSync() {
	select {
	case h.references.pro.wake <- struct{}{}:
	default:
	}
}

// runProFollowSync imports the followed teams' new pro matches every few hours,
// and right after a team is followed, until ctx ends.
func (h *Handlers) runProFollowSync(ctx context.Context, wake <-chan struct{}) {
	wait := proFollowFirstWait
	for {
		select {
		case <-ctx.Done():
			return
		case <-wake:
		case <-time.After(wait):
		}
		wait = proFollowEvery
		if n, err := h.syncProFollows(ctx); err != nil && ctx.Err() == nil {
			log.Printf("httpapi: pro follow sync: %v", err)
		} else if n > 0 {
			log.Printf("httpapi: pro follow sync imported %d maps", n)
		}
	}
}

// syncProFollows imports every catalog map of a followed team the library does
// not hold yet. A team is marked synced only when all its maps went through.
func (h *Handlers) syncProFollows(ctx context.Context) (int, error) {
	client := h.references.community.Client
	if client == nil || h.libraryRepo == nil {
		return 0, nil
	}
	follows, err := h.libraryRepo.ListProFollows(ctx)
	if err != nil {
		return 0, err
	}
	imported := 0
	for _, f := range follows {
		started := time.Now().UTC()
		n, err := h.syncProFollow(ctx, client, f)
		imported += n
		if errors.Is(err, community.ErrNoProCatalog) || ctx.Err() != nil {
			return imported, err
		}
		if err != nil {
			log.Printf("httpapi: pro follow %d (%s): %v", f.TeamID, f.Name, err)
			continue
		}
		if err := h.libraryRepo.MarkProFollowSynced(ctx, f.TeamID, started); err != nil && !errors.Is(err, library.ErrNotFound) {
			return imported, err
		}
	}
	return imported, nil
}

func (h *Handlers) syncProFollow(ctx context.Context, client *community.Client, f library.ProFollow) (int, error) {
	q := community.ProMatchQuery{Days: proFollowFirstDays, Team: strconv.Itoa(f.TeamID), Page: 1}
	if f.SyncedAt != nil {
		q.Days = min(max(int(time.Since(*f.SyncedAt).Hours()/24)+2, 1), community.MaxProMatchDays)
	}
	imported := 0
	for {
		page, err := client.ProMatches(ctx, q)
		if err != nil {
			return imported, err
		}
		for _, m := range page.Matches {
			for _, file := range m.Maps {
				if _, err := h.libraryRepo.GetProMap(ctx, m.ID, file.Entry); err == nil {
					continue
				}
				_, err := h.importProMap(ctx, proMapRef{matchID: m.ID, entry: file.Entry})
				switch {
				case err == nil:
					imported++
				case errors.Is(err, errProDownload):
					return imported, err
				case !errors.Is(err, errProMapRemoved):
					log.Printf("httpapi: pro follow %d skips %d/%s: %v", f.TeamID, m.ID, file.Entry, err)
				}
			}
		}
		if page.Page >= page.Pages || len(page.Matches) == 0 {
			return imported, nil
		}
		q.Page++
	}
}
