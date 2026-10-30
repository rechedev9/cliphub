package store

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// MemoryLibraryRepository is the in-memory library behind
// ZV_DATABASE_URL=memory and the handler tests. It follows the SQLite
// repository's ordering and derivation rules exactly; contract tests run both.
type MemoryLibraryRepository struct {
	mu           sync.RWMutex
	demos        map[uuid.UUID]library.Demo
	matches      map[uuid.UUID]memoryLibraryMatch
	matchTeams   map[uuid.UUID][]memoryMatchTeam
	matchPlayers map[uuid.UUID][]library.MatchPlayer
	teams        map[uuid.UUID]memoryLibraryTeam
	aliases      map[string]memoryLibraryAlias
	players      map[string]library.Player
	series       map[uuid.UUID]library.Series
	jobLinks     map[uuid.UUID]memoryJobLink
	reports      map[uuid.UUID]library.Report
	annotations  map[uuid.UUID]library.Annotation
	plays        map[uuid.UUID]library.Play
}

type memoryJobLink struct {
	demoID   uuid.UUID
	linkedAt time.Time
}

type memoryLibraryMatch struct {
	id, demoID           uuid.UUID
	seriesID             *uuid.UUID
	mapName              string
	playedAt             time.Time
	tickrate             float64
	durationTicks        int
	roundCount           int
	createdAt, updatedAt time.Time
}

type memoryMatchTeam struct {
	key, startSide, clan, clanKey, assignment string
	teamID                                    *uuid.UUID
	score                                     int
}

type memoryLibraryTeam struct {
	id                   uuid.UUID
	name, nameKey        string
	createdAt, updatedAt time.Time
}

type memoryLibraryAlias struct {
	alias  string
	teamID uuid.UUID
}

// NewMemoryLibraryRepository returns an empty in-memory library.
func NewMemoryLibraryRepository() *MemoryLibraryRepository {
	return &MemoryLibraryRepository{
		demos:        map[uuid.UUID]library.Demo{},
		matches:      map[uuid.UUID]memoryLibraryMatch{},
		matchTeams:   map[uuid.UUID][]memoryMatchTeam{},
		matchPlayers: map[uuid.UUID][]library.MatchPlayer{},
		teams:        map[uuid.UUID]memoryLibraryTeam{},
		aliases:      map[string]memoryLibraryAlias{},
		players:      map[string]library.Player{},
		series:       map[uuid.UUID]library.Series{},
		jobLinks:     map[uuid.UUID]memoryJobLink{},
		reports:      map[uuid.UUID]library.Report{},
		annotations:  map[uuid.UUID]library.Annotation{},
		plays:        map[uuid.UUID]library.Play{},
	}
}

func (r *MemoryLibraryRepository) demoCopy(d library.Demo) library.Demo {
	d.Tags = append([]string{}, d.Tags...)
	if d.PlayedAt != nil {
		t := *d.PlayedAt
		d.PlayedAt = &t
	}
	d.MatchID = nil
	d.JobIDs = r.demoJobIDs(d.ID)
	for _, m := range r.matches {
		if m.demoID == d.ID {
			d.MatchID = uuidPtr(m.id)
		}
	}
	return d
}

// demoJobIDs mirrors the SQLite order: oldest link first, then job id.
func (r *MemoryLibraryRepository) demoJobIDs(demoID uuid.UUID) []uuid.UUID {
	type entry struct {
		id uuid.UUID
		at time.Time
	}
	var entries []entry
	for jobID, link := range r.jobLinks {
		if link.demoID == demoID {
			entries = append(entries, entry{id: jobID, at: link.linkedAt})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if !entries[i].at.Equal(entries[j].at) {
			return entries[i].at.Before(entries[j].at)
		}
		return entries[i].id.String() < entries[j].id.String()
	})
	ids := make([]uuid.UUID, len(entries))
	for i, e := range entries {
		ids[i] = e.id
	}
	return ids
}

func (r *MemoryLibraryRepository) LinkDemoJob(ctx context.Context, demoID, jobID uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.demos[demoID]; !ok {
		return library.ErrNotFound
	}
	if _, linked := r.jobLinks[jobID]; !linked {
		r.jobLinks[jobID] = memoryJobLink{demoID: demoID, linkedAt: libraryNow()}
	}
	return nil
}

