package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// Libreta file format (named stratbook since version 1). A file carries plays
// and role sheets with their team by name, since team ids mean nothing on
// another PC; record ids make a reimport idempotent.
const (
	stratbookFormat       = "cliphub-stratbook"
	stratbookVersion      = 2
	maxStratbookFileBytes = 8 << 20
	maxFileTeamRoles      = 1000
	maxFileCalloutNames   = 5000
)

// stratbookFile is an exported Libreta: its plays and, from the whole
// Libreta export, its lineups, role sheets, calls and callout names.
type stratbookFile struct {
	Format       string                `json:"format"`
	Version      int                   `json:"version"`
	ExportedAt   time.Time             `json:"exported_at"`
	Plays        []library.Play        `json:"plays"`
	Lineups      []library.Lineup      `json:"lineups,omitempty"`
	TeamRoles    []fileTeamRoles       `json:"team_roles,omitempty"`
	Calls        []library.Call        `json:"calls,omitempty"`
	CalloutNames []library.CalloutName `json:"callout_names,omitempty"`
}

// fileTeamRoles is a role sheet with its team by name.
type fileTeamRoles struct {
	TeamName string             `json:"team_name"`
	Map      string             `json:"map"`
	T        []library.RoleSlot `json:"t"`
	CT       []library.RoleSlot `json:"ct"`
}

func newStratbookFile(plays []library.Play) stratbookFile {
	return stratbookFile{Format: stratbookFormat, Version: stratbookVersion, ExportedAt: time.Now().UTC().Truncate(time.Millisecond), Plays: plays}
}

// ExportLibraryPlays handles GET /api/library/plays/export: the plays the list
// filter selects, as a Libreta file.
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
	writeJSON(w, http.StatusOK, newStratbookFile(plays))
}

// ExportLibreta handles GET /api/library/libreta/export: the whole Libreta,
// archived records included.
func (h *Handlers) ExportLibreta(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	file, err := h.libretaFile(r.Context())
	if err != nil {
		internalError(w, "export libreta", err)
		return
	}
	writeJSON(w, http.StatusOK, file)
}

func (h *Handlers) libretaFile(ctx context.Context) (stratbookFile, error) {
	plays, err := h.libraryRepo.ListPlays(ctx, library.PlayFilter{Limit: library.MaxPlayList})
	if err != nil {
		return stratbookFile{}, err
	}
	file := newStratbookFile(plays)
	if file.Lineups, err = h.libraryRepo.ListLineups(ctx, library.LineupFilter{Limit: library.MaxLineupList}); err != nil {
		return stratbookFile{}, err
	}
	if file.Calls, err = h.libraryRepo.ListCalls(ctx, library.CallFilter{Limit: library.MaxCallList}); err != nil {
		return stratbookFile{}, err
	}
	if file.CalloutNames, err = h.libraryRepo.ListCalloutNames(ctx, ""); err != nil {
		return stratbookFile{}, err
	}
	sheets, err := h.libraryRepo.ListTeamRoles(ctx)
	if err != nil {
		return stratbookFile{}, err
	}
	names := map[uuid.UUID]string{}
	for _, s := range sheets {
		if _, ok := names[s.TeamID]; !ok {
			team, err := h.libraryRepo.GetTeam(ctx, s.TeamID)
			if err != nil {
				return stratbookFile{}, err
			}
			names[s.TeamID] = team.Name
		}
		file.TeamRoles = append(file.TeamRoles, fileTeamRoles{TeamName: names[s.TeamID], Map: s.Map, T: s.T, CT: s.CT})
	}
	return file, nil
}

// stratbookRejected is a play of an imported file that was not stored.
type stratbookRejected struct {
	Index int    `json:"index"`
	Title string `json:"title"`
	Error string `json:"error"`
}

// sectionImport counts what one section of an imported file did.
type sectionImport struct {
	Imported   int `json:"imported"`
	Duplicates int `json:"duplicates"`
	Rejected   int `json:"rejected"`
}

func (s *sectionImport) add(err error) {
	switch {
	case errors.Is(err, library.ErrConflict):
		s.Duplicates++
	case err != nil:
		s.Rejected++
	default:
		s.Imported++
	}
}

type stratbookImportResponse struct {
	Imported   int                 `json:"imported"`
	Duplicates int                 `json:"duplicates"`
	Rejected   []stratbookRejected `json:"rejected"`
	Plays      []library.Play      `json:"plays"`
	// Sections counts the other sections the file held, by their key.
	Sections map[string]sectionImport `json:"sections,omitempty"`
}

