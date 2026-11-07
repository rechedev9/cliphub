package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// libretaRepository is the Libreta part of LibraryRepository.
type libretaRepository interface {
	CreateLineup(ctx context.Context, l *library.Lineup) error
	GetLineup(ctx context.Context, id uuid.UUID) (library.Lineup, error)
	ListLineups(ctx context.Context, filter library.LineupFilter) ([]library.Lineup, error)
	UpdateLineup(ctx context.Context, id uuid.UUID, patch library.LineupPatch) (library.Lineup, error)
	DeleteLineup(ctx context.Context, id uuid.UUID) error
	GetTeamRoles(ctx context.Context, teamID uuid.UUID, mapName string) (library.TeamRoleSheet, error)
	SetTeamRoles(ctx context.Context, sheet *library.TeamRoleSheet) error
}

// Error codes of the lineup routes.
const (
	codeThrowNotFound = "throw_not_found"
	codeThrowMissing  = "throw_missing"
)

// throwTechniqueWindow is how far before the throw the movement is read.
const throwTechniqueWindow = 0.25

// ListLibraryLineups handles GET /api/library/lineups.
func (h *Handlers) ListLibraryLineups(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	filter, err := lineupFilterQuery(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	lineups, err := h.libraryRepo.ListLineups(r.Context(), filter)
	if err != nil {
		internalError(w, "list lineups", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"lineups": lineups})
}

func lineupFilterQuery(q url.Values) (library.LineupFilter, error) {
	var f library.LineupFilter
	if f.Map = q.Get("map"); f.Map != "" && !mapNamePattern.MatchString(f.Map) {
		return f, errors.New("map must match ^[a-z0-9_]+$")
	}
	side, err := parseScoutSide(q.Get("side"))
	if err != nil {
		return f, err
	}
	f.Side = string(side)
	if f.Grenade = q.Get("grenade"); f.Grenade != "" && !library.ValidGrenade(f.Grenade) {
		return f, errors.New("grenade must be smoke, flash, he, molotov or decoy")
	}
	if f.Status = q.Get("status"); f.Status != "" && !library.ValidStatus(f.Status) {
		return f, errors.New("status must be active, practice, draft, review or archived")
	}
	if f.Tag = library.CleanName(q.Get("tag")); len([]rune(f.Tag)) > library.MaxTagRunes {
		return f, fmt.Errorf("tag must be at most %d characters", library.MaxTagRunes)
	}
	switch q.Get("include_archived") {
	case "", "false":
		f.ExcludeArchived = f.Status == ""
	case "true":
	default:
		return f, errors.New("include_archived must be true or false")
	}
	for _, p := range []struct {
		name string
		dst  *int
		min  int
	}{{"limit", &f.Limit, 1}, {"offset", &f.Offset, 0}} {
		if raw := q.Get(p.name); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < p.min || (p.name == "limit" && n > library.MaxLineupList) {
				return f, fmt.Errorf("%s is out of range", p.name)
			}
			*p.dst = n
		}
	}
	return f, nil
}

// lineupRequest is a lineup as a client sends it. From asks the server to
// read the throw of a library round; the other fields then override what it read.
type lineupRequest struct {
	From           *lineupFrom `json:"from"`
	Title          *string     `json:"title"`
	Map            *string     `json:"map"`
	Side           *string     `json:"side"`
	Grenade        *string     `json:"grenade"`
	Status         *string     `json:"status"`
	Tags           *[]string   `json:"tags"`
	Notes          *string     `json:"notes"`
	Origin         *[3]float64 `json:"origin"`
	Pitch          *float64    `json:"pitch"`
	Yaw            *float64    `json:"yaw"`
	ThrowCallout   *string     `json:"throw_callout"`
	LandingCallout *string     `json:"landing_callout"`
	Movement       *string     `json:"movement"`
	Jump           *bool       `json:"jump"`
	Click          *string     `json:"click"`
}

type lineupFrom struct {
	MatchID   uuid.UUID `json:"match_id"`
	Round     int       `json:"round"`
	ThrowTick int       `json:"throw_tick"`
}

