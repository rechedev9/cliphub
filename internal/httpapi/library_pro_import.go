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
// follow sync passes false and so skips a map the user deleted or one that failed to read.
type proMapRef struct {
	matchID  int
	file     community.ProMapFile
	explicit bool
}

// proMapNewer reports a catalog map the server prepared again since the import.
func proMapNewer(rec library.ProMap, file community.ProMapFile) bool {
	return (file.SHA256 != "" && file.SHA256 != rec.SHA256) || file.AnalysisVersion > rec.AnalysisVersion
}

func (h *Handlers) importProMap(ctx context.Context, ref proMapRef) (proImportResponse, error) {
	h.references.pro.importMu.Lock()
	defer h.references.pro.importMu.Unlock()
	var replace *library.Demo
	rec, err := h.libraryRepo.GetProMap(ctx, ref.matchID, ref.file.Entry)
	switch {
	case err == nil && rec.DemoID != nil:
		d, err := h.libraryRepo.GetDemo(ctx, *rec.DemoID)
		if err == nil && d.MatchID != nil {
			if d.Source != library.SourcePro || !proMapNewer(rec, ref.file) {
				return proImportResponse{MatchID: *d.MatchID, DemoID: d.ID, StudioPath: proStudioPath(*d.MatchID),
					RadarMayBeOff: library.RadarMayBeOff(rec.Map, rec.AnalysisVersion)}, nil
			}
			replace = &d
		}
	case err == nil && !ref.explicit:
		return proImportResponse{}, errProMapRemoved
	case err != nil && !errors.Is(err, library.ErrNotFound):
		return proImportResponse{}, err
	}
	if !ref.explicit && h.references.pro.broken[ref.file.SHA256] {
		return proImportResponse{}, errProMapBroken
	}
	header, blob, err := h.downloadProMap(ctx, ref)
	if errors.Is(err, errProMapBroken) {
		if h.references.pro.broken == nil {
			h.references.pro.broken = map[string]bool{}
		}
		h.references.pro.broken[ref.file.SHA256] = true
	}
	if err != nil {
		return proImportResponse{}, err
	}
	header.Document = proDocument(header.Document)
	if replace != nil {
		return h.replaceProMap(ctx, proStore{ref: ref, header: header, blob: blob, rec: rec, demo: *replace})
	}
	return h.storeProMap(ctx, proStore{ref: ref, header: header, blob: blob})
}

// downloadProMap fetches every part of a map and joins them into one analysis.
func (h *Handlers) downloadProMap(ctx context.Context, ref proMapRef) (community.ProAnalysisHeader, []byte, error) {
	if len(ref.file.Parts) == 0 {
		return community.ProAnalysisHeader{}, nil, community.ErrNoProMap
	}
	parts := make([]community.ProPart, 0, len(ref.file.Parts))
	for _, part := range ref.file.Parts {
		raw, err := h.references.community.Client.ProMapFileBytes(ctx, ref.matchID, part.Entry)
		if errors.Is(err, community.ErrNoProMap) {
			return community.ProAnalysisHeader{}, nil, err
		}
		if err != nil {
			return community.ProAnalysisHeader{}, nil, fmt.Errorf("%w: %w", errProDownload, err)
		}
		header, blob, err := community.ReadProAnalysis(bytes.NewReader(raw))
		if err == nil {
			err = community.CheckProAnalysis(header, blob)
		}
		if err == nil && (header.Match.MatchID != ref.matchID || strings.TrimSuffix(header.Entry, ".dem") != part.Entry) {
			err = errors.New("the file is of another map")
		}
		if err != nil {
			return community.ProAnalysisHeader{}, nil, fmt.Errorf("%w: %s: %w", errProMapBroken, part.Entry, err)
		}
		parts = append(parts, community.ProPart{Header: header, Blob: blob})
	}
	header, blob, err := community.MergeProParts(parts)
	if err != nil {
		return community.ProAnalysisHeader{}, nil, fmt.Errorf("%w: %w", errProMapBroken, err)
	}
	return header, blob, nil
}

// proDocument leaves overtime out of an analysis older than version 7, whose
// overtime rounds can give a side's numbers to the other team.
func proDocument(doc tacticalplan.Document) tacticalplan.Document {
	if doc.AnalysisVersion >= 7 {
		return doc
	}
	rounds := []tacticalplan.Round{}
	for _, r := range doc.Rounds {
		if r.Overtime == 0 {
			rounds = append(rounds, r)
		}
	}
	if len(rounds) < len(doc.Rounds) {
		doc.Rounds = rounds
		doc.Warnings = append(slices.Clone(doc.Warnings), "overtime rounds left out: the analysis predates version 7")
	}
	return doc
}

