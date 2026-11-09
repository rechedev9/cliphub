package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

var (
	errProMapPending = errors.New("the demo of this pro map is in your library but not analyzed yet")
	errProMapBroken  = errors.New("the community server sent an analysis Studio cannot read")
	errProDownload   = errors.New("download pro map")
	// errProMapRemoved marks a map whose match the user deleted; only an explicit import brings it back.
	errProMapRemoved = errors.New("the user deleted this pro map")
)

// proMapRef names one map of the catalog. explicit is a user's import; the
// follow sync passes false and so skips a map the user deleted.
type proMapRef struct {
	matchID  int
	entry    string
	explicit bool
}

func (h *Handlers) importProMap(ctx context.Context, ref proMapRef) (proImportResponse, error) {
	h.references.pro.importMu.Lock()
	defer h.references.pro.importMu.Unlock()
	rec, err := h.libraryRepo.GetProMap(ctx, ref.matchID, ref.entry)
	switch {
	case err == nil && rec.DemoID != nil:
		if d, err := h.libraryRepo.GetDemo(ctx, *rec.DemoID); err == nil && d.MatchID != nil {
			return proImportResponse{MatchID: *d.MatchID, DemoID: d.ID, StudioPath: proStudioPath(*d.MatchID),
				RadarMayBeOff: library.RadarMayBeOff(rec.Map, rec.AnalysisVersion)}, nil
		}
	case err == nil && !ref.explicit:
		return proImportResponse{}, errProMapRemoved
	case err != nil && !errors.Is(err, library.ErrNotFound):
		return proImportResponse{}, err
	}
	raw, err := h.references.community.Client.ProMapFileBytes(ctx, ref.matchID, ref.entry)
	if errors.Is(err, community.ErrNoProMap) {
		return proImportResponse{}, err
	}
	if err != nil {
		return proImportResponse{}, fmt.Errorf("%w: %w", errProDownload, err)
	}
	header, blob, err := community.ReadProAnalysis(bytes.NewReader(raw))
	if err == nil {
		err = community.CheckProAnalysis(header, blob)
	}
	if err == nil && (header.Match.MatchID != ref.matchID || strings.TrimSuffix(header.Entry, ".dem") != ref.entry) {
		err = errors.New("the file is of another map")
	}
	if err != nil {
		return proImportResponse{}, fmt.Errorf("%w: %w", errProMapBroken, err)
	}
	return h.storeProMap(ctx, ref, header, blob)
}

func (h *Handlers) storeProMap(ctx context.Context, ref proMapRef, header community.ProAnalysisHeader, blob []byte) (proImportResponse, error) {
	doc := header.Document
	played := time.Now().UTC()
	if t, err := time.Parse(time.RFC3339, header.Match.PlayedAt); err == nil {
		played = t.UTC()
	}
	d := library.Demo{SHA256: proDemoSHA(ref, doc), FileName: ref.entry + ".dem", Source: library.SourcePro, SourceManual: true,
		State: library.ParseParsing, PlayedAt: &played}
	created, err := h.libraryRepo.CreateDemo(ctx, &d)
	if err != nil {
		return proImportResponse{}, err
	}
	teams := proTeams(header.Match.Teams)
	rec := library.ProMap{MatchID: ref.matchID, Entry: ref.entry, DemoID: &d.ID, Map: doc.Demo.Map, Event: header.Match.Event,
		PlayedAt: played, Teams: teams, AnalysisVersion: doc.AnalysisVersion, SHA256: d.SHA256}
	resp := proImportResponse{DemoID: d.ID, RadarMayBeOff: library.RadarMayBeOff(rec.Map, rec.AnalysisVersion)}
	if !created && d.MatchID != nil {
		// The real .dem of this map is already in the library: the pro map points at it.
		resp.MatchID, resp.StudioPath = *d.MatchID, proStudioPath(*d.MatchID)
		return resp, h.libraryRepo.SaveProMap(ctx, &rec)
	}
	if !created && !d.Source.WithoutDemo() {
		return proImportResponse{}, errProMapPending
	}
	match, err := h.saveProAnalysis(ctx, d, doc, blob, teams)
	if err == nil {
		err = h.libraryRepo.SaveProMap(ctx, &rec)
	}
	if err != nil {
		h.dropSharedDemos([]uuid.UUID{d.ID})
		return proImportResponse{}, fmt.Errorf("store pro map %d/%s: %w", ref.matchID, ref.entry, err)
	}
	h.groupProSeries(ctx, header, match)
	log.Printf("httpapi: imported pro map match=%d entry=%s library_match=%s", ref.matchID, ref.entry, match.ID)
	resp.MatchID, resp.Created, resp.StudioPath = match.ID, true, proStudioPath(match.ID)
	return resp, nil
}

func (h *Handlers) saveProAnalysis(ctx context.Context, d library.Demo, doc tacticalplan.Document, blob []byte, teams []library.ProTeam) (library.Match, error) {
	body, err := json.Marshal(doc)
	if err != nil {
		return library.Match{}, err
	}
	if err := h.storage.Put(artifacts.LibraryTacticalPositionsKey(d.ID), bytes.NewReader(blob)); err != nil {
		return library.Match{}, err
	}
	if err := h.storage.Put(artifacts.LibraryTacticalIndexKey(d.ID), bytes.NewReader(body)); err != nil {
		return library.Match{}, err
	}
	h.aliasProTeams(ctx, doc, teams)
	return h.libraryRepo.SaveAnalysis(ctx, library.AnalysisFromDocument(d.ID, doc))
}

