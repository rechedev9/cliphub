package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/library"
)

const proTeamImportBatch = 4

type proTeamImportResponse struct {
	TeamID    *uuid.UUID `json:"team_id,omitempty"`
	Name      string     `json:"name"`
	Days      int        `json:"days"`
	Total     int        `json:"total"`
	Held      int        `json:"held"`
	Imported  int        `json:"imported"`
	Remaining int        `json:"remaining"`
}

func (h *Handlers) ImportLibraryProTeam(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	teamID, err := strconv.Atoi(chi.URLParam(r, "team"))
	if err != nil || teamID <= 0 {
		writeError(w, http.StatusBadRequest, "team must be an HLTV team id")
		return
	}
	if h.references.community.Client == nil {
		writeCodedError(w, http.StatusServiceUnavailable, codeCommunityOff, "the community server is disabled")
		return
	}
	resp, err := h.importProTeam(r.Context(), teamID)
	switch {
	case errors.Is(err, community.ErrNoProCatalog):
		writeCodedError(w, http.StatusNotFound, codeNoProMap, "the community server has no pro catalog yet")
	case errors.Is(err, errProDownload):
		writeCodedError(w, http.StatusBadGateway, codeCommunityDown, "the community server did not answer")
	case err != nil:
		internalError(w, "import pro team", err)
	default:
		writeJSON(w, http.StatusOK, resp)
	}
}

func (h *Handlers) importProTeam(ctx context.Context, teamID int) (proTeamImportResponse, error) {
	refs, name, err := h.proTeamMaps(ctx, teamID)
	if err != nil {
		return proTeamImportResponse{}, err
	}
	resp := proTeamImportResponse{Name: name, Days: community.MaxProMatchDays, Total: len(refs)}
	missing := []proMapRef{}
	for _, ref := range refs {
		wanted, err := h.proMapWanted(ctx, ref)
		if err != nil {
			return resp, err
		}
		if wanted {
			missing = append(missing, ref)
		}
	}
	batch := missing[:min(len(missing), proTeamImportBatch)]
	for _, ref := range batch {
		if _, err := h.syncProMap(ctx, ref); err != nil {
			return resp, err
		}
		if held, err := h.proMapHeld(ctx, ref); err != nil {
			return resp, err
		} else if held {
			resp.Imported++
		}
	}
	resp.Remaining = len(missing) - len(batch)
	for _, ref := range refs {
		held, err := h.proMapHeld(ctx, ref)
		if err != nil {
			return resp, err
		}
		if held {
			resp.Held++
		}
	}
	if name == "" {
		return resp, nil
	}
	team, found, err := h.findTeamByName(ctx, name)
	if err != nil {
		return resp, err
	}
	if found {
		resp.TeamID = &team.ID
	}
	return resp, nil
}

func (h *Handlers) proTeamMaps(ctx context.Context, teamID int) ([]proMapRef, string, error) {
	q := community.ProMatchQuery{Days: community.MaxProMatchDays, Team: strconv.Itoa(teamID), Page: 1}
	refs, name := []proMapRef{}, ""
	for {
		page, err := h.references.community.Client.ProMatches(ctx, q)
		if errors.Is(err, community.ErrNoProCatalog) {
			return nil, "", err
		}
		if err != nil {
			return nil, "", fmt.Errorf("%w: %w", errProDownload, err)
		}
		for _, m := range page.Matches {
			for _, t := range m.Teams {
				if t.ID == teamID && name == "" {
					name = cleanLibraryText(t.Name)
				}
			}
			for _, f := range m.Maps {
				refs = append(refs, proMapRef{matchID: m.ID, file: f})
			}
		}
		if page.Page >= page.Pages || len(page.Matches) == 0 {
			return refs, name, nil
		}
		q.Page++
	}
}

func (h *Handlers) proMapWanted(ctx context.Context, ref proMapRef) (bool, error) {
	rec, err := h.libraryRepo.GetProMap(ctx, ref.matchID, ref.file.Entry)
	switch {
	case errors.Is(err, library.ErrNotFound):
		return !h.proMapBroken(ref.file.SHA256), nil
	case err != nil:
		return false, err
	case rec.DemoID == nil || !proMapNewer(rec, ref.file) || h.proMapBroken(ref.file.SHA256):
		return false, nil
	}
	d, err := h.libraryRepo.GetDemo(ctx, *rec.DemoID)
	if errors.Is(err, library.ErrNotFound) {
		return true, nil
	}
	return err == nil && d.Source == library.SourcePro, err
}

func (h *Handlers) proMapHeld(ctx context.Context, ref proMapRef) (bool, error) {
	rec, err := h.libraryRepo.GetProMap(ctx, ref.matchID, ref.file.Entry)
	if errors.Is(err, library.ErrNotFound) {
		return false, nil
	}
	return err == nil && rec.DemoID != nil, err
}

func (h *Handlers) proMapBroken(sha string) bool {
	h.references.pro.importMu.Lock()
	defer h.references.pro.importMu.Unlock()
	return h.references.pro.broken[sha]
}
