package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/faceit"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/storage"
)

const overlayFaceitTimeout = 20 * time.Second

func (h *Handlers) storeFullDemoFaceit(j job.Job) {
	if h == nil || h.storage == nil {
		return
	}
	enrichment := map[string]demooverlay.Enrichment{}
	if h.faceit != nil {
		ctx, cancel := context.WithTimeout(context.Background(), overlayFaceitTimeout)
		defer cancel()
		enrichment = h.lookupRosterFaceit(ctx, j.ID)
	}
	if len(enrichment) == 0 && h.faceitFollows != nil {
		players, err := h.faceitFollows.List()
		if err != nil || len(players) == 0 {
			return
		}
		for _, player := range players {
			if player.SteamID64 == "" {
				continue
			}
			enrichment[player.SteamID64] = demooverlay.Enrichment{
				Nickname:   player.Nickname,
				Country:    player.Country,
				ELO:        player.ELO,
				SkillLevel: player.SkillLevel,
				Avatar:     player.Avatar,
			}
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

func (h *Handlers) lookupRosterFaceit(ctx context.Context, jobID uuid.UUID) map[string]demooverlay.Enrichment {
	rc, err := h.storage.Open(artifacts.RosterKey(jobID))
	if err != nil {
		if !storage.IsNotExist(err) {
			return nil
		}
		return nil
	}
	defer rc.Close()
	var roster parser.RosterResult
	if err := json.NewDecoder(rc).Decode(&roster); err != nil {
		return nil
	}
	ids := make([]string, 0, len(roster.Players))
	seen := map[string]bool{}
	for _, p := range roster.Players {
		if p.SteamID64 == "" || seen[p.SteamID64] {
			continue
		}
		seen[p.SteamID64] = true
		ids = append(ids, p.SteamID64)
	}
	out := map[string]demooverlay.Enrichment{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	gate := make(chan struct{}, 4)
	for _, steamID := range ids {
		steamID := steamID
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case gate <- struct{}{}:
				defer func() { <-gate }()
			case <-ctx.Done():
				return
			}
			en, ok := h.overlayEnrichment(ctx, steamID)
			if !ok {
				return
			}
			mu.Lock()
			out[steamID] = en
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}

func (h *Handlers) overlayEnrichment(ctx context.Context, steamID string) (demooverlay.Enrichment, bool) {
	player, err := h.faceit.LookupBySteamID(ctx, steamID)
	if err != nil {
		return demooverlay.Enrichment{}, false
	}
	en := demooverlay.Enrichment{
		Nickname:   player.Nickname,
		Country:    player.Country,
		ELO:        player.ELO,
		SkillLevel: player.SkillLevel,
		Avatar:     player.Avatar,
	}
	if player.ID != "" {
		matches, err := h.faceit.RecentMatches(ctx, player.ID, 30)
		if err == nil {
			last := faceit.AggregateLast20(matches)
			en.Last20 = last20FromFACEIT(last)
		}
		if pos, err := h.faceit.RankingPosition(ctx, player.Region, player.ID); err == nil && pos > 0 {
			en.Ranking = &pos
		}
	}
	return en, true
}

func last20FromFACEIT(src faceit.Last20) *demooverlay.Last20 {
	out := demooverlay.Last20{
		Matches: src.Matches,
		WinPct:  src.WinPct,
		Kills:   src.Kills,
		Deaths:  src.Deaths,
		Assists: src.Assists,
		KD:      src.KD,
		KR:      src.KR,
		ADR:     src.ADR,
	}
	if !last20HasValues(out) {
		return nil
	}
	return &out
}

func last20HasValues(l demooverlay.Last20) bool {
	return l.Matches != nil || l.WinPct != nil || l.Kills != nil || l.KD != nil || l.KR != nil || l.ADR != nil
}
