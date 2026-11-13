package workers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// faceitDateTimeout bounds the FACEIT lookup after a scan, which holds a scan slot.
const faceitDateTimeout = 15 * time.Second

// FaceitMatchDates gives when a FACEIT match started, or the zero time when FACEIT does not know.
type FaceitMatchDates interface {
	MatchStartedAt(ctx context.Context, matchID string) (time.Time, error)
}

type matchDateUpdater interface {
	UpdateMatch(ctx context.Context, id uuid.UUID, patch library.MatchPatch) (library.Match, error)
}

// FaceitDateRepository is the slice of the library store ReconcileFaceitMatchDates reads and writes.
type FaceitDateRepository interface {
	ListDemos(ctx context.Context, filter library.DemoFilter) ([]library.Demo, error)
	GetMatch(ctx context.Context, id uuid.UUID) (library.Match, error)
	matchDateUpdater
}

// UseFaceitDates lets a FACEIT match the demo gave no date take the time FACEIT says it started.
func (w *LibraryWorker) UseFaceitDates(dates FaceitMatchDates) {
	w.faceitDates = dates
}

// dateFaceitMatch never fails the scan: the next start tries a match still dated by its import again.
func (w *LibraryWorker) dateFaceitMatch(ctx context.Context, d library.Demo, m library.Match) {
	repo, ok := w.repo.(matchDateUpdater)
	if !ok || w.faceitDates == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, faceitDateTimeout)
	defer cancel()
	if _, err := faceitMatchDate(ctx, repo, w.faceitDates, d, m); err != nil {
		logWorkerError(d.ID, "date FACEIT match", err)
	}
}

// faceitMatchDate gives a FACEIT match still dated by its import the time FACEIT says it
// started, and reports whether it changed the date.
func faceitMatchDate(ctx context.Context, repo matchDateUpdater, dates FaceitMatchDates, d library.Demo, m library.Match) (bool, error) {
	matchID, ok := library.FaceitMatchID(d.FileName)
	if !ok || !library.DatedByImport(m, d) {
		return false, nil
	}
	started, err := dates.MatchStartedAt(ctx, matchID)
	if err != nil {
		return false, err
	}
	if started.IsZero() || started.After(d.ImportedAt) {
		return false, nil
	}
	if _, err := repo.UpdateMatch(ctx, m.ID, library.MatchPatch{PlayedAt: &started}); err != nil {
		return false, fmt.Errorf("date match %s: %w", m.ID, err)
	}
	return true, nil
}

// ReconcileFaceitMatchDates dates every analysed FACEIT match still on its import time by the
// time FACEIT says it started. It stops at the first FACEIT error, so an outage costs one request.
func ReconcileFaceitMatchDates(ctx context.Context, repo FaceitDateRepository, dates FaceitMatchDates) (int, error) {
	fixed := 0
	for offset := 0; ; offset += library.MaxListLimit {
		page, err := repo.ListDemos(ctx, library.DemoFilter{States: []library.ParseState{library.ParseReady},
			Source: library.SourceFaceit, Limit: library.MaxListLimit, Offset: offset})
		if err != nil {
			return fixed, err
		}
		for _, d := range page {
			if d.MatchID == nil || d.PlayedAt != nil {
				continue
			}
			m, err := repo.GetMatch(ctx, *d.MatchID)
			if errors.Is(err, library.ErrNotFound) {
				continue
			}
			if err != nil {
				return fixed, err
			}
			changed, err := faceitMatchDate(ctx, repo, dates, d, m)
			if err != nil {
				return fixed, err
			}
			if changed {
				fixed++
			}
		}
		if len(page) < library.MaxListLimit {
			return fixed, nil
		}
	}
}