// patch is the request as an edit; technique fields mark the lineup as checked by the user.
func (req lineupRequest) patch() library.LineupPatch {
	p := library.LineupPatch{Title: req.Title, Status: req.Status, Tags: req.Tags, Notes: req.Notes, Origin: req.Origin,
		Pitch: req.Pitch, Yaw: req.Yaw, ThrowCallout: req.ThrowCallout, LandingCallout: req.LandingCallout,
		Movement: req.Movement, Jump: req.Jump, Click: req.Click}
	if req.Movement != nil || req.Jump != nil || req.Click != nil {
		checked := false
		p.TechniqueEstimated = &checked
	}
	return p
}

// CreateLibraryLineup handles POST /api/library/lineups.
func (h *Handlers) CreateLibraryLineup(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	var req lineupRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid lineup JSON")
		return
	}
	var base library.Lineup
	if req.From != nil {
		read, code, err := h.lineupFromThrow(r.Context(), *req.From)
		if err != nil {
			if code == "" {
				internalError(w, "read throw", err)
				return
			}
			status := http.StatusBadRequest
			if code == codeTacticalNotReady {
				status = http.StatusConflict
			}
			writeCodedError(w, status, code, err.Error())
			return
		}
		base = read
	}
	for _, f := range []struct {
		dst *string
		src *string
	}{{&base.Map, req.Map}, {&base.Side, req.Side}, {&base.Grenade, req.Grenade}} {
		if f.src != nil {
			*f.dst = *f.src
		}
	}
	lineup, err := library.CleanLineup(library.ApplyLineupPatch(base, req.patch()))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.From != nil && req.Movement == nil && req.Jump == nil && req.Click == nil {
		lineup.TechniqueEstimated = true
	}
	if err := h.libraryRepo.CreateLineup(r.Context(), &lineup); err != nil {
		writeLibraryError(w, "create lineup", err)
		return
	}
	writeJSON(w, http.StatusCreated, lineup)
}

// lineupFromThrow reads one throw of a library round: where it left the hand,
// the eye angles, where it went off, the callouts and an estimate of the technique.
func (h *Handlers) lineupFromThrow(ctx context.Context, from lineupFrom) (library.Lineup, string, error) {
	m, err := h.libraryRepo.GetMatch(ctx, from.MatchID)
	if errors.Is(err, library.ErrNotFound) {
		return library.Lineup{}, codeUnknownMatch, errors.New("match is not in the library")
	} else if err != nil {
		return library.Lineup{}, "", err
	}
	doc, err := h.readLibraryDocument(m.DemoID)
	if storage.IsNotExist(err) {
		return library.Lineup{}, codeTacticalNotReady, errors.New("tactical analysis is not ready")
	} else if err != nil {
		return library.Lineup{}, "", err
	}
	round, ok := doc.RoundByNumber(from.Round)
	if !ok {
		return library.Lineup{}, codeThrowNotFound, errors.New("round not found")
	}
	var ev *tacticalplan.Event
	thrown := false
	for i := range round.Events {
		e := &round.Events[i]
		if e.Throw == nil || !library.ValidGrenade(string(e.Kind)) {
			continue
		}
		thrown = true
		if e.Throw.Tick == from.ThrowTick {
			ev = e
			break
		}
	}
	switch {
	case ev == nil && !thrown:
		return library.Lineup{}, codeThrowMissing, errors.New("this analysis has no throw positions; analyze the demo again")
	case ev == nil:
		return library.Lineup{}, codeThrowNotFound, errors.New("no grenade of the round was thrown on that tick")
	}
	landing := ev.Pos
	l := library.Lineup{
		Map: m.Map, Side: string(ev.Side), Grenade: string(ev.Kind), Origin: ev.Throw.Pos, Pitch: ev.Throw.Pitch, Yaw: ev.Throw.Yaw,
		Landing: &landing, ThrowCallout: tacticalplan.NearestCallout(doc.Geometry, ev.Throw.Pos),
		LandingCallout: tacticalplan.NearestCallout(doc.Geometry, landing),
		Source:         &tacticalplan.Evidence{MatchID: m.ID, Round: round.Number, Tick: ev.Throw.Tick},
	}
	if ev.ActorSlot != nil {
		if t, ok := h.throwTechnique(m.DemoID, doc, round.Number, *ev.ActorSlot, ev.Throw.Tick); ok {
			l.Movement, l.Jump = t.Movement, t.Jump
		}
	}
	return l, "", nil
}

