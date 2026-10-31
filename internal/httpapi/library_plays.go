package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/lineups"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// Stratbook file format. A file carries plays with their team by name, since
// team ids mean nothing on another PC; the play ids make a reimport idempotent.
const (
	stratbookFormat       = "cliphub-stratbook"
	stratbookVersion      = 1
	maxStratbookFileBytes = 8 << 20
)

// Error codes of the stratbook routes.
const (
	codeUnknownMatch     = "unknown_match"
	codePlayMapMismatch  = "map_mismatch"
	codeStratbookFormat  = "stratbook_format"
	codeTacticalNotReady = "tactical_not_ready"
)

// Lineup comparison outcomes of one grenade.
const (
	lineupMatched    = "matched"     // a catalog smoke landed within its radius
	lineupUnmatched  = "unmatched"   // a smoke on a map with lineups, none close enough
	lineupNoCatalog  = "no_catalog"  // the catalog has no lineups for the map
	lineupNotCovered = "not_covered" // the catalog lists smokes only
)

// WithLineupCatalog sets the smoke lineup catalog the round comparison and
// the stratbook drafts read.
func WithLineupCatalog(catalog lineups.Catalog) Option {
	return func(h *Handlers) {
		h.lineups = catalog
	}
}

// annotateLineups names the catalog lineup of every smoke a play holds.
func (h *Handlers) annotateLineups(mapName string, grenades []tacticalplan.PlayGrenade) {
	for i, g := range grenades {
		if g.Kind != tacticalplan.EventSmoke {
			continue
		}
		if m, ok := h.lineups.MatchLanding(mapName, g.Pos); ok {
			grenades[i].LineupID, grenades[i].Lineup = m.ID, m.Destination
		}
	}
}

