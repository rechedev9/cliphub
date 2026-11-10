package main

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// Match kinds of the Tipo de partida control (web/lib/library/partida-groups.ts matchKind).
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
	Map      string `json:"map,omitempty" jsonschema:"filter by map, e.g. de_mirage"`
	SteamID  string `json:"steamid,omitempty" jsonschema:"filter by a player's SteamID64"`
	Team     string `json:"team,omitempty" jsonschema:"only matches this team played, id from list_teams (the Equipo filter)"`
	Opponent string `json:"opponent,omitempty" jsonschema:"with team: only matches against this team, id from list_teams (the Rival filter)"`
	Event    string `json:"event,omitempty" jsonschema:"only matches in a series whose event contains this text (the Evento filter)"`
	Kind     string `json:"kind,omitempty" jsonschema:"scrim (scrim demos), oficial (tournament demos and matches in a series with an event) or importada (the rest, such as FACEIT pugs); empty keeps all"`
	HasPOV   bool   `json:"has_pov,omitempty" jsonschema:"true keeps the matches with a finished video among their jobs (Con POV)"`
	HasVoice bool   `json:"has_voice,omitempty" jsonschema:"true keeps the matches with imported team voice (Con voz)"`
	Sort     string `json:"sort,omitempty" jsonschema:"played (default, the match date) or imported (the import date)"`
}