// throwTechnique reads the thrower's samples just before and at the throw; a
// match without a position blob, or a thrower missing from it, has no estimate.
func (h *Handlers) throwTechnique(demoID uuid.UUID, doc tacticalplan.Document, round int, slot uint8, tick int) (tacticalplan.Technique, bool) {
	rc, err := h.storage.Open(artifacts.LibraryTacticalPositionsKey(demoID))
	if err != nil {
		return tacticalplan.Technique{}, false
	}
	defer rc.Close()
	blob, err := io.ReadAll(rc)
	if err != nil {
		log.Printf("httpapi: read positions of demo %s: %v", demoID, err)
		return tacticalplan.Technique{}, false
	}
	rate := doc.Demo.Tickrate
	at, okAt, errAt := tacticalplan.FrameAt(blob, doc, round, tick)
	before, okBefore, errBefore := tacticalplan.FrameAt(blob, doc, round, tick-int(throwTechniqueWindow*rate))
	if errAt != nil || errBefore != nil || !okAt || !okBefore || rate <= 0 {
		return tacticalplan.Technique{}, false
	}
	find := func(f tacticalplan.Frame) (tacticalplan.Sample, bool) {
		for _, s := range f.Samples {
			if s.Slot == slot {
				return s, true
			}
		}
		return tacticalplan.Sample{}, false
	}
	sAt, ok1 := find(at)
	sBefore, ok2 := find(before)
	if !ok1 || !ok2 {
		return tacticalplan.Technique{}, false
	}
	return tacticalplan.EstimateTechnique(sBefore, sAt, float64(at.Tick-before.Tick)/rate), true
}

func (h *Handlers) loadLibraryLineup(w http.ResponseWriter, r *http.Request) (library.Lineup, bool) {
	if !h.libraryReady(w) {
		return library.Lineup{}, false
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return library.Lineup{}, false
	}
	l, err := h.libraryRepo.GetLineup(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get lineup", err)
		return library.Lineup{}, false
	}
	return l, true
}

// GetLibraryLineup handles GET /api/library/lineups/{id}.
func (h *Handlers) GetLibraryLineup(w http.ResponseWriter, r *http.Request) {
	if l, ok := h.loadLibraryLineup(w, r); ok {
		writeJSON(w, http.StatusOK, l)
	}
}

// PatchLibraryLineup handles PATCH /api/library/lineups/{id}. Map, side and
// grenade are what the lineup is, so they do not change.
func (h *Handlers) PatchLibraryLineup(w http.ResponseWriter, r *http.Request) {
	current, ok := h.loadLibraryLineup(w, r)
	if !ok {
		return
	}
	var req lineupRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid lineup JSON")
		return
	}
	if req.From != nil || req.Map != nil || req.Side != nil || req.Grenade != nil {
		writeError(w, http.StatusBadRequest, "from, map, side and grenade cannot change")
		return
	}
	next, err := library.CleanLineup(library.ApplyLineupPatch(current, req.patch()))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := h.libraryRepo.UpdateLineup(r.Context(), current.ID, wholeLineupPatch(next))
	if err != nil {
		writeLibraryError(w, "update lineup", err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// wholeLineupPatch writes back every editable field of a cleaned lineup.
func wholeLineupPatch(l library.Lineup) library.LineupPatch {
	return library.LineupPatch{Title: &l.Title, Status: &l.Status, Tags: &l.Tags, Notes: &l.Notes, Origin: &l.Origin,
		Pitch: &l.Pitch, Yaw: &l.Yaw, ThrowCallout: &l.ThrowCallout, LandingCallout: &l.LandingCallout,
		Movement: &l.Movement, Jump: &l.Jump, Click: &l.Click, TechniqueEstimated: &l.TechniqueEstimated}
}

// DeleteLibraryLineup handles DELETE /api/library/lineups/{id}.
func (h *Handlers) DeleteLibraryLineup(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.libraryRepo.DeleteLineup(r.Context(), id); err != nil {
		writeLibraryError(w, "delete lineup", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