// proStore is one downloaded map to store; rec and demo are set when it replaces an import.
type proStore struct {
	ref    proMapRef
	header community.ProAnalysisHeader
	blob   []byte
	rec    library.ProMap
	demo   library.Demo
}

func (h *Handlers) storeProMap(ctx context.Context, in proStore) (proImportResponse, error) {
	doc := in.header.Document
	played := time.Now().UTC()
	if t, err := time.Parse(time.RFC3339, in.header.Match.PlayedAt); err == nil {
		played = t.UTC()
	}
	d := library.Demo{SHA256: proDemoSHA(in.ref, doc), FileName: in.ref.file.Entry + ".dem", Source: library.SourcePro, SourceManual: true,
		State: library.ParseParsing, PlayedAt: &played}
	created, err := h.libraryRepo.CreateDemo(ctx, &d)
	if err != nil {
		return proImportResponse{}, err
	}
	rec := library.ProMap{MatchID: in.ref.matchID, Entry: in.ref.file.Entry, DemoID: &d.ID, Map: doc.Demo.Map, Event: in.header.Match.Event,
		PlayedAt: played, Teams: proTeams(in.header.Match.Teams), AnalysisVersion: doc.AnalysisVersion, SHA256: in.ref.file.SHA256}
	resp := proImportResponse{DemoID: d.ID, RadarMayBeOff: library.RadarMayBeOff(rec.Map, rec.AnalysisVersion)}
	if !created && d.MatchID != nil {
		// The real .dem of this map is already in the library: the pro map points at it.
		resp.MatchID, resp.StudioPath = *d.MatchID, proStudioPath(*d.MatchID)
		return resp, h.libraryRepo.SaveProMap(ctx, &rec)
	}
	if !created && !d.Source.WithoutDemo() {
		return proImportResponse{}, errProMapPending
	}
	match, err := h.saveProAnalysis(ctx, proAnalysisSave{demo: d, doc: doc, blob: in.blob, rec: &rec})
	if err == nil {
		err = h.libraryRepo.SaveProMap(ctx, &rec)
	}
	if err != nil {
		h.dropSharedDemos([]uuid.UUID{d.ID})
		return proImportResponse{}, fmt.Errorf("store pro map %d/%s: %w", in.ref.matchID, in.ref.file.Entry, err)
	}
	h.groupProSeries(ctx, in.header, match)
	log.Printf("httpapi: imported pro map match=%d entry=%s parts=%d library_match=%s", in.ref.matchID, in.ref.file.Entry, len(in.ref.file.Parts), match.ID)
	resp.MatchID, resp.Created, resp.StudioPath = match.ID, true, proStudioPath(match.ID)
	return resp, nil
}

// replaceProMap swaps the analysis of an imported map for the one the server prepared again.
func (h *Handlers) replaceProMap(ctx context.Context, in proStore) (proImportResponse, error) {
	rec := in.rec
	rec.Map, rec.Event, rec.Teams = in.header.Document.Demo.Map, in.header.Match.Event, proTeams(in.header.Match.Teams)
	rec.AnalysisVersion, rec.SHA256 = in.header.Document.AnalysisVersion, in.ref.file.SHA256
	match, err := h.saveProAnalysis(ctx, proAnalysisSave{demo: in.demo, doc: in.header.Document, blob: in.blob, rec: &rec})
	if err == nil {
		err = h.libraryRepo.SaveProMap(ctx, &rec)
	}
	if err != nil {
		return proImportResponse{}, fmt.Errorf("replace pro map %d/%s: %w", in.ref.matchID, in.ref.file.Entry, err)
	}
	if match.SeriesID != nil {
		h.fillProSeries(ctx, proSeriesFill{id: *match.SeriesID, sidecar: in.header.Match})
	}
	log.Printf("httpapi: replaced pro map match=%d entry=%s analysis_version=%d", in.ref.matchID, in.ref.file.Entry, rec.AnalysisVersion)
	return proImportResponse{MatchID: match.ID, DemoID: in.demo.ID, StudioPath: proStudioPath(match.ID),
		RadarMayBeOff: library.RadarMayBeOff(rec.Map, rec.AnalysisVersion)}, nil
}

// proAnalysisSave is one analysis to store for a pro demo; rec gets each team's players.
type proAnalysisSave struct {
	demo library.Demo
	doc  tacticalplan.Document
	blob []byte
	rec  *library.ProMap
}