func (r *MemoryLibraryRepository) DemoForJob(ctx context.Context, jobID uuid.UUID) (library.Demo, error) {
	if err := ctx.Err(); err != nil {
		return library.Demo{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	link, ok := r.jobLinks[jobID]
	if !ok {
		return library.Demo{}, library.ErrNotFound
	}
	d, ok := r.demos[link.demoID]
	if !ok {
		return library.Demo{}, library.ErrNotFound
	}
	return r.demoCopy(d), nil
}

func (r *MemoryLibraryRepository) CreateDemo(ctx context.Context, d *library.Demo) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	prepareNewDemo(d)
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.demos {
		if existing.SHA256 == d.SHA256 {
			*d = r.demoCopy(existing)
			return false, nil
		}
	}
	if _, exists := r.demos[d.ID]; exists {
		return false, library.ErrConflict
	}
	stored := *d
	stored.Tags = append([]string{}, d.Tags...)
	r.demos[d.ID] = stored
	return true, nil
}

func (r *MemoryLibraryRepository) GetDemo(ctx context.Context, id uuid.UUID) (library.Demo, error) {
	if err := ctx.Err(); err != nil {
		return library.Demo{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.demos[id]
	if !ok {
		return library.Demo{}, library.ErrNotFound
	}
	return r.demoCopy(d), nil
}

func hasTag(tags []string, tag string) bool {
	key := libraryTagKey(library.CleanName(tag))
	for _, t := range tags {
		if libraryTagKey(t) == key {
			return true
		}
	}
	return false
}

func (r *MemoryLibraryRepository) ListDemos(ctx context.Context, filter library.DemoFilter) ([]library.Demo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []library.Demo{}
	for _, d := range r.demos {
		if len(filter.States) > 0 && !containsState(filter.States, d.State) {
			continue
		}
		if filter.Source != "" && d.Source != filter.Source {
			continue
		}
		if filter.Tag != "" && !hasTag(d.Tags, filter.Tag) {
			continue
		}
		out = append(out, r.demoCopy(d))
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].ImportedAt.Equal(out[j].ImportedAt) {
			return out[i].ImportedAt.After(out[j].ImportedAt)
		}
		return out[i].ID.String() < out[j].ID.String()
	})
	return libraryPage(out, filter.Limit, filter.Offset), nil
}

func containsState(states []library.ParseState, s library.ParseState) bool {
	for _, x := range states {
		if x == s {
			return true
		}
	}
	return false
}

func (r *MemoryLibraryRepository) UpdateDemo(ctx context.Context, id uuid.UUID, patch library.DemoPatch) (library.Demo, error) {
	if err := ctx.Err(); err != nil {
		return library.Demo{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.demos[id]
	if !ok {
		return library.Demo{}, library.ErrNotFound
	}
	if patch.Source != nil {
		d.Source, d.SourceManual = *patch.Source, true
	}
	if patch.Tags != nil {
		d.Tags = library.NormalizeTags(*patch.Tags)
	}
	d.UpdatedAt = libraryNow()
	r.demos[id] = d
	return r.demoCopy(d), nil
}

func (r *MemoryLibraryRepository) SetDemoState(ctx context.Context, id uuid.UUID, state library.ParseState, errorCode, message string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.demos[id]
	if !ok {
		return library.ErrNotFound
	}
	if state != library.ParseFailed {
		errorCode, message = "", ""
	}
	d.State, d.ErrorCode, d.Error, d.UpdatedAt = state, errorCode, message, libraryNow()
	r.demos[id] = d
	return nil
}

func (r *MemoryLibraryRepository) DeleteDemo(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.demos[id]; !ok {
		return nil
	}
	delete(r.demos, id)
	for jobID, link := range r.jobLinks {
		if link.demoID == id {
			delete(r.jobLinks, jobID)
		}
	}
	for matchID, m := range r.matches {
		if m.demoID != id {
			continue
		}
		var steamIDs []string
		for _, p := range r.matchPlayers[matchID] {
			steamIDs = append(steamIDs, p.SteamID64)
		}
		delete(r.matches, matchID)
		delete(r.matchTeams, matchID)
		delete(r.matchPlayers, matchID)
		r.dropMatchAnnotations(matchID)
		r.refreshNicks(steamIDs)
	}
	return nil
}

// refreshNicks mirrors refreshPlayerNicks: a player's nick is the name of
// their most recent match by date, then by match id.
func (r *MemoryLibraryRepository) refreshNicks(steamIDs []string) {
	for _, steamID := range steamIDs {
		player, ok := r.players[steamID]
		if !ok {
			continue
		}
		var best *memoryLibraryMatch
		name := ""
		for matchID, lines := range r.matchPlayers {
			for _, line := range lines {
				if line.SteamID64 != steamID {
					continue
				}
				m := r.matches[matchID]
				if best == nil || m.playedAt.After(best.playedAt) || (m.playedAt.Equal(best.playedAt) && m.id.String() > best.id.String()) {
					best = &m
					name = line.Name
				}
			}
		}
		if best != nil {
			player.Nick = name
			r.players[steamID] = player
		}
	}
}

func (r *MemoryLibraryRepository) SaveAnalysis(ctx context.Context, analysis library.Analysis) (library.Match, error) {
	if err := ctx.Err(); err != nil {
		return library.Match{}, err
	}
	r.mu.Lock()
	demo, ok := r.demos[analysis.DemoID]
	if !ok {
		r.mu.Unlock()
		return library.Match{}, library.ErrNotFound
	}
	now := libraryNow()
	if !demo.SourceManual && library.ValidSource(analysis.Source) {
		demo.Source = analysis.Source
	}
	demo.CS2Build, demo.PatchVersion = analysis.CS2Build, analysis.PatchVersion
	demo.State, demo.ErrorCode, demo.Error, demo.UpdatedAt = library.ParseReady, "", "", now
	r.demos[demo.ID] = demo

	var match memoryLibraryMatch
	found := false
	for _, m := range r.matches {
		if m.demoID == demo.ID {
			match, found = m, true
		}
	}
	if !found {
		match = memoryLibraryMatch{id: uuid.New(), demoID: demo.ID, playedAt: demo.ImportedAt, createdAt: now}
		if demo.PlayedAt != nil {
			match.playedAt = *demo.PlayedAt
		}
	}
	match.mapName, match.tickrate, match.durationTicks, match.roundCount = analysis.Map, analysis.Tickrate, analysis.DurationTicks, analysis.RoundCount
	match.updatedAt = now
	r.matches[match.id] = match

	previous := map[string]memoryMatchTeam{}
	for _, t := range r.matchTeams[match.id] {
		previous[t.key] = t
	}
	var sides []memoryMatchTeam
	var taken []uuid.UUID
	for _, team := range analysis.Teams {
		row := memoryMatchTeam{
			key: team.Key, startSide: team.StartSide, clan: team.ClanName,
			clanKey: library.NormalizeName(team.ClanName), score: team.Score, assignment: library.AssignmentNone,
		}
		if prev, ok := previous[team.Key]; ok && prev.assignment == library.AssignmentManual {
			row.assignment, row.teamID = library.AssignmentManual, prev.teamID
		} else if row.clanKey != "" {
			teamID := r.resolveClanTeam(team.ClanName, now)
			if !containsUUID(taken, teamID) {
				row.assignment, row.teamID = library.AssignmentAuto, uuidPtr(teamID)
			}
		}
		if row.teamID != nil {
			taken = append(taken, *row.teamID)
		}
		sides = append(sides, row)
	}
	r.matchTeams[match.id] = sides

	var affected []string
	for _, p := range r.matchPlayers[match.id] {
		affected = append(affected, p.SteamID64)
	}
	lines := make([]library.MatchPlayer, 0, len(analysis.Players))
	for _, p := range analysis.Players {
		if _, ok := r.players[p.SteamID64]; !ok {
			r.players[p.SteamID64] = library.Player{SteamID64: p.SteamID64, Nick: p.Name, CreatedAt: now, UpdatedAt: now}
		}
		lines = append(lines, p)
		affected = append(affected, p.SteamID64)
	}
	r.matchPlayers[match.id] = lines
	r.refreshNicks(affected)
	r.mu.Unlock()
	return r.GetMatch(ctx, match.id)
}

func (r *MemoryLibraryRepository) resolveClanTeam(clan string, now time.Time) uuid.UUID {
	key := library.NormalizeName(clan)
	if alias, ok := r.aliases[key]; ok {
		return alias.teamID
	}
	name := library.CleanName(clan)
	id := uuid.New()
	r.teams[id] = memoryLibraryTeam{id: id, name: name, nameKey: library.NormalizeName(name), createdAt: now, updatedAt: now}
	r.aliases[key] = memoryLibraryAlias{alias: name, teamID: id}
	return id
}

func (r *MemoryLibraryRepository) matchView(m memoryLibraryMatch) library.Match {
	out := library.Match{
		ID: m.id, DemoID: m.demoID, Map: m.mapName, PlayedAt: m.playedAt,
		Source: r.demos[m.demoID].Source, Tickrate: m.tickrate, DurationTicks: m.durationTicks,
		RoundCount: m.roundCount, CreatedAt: m.createdAt, UpdatedAt: m.updatedAt,
		Teams: []library.MatchTeam{},
	}
	if m.seriesID != nil {
		out.SeriesID = uuidPtr(*m.seriesID)
	}
	sides := append([]memoryMatchTeam{}, r.matchTeams[m.id]...)
	sort.Slice(sides, func(i, j int) bool { return sides[i].key < sides[j].key })
	for _, side := range sides {
		team := library.MatchTeam{
			Key: side.key, StartSide: side.startSide, ClanName: side.clan,
			Assignment: side.assignment, Score: side.score,
		}
		if side.teamID != nil {
			team.TeamID = uuidPtr(*side.teamID)
			team.TeamName = r.teams[*side.teamID].name
		}
		out.Teams = append(out.Teams, team)
	}
	return out
}

func (r *MemoryLibraryRepository) GetMatch(ctx context.Context, id uuid.UUID) (library.Match, error) {
	if err := ctx.Err(); err != nil {
		return library.Match{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.matches[id]
	if !ok {
		return library.Match{}, library.ErrNotFound
	}
	out := r.matchView(m)
	out.Players = append([]library.MatchPlayer{}, r.matchPlayers[id]...)
	sort.Slice(out.Players, func(i, j int) bool {
		if out.Players[i].Slot != out.Players[j].Slot {
			return out.Players[i].Slot < out.Players[j].Slot
		}
		return out.Players[i].SteamID64 < out.Players[j].SteamID64
	})
	return out, nil
}

func (r *MemoryLibraryRepository) ListMatches(ctx context.Context, filter library.MatchFilter) ([]library.Match, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []library.Match{}
	for _, m := range r.matches {
		if !r.matchSelected(m, filter) {
			continue
		}
		out = append(out, r.matchView(m))
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].PlayedAt.Equal(out[j].PlayedAt) {
			return out[i].PlayedAt.After(out[j].PlayedAt)
		}
		return out[i].ID.String() < out[j].ID.String()
	})
	return libraryPage(out, filter.Limit, filter.Offset), nil
}

func (r *MemoryLibraryRepository) matchSelected(m memoryLibraryMatch, f library.MatchFilter) bool {
	demo := r.demos[m.demoID]
	switch {
	case f.Map != "" && m.mapName != f.Map,
		f.SeriesID != nil && !sameUUID(m.seriesID, f.SeriesID),
		f.Source != "" && demo.Source != f.Source,
		f.Tag != "" && !hasTag(demo.Tags, f.Tag),
		f.From != nil && m.playedAt.Before(libraryTime(*f.From)),
		f.To != nil && m.playedAt.After(libraryTime(*f.To)):
		return false
	}
	if f.TeamID != nil {
		found := false
		for _, side := range r.matchTeams[m.id] {
			found = found || sameUUID(side.teamID, f.TeamID)
		}
		if !found {
			return false
		}
	}
	if f.SteamID != "" {
		found := false
		for _, p := range r.matchPlayers[m.id] {
			found = found || p.SteamID64 == f.SteamID
		}
		if !found {
			return false
		}
	}
	return true
}

func (r *MemoryLibraryRepository) UpdateMatch(ctx context.Context, id uuid.UUID, patch library.MatchPatch) (library.Match, error) {
	if err := ctx.Err(); err != nil {
		return library.Match{}, err
	}
	r.mu.Lock()
	m, ok := r.matches[id]
	if !ok {
		r.mu.Unlock()
		return library.Match{}, library.ErrNotFound
	}
	now := libraryNow()
	switch {
	case patch.ClearSeries:
		m.seriesID = nil
	case patch.SeriesID != nil:
		s, ok := r.series[*patch.SeriesID]
		if !ok {
			r.mu.Unlock()
			return library.Match{}, library.ErrNotFound
		}
		s.UpdatedAt = now
		r.series[s.ID] = s
		m.seriesID = uuidPtr(*patch.SeriesID)
	}
	if patch.PlayedAt != nil {
		m.playedAt = libraryTime(*patch.PlayedAt)
	}
	m.updatedAt = now
	r.matches[id] = m
	if patch.PlayedAt != nil {
		var steamIDs []string
		for _, p := range r.matchPlayers[id] {
			steamIDs = append(steamIDs, p.SteamID64)
		}
		r.refreshNicks(steamIDs)
	}
	r.mu.Unlock()
	return r.GetMatch(ctx, id)
}

func (r *MemoryLibraryRepository) AssignMatchTeam(ctx context.Context, a library.TeamAssignment) (library.Match, error) {
	if err := ctx.Err(); err != nil {
		return library.Match{}, err
	}
	r.mu.Lock()
	sides := r.matchTeams[a.MatchID]
	index := -1
	for i, side := range sides {
		if side.key == a.TeamKey {
			index = i
		}
	}
	if _, ok := r.matches[a.MatchID]; !ok || index < 0 {
		r.mu.Unlock()
		return library.Match{}, library.ErrNotFound
	}
	if a.TeamID != nil {
		if _, ok := r.teams[*a.TeamID]; !ok {
			r.mu.Unlock()
			return library.Match{}, library.ErrNotFound
		}
		for i, side := range sides {
			if i != index && sameUUID(side.teamID, a.TeamID) {
				r.mu.Unlock()
				return library.Match{}, library.ErrConflict
			}
		}
	}
	side := sides[index]
	side.assignment = library.AssignmentManual
	side.teamID = nil
	if a.TeamID != nil {
		side.teamID = uuidPtr(*a.TeamID)
	}
	sides[index] = side
	if a.RememberAlias && a.TeamID != nil && side.clanKey != "" {
		alias, ok := r.aliases[side.clanKey]
		if !ok {
			alias.alias = side.clan
		}
		previousOwner := alias.teamID
		alias.teamID = *a.TeamID
		r.aliases[side.clanKey] = alias
		r.adoptAutoMatches(*a.TeamID, []string{side.clanKey})
		if ok && previousOwner != *a.TeamID && r.teamMatchCount(previousOwner) == 0 && len(r.teamView(r.teams[previousOwner]).Aliases) == 0 &&
			!r.teamHasReports(previousOwner) && !r.teamHasPlays(previousOwner) && !r.teamHasSeries(previousOwner) {
			delete(r.teams, previousOwner)
		}
	}
	m := r.matches[a.MatchID]
	m.updatedAt = libraryNow()
	r.matches[a.MatchID] = m
	r.mu.Unlock()
	return r.GetMatch(ctx, a.MatchID)
}

// adoptAutoMatches mirrors the SQLite statement of the same name, side by
// side in key order within each match.
func (r *MemoryLibraryRepository) adoptAutoMatches(teamID uuid.UUID, keys []string) {
	wanted := map[string]bool{}
	for _, k := range keys {
		wanted[k] = true
	}
	for matchID, sides := range r.matchTeams {
		for i, side := range sides {
			if side.assignment != library.AssignmentAuto || !wanted[side.clanKey] {
				continue
			}
			otherHasTeam := false
			for j, other := range sides {
				otherHasTeam = otherHasTeam || (j != i && other.teamID != nil && *other.teamID == teamID)
			}
			if !otherHasTeam {
				sides[i].teamID = uuidPtr(teamID)
			}
		}
		r.matchTeams[matchID] = sides
	}
}

func (r *MemoryLibraryRepository) claimAliases(owner uuid.UUID, aliases []libraryAlias) error {
	for _, a := range aliases {
		if existing, ok := r.aliases[a.key]; ok && existing.teamID != owner {
			return library.ErrConflict
		}
	}
	return nil
}

func (r *MemoryLibraryRepository) CreateTeam(ctx context.Context, t *library.Team) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	t.Name = library.CleanName(t.Name)
	now := libraryNow()
	aliases := normalizeLibraryAliases(append([]string{t.Name}, t.Aliases...))
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.teams[t.ID]; exists {
		return library.ErrConflict
	}
	if err := r.claimAliases(t.ID, aliases); err != nil {
		return err
	}
	r.teams[t.ID] = memoryLibraryTeam{id: t.ID, name: t.Name, nameKey: library.NormalizeName(t.Name), createdAt: now, updatedAt: now}
	for _, a := range aliases {
		r.aliases[a.key] = memoryLibraryAlias{alias: a.alias, teamID: t.ID}
	}
	r.adoptAutoMatches(t.ID, aliasKeys(aliases))
	t.CreatedAt, t.UpdatedAt = now, now
	t.Aliases = libraryAliasNames(aliases)
	t.MatchCount = r.teamMatchCount(t.ID)
	t.Roster = nil
	return nil
}

func (r *MemoryLibraryRepository) teamMatchCount(id uuid.UUID) int {
	n := 0
	for _, sides := range r.matchTeams {
		for _, side := range sides {
			if side.teamID != nil && *side.teamID == id {
				n++
			}
		}
	}
	return n
}

func (r *MemoryLibraryRepository) teamView(t memoryLibraryTeam) library.Team {
	out := library.Team{
		ID: t.id, Name: t.name, MatchCount: r.teamMatchCount(t.id),
		CreatedAt: t.createdAt, UpdatedAt: t.updatedAt, Aliases: []string{},
	}
	var keys []string
	for key, a := range r.aliases {
		if a.teamID == t.id {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		out.Aliases = append(out.Aliases, r.aliases[key].alias)
	}
	return out
}

// memoryLink is one match_players row joined with its side and match, the
// shape every derived read (roster, nicks, team history) groups over.
type memoryLink struct {
	match  memoryLibraryMatch
	player library.MatchPlayer
	teamID *uuid.UUID
}

func (r *MemoryLibraryRepository) links() []memoryLink {
	var out []memoryLink
	for matchID, lines := range r.matchPlayers {
		sides := map[string]*uuid.UUID{}
		for _, side := range r.matchTeams[matchID] {
			sides[side.key] = side.teamID
		}
		for _, line := range lines {
			out = append(out, memoryLink{match: r.matches[matchID], player: line, teamID: sides[line.TeamKey]})
		}
	}
	return out
}

func (r *MemoryLibraryRepository) GetTeam(ctx context.Context, id uuid.UUID) (library.Team, error) {
	if err := ctx.Err(); err != nil {
		return library.Team{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.teams[id]
	if !ok {
		return library.Team{}, library.ErrNotFound
	}
	out := r.teamView(t)
	roster := map[string]*library.RosterEntry{}
	for _, link := range r.links() {
		if link.teamID == nil || *link.teamID != id {
			continue
		}
		e, ok := roster[link.player.SteamID64]
		if !ok {
			e = &library.RosterEntry{SteamID64: link.player.SteamID64, Nick: r.players[link.player.SteamID64].Nick,
				FirstPlayedAt: link.match.playedAt, LastPlayedAt: link.match.playedAt}
			roster[link.player.SteamID64] = e
		}
		e.Matches++
		if link.match.playedAt.Before(e.FirstPlayedAt) {
			e.FirstPlayedAt = link.match.playedAt
		}
		if link.match.playedAt.After(e.LastPlayedAt) {
			e.LastPlayedAt = link.match.playedAt
		}
	}
	out.Roster = []library.RosterEntry{}
	for _, e := range roster {
		out.Roster = append(out.Roster, *e)
	}
	sort.Slice(out.Roster, func(i, j int) bool {
		if !out.Roster[i].LastPlayedAt.Equal(out.Roster[j].LastPlayedAt) {
			return out.Roster[i].LastPlayedAt.After(out.Roster[j].LastPlayedAt)
		}
		return out.Roster[i].SteamID64 < out.Roster[j].SteamID64
	})
	return out, nil
}

func (r *MemoryLibraryRepository) ListTeams(ctx context.Context, filter library.TeamFilter) ([]library.Team, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	q := library.NormalizeName(filter.Query)
	var selected []memoryLibraryTeam
	for _, t := range r.teams {
		if q != "" && !strings.Contains(t.nameKey, q) && !r.aliasContains(t.id, q) {
			continue
		}
		selected = append(selected, t)
	}
	sort.Slice(selected, func(i, j int) bool {
		if selected[i].nameKey != selected[j].nameKey {
			return selected[i].nameKey < selected[j].nameKey
		}
		return selected[i].id.String() < selected[j].id.String()
	})
	out := make([]library.Team, 0, len(selected))
	for _, t := range selected {
		out = append(out, r.teamView(t))
	}
	return libraryPage(out, filter.Limit, filter.Offset), nil
}

func (r *MemoryLibraryRepository) aliasContains(teamID uuid.UUID, q string) bool {
	for key, a := range r.aliases {
		if a.teamID == teamID && strings.Contains(key, q) {
			return true
		}
	}
	return false
}

func (r *MemoryLibraryRepository) UpdateTeam(ctx context.Context, id uuid.UUID, patch library.TeamPatch) (library.Team, error) {
	if err := ctx.Err(); err != nil {
		return library.Team{}, err
	}
	r.mu.Lock()
	t, ok := r.teams[id]
	if !ok {
		r.mu.Unlock()
		return library.Team{}, library.ErrNotFound
	}
	if patch.Aliases != nil {
		aliases := normalizeLibraryAliases(*patch.Aliases)
		if err := r.claimAliases(id, aliases); err != nil {
			r.mu.Unlock()
			return library.Team{}, err
		}
		for key, a := range r.aliases {
			if a.teamID == id {
				delete(r.aliases, key)
			}
		}
		for _, a := range aliases {
			r.aliases[a.key] = memoryLibraryAlias{alias: a.alias, teamID: id}
		}
		r.adoptAutoMatches(id, aliasKeys(aliases))
	}
	if patch.Name != nil {
		t.name = library.CleanName(*patch.Name)
		t.nameKey = library.NormalizeName(t.name)
	}
	t.updatedAt = libraryNow()
	r.teams[id] = t
	r.mu.Unlock()
	return r.GetTeam(ctx, id)
}

func (r *MemoryLibraryRepository) MergeTeams(ctx context.Context, from, into uuid.UUID) (library.Team, error) {
	if err := ctx.Err(); err != nil {
		return library.Team{}, err
	}
	if from == into {
		return library.Team{}, library.ErrConflict
	}
	r.mu.Lock()
	_, okFrom := r.teams[from]
	target, okInto := r.teams[into]
	if !okFrom || !okInto {
		r.mu.Unlock()
		return library.Team{}, library.ErrNotFound
	}
	for matchID, sides := range r.matchTeams {
		hasInto := false
		for _, side := range sides {
			hasInto = hasInto || (side.teamID != nil && *side.teamID == into)
		}
		for i, side := range sides {
			if side.teamID == nil || *side.teamID != from {
				continue
			}
			if hasInto {
				sides[i].teamID, sides[i].assignment = nil, library.AssignmentNone
			} else {
				sides[i].teamID = uuidPtr(into)
			}
		}
		r.matchTeams[matchID] = sides
	}
	for key, a := range r.aliases {
		if a.teamID == from {
			a.teamID = into
			r.aliases[key] = a
		}
	}
	for id, report := range r.reports {
		if report.TeamID == from {
			report.TeamID = into
			r.reports[id] = report
		}
	}
	for id, play := range r.plays {
		if play.TeamID != nil && *play.TeamID == from {
			play.TeamID = uuidPtr(into)
			r.plays[id] = play
		}
	}
	r.moveSeriesTeam(from, into)
	delete(r.teams, from)
	target.updatedAt = libraryNow()
	r.teams[into] = target
	r.mu.Unlock()
	return r.GetTeam(ctx, into)
}

func (r *MemoryLibraryRepository) playerMatchCount(steamID string) int {
	n := 0
	for _, lines := range r.matchPlayers {
		for _, line := range lines {
			if line.SteamID64 == steamID {
				n++
			}
		}
	}
	return n
}

func (r *MemoryLibraryRepository) GetPlayer(ctx context.Context, steamID string) (library.Player, error) {
	if err := ctx.Err(); err != nil {
		return library.Player{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.players[steamID]
	if !ok {
		return library.Player{}, library.ErrNotFound
	}
	p.MatchCount = r.playerMatchCount(steamID)
	nicks := map[string]*library.PlayerNick{}
	teams := map[uuid.UUID]*library.PlayerTeam{}
	for _, link := range r.links() {
		if link.player.SteamID64 != steamID {
			continue
		}
		at := link.match.playedAt
		n, ok := nicks[link.player.Name]
		if !ok {
			n = &library.PlayerNick{Nick: link.player.Name, FirstSeenAt: at, LastSeenAt: at}
			nicks[link.player.Name] = n
		}
		n.FirstSeenAt, n.LastSeenAt = minTime(n.FirstSeenAt, at), maxTime(n.LastSeenAt, at)
		if link.teamID == nil {
			continue
		}
		pt, ok := teams[*link.teamID]
		if !ok {
			pt = &library.PlayerTeam{TeamID: *link.teamID, TeamName: r.teams[*link.teamID].name, FirstPlayedAt: at, LastPlayedAt: at}
			teams[*link.teamID] = pt
		}
		pt.Matches++
		pt.FirstPlayedAt, pt.LastPlayedAt = minTime(pt.FirstPlayedAt, at), maxTime(pt.LastPlayedAt, at)
	}
	p.Nicks = []library.PlayerNick{}
	for _, n := range nicks {
		p.Nicks = append(p.Nicks, *n)
	}
	sort.Slice(p.Nicks, func(i, j int) bool {
		if !p.Nicks[i].LastSeenAt.Equal(p.Nicks[j].LastSeenAt) {
			return p.Nicks[i].LastSeenAt.After(p.Nicks[j].LastSeenAt)
		}
		return p.Nicks[i].Nick < p.Nicks[j].Nick
	})
	p.Teams = []library.PlayerTeam{}
	for _, pt := range teams {
		p.Teams = append(p.Teams, *pt)
	}
	sort.Slice(p.Teams, func(i, j int) bool {
		if !p.Teams[i].LastPlayedAt.Equal(p.Teams[j].LastPlayedAt) {
			return p.Teams[i].LastPlayedAt.After(p.Teams[j].LastPlayedAt)
		}
		return p.Teams[i].TeamID.String() < p.Teams[j].TeamID.String()
	})
	return p, nil
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

func (r *MemoryLibraryRepository) ListPlayers(ctx context.Context, filter library.PlayerFilter) ([]library.Player, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	q := strings.ToLower(strings.TrimSpace(filter.Query))
	onTeam := map[string]bool{}
	if filter.TeamID != nil {
		for _, link := range r.links() {
			if sameUUID(link.teamID, filter.TeamID) {
				onTeam[link.player.SteamID64] = true
			}
		}
	}
	out := []library.Player{}
	for _, p := range r.players {
		if q != "" && !strings.Contains(strings.ToLower(p.Nick), q) && p.SteamID64 != q {
			continue
		}
		if filter.TeamID != nil && !onTeam[p.SteamID64] {
			continue
		}
		p.MatchCount = r.playerMatchCount(p.SteamID64)
		p.Nicks, p.Teams = nil, nil
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i].Nick), strings.ToLower(out[j].Nick)
		if a != b {
			return a < b
		}
		return out[i].SteamID64 < out[j].SteamID64
	})
	return libraryPage(out, filter.Limit, filter.Offset), nil
}

func (r *MemoryLibraryRepository) SetPlayerFaceit(ctx context.Context, steamID, faceitPlayerID, faceitNickname string) (library.Player, error) {
	if err := ctx.Err(); err != nil {
		return library.Player{}, err
	}
	r.mu.Lock()
	p, ok := r.players[steamID]
	if !ok {
		r.mu.Unlock()
		return library.Player{}, library.ErrNotFound
	}
	p.FaceitPlayerID, p.FaceitNickname, p.UpdatedAt = faceitPlayerID, faceitNickname, libraryNow()
	r.players[steamID] = p
	r.mu.Unlock()
	return r.GetPlayer(ctx, steamID)
}
