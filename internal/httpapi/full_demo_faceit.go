package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/faceit"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/tasks"
)

const faceitRosterIncomplete = "faceit_roster_incomplete"

type steamAvatarResolver interface {
	ResolveSteamAvatars(context.Context, []string) map[string]faceit.SteamAvatar
}

func (h *Handlers) persistFullDemoSource(id uuid.UUID, source string) {
	if h == nil || h.storage == nil {
		return
	}
	source = demooverlay.NormalizeSource(source)
	body, err := json.MarshalIndent(map[string]string{"source": source}, "", "  ")
	if err != nil {
		return
	}
	_ = h.storage.Put(artifacts.FullDemoSourceKey(id), bytes.NewReader(body))
}

// storeFullDemoOverlaySnapshot persists the roster decoration selected by an
// approved Full Demo document. FACEIT remains an admission requirement, while
// a local Steam profile image is optional: parsed demo facts are sufficient to
// render the job when Steam is unavailable.
func (h *Handlers) storeFullDemoOverlaySnapshot(ctx context.Context, j job.Job, source string) error {
	if demooverlay.UsesFACEITEnrichment(source) {
		return h.storeFullDemoFaceit(ctx, j)
	}
	if err := h.storeFullDemoSteamAvatars(ctx, j); err != nil {
		log.Printf("full demo Steam avatar snapshot %s: %v", j.ID, err)
	}
	return nil
}