func (h *Handlers) saveProAnalysis(ctx context.Context, in proAnalysisSave) (library.Match, error) {
	body, err := json.Marshal(in.doc)
	if err != nil {
		return library.Match{}, err
	}
	if err := h.storage.Put(artifacts.LibraryTacticalPositionsKey(in.demo.ID), bytes.NewReader(in.blob)); err != nil {
		return library.Match{}, err
	}
	if err := h.storage.Put(artifacts.LibraryTacticalIndexKey(in.demo.ID), bytes.NewReader(body)); err != nil {
		return library.Match{}, err
	}
	pairs := h.pairProSides(ctx, in)
	for side, idx := range pairs {
		if idx >= 0 {
			in.rec.Teams[idx].SteamIDs = proRoster(in.doc, in.doc.Teams[side].Key)
		}
	}
	h.aliasProTeams(ctx, proPairing{doc: in.doc, teams: in.rec.Teams, pairs: pairs})
	match, err := h.libraryRepo.SaveAnalysis(ctx, library.AnalysisFromDocument(in.demo.ID, in.doc))
	if err != nil {
		return library.Match{}, err
	}
	sides := []library.MatchTeam{}
	for side, idx := range pairs {
		if idx >= 0 {
			sides = append(sides, library.MatchTeam{Key: in.doc.Teams[side].Key, TeamName: in.rec.Teams[idx].Name})
		}
	}
	h.assignSharedTeams(ctx, match, sides)
	return match, nil
}

// proRoster is the SteamIDs of the players the scan put on a team.
func proRoster(doc tacticalplan.Document, teamKey string) []string {
	out := []string{}
	for _, p := range doc.Players {
		if p.TeamKey == teamKey && p.SteamID64 != "" && p.SteamID64 != "0" {
			out = append(out, p.SteamID64)
		}
	}
	return out
}

// pairProSides pairs each side of the demo with an HLTV team, by clan name or
// by the players earlier imports of that team fielded.
func (h *Handlers) pairProSides(ctx context.Context, save proAnalysisSave) []int {
	doc, teams := save.doc, save.rec.Teams
	in := community.ProSidePairing{Clans: make([]string, len(doc.Teams)), Rosters: make([][]string, len(doc.Teams)),
		Teams: make([]community.ProTeam, len(teams)), Known: map[int][]string{}}
	for i, t := range doc.Teams {
		if t.Name != tacticalplan.FallbackTeamName(t.StartSide) {
			in.Clans[i] = t.Name
		}
		in.Rosters[i] = proRoster(doc, t.Key)
	}
	for i, t := range teams {
		in.Teams[i] = community.ProTeam{ID: t.ID, Name: t.Name}
	}
	if records, err := h.libraryRepo.ListProMaps(ctx, 0); err == nil {
		for _, rec := range records {
			for _, t := range rec.Teams {
				in.Known[t.ID] = append(in.Known[t.ID], t.SteamIDs...)
			}
		}
	} else {
		log.Printf("httpapi: pro rosters: %v", err)
	}
	return community.PairProSides(in)
}

// proDemoSHA is the hash of the demo the analysis was read from, so adding that
// .dem later attaches it to this match; old analyses without it get a stable stand in.
func proDemoSHA(ref proMapRef, doc tacticalplan.Document) string {
	if len(doc.Demo.SHA256) == sha256.Size*2 {
		return strings.ToLower(doc.Demo.SHA256)
	}
	sum := sha256.Sum256([]byte("hltv-pro/" + strconv.Itoa(ref.matchID) + "/" + ref.file.Entry))
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

// proPairing is the sides of a demo and the HLTV team index paired with each.
type proPairing struct {
	doc   tacticalplan.Document
	teams []library.ProTeam
	pairs []int
}

// aliasProTeams makes each paired clan name an alias of its HLTV team, so a later
// scan of the real demo assigns that team. A clan the user already gave a team keeps it.
func (h *Handlers) aliasProTeams(ctx context.Context, p proPairing) {
	for side, idx := range p.pairs {
		clan := p.doc.Teams[side].Name
		if idx < 0 || clan == tacticalplan.FallbackTeamName(p.doc.Teams[side].StartSide) || library.NormalizeName(clan) == library.NormalizeName(p.teams[idx].Name) {
			continue
		}
		if _, found, err := h.findTeamByName(ctx, clan); found || err != nil {
			continue
		}
		team, err := h.teamByName(ctx, p.teams[idx].Name)
		if err == nil {
			aliases := append(slices.Clone(team.Aliases), clan)
			_, err = h.libraryRepo.UpdateTeam(ctx, team.ID, library.TeamPatch{Aliases: &aliases})
		}
		if err != nil {
			log.Printf("httpapi: pro team %q keeps clan %q apart: %v", p.teams[idx].Name, clan, err)
		}
	}
}
