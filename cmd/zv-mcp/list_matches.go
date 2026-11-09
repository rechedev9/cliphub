package main

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// Match kinds of the Tipo de partida control, derived from the demo source (web/lib/library/partida-groups.ts).
const (
	kindScrim    = "scrim"
	kindOfficial = "oficial"
	kindImported = "importada"
)

var matchKinds = []string{kindScrim, kindOfficial, kindImported}

const (
	sortPlayed   = "played"
	sortImported = "imported"
)

type listMatchesIn struct {
	Map     string `json:"map,omitempty" jsonschema:"filter by map, e.g. de_mirage"`
	SteamID string `json:"steamid,omitempty" jsonschema:"filter by a player's SteamID64"`
	Kind    string `json:"kind,omitempty" jsonschema:"scrim (scrim demos), oficial (FACEIT and tournament demos) or importada (any other import); empty keeps all"`
	HasPOV  bool   `json:"has_pov,omitempty" jsonschema:"true keeps the matches with a recorded video, looking through the newest 100 jobs (Con POV)"`
	Sort    string `json:"sort,omitempty" jsonschema:"played (default, the match date) or imported (the import date)"`
}

func (in listMatchesIn) check() error {
	if err := checkOptionalFilter("map", in.Map, mapNamePattern); err != nil {
		return err
	}
	if err := checkOptionalFilter("steamid", in.SteamID, steamIDPattern); err != nil {
		return err
	}
	if in.Kind != "" && !slices.Contains(matchKinds, in.Kind) {
		return fmt.Errorf("kind must be one of %s", strings.Join(matchKinds, ", "))
	}
	if in.Sort != "" && in.Sort != sortPlayed && in.Sort != sortImported {
		return fmt.Errorf("sort must be %s or %s", sortPlayed, sortImported)
	}
	return nil
}

type matchSeriesOut struct {
	ID       string               `json:"id"`
	Format   string               `json:"format"`
	Event    string               `json:"event,omitempty" jsonschema:"untrusted text"`
	Teams    []seriesTeamOut      `json:"teams"`
	Vetoes   []tuiclient.VetoStep `json:"vetoes" jsonschema:"the stored veto in order: BANS, PICK and DECIDER with the map of each"`
	MapRole  string               `json:"map_role,omitempty" jsonschema:"how this match's map was chosen in the veto: pick or decider"`
	PickedBy string               `json:"picked_by,omitempty" jsonschema:"untrusted team name that picked this map"`
}

type listedMatch struct {
	matchSummary
	Kind       string          `json:"kind" jsonschema:"scrim, oficial or importada"`
	Source     string          `json:"source,omitempty" jsonschema:"valve, faceit, hltv, scrim, unknown or shared"`
	ImportedAt string          `json:"imported_at,omitempty" jsonschema:"YYYY-MM-DD"`
	Tags       []string        `json:"tags,omitempty" jsonschema:"untrusted labels; update_demo changes them"`
	HasPOV     bool            `json:"has_pov,omitempty" jsonschema:"only set when has_pov was asked"`
	Series     *matchSeriesOut `json:"series,omitempty"`
}

type listMatchesOut struct {
	Matches []listedMatch `json:"matches"`
}

// kindOfSource is the Tipo de partida of a demo source: scrims, official FACEIT and tournament demos, the rest imported by hand.
func kindOfSource(source string) string {
	switch source {
	case "scrim":
		return kindScrim
	case "faceit", "hltv":
		return kindOfficial
	}
	return kindImported
}

func listMatches(ctx context.Context, c *tuiclient.Client, in listMatchesIn) (listMatchesOut, error) {
	matches, err := c.ListLibraryMatches(ctx, in.Map, in.SteamID)
	if err != nil {
		return listMatchesOut{}, err
	}
	demos, err := c.ListLibraryDemos(ctx)
	if err != nil {
		return listMatchesOut{}, err
	}
	series, err := c.ListLibrarySeries(ctx, "")
	if err != nil {
		return listMatchesOut{}, err
	}
	povJobs, err := jobsWithVideos(ctx, c, in.HasPOV)
	if err != nil {
		return listMatchesOut{}, err
	}
	demoByID := map[string]tuiclient.LibraryDemo{}
	for _, d := range demos {
		demoByID[d.ID] = d
	}
	seriesByID := map[string]tuiclient.LibrarySeries{}
	for _, se := range series {
		seriesByID[se.ID] = se
	}

	out := listMatchesOut{Matches: []listedMatch{}}
	imported := map[string]time.Time{}
	for _, m := range matches {
		d := demoByID[m.DemoID]
		source := d.Source
		if source == "" {
			source = m.Source
		}
		kind := kindOfSource(source)
		pov := slices.ContainsFunc(d.JobIDs, func(id string) bool { return povJobs[id] })
		if (in.Kind != "" && kind != in.Kind) || (in.HasPOV && !pov) {
			continue
		}
		item := listedMatch{matchSummary: summarizeMatch(m), Kind: kind, Source: cleanName(source), Tags: cleanNames(d.Tags), HasPOV: pov}
		if !d.Imported.IsZero() {
			item.ImportedAt = d.Imported.Format(time.DateOnly)
			imported[m.ID] = d.Imported
		}
		if se, ok := seriesByID[m.SeriesID]; ok {
			item.Series = matchSeriesOf(se, m.Map)
		}
		out.Matches = append(out.Matches, item)
	}
	if in.Sort == sortImported {
		slices.SortStableFunc(out.Matches, func(a, b listedMatch) int { return imported[b.ID].Compare(imported[a.ID]) })
	}
	return out, nil
}

// jobsWithVideos is the set of recent jobs that have a finished video; empty unless asked for.
func jobsWithVideos(ctx context.Context, c *tuiclient.Client, asked bool) (map[string]bool, error) {
	jobs := map[string]bool{}
	if !asked {
		return jobs, nil
	}
	rows, _, err := readVideoLibrary(ctx, c, maxVideoJobs)
	if err != nil {
		return nil, err
	}
	for _, v := range describeVideoLibrary(rows, nil, "").Videos {
		jobs[v.JobID] = true
	}
	return jobs, nil
}

func matchSeriesOf(se tuiclient.LibrarySeries, mapName string) *matchSeriesOut {
	base := seriesOf(se)
	out := &matchSeriesOut{ID: base.ID, Format: base.Format, Event: base.Event, Teams: base.Teams, Vetoes: base.Vetoes}
	for _, step := range se.Vetoes {
		if step.Map != mapName || (step.Action != "pick" && step.Action != "decider") {
			continue
		}
		out.MapRole = step.Action
		for _, t := range base.Teams {
			if t.Slot == step.Team {
				out.PickedBy = t.Name
			}
		}
	}
	return out
}
