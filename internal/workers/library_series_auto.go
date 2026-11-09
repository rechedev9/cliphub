package workers

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"slices"
	"strconv"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// faceitDemoName matches FACEIT's file name: 1-<match uuid>-<map number>-<part>.dem.
var faceitDemoName = regexp.MustCompile(`(?i)^1-([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})-(\d+)-\d+\.dem$`)

// faceitSeriesScanLimit bounds the demos read, newest first, to find the other maps of a match.
const faceitSeriesScanLimit = 1000

// librarySeriesGrouper is the store capability that lets the worker put the
// maps of one FACEIT match into one series.
type librarySeriesGrouper interface {
	ListDemos(ctx context.Context, filter library.DemoFilter) ([]library.Demo, error)
	GetMatch(ctx context.Context, id uuid.UUID) (library.Match, error)
	CreateSeries(ctx context.Context, s *library.Series) error
	UpdateMatch(ctx context.Context, id uuid.UUID, patch library.MatchPatch) (library.Match, error)
}

// faceitMatchKey is the FACEIT match id and map number a demo's file name carries.
func faceitMatchKey(fileName string) (string, int, bool) {
	parts := faceitDemoName.FindStringSubmatch(fileName)
	if parts == nil {
		return "", 0, false
	}
	number, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", 0, false
	}
	return parts[1], number, true
}

type faceitMap struct {
	number int
	match  library.Match
}

// faceitGrouping is the map a scan just analysed and the store it groups in.
type faceitGrouping struct {
	repo   librarySeriesGrouper
	key    string
	demoID uuid.UUID
}

// groupFaceitSeries puts the analysed maps of one FACEIT match in a series. It
// never fails the scan; a store error is only logged.
func (w *LibraryWorker) groupFaceitSeries(ctx context.Context, d library.Demo) {
	key, _, ok := faceitMatchKey(d.FileName)
	if !ok {
		return
	}
	grouper, ok := w.repo.(librarySeriesGrouper)
	if !ok {
		return
	}
	// Maps of one match finish their scans together; one grouping at a time keeps them in one series.
	w.seriesMu.Lock()
	defer w.seriesMu.Unlock()
	if err := (faceitGrouping{repo: grouper, key: key, demoID: d.ID}).run(ctx); err != nil {
		log.Printf("worker demo=%s group faceit series: %v", d.ID, err)
	}
}

// run makes the series once two maps are analysed. After that only the map just
// analysed joins it, so a map the user took out of the series stays out.
func (g faceitGrouping) run(ctx context.Context) error {
	repo, key := g.repo, g.key
	demos, err := readyDemos(ctx, repo)
	if err != nil {
		return err
	}
	maps := []faceitMap{}
	for _, d := range demos {
		other, number, ok := faceitMatchKey(d.FileName)
		if !ok || other != key || d.MatchID == nil {
			continue
		}
		m, err := repo.GetMatch(ctx, *d.MatchID)
		if err != nil {
			return fmt.Errorf("load match %s: %w", *d.MatchID, err)
		}
		maps = append(maps, faceitMap{number: number, match: m})
	}
	if len(maps) < 2 {
		return nil
	}
	slices.SortFunc(maps, func(a, b faceitMap) int { return a.number - b.number })
	seriesID := existingSeries(maps)
	if seriesID == nil {
		created := library.Series{Format: seriesFormat(maps), TeamIDs: seriesTeams(maps[0].match)}
		if err := repo.CreateSeries(ctx, &created); err != nil {
			return fmt.Errorf("create series: %w", err)
		}
		seriesID = &created.ID
	} else {
		maps = slices.DeleteFunc(maps, func(entry faceitMap) bool { return entry.match.DemoID != g.demoID })
	}
	for _, entry := range maps {
		if entry.match.SeriesID != nil {
			continue
		}
		if _, err := repo.UpdateMatch(ctx, entry.match.ID, library.MatchPatch{SeriesID: seriesID}); err != nil {
			return fmt.Errorf("attach match %s: %w", entry.match.ID, err)
		}
	}
	return nil
}

func readyDemos(ctx context.Context, repo librarySeriesGrouper) ([]library.Demo, error) {
	out := []library.Demo{}
	for len(out) < faceitSeriesScanLimit {
		page, err := repo.ListDemos(ctx, library.DemoFilter{States: []library.ParseState{library.ParseReady}, Limit: library.MaxListLimit, Offset: len(out)})
		if err != nil {
			return nil, fmt.Errorf("list demos: %w", err)
		}
		out = append(out, page...)
		if len(page) < library.MaxListLimit {
			break
		}
	}
	return out, nil
}

// existingSeries is the series the lowest map already sits in, so a series the user built is kept.
func existingSeries(maps []faceitMap) *uuid.UUID {
	for _, entry := range maps {
		if entry.match.SeriesID != nil {
			return entry.match.SeriesID
		}
	}
	return nil
}

// seriesFormat reads the format off the highest map number seen: map 4 or 5 means a BO5.
func seriesFormat(maps []faceitMap) string {
	if maps[len(maps)-1].number > 3 {
		return "bo5"
	}
	return "bo3"
}

// seriesTeams is both resolved teams of the match, or none when either side has no team yet.
func seriesTeams(m library.Match) []uuid.UUID {
	ids := []uuid.UUID{}
	for _, team := range m.Teams {
		if team.TeamID == nil || slices.Contains(ids, *team.TeamID) {
			return []uuid.UUID{}
		}
		ids = append(ids, *team.TeamID)
	}
	if len(ids) != 2 {
		return []uuid.UUID{}
	}
	return ids
}