// storeFullDemoFaceit resolves the entire parsed roster before HLAE is queued.
// The persisted snapshot makes the later render deterministic and prevents a
// transient API response from silently changing an already accepted job.
func (h *Handlers) storeFullDemoFaceit(ctx context.Context, j job.Job) error {
	if h == nil || h.storage == nil {
		return fmt.Errorf("FACEIT overlay storage is unavailable")
	}
	rc, err := h.storage.Open(artifacts.RosterKey(j.ID))
	if err != nil {
		if storage.IsNotExist(err) {
			return fmt.Errorf("FACEIT overlay requires a parsed roster: %w", err)
		}
		return fmt.Errorf("open roster for FACEIT overlay: %w", err)
	}
	defer rc.Close()
	var parsed parser.RosterResult
	if err := json.NewDecoder(rc).Decode(&parsed); err != nil {
		return fmt.Errorf("decode roster for FACEIT overlay: %w", err)
	}

	target := j.TargetSteamID
	if j.KillPlan != nil {
		target = j.KillPlan.Target.SteamID64
	}
	roster := demooverlay.FromRosterScan(parsed, target)
	if stored, found, err := h.readStoredFullDemoFaceit(j.ID); err != nil {
		return err
	} else if found {
		if err := demooverlay.ValidateFACEITEnrichment(roster, stored); err == nil {
			return nil
		}
	}

	ids := make([]string, 0, len(parsed.Players))
	for _, player := range parsed.Players {
		ids = append(ids, player.SteamID64)
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	players, err := h.faceit.OverlayPlayers(lookupCtx, ids)
	if err != nil {
		return err
	}
	enrichment := make(map[string]demooverlay.Enrichment, len(players))
	for steamID, player := range players {
		enrichment[steamID] = fullDemoEnrichment(player)
	}
	if err := demooverlay.ValidateFACEITEnrichment(roster, enrichment); err != nil {
		return err
	}
	body, err := json.MarshalIndent(enrichment, "", "  ")
	if err != nil {
		return fmt.Errorf("encode FACEIT overlay snapshot: %w", err)
	}
	if err := h.storage.Put(artifacts.FullDemoFaceitKey(j.ID), bytes.NewReader(body)); err != nil {
		return fmt.Errorf("store FACEIT overlay snapshot: %w", err)
	}
	return nil
}

// storeFullDemoSteamAvatars snapshots only public Steam avatar metadata for a
// local roster. It is intentionally best-effort at its caller: no profile,
// network outage, or private account blocks a demo whose stats are already
// present in the parsed roster.
func (h *Handlers) storeFullDemoSteamAvatars(ctx context.Context, j job.Job) error {
	if h == nil || h.storage == nil {
		return fmt.Errorf("Steam avatar storage is unavailable")
	}
	if rc, err := h.storage.Open(artifacts.FullDemoSteamAvatarsKey(j.ID)); err == nil {
		_ = rc.Close()
		return nil
	} else if !storage.IsNotExist(err) {
		return fmt.Errorf("open stored Steam avatar snapshot: %w", err)
	}
	rc, err := h.storage.Open(artifacts.RosterKey(j.ID))
	if err != nil {
		if storage.IsNotExist(err) {
			return fmt.Errorf("Steam avatars require a parsed roster: %w", err)
		}
		return fmt.Errorf("open roster for Steam avatars: %w", err)
	}
	defer rc.Close()
	var parsed parser.RosterResult
	if err := json.NewDecoder(rc).Decode(&parsed); err != nil {
		return fmt.Errorf("decode roster for Steam avatars: %w", err)
	}
	ids := make([]string, 0, len(parsed.Players))
	for _, player := range parsed.Players {
		ids = append(ids, player.SteamID64)
	}
	resolver := h.steamAvatarResolver
	if resolver == nil {
		resolver = faceit.SteamAvatarService{}
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	resolved := resolver.ResolveSteamAvatars(lookupCtx, ids)
	// The resolver normally already removes a private profile's URL. Retain the
	// privacy state but enforce that boundary again before its response becomes
	// durable input to the renderer.
	avatars := make(map[string]faceit.SteamAvatar, len(resolved))
	for steamID, avatar := range resolved {
		if avatar.Private {
			avatars[steamID] = faceit.SteamAvatar{Private: true}
			continue
		}
		avatars[steamID] = avatar
	}
	body, err := json.MarshalIndent(avatars, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Steam avatar snapshot: %w", err)
	}
	if err := h.storage.Put(artifacts.FullDemoSteamAvatarsKey(j.ID), bytes.NewReader(body)); err != nil {
		return fmt.Errorf("store Steam avatar snapshot: %w", err)
	}
	return nil
}

func (h *Handlers) readStoredFullDemoFaceit(id uuid.UUID) (map[string]demooverlay.Enrichment, bool, error) {
	rc, err := h.storage.Open(artifacts.FullDemoFaceitKey(id))
	if err != nil {
		if storage.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("open stored FACEIT overlay snapshot: %w", err)
	}
	defer rc.Close()
	var enrichment map[string]demooverlay.Enrichment
	if err := json.NewDecoder(rc).Decode(&enrichment); err != nil {
		return nil, false, fmt.Errorf("decode stored FACEIT overlay snapshot: %w", err)
	}
	return enrichment, true, nil
}

// fullDemoDefaultOptions seeds a job's first plan with the overlay origin the
// job already proved: the source persisted by an earlier Full Demo capture,
// or FACEIT when its roster snapshot is stored. A stored origin is evidence,
// so a fresh plan keeps the FACEIT layout instead of silently downgrading to
// demo-facts-only overlays.
func (h *Handlers) fullDemoDefaultOptions(id uuid.UUID) recapplan.Options {
	options := recapplan.DefaultOptions()
	if h == nil || h.storage == nil {
		return options
	}
	if rc, err := h.storage.Open(artifacts.FullDemoSourceKey(id)); err == nil {
		var doc struct {
			Source string `json:"source"`
		}
		decodeErr := json.NewDecoder(rc).Decode(&doc)
		_ = rc.Close()
		if source := demooverlay.NormalizeSource(doc.Source); decodeErr == nil && source != "" {
			options.SourceKind = source
			return options
		}
	}
	if _, found, err := h.readStoredFullDemoFaceit(id); err == nil && found {
		options.SourceKind = demooverlay.SourceFACEIT
	}
	return options
}

func fullDemoEnrichment(player faceit.OverlayPlayer) demooverlay.Enrichment {
	return demooverlay.Enrichment{
		Nickname:        player.Nickname,
		Country:         player.Country,
		ELO:             player.ELO,
		SkillLevel:      player.SkillLevel,
		Ranking:         player.Ranking,
		AvatarURL:       player.Avatar,
		Last20:          fullDemoLast20(player.Recent),
		LifetimeMatches: player.LifetimeMatches,
		Verified:        player.Verified,
		Premium:         player.Premium,
	}
}

func fullDemoLast20(src faceit.Last20) *demooverlay.Last20 {
	out := demooverlay.Last20{
		Matches: src.Matches,
		WinPct:  src.WinPct,
		Kills:   src.Kills,
		Deaths:  src.Deaths,
		Assists: src.Assists,
		KD:      src.KD,
		KR:      src.KR,
		ADR:     src.ADR,
		HSPct:   src.HSPct,
	}
	if out.Matches == nil && out.WinPct == nil && out.Kills == nil && out.Deaths == nil &&
		out.Assists == nil && out.KD == nil && out.KR == nil && out.ADR == nil && out.HSPct == nil {
		return nil
	}
	return &out
}

func (h *Handlers) rejectFullDemoFaceit(w http.ResponseWriter, j job.Job, err error) {
	if recorder := obs.Default(); recorder != nil {
		_ = recorder.RecordError(obs.Event{
			JobID:   j.ID.String(),
			Stage:   obs.StageRecord,
			Task:    tasks.TypeRecordDemo,
			Class:   faceitRosterIncomplete,
			Message: err.Error(),
			Target:  strings.TrimSpace(j.TargetSteamID),
		})
	}
	switch {
	case errors.Is(err, faceit.ErrNotConfigured),
		errors.Is(err, faceit.ErrUnauthorized),
		errors.Is(err, faceit.ErrRateLimited),
		errors.Is(err, faceit.ErrUnavailable),
		errors.Is(err, faceit.ErrInvalidResponse):
		writeFaceitError(w, err)
	default:
		writeCodedError(w, http.StatusUnprocessableEntity, faceitRosterIncomplete,
			fmt.Sprintf("Full Demo requires FACEIT data for every roster player: %v", err))
	}
}