func (in listMatchesIn) check() error {
	if err := checkOptionalFilter("map", in.Map, mapNamePattern); err != nil {
		return err
	}
	if err := checkOptionalFilter("steamid", in.SteamID, steamIDPattern); err != nil {
		return err
	}
	for field, id := range map[string]string{"team": in.Team, "opponent": in.Opponent} {
		if id != "" {
			if err := checkUUID(field, id); err != nil {
				return err
			}
		}
	}
	if in.Opponent != "" && in.Team == "" {
		return fmt.Errorf("opponent needs team")
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
	Source     string          `json:"source,omitempty" jsonschema:"valve, faceit, hltv, scrim, unknown or shared; update_demo changes it"`
	ImportedAt string          `json:"imported_at,omitempty" jsonschema:"YYYY-MM-DD"`
	Tags       []string        `json:"tags,omitempty" jsonschema:"untrusted labels; update_demo changes them"`
	HasPOV     bool            `json:"has_pov,omitempty" jsonschema:"only set when has_pov was asked"`
	HasVoice   bool            `json:"has_voice" jsonschema:"the match has imported team voice (the voice icon of its row)"`
	VoiceSync  string          `json:"voice_sync,omitempty" jsonschema:"with voice: auto (synced on its own), manual (synced by hand) or unsynced; sync_voice with auto true syncs again"`
	VoiceNote  string          `json:"voice_note,omitempty" jsonschema:"untrusted reason an unsynced recording gave"`
	Series     *matchSeriesOut `json:"series,omitempty"`
}

type listMatchesOut struct {
	Matches []listedMatch `json:"matches"`
}

// kindOf is the Tipo de partida of a match: scrims, tournament demos and matches of a series with an event
// are official, the rest (FACEIT pugs, Valve and unidentified demos) were imported.
func kindOf(source string, inEventSeries bool) string {
	switch {
	case source == "scrim":
		return kindScrim
	case source == "hltv" || inEventSeries:
		return kindOfficial
	}
	return kindImported
}

func listMatches(ctx context.Context, c *tuiclient.Client, in listMatchesIn) (listMatchesOut, error) {
	matches, err := c.ListLibraryMatches(ctx, tuiclient.LibraryMatchQuery{Map: in.Map, SteamID: in.SteamID, TeamID: in.Team})
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
	voices, err := c.ListVoiceSummaries(ctx)
	if err != nil {
		return listMatchesOut{}, err
	}
	voiceByMatch := map[string]tuiclient.VoiceSummary{}
	for _, v := range voices {
		voiceByMatch[v.MatchID] = v
	}
	demoByID := map[string]tuiclient.LibraryDemo{}
	for _, d := range demos {
		demoByID[d.ID] = d
	}
	seriesByID := map[string]tuiclient.LibrarySeries{}
	for _, se := range series {
		seriesByID[se.ID] = se
	}

	kept := []listedMatch{}
	jobsOf := map[string][]string{}
	imported := map[string]time.Time{}
	for _, m := range matches {
		d := demoByID[m.DemoID]
		se, inSeries := seriesByID[m.SeriesID]
		source := d.Source
		if source == "" {
			source = m.Source
		}
		kind := kindOf(source, inSeries && se.Event != "")
		voice, hasVoice := voiceByMatch[m.ID]
		if (in.Kind != "" && kind != in.Kind) || !playedAgainst(m, in.Opponent) || !inEvent(se, in.Event) || (in.HasVoice && !hasVoice) {
			continue
		}
		item := listedMatch{matchSummary: summarizeMatch(m), Kind: kind, Source: cleanName(source), Tags: cleanNames(d.Tags), HasVoice: hasVoice}
		if hasVoice {
			item.VoiceSync, item.VoiceNote = voice.Sync.State, cleanText(voice.Sync.Note, 300)
		}
		if !d.Imported.IsZero() {
			item.ImportedAt = d.Imported.Format(time.DateOnly)
			imported[m.ID] = d.Imported
		}
		if inSeries {
			item.Series = matchSeriesOf(se, m.Map)
		}
		jobsOf[m.ID] = d.JobIDs
		kept = append(kept, item)
	}
	if in.HasPOV {
		kept, err = withPOV(ctx, c, kept, jobsOf)
		if err != nil {
			return listMatchesOut{}, err
		}
	}
	if in.Sort == sortImported {
		slices.SortStableFunc(kept, func(a, b listedMatch) int { return imported[b.ID].Compare(imported[a.ID]) })
	}
	return listMatchesOut{Matches: kept}, nil
}

func playedAgainst(m tuiclient.LibraryMatch, opponent string) bool {
	return opponent == "" || slices.ContainsFunc(m.Teams, func(t tuiclient.LibraryMatchTeam) bool { return t.TeamID == opponent })
}

// inEvent keeps a match whose series event contains the text, case apart; no text keeps all.
func inEvent(se tuiclient.LibrarySeries, text string) bool {
	return text == "" || strings.Contains(strings.ToLower(se.Event), strings.ToLower(text))
}

// withPOV keeps the matches with a finished video among their jobs and marks them.
func withPOV(ctx context.Context, c *tuiclient.Client, matches []listedMatch, jobsOf map[string][]string) ([]listedMatch, error) {
	var all []string
	for _, m := range matches {
		all = append(all, jobsOf[m.ID]...)
	}
	withVideo, err := jobsWithVideos(ctx, c, all)
	if err != nil {
		return nil, err
	}
	kept := matches[:0]
	for _, m := range matches {
		if slices.ContainsFunc(jobsOf[m.ID], func(id string) bool { return withVideo[id] }) {
			m.HasPOV = true
			kept = append(kept, m)
		}
	}
	return kept, nil
}

// jobsWithVideos is the set of jobs among jobIDs with a ready render that wrote a video, the rows Partidas reads for its POV chip.
func jobsWithVideos(ctx context.Context, c *tuiclient.Client, jobIDs []string) (map[string]bool, error) {
	found := map[string]bool{}
	presets, err := c.Presets(ctx)
	if err != nil {
		return nil, err
	}
	var items []tuiclient.BatchItem
	for _, id := range jobIDs {
		if checkUUID("job_id", id) != nil {
			continue
		}
		for _, p := range presets.Presets {
			if artifactNamePattern.MatchString(p.Name) {
				items = append(items, tuiclient.BatchItem{JobID: id, Variant: p.Name})
			}
		}
	}
	for chunk := range slices.Chunk(items, tuiclient.BatchStatusMaxItems) {
		rows, err := c.BatchStatus(ctx, chunk)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if row.Error == nil && row.Render != nil && renderReady(row.Render.Status) && len(row.Render.Videos) > 0 {
				found[row.JobID] = true
			}
		}
	}
	return found, nil
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