// ImportLibraryPlays handles POST /api/library/plays/import with a Libreta
// file. Each record is stored on its own: one whose id is already stored is a
// duplicate, a bad one is rejected, and the rest are kept. A role sheet or a
// callout name the Libreta already has is a duplicate too. Teams are found by
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
	if file.Format != stratbookFormat || file.Version < 1 || file.Version > stratbookVersion {
		writeCodedError(w, http.StatusBadRequest, codeStratbookFormat,
			fmt.Sprintf("expected format %q version 1 to %d", stratbookFormat, stratbookVersion))
		return
	}
	for _, limit := range []struct {
		what  string
		n, at int
	}{{"plays", len(file.Plays), library.MaxPlayList}, {"lineups", len(file.Lineups), library.MaxLineupList},
		{"calls", len(file.Calls), library.MaxCallList}, {"role sheets", len(file.TeamRoles), maxFileTeamRoles},
		{"callout names", len(file.CalloutNames), maxFileCalloutNames}} {
		if limit.n > limit.at {
			writeCodedError(w, http.StatusBadRequest, codeStratbookFormat, fmt.Sprintf("a stratbook holds at most %d %s", limit.at, limit.what))
			return
		}
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
	sections, err := h.importSections(r.Context(), file)
	if err != nil {
		internalError(w, "import libreta", err)
		return
	}
	if len(sections) > 0 {
		resp.Sections = sections
	}
	writeJSON(w, http.StatusOK, resp)
}

// importSections stores every section of the file besides plays.
func (h *Handlers) importSections(ctx context.Context, file stratbookFile) (map[string]sectionImport, error) {
	out := map[string]sectionImport{}
	if len(file.Lineups) > 0 {
		var s sectionImport
		for _, in := range file.Lineups {
			l, err := library.CleanLineup(library.Lineup{ID: in.ID, Title: in.Title, Map: in.Map, Side: in.Side, Grenade: in.Grenade,
				Status: in.Status, Tags: in.Tags, Notes: in.Notes, Origin: in.Origin, Pitch: in.Pitch, Yaw: in.Yaw, Landing: in.Landing,
				ThrowCallout: in.ThrowCallout, LandingCallout: in.LandingCallout, Movement: in.Movement, Jump: in.Jump, Click: in.Click,
				TechniqueEstimated: in.TechniqueEstimated, Source: in.Source})
			if err == nil {
				err = h.libraryRepo.CreateLineup(ctx, &l)
			}
			s.add(err)
		}
		out["lineups"] = s
	}
	if len(file.Calls) > 0 {
		var s sectionImport
		for _, in := range file.Calls {
			c, err := library.CleanCall(library.Call{ID: in.ID, Name: in.Name, Side: in.Side, Meaning: in.Meaning, Status: in.Status, Tags: in.Tags})
			if err == nil {
				err = h.libraryRepo.CreateCall(ctx, &c)
			}
			s.add(err)
		}
		out["calls"] = s
	}
	if len(file.CalloutNames) > 0 {
		s, err := h.importCalloutNames(ctx, file.CalloutNames)
		if err != nil {
			return nil, err
		}
		out["callout_names"] = s
	}
	if len(file.TeamRoles) > 0 {
		var s sectionImport
		for _, in := range file.TeamRoles {
			s.add(h.importTeamRoles(ctx, in))
		}
		out["team_roles"] = s
	}
	return out, nil
}

func (h *Handlers) importCalloutNames(ctx context.Context, names []library.CalloutName) (sectionImport, error) {
	var s sectionImport
	existing, err := h.libraryRepo.ListCalloutNames(ctx, "")
	if err != nil {
		return s, err
	}
	taken := map[[2]string]bool{}
	for _, c := range existing {
		taken[[2]string{c.Map, c.Token}] = true
	}
	for _, in := range names {
		c, err := library.CleanCalloutName(in)
		switch {
		case err != nil:
		case taken[[2]string{c.Map, c.Token}]:
			err = library.ErrConflict
		default:
			err = h.libraryRepo.SetCalloutName(ctx, &c)
			taken[[2]string{c.Map, c.Token}] = err == nil
		}
		s.add(err)
	}
	return s, nil
}

// importTeamRoles keeps a sheet the team already has on the map.
func (h *Handlers) importTeamRoles(ctx context.Context, in fileTeamRoles) error {
	name := cleanLibraryText(in.TeamName)
	if name == "" {
		return errors.New("team_name is required")
	}
	sheet, err := library.CleanTeamRoleSheet(library.TeamRoleSheet{Map: in.Map, T: in.T, CT: in.CT})
	if err != nil {
		return err
	}
	team, err := h.teamByName(ctx, name)
	if err != nil {
		return err
	}
	current, err := h.libraryRepo.GetTeamRoles(ctx, team.ID, sheet.Map)
	if err != nil {
		return err
	}
	if len(current.T) > 0 || len(current.CT) > 0 {
		return library.ErrConflict
	}
	sheet.TeamID = team.ID
	return h.libraryRepo.SetTeamRoles(ctx, &sheet)
}

func (h *Handlers) importPlay(ctx context.Context, in library.Play) (library.Play, error) {
	play, err := library.CleanPlay(library.Play{
		ID: in.ID, Title: in.Title, Map: in.Map, Side: in.Side, Kind: in.Kind, Site: in.Site,
		Origin: library.PlayOriginImport, PositionsSeconds: in.PositionsSeconds,
		Grenades: in.Grenades, Positions: in.Positions, Notes: in.Notes, Examples: in.Examples,
		Status: in.Status, Tags: in.Tags, Roles: in.Roles,
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
