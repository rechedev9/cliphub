package httpapi

import (
	"context"
	"log"
	"net/http"

	"github.com/rechedev9/cliphub/internal/faceit"
	"github.com/rechedev9/cliphub/internal/htmlstill"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/playercard"
)

const playerCardMissingStats = "Faltan las estadísticas del jugador en esta partida."

type playerCardInfo struct {
	Templates []playercard.Template `json:"templates"`
	Available bool                  `json:"available"`
	Reason    string                `json:"reason,omitempty"`
	Player    *playerCardPlayer     `json:"player,omitempty"`
}

type playerCardPlayer struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name"`
	// Image is where the card's image comes from: "upload", "faceit" or empty for the initials.
	Image string `json:"image"`
}

func (h *Handlers) playerCardSources() playercard.Sources {
	src := playercard.Sources{Files: h.storage, Avatar: func(ctx context.Context, url string) ([]byte, error) {
		return faceit.FetchAvatar(ctx, http.DefaultClient, url)
	}}
	if h.faceit != nil {
		src.FACEIT = h.faceit
	}
	if h.libraryRepo != nil {
		src.Library = h.libraryRepo
	}
	return src
}

// playerCardFacts gathers the job's player once for the card info and the previews of its panel.
func (h *Handlers) playerCardFacts(ctx context.Context, j job.Job) (playercard.Facts, error) {
	key := playerCardKey{job: j.ID, steamID: playercard.Target(j)}
	return h.playerCards.facts(ctx, key, func(ctx context.Context) (playercard.Facts, error) {
		return playercard.Gather(ctx, h.playerCardSources(), key.job, key.steamID)
	})
}

// GetPlayerCard handles GET /api/jobs/{id}/player-card: the card catalog and the player a
// one-player Short of the job shows, or why its Shorts cannot carry a card.
func (h *Handlers) GetPlayerCard(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	info := playerCardInfo{Templates: playercard.Templates, Reason: playercard.Unavailable(j)}
	if info.Reason == "" {
		facts, err := h.playerCardFacts(r.Context(), j)
		if err != nil {
			log.Printf("httpapi: player card of job %s: %v", j.ID, err)
			info.Reason = playerCardMissingStats
		} else {
			info.Available = true
			info.Player = &playerCardPlayer{SteamID64: facts.Card.SteamID64, Name: facts.Card.Name, Image: facts.ImageSource}
		}
	}
	writeJSON(w, http.StatusOK, info)
}

// GetPlayerCardPreview handles GET /api/jobs/{id}/player-card/preview?template=: the job's card in
// that template as an SVG of the renderer's own page.
func (h *Handlers) GetPlayerCardPreview(w http.ResponseWriter, r *http.Request) {
	template := r.URL.Query().Get("template")
	if _, ok := playercard.Lookup(template); !ok {
		writeError(w, http.StatusBadRequest, "template must be a player card id from the catalog")
		return
	}
	j, ok := h.cardJob(w, r)
	if !ok {
		return
	}
	facts, err := h.playerCardFacts(r.Context(), j)
	if err != nil {
		log.Printf("httpapi: player card preview of job %s: %v", j.ID, err)
		writeError(w, http.StatusConflict, playerCardMissingStats)
		return
	}
	page, err := playercard.HTML(template, facts.Card, facts.Image)
	if err != nil {
		internalError(w, "player card preview", err)
		return
	}
	svg, err := htmlstill.PreviewSVG(page, htmlstill.Portrait)
	if err != nil {
		internalError(w, "player card preview", err)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; font-src data:; img-src data:")
	_, _ = w.Write(svg)
}

// SetPlayerCardImage handles POST /api/jobs/{id}/player-card/image: a PNG or JPEG that every card
// of the job's player shows from now on, in this match and the next ones.
func (h *Handlers) SetPlayerCardImage(w http.ResponseWriter, r *http.Request) {
	j, ok := h.cardJob(w, r)
	if !ok {
		return
	}
	asset, ok := h.storeUploadedImage(w, r)
	if !ok {
		return
	}
	if err := playercard.SaveImage(h.storage, playercard.Target(j), playercard.ImageRef{ID: asset.ID.String(), SHA256: asset.SHA256}); err != nil {
		internalError(w, "save player image", err)
		return
	}
	h.playerCards.forget(playercard.Target(j))
	writeJSON(w, http.StatusOK, map[string]string{"image": playercard.ImageUpload})
}

// DeletePlayerCardImage handles DELETE /api/jobs/{id}/player-card/image: the cards go back to the
// FACEIT avatar or the initials.
func (h *Handlers) DeletePlayerCardImage(w http.ResponseWriter, r *http.Request) {
	j, ok := h.cardJob(w, r)
	if !ok {
		return
	}
	if err := playercard.SaveImage(h.storage, playercard.Target(j), playercard.ImageRef{}); err != nil {
		internalError(w, "remove player image", err)
		return
	}
	h.playerCards.forget(playercard.Target(j))
	w.WriteHeader(http.StatusNoContent)
}

// cardJob loads a job whose Shorts can carry a player card; 409 with the reason otherwise.
func (h *Handlers) cardJob(w http.ResponseWriter, r *http.Request) (job.Job, bool) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return job.Job{}, false
	}
	if reason := playercard.Unavailable(j); reason != "" {
		writeError(w, http.StatusConflict, reason)
		return job.Job{}, false
	}
	return j, true
}
