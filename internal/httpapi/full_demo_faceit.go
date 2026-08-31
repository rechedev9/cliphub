package httpapi

import (
	"bytes"
	"encoding/json"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/job"
)

func (h *Handlers) storeFullDemoFaceit(j job.Job) {
	if h == nil || h.faceitFollows == nil || h.storage == nil {
		return
	}
	players, err := h.faceitFollows.List()
	if err != nil || len(players) == 0 {
		return
	}
	enrichment := map[string]demooverlay.Enrichment{}
	for _, player := range players {
		if player.SteamID64 == "" {
			continue
		}
		enrichment[player.SteamID64] = demooverlay.Enrichment{
			Nickname:   player.Nickname,
			Country:    player.Country,
			ELO:        player.ELO,
			SkillLevel: player.SkillLevel,
			AvatarURL:  player.Avatar,
		}
	}
	if len(enrichment) == 0 {
		return
	}
	body, err := json.MarshalIndent(enrichment, "", "  ")
	if err != nil {
		return
	}
	_ = h.storage.Put(artifacts.FullDemoFaceitKey(j.ID), bytes.NewReader(body))
}