// ListLibraryPlays handles GET /api/library/plays.
func (h *Handlers) ListLibraryPlays(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	filter, err := playFilterQuery(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	plays, err := h.libraryRepo.ListPlays(r.Context(), filter)
	if err != nil {
		internalError(w, "list plays", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plays": plays})
}

func playFilterQuery(q url.Values) (library.PlayFilter, error) {
	var f library.PlayFilter
	var err error
	if f.TeamID, err = optionalUUIDQuery(q, "team_id"); err != nil {
		return f, err
	}
	if f.Map = q.Get("map"); f.Map != "" && !mapNamePattern.MatchString(f.Map) {
		return f, errors.New("map must match ^[a-z0-9_]+$")
	}
	side, err := parseScoutSide(q.Get("side"))
	if err != nil {
		return f, err
	}
	f.Side = string(side)
	if f.Kind = q.Get("kind"); f.Kind != "" && !library.ValidPlayKind(string(tacticalplan.SideT), f.Kind) &&
		!library.ValidPlayKind(string(tacticalplan.SideCT), f.Kind) {
		return f, errors.New("kind must be a round pattern")
	}
	if raw := q.Get("limit"); raw != "" {
		f.Limit, err = strconv.Atoi(raw)
		if err != nil || f.Limit < 1 || f.Limit > library.MaxPlayList {
			return f, fmt.Errorf("limit must be an integer from 1 to %d", library.MaxPlayList)
		}
	}
	if raw := q.Get("offset"); raw != "" {
		f.Offset, err = strconv.Atoi(raw)
		if err != nil || f.Offset < 0 {
			return f, errors.New("offset must be a non-negative integer")
		}
	}
	return f, nil
}

// libraryPlayRequest is a play as a client sends it. team_id is raw so a
// PATCH can tell "leave it" (absent) from "detach it" (null).
type libraryPlayRequest struct {
	Title            *string                      `json:"title"`
	TeamID           json.RawMessage              `json:"team_id"`
	Map              *string                      `json:"map"`
	Side             *string                      `json:"side"`
	Kind             *string                      `json:"kind"`
	Site             *string                      `json:"site"`
	Origin           *string                      `json:"origin"`
	PositionsSeconds *float64                     `json:"positions_seconds"`
	Grenades         *[]tacticalplan.PlayGrenade  `json:"grenades"`
	Positions        *[]tacticalplan.PlayPosition `json:"positions"`
	Notes            *string                      `json:"notes"`
	Examples         *[]tacticalplan.Evidence     `json:"examples"`
}

// teamField reads team_id: set is false when absent, id is nil for null.
func (req libraryPlayRequest) teamField() (id *uuid.UUID, set bool, err error) {
	raw := strings.TrimSpace(string(req.TeamID))
	switch raw {
	case "":
		return nil, false, nil
	case "null":
		return nil, true, nil
	}
	var parsed uuid.UUID
	if err := json.Unmarshal(req.TeamID, &parsed); err != nil {
		return nil, true, errors.New("team_id must be a UUID or null")
	}
	return &parsed, true, nil
}

// apply copies the request's fields onto a play.
func (req libraryPlayRequest) apply(p library.Play) library.Play {
	set := func(dst *string, src *string) {
		if src != nil {
			*dst = *src
		}
	}
	set(&p.Title, req.Title)
	set(&p.Map, req.Map)
	set(&p.Side, req.Side)
	set(&p.Kind, req.Kind)
	set(&p.Site, req.Site)
	set(&p.Origin, req.Origin)
	set(&p.Notes, req.Notes)
	if req.PositionsSeconds != nil {
		p.PositionsSeconds = *req.PositionsSeconds
	}
	if req.Grenades != nil {
		p.Grenades = *req.Grenades
	}
	if req.Positions != nil {
		p.Positions = *req.Positions
	}
	if req.Examples != nil {
		p.Examples = *req.Examples
	}
	return p
}

// checkPlayExamples requires every example round to be a round of a library
// match on the play's map.
func (h *Handlers) checkPlayExamples(ctx context.Context, p library.Play) (string, error) {
	seen := map[uuid.UUID]library.Match{}
	for _, e := range p.Examples {
		m, ok := seen[e.MatchID]
		if !ok {
			var err error
			m, err = h.libraryRepo.GetMatch(ctx, e.MatchID)
			if errors.Is(err, library.ErrNotFound) {
				return codeUnknownMatch, fmt.Errorf("match %s does not exist", e.MatchID)
			}
			if err != nil {
				return "", err
			}
			seen[e.MatchID] = m
		}
		if m.Map != p.Map {
			return codePlayMapMismatch, fmt.Errorf("match %s is on %s, not on %s", m.ID, m.Map, p.Map)
		}
		if m.RoundCount > 0 && e.Round > m.RoundCount {
			return codeUnknownMatch, fmt.Errorf("match %s has no round %d", m.ID, e.Round)
		}
	}
	return "", nil
}

func writePlayError(w http.ResponseWriter, code string, err error) {
	if code != "" {
		writeCodedError(w, http.StatusBadRequest, code, err.Error())
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}

// CreateLibraryPlay handles POST /api/library/plays.
func (h *Handlers) CreateLibraryPlay(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	var req libraryPlayRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid play JSON")
		return
	}
	teamID, _, err := req.teamField()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	play, err := library.CleanPlay(req.apply(library.Play{TeamID: teamID, Origin: library.PlayOriginRound}))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if code, err := h.checkPlayExamples(r.Context(), play); err != nil {
		if code == "" {
			internalError(w, "check play examples", err)
			return
		}
		writePlayError(w, code, err)
		return
	}
	if err := h.libraryRepo.CreatePlay(r.Context(), &play); err != nil {
		writeLibraryError(w, "create play", err)
		return
	}
	writeJSON(w, http.StatusCreated, play)
}

// playResponse is one play with the example matches that are no longer in
// the library, such as those of an imported stratbook.
type playResponse struct {
	library.Play
	MissingMatchIDs []uuid.UUID `json:"missing_match_ids"`
}

func (h *Handlers) loadLibraryPlay(w http.ResponseWriter, r *http.Request) (library.Play, bool) {
	if !h.libraryReady(w) {
		return library.Play{}, false
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return library.Play{}, false
	}
	play, err := h.libraryRepo.GetPlay(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get play", err)
		return library.Play{}, false
	}
	return play, true
}

// GetLibraryPlay handles GET /api/library/plays/{id}.
func (h *Handlers) GetLibraryPlay(w http.ResponseWriter, r *http.Request) {
	play, ok := h.loadLibraryPlay(w, r)
	if !ok {
		return
	}
	resp := playResponse{Play: play, MissingMatchIDs: []uuid.UUID{}}
	seen := map[uuid.UUID]bool{}
	for _, e := range play.Examples {
		if seen[e.MatchID] {
			continue
		}
		seen[e.MatchID] = true
		if _, err := h.libraryRepo.GetMatch(r.Context(), e.MatchID); errors.Is(err, library.ErrNotFound) {
			resp.MissingMatchIDs = append(resp.MissingMatchIDs, e.MatchID)
		} else if err != nil {
			internalError(w, "get play match", err)
			return
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// PatchLibraryPlay handles PATCH /api/library/plays/{id}. Map, side and origin
// are what the play is, so they do not change; a play on another map is a new play.
func (h *Handlers) PatchLibraryPlay(w http.ResponseWriter, r *http.Request) {
	current, ok := h.loadLibraryPlay(w, r)
	if !ok {
		return
	}
	var req libraryPlayRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid play JSON")
		return
	}
	if req.Map != nil || req.Side != nil || req.Origin != nil || req.PositionsSeconds != nil {
		writeError(w, http.StatusBadRequest, "map, side, origin and positions_seconds cannot change")
		return
	}
	teamID, teamSet, err := req.teamField()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	next := req.apply(current)
	if teamSet {
		next.TeamID = teamID
	}
	next, err = library.CleanPlay(next)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Examples != nil {
		if code, err := h.checkPlayExamples(r.Context(), next); err != nil {
			if code == "" {
				internalError(w, "check play examples", err)
				return
			}
			writePlayError(w, code, err)
			return
		}
	}
	patch := library.PlayPatch{}
	if req.Title != nil {
		patch.Title = &next.Title
	}
	if req.Kind != nil {
		patch.Kind = &next.Kind
	}
	if req.Site != nil {
		patch.Site = &next.Site
	}
	if req.Notes != nil {
		patch.Notes = &next.Notes
	}
	if req.Grenades != nil {
		patch.Grenades = &next.Grenades
	}
	if req.Positions != nil {
		patch.Positions = &next.Positions
	}
	if req.Examples != nil {
		patch.Examples = &next.Examples
	}
	if teamSet {
		patch.TeamID, patch.ClearTeam = teamID, teamID == nil
	}
	updated, err := h.libraryRepo.UpdatePlay(r.Context(), current.ID, patch)
	if err != nil {
		writeLibraryError(w, "update play", err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// DeleteLibraryPlay handles DELETE /api/library/plays/{id}.
func (h *Handlers) DeleteLibraryPlay(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.libraryRepo.DeletePlay(r.Context(), id); err != nil {
		writeLibraryError(w, "delete play", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// libraryRound reads a match, its tactical document and one of its rounds.
func (h *Handlers) libraryRound(w http.ResponseWriter, r *http.Request) (library.Match, tacticalplan.Document, tacticalplan.Round, bool) {
	m, ok := h.loadLibraryMatch(w, r)
	if !ok {
		return library.Match{}, tacticalplan.Document{}, tacticalplan.Round{}, false
	}
	number, err := strconv.Atoi(chi.URLParam(r, "round"))
	if err != nil || number < 1 {
		writeError(w, http.StatusBadRequest, "round must be a positive integer")
		return library.Match{}, tacticalplan.Document{}, tacticalplan.Round{}, false
	}
	doc, err := h.readLibraryDocument(m.DemoID)
	if err != nil {
		if storage.IsNotExist(err) {
			writeCodedError(w, http.StatusConflict, codeTacticalNotReady, "tactical analysis is not ready")
		} else {
			internalError(w, "read tactical document", err)
		}
		return library.Match{}, tacticalplan.Document{}, tacticalplan.Round{}, false
	}
	round, ok := doc.RoundByNumber(number)
	if !ok {
		writeError(w, http.StatusNotFound, "round not found")
		return library.Match{}, tacticalplan.Document{}, tacticalplan.Round{}, false
	}
	return m, doc, round, true
}

// roundFrame reads the positions frame at tick from a match's blob; a match
// without a blob has none.
func (h *Handlers) roundFrame(demoID uuid.UUID, doc tacticalplan.Document, round, tick int) *tacticalplan.Frame {
	rc, err := h.storage.Open(artifacts.LibraryTacticalPositionsKey(demoID))
	if err != nil {
		return nil
	}
	defer rc.Close()
	blob, err := io.ReadAll(rc)
	if err != nil {
		log.Printf("httpapi: read positions of demo %s: %v", demoID, err)
		return nil
	}
	frame, ok, err := tacticalplan.FrameAt(blob, doc, round, tick)
	if err != nil {
		log.Printf("httpapi: frame of demo %s round %d: %v", demoID, round, err)
		return nil
	}
	if !ok {
		return nil
	}
	return &frame
}

// GetLibraryRoundPlay handles GET
// /api/library/matches/{id}/tactical/rounds/{round}/play?team_key=&tick=: the
// draft of the play one team ran in the round, for "guardar como jugada". The
// positions are read at tick, or PlayPositionSeconds after freeze end.
func (h *Handlers) GetLibraryRoundPlay(w http.ResponseWriter, r *http.Request) {
	m, doc, round, ok := h.libraryRound(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	teamKey := q.Get("team_key")
	var team *library.MatchTeam
	for i := range m.Teams {
		if m.Teams[i].Key == teamKey {
			team = &m.Teams[i]
		}
	}
	if team == nil {
		writeError(w, http.StatusBadRequest, "team_key must be ct-start or t-start")
		return
	}
	tick := round.TickFreezeEnd + int(tacticalplan.PlayPositionSeconds*doc.Demo.Tickrate)
	if raw := q.Get("tick"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < round.TickStart || (round.TickEnd > 0 && parsed > round.TickEnd) {
			writeError(w, http.StatusBadRequest, "tick must be inside the round")
			return
		}
		tick = parsed
	}
	sketch, err := tacticalplan.SketchRound(doc, round, teamKey, h.roundFrame(m.DemoID, doc, round.Number, tick))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.annotateLineups(m.Map, sketch.Grenades)
	draft := library.Play{
		TeamID: team.TeamID, TeamName: team.TeamName, Map: m.Map, Side: string(sketch.Side), Kind: sketch.Kind,
		Site: string(sketch.Site), Origin: library.PlayOriginRound, PositionsSeconds: sketch.PositionSeconds,
		Grenades: sketch.Grenades, Positions: sketch.Positions,
		Examples: []tacticalplan.Evidence{{MatchID: m.ID, Round: round.Number, Tick: tick}},
	}
	writeJSON(w, http.StatusOK, map[string]any{"draft": draft})
}

// roundLineup is one grenade of a round compared with the lineup catalog.
type roundLineup struct {
	Tick      int                    `json:"tick"`
	Seconds   float64                `json:"seconds"`
	Kind      tacticalplan.EventKind `json:"kind"`
	Side      tacticalplan.Side      `json:"side,omitempty"`
	Player    string                 `json:"player,omitempty"`
	SteamID64 string                 `json:"steamid64,omitempty"`
	Pos       [3]float64             `json:"pos"`
	Callout   string                 `json:"callout,omitempty"`
	Status    string                 `json:"status"`
	Lineup    *killplan.LineupMatch  `json:"lineup,omitempty"`
}

// roundLineupsResponse is a round's grenades against the catalog. ThrowChecked
// is false: the tactical document records where a grenade landed, not where
// it was thrown from, so a match reads the landing alone.
type roundLineupsResponse struct {
	MatchID      uuid.UUID     `json:"match_id"`
	Round        int           `json:"round"`
	Map          string        `json:"map"`
	CatalogSize  int           `json:"catalog_size"`
	ThrowChecked bool          `json:"throw_checked"`
	Smokes       int           `json:"smokes"`
	Matched      int           `json:"matched"`
	Grenades     []roundLineup `json:"grenades"`
}

// GetLibraryRoundLineups handles GET
// /api/library/matches/{id}/tactical/rounds/{round}/lineups.
func (h *Handlers) GetLibraryRoundLineups(w http.ResponseWriter, r *http.Request) {
	m, doc, round, ok := h.libraryRound(w, r)
	if !ok {
		return
	}
	resp := roundLineupsResponse{
		MatchID: m.ID, Round: round.Number, Map: m.Map, CatalogSize: h.lineups.MapSize(m.Map), Grenades: []roundLineup{},
	}
	// Some demos report one flash twice at the same tick and spot; it is listed once.
	seen := map[string]bool{}
	for _, ev := range round.Events {
		switch ev.Kind {
		case tacticalplan.EventSmoke, tacticalplan.EventFlash, tacticalplan.EventHE, tacticalplan.EventMolotov:
		default:
			continue
		}
		key := fmt.Sprintf("%s|%d|%v", ev.Kind, ev.Tick, ev.Pos)
		if seen[key] {
			continue
		}
		seen[key] = true
		g := roundLineup{Tick: ev.Tick, Kind: ev.Kind, Side: ev.Side, Pos: ev.Pos, Callout: tacticalplan.NearestCallout(doc.Geometry, ev.Pos)}
		if doc.Demo.Tickrate > 0 && ev.Tick >= round.TickFreezeEnd {
			g.Seconds = math.Round(float64(ev.Tick-round.TickFreezeEnd)/doc.Demo.Tickrate*10) / 10
		}
		if ev.ActorSlot != nil {
			if p, ok := doc.PlayerBySlot(*ev.ActorSlot); ok {
				g.Player, g.SteamID64 = p.Name, p.SteamID64
			}
		}
		switch {
		case ev.Kind != tacticalplan.EventSmoke:
			g.Status = lineupNotCovered
		case resp.CatalogSize == 0:
			resp.Smokes++
			g.Status = lineupNoCatalog
		default:
			resp.Smokes++
			g.Status = lineupUnmatched
			if match, ok := h.lineups.MatchLanding(m.Map, ev.Pos); ok {
				g.Status, g.Lineup = lineupMatched, &match
				resp.Matched++
			}
		}
		resp.Grenades = append(resp.Grenades, g)
	}
	writeJSON(w, http.StatusOK, resp)
}

// playDetectionResponse is the repeated plays of a team with the matches skipped.
type playDetectionResponse struct {
	TeamID   uuid.UUID      `json:"team_id"`
	TeamName string         `json:"team_name"`
	Skipped  []scoutSkipped `json:"skipped"`
	tacticalplan.PlayDetection
}

// DetectLibraryTeamPlays handles GET /api/library/teams/{id}/plays: the
// rounds the team repeats, grouped by utility and positions. It takes the
// scout filter (map, side, from, to, opponent_id, match_id).
func (h *Handlers) DetectLibraryTeamPlays(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	filter, matchIDs, err := scoutQuery(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	team, err := h.libraryRepo.GetTeam(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get team", err)
		return
	}
	loaded, skipped, err := h.loadTeamMatches(r.Context(), team, matchIDs, filter)
	if err != nil {
		internalError(w, "detect plays", err)
		return
	}
	det := tacticalplan.DetectPlays(loaded, filter)
	for i := range det.Plays {
		h.annotateLineups(det.Plays[i].Map, det.Plays[i].Grenades)
	}
	writeJSON(w, http.StatusOK, playDetectionResponse{TeamID: team.ID, TeamName: team.Name, Skipped: skipped, PlayDetection: det})
}

// stratbookFile is an exported stratbook.
type stratbookFile struct {
	Format     string         `json:"format"`
	Version    int            `json:"version"`
	ExportedAt time.Time      `json:"exported_at"`
	Plays      []library.Play `json:"plays"`
}

// ExportLibraryPlays handles GET /api/library/plays/export: the plays the list
// filter selects, as a stratbook file.
func (h *Handlers) ExportLibraryPlays(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	filter, err := playFilterQuery(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	filter.Limit, filter.Offset = library.MaxPlayList, 0
	plays, err := h.libraryRepo.ListPlays(r.Context(), filter)
	if err != nil {
		internalError(w, "export plays", err)
		return
	}
	writeJSON(w, http.StatusOK, stratbookFile{
		Format: stratbookFormat, Version: stratbookVersion, ExportedAt: time.Now().UTC().Truncate(time.Millisecond), Plays: plays,
	})
}

// stratbookRejected is a play of an imported file that was not stored.
type stratbookRejected struct {
	Index int    `json:"index"`
	Title string `json:"title"`
	Error string `json:"error"`
}

type stratbookImportResponse struct {
	Imported   int                 `json:"imported"`
	Duplicates int                 `json:"duplicates"`
	Rejected   []stratbookRejected `json:"rejected"`
	Plays      []library.Play      `json:"plays"`
}

// ImportLibraryPlays handles POST /api/library/plays/import with a stratbook
// file. Each play is stored on its own: a play whose id is already stored is a
// duplicate, a bad one is rejected, and the rest are kept. Teams are found by
// name or alias and created when missing. Examples are kept even when their
// match is not in this library; the play page lists them as missing.
func (h *Handlers) ImportLibraryPlays(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	var file stratbookFile
	if err := decodeJSONBody(w, r, &file, false, maxStratbookFileBytes); err != nil {
		writeCodedError(w, http.StatusBadRequest, codeStratbookFormat, "the file is not a ClipHub stratbook")
		return
	}
	if file.Format != stratbookFormat || file.Version != stratbookVersion {
		writeCodedError(w, http.StatusBadRequest, codeStratbookFormat,
			fmt.Sprintf("expected format %q version %d", stratbookFormat, stratbookVersion))
		return
	}
	if len(file.Plays) > library.MaxPlayList {
		writeCodedError(w, http.StatusBadRequest, codeStratbookFormat, fmt.Sprintf("a stratbook holds at most %d plays", library.MaxPlayList))
		return
	}
	resp := stratbookImportResponse{Rejected: []stratbookRejected{}, Plays: []library.Play{}}
	for i, in := range file.Plays {
		play, err := h.importPlay(r.Context(), in)
		switch {
		case errors.Is(err, library.ErrConflict):
			resp.Duplicates++
		case err != nil:
			resp.Rejected = append(resp.Rejected, stratbookRejected{Index: i, Title: in.Title, Error: err.Error()})
		default:
			resp.Imported++
			resp.Plays = append(resp.Plays, play)
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handlers) importPlay(ctx context.Context, in library.Play) (library.Play, error) {
	play, err := library.CleanPlay(library.Play{
		ID: in.ID, Title: in.Title, Map: in.Map, Side: in.Side, Kind: in.Kind, Site: in.Site,
		Origin: library.PlayOriginImport, PositionsSeconds: in.PositionsSeconds,
		Grenades: in.Grenades, Positions: in.Positions, Notes: in.Notes, Examples: in.Examples,
	})
	if err != nil {
		return library.Play{}, err
	}
	if in.ID != uuid.Nil {
		if _, err := h.libraryRepo.GetPlay(ctx, in.ID); err == nil {
			return library.Play{}, library.ErrConflict
		} else if !errors.Is(err, library.ErrNotFound) {
			return library.Play{}, err
		}
	}
	if name := cleanLibraryText(in.TeamName); name != "" {
		team, err := h.teamByName(ctx, name)
		if err != nil {
			return library.Play{}, fmt.Errorf("team %q: %w", name, err)
		}
		play.TeamID = &team.ID
	}
	if err := h.libraryRepo.CreatePlay(ctx, &play); err != nil {
		return library.Play{}, err
	}
	return play, nil
}
