package httpapi

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/rechedev9/cliphub/internal/library"
)

// uploadFaceitDateTimeout bounds the FACEIT lookup an upload waits for.
const uploadFaceitDateTimeout = 10 * time.Second

// uploadPlayedAt reads the "played_at" field (RFC 3339): the date the browser scan read from the
// demo's signed sprays. A missing or unreadable field is no date.
func uploadPlayedAt(raw string) *time.Time {
	at, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil || at.IsZero() {
		return nil
	}
	at = at.UTC()
	return &at
}

// dateUploadedMatch gives a match still dated by its import the date the demo shows, else the
// start of its FACEIT room, the way Studio dates a scan (docs/incidents.md, Match dates).
// It never fails the upload: a match it cannot date keeps its import time.
func (h *Handlers) dateUploadedMatch(ctx context.Context, demo library.Demo, match library.Match, shown *time.Time) library.Match {
	if !library.DatedByImport(match, demo) {
		return match
	}
	at := library.MatchDate(demo, shown)
	if at.Equal(demo.ImportedAt) {
		at = h.faceitRoomStart(ctx, demo)
	}
	if at.IsZero() || at.Equal(match.PlayedAt) {
		return match
	}
	dated, err := h.libraryRepo.UpdateMatch(ctx, match.ID, library.MatchPatch{PlayedAt: &at})
	if err != nil {
		log.Printf("library upload: date match %s: %v", match.ID, err)
		return match
	}
	return dated
}

// faceitRoomStart is when the FACEIT room a demo is named after started, or the zero time.
func (h *Handlers) faceitRoomStart(ctx context.Context, demo library.Demo) time.Time {
	roomID, ok := library.FaceitMatchID(demo.FileName)
	if !ok || h.faceit == nil {
		return time.Time{}
	}
	ctx, cancel := context.WithTimeout(ctx, uploadFaceitDateTimeout)
	defer cancel()
	started, err := h.faceit.MatchStartedAt(ctx, roomID)
	if err != nil {
		log.Printf("library upload: FACEIT start of %s: %v", roomID, err)
		return time.Time{}
	}
	if started.After(demo.ImportedAt) {
		return time.Time{}
	}
	return started
}