// proDemoSHA is the hash of the demo the analysis was read from, so adding that
// .dem later attaches it to this match; old analyses without it get a stable stand in.
func proDemoSHA(ref proMapRef, doc tacticalplan.Document) string {
	if len(doc.Demo.SHA256) == sha256.Size*2 {
		return strings.ToLower(doc.Demo.SHA256)
	}
	sum := sha256.Sum256([]byte("hltv-pro/" + strconv.Itoa(ref.matchID) + "/" + ref.entry))
	return hex.EncodeToString(sum[:])
}

func proTeams(teams []community.ProSidecarTeam) []library.ProTeam {
	out := make([]library.ProTeam, 0, len(teams))
	for _, t := range teams {
		team := library.ProTeam{ID: t.ID, Name: cleanLibraryText(t.Name)}
		if t.Rank != nil {
			team.Rank = *t.Rank
		}
		out = append(out, team)
	}
	return out
}

// aliasProTeams makes each clan name of the demo an alias of the team HLTV names,
// so the scan assigns the HLTV team. A clan the user already gave a team keeps it.
func (h *Handlers) aliasProTeams(ctx context.Context, doc tacticalplan.Document, teams []library.ProTeam) {
	clans := make([]string, len(doc.Teams))
	wire := make([]community.ProTeam, len(teams))
	for i, t := range doc.Teams {
		if t.Name != tacticalplan.FallbackTeamName(t.StartSide) {
			clans[i] = t.Name
		}
	}
	for i, t := range teams {
		wire[i] = community.ProTeam{ID: t.ID, Name: t.Name}
	}
	for i, idx := range community.MatchProSides(clans, wire) {
		clan := clans[i]
		if idx < 0 || clan == "" || library.NormalizeName(clan) == library.NormalizeName(teams[idx].Name) {
			continue
		}
		if _, found, err := h.findTeamByName(ctx, clan); found || err != nil {
			continue
		}
		team, err := h.teamByName(ctx, teams[idx].Name)
		if err == nil {
			aliases := append(slices.Clone(team.Aliases), clan)
			_, err = h.libraryRepo.UpdateTeam(ctx, team.ID, library.TeamPatch{Aliases: &aliases})
		}
		if err != nil {
			log.Printf("httpapi: pro team %q keeps clan %q apart: %v", teams[idx].Name, clan, err)
		}
	}
}

// groupProSeries puts the imported maps of one HLTV match in a series once two
// are in; after that only the new map joins, so a map the user took out stays out.
func (h *Handlers) groupProSeries(ctx context.Context, header community.ProAnalysisHeader, added library.Match) {
	records, err := h.libraryRepo.ListProMaps(ctx, header.Match.MatchID)
	if err != nil {
		log.Printf("httpapi: group pro series: %v", err)
		return
	}
	matches := []library.Match{}
	for _, rec := range records {
		if rec.DemoID == nil {
			continue
		}
		if d, err := h.libraryRepo.GetDemo(ctx, *rec.DemoID); err == nil && d.MatchID != nil {
			if m, err := h.libraryRepo.GetMatch(ctx, *d.MatchID); err == nil && !slices.ContainsFunc(matches, func(x library.Match) bool { return x.ID == m.ID }) {
				matches = append(matches, m)
			}
		}
	}
	if len(matches) < 2 {
		return
	}
	var seriesID *uuid.UUID
	for _, m := range matches {
		if m.SeriesID != nil && seriesID == nil {
			seriesID = m.SeriesID
		}
	}
	if seriesID == nil {
		s := library.Series{Format: proSeriesFormat(len(header.Match.Maps), len(matches)), Event: header.Match.Event, TeamIDs: proSeriesTeams(added)}
		if cleaned, err := library.CleanSeries(s); err == nil {
			s = cleaned
		}
		if err := h.libraryRepo.CreateSeries(ctx, &s); err != nil {
			log.Printf("httpapi: create pro series: %v", err)
			return
		}
		seriesID = &s.ID
	} else {
		matches = []library.Match{added}
	}
	for _, m := range matches {
		if m.SeriesID != nil {
			continue
		}
		if _, err := h.libraryRepo.UpdateMatch(ctx, m.ID, library.MatchPatch{SeriesID: seriesID}); err != nil {
			log.Printf("httpapi: attach pro map %s to its series: %v", m.ID, err)
		}
	}
}

func proSeriesFormat(sidecarMaps, imported int) string {
	if max(sidecarMaps, imported) > 3 {
		return library.FormatBO5
	}
	return library.FormatBO3
}

// proSeriesTeams is both resolved teams of the map, or none when either side has none.
func proSeriesTeams(m library.Match) []uuid.UUID {
	ids := []uuid.UUID{}
	for _, t := range m.Teams {
		if t.TeamID == nil || slices.Contains(ids, *t.TeamID) {
			return []uuid.UUID{}
		}
		ids = append(ids, *t.TeamID)
	}
	if len(ids) != 2 {
		return []uuid.UUID{}
	}
	return ids
}
