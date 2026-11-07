package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/storage"
)

// multiPOVError is a client mistake in a multi-POV pick: the message is
// written for the Studio user and returned as a 400.
type multiPOVError struct{ msg string }

func (e *multiPOVError) Error() string { return e.msg }

// parseTargets resolves a parse request into the job's parse inputs. One
// target keeps the single-POV path; several must be teammates in the roster.
func (h *Handlers) parseTargets(j job.Job, req startParseRequest) (job.ParseInputs, error) {
	single, many := req.TargetSteamID, req.TargetSteamIDs
	if single != "" && len(many) > 0 {
		return job.ParseInputs{}, &multiPOVError{"send target_steamid or target_steamids, not both"}
	}
	if len(many) == 1 {
		single, many = many[0], nil
	}
	if len(many) == 0 {
		if _, err := strconv.ParseUint(single, 10, 64); err != nil {
			return job.ParseInputs{}, &multiPOVError{"target_steamid must be a 64-bit unsigned integer"}
		}
		return job.ParseInputs{TargetSteamID: single}, nil
	}
	roster, err := readRosterArtifact(h.storage, j)
	if err != nil {
		return job.ParseInputs{}, err
	}
	err = checkMultiPOVPick(many, func(id string) (string, bool) {
		i := slices.IndexFunc(roster.Players, func(p rosterPlayer) bool { return p.SteamID64 == id })
		if i < 0 {
			return "", false
		}
		return roster.Players[i].Team, true
	})
	if err != nil {
		return job.ParseInputs{}, err
	}
	return job.ParseInputs{TargetSteamID: many[0], MultiPOV: &job.MultiPOV{SteamIDs: slices.Clone(many)}}, nil
}

// checkMultiPOVPick validates a multi-POV player list against the demo's
// teams: teamOf answers a player's team, or false when the demo lacks them.
func checkMultiPOVPick(ids []string, teamOf func(string) (string, bool)) error {
	if len(ids) < job.MinMultiPOVPlayers || len(ids) > job.MaxMultiPOVPlayers {
		return &multiPOVError{fmt.Sprintf("a multi-POV video takes %d to %d players", job.MinMultiPOVPlayers, job.MaxMultiPOVPlayers)}
	}
	team := ""
	for i, id := range ids {
		if !validSteamID(id) {
			return &multiPOVError{"every target_steamids entry must be a 64-bit unsigned integer"}
		}
		if slices.Contains(ids[:i], id) {
			return &multiPOVError{"target_steamids repeats a player"}
		}
		got, ok := teamOf(id)
		switch {
		case !ok:
			return &multiPOVError{fmt.Sprintf("player %s is not in this demo", id)}
		case got == "":
			return &multiPOVError{fmt.Sprintf("player %s has no team in this demo", id)}
		case team == "":
			team = got
		case got != team:
			return &multiPOVError{"a multi-POV video takes players of one team"}
		}
	}
	return nil
}

// libraryMultiPOV validates a multi-POV pick over a library demo against the
// teams of its analysed match.
func (h *Handlers) libraryMultiPOV(ctx context.Context, d library.Demo, ids []string) (*job.MultiPOV, error) {
	if d.MatchID == nil {
		return nil, &multiPOVError{"the demo is not analysed yet"}
	}
	m, err := h.libraryRepo.GetMatch(ctx, *d.MatchID)
	if err != nil {
		return nil, fmt.Errorf("get match: %w", err)
	}
	err = checkMultiPOVPick(ids, func(id string) (string, bool) {
		i := slices.IndexFunc(m.Players, func(p library.MatchPlayer) bool { return p.SteamID64 == id })
		if i < 0 {
			return "", false
		}
		return m.Players[i].TeamKey, true
	})
	if err != nil {
		return nil, err
	}
	return &job.MultiPOV{SteamIDs: slices.Clone(ids)}, nil
}

func readRosterArtifact(store storage.Storage, j job.Job) (rosterArtifact, error) {
	rc, err := store.Open(artifacts.RosterKey(j.ID))
	if err != nil {
		if storage.IsNotExist(err) {
			return rosterArtifact{}, &multiPOVError{"the demo roster is not ready yet"}
		}
		return rosterArtifact{}, fmt.Errorf("open roster: %w", err)
	}
	defer rc.Close()
	var roster rosterArtifact
	if err := json.NewDecoder(rc).Decode(&roster); err != nil {
		return rosterArtifact{}, fmt.Errorf("decode roster: %w", err)
	}
	return roster, nil
}

// povClash names the first pair of chosen segments that film different
// players at the same time; an empty string means the selection is filmable.
func povClash(plan killplan.Plan, ids []string) string {
	var chosen []killplan.Segment
	for _, s := range plan.Segments {
		if slices.Contains(ids, s.ID) {
			chosen = append(chosen, s)
		}
	}
	for i, a := range chosen {
		for _, b := range chosen[i+1:] {
			if plan.POVClash(a, b) {
				return fmt.Sprintf("%s (%s) and %s (%s) happen at the same time; pick one of them",
					a.ID, plan.SegmentPOV(a).NameInDemo, b.ID, plan.SegmentPOV(b).NameInDemo)
			}
		}
	}
	return ""
}

// validateMultiPOVSelection adds the multi-POV rules to a segment pick: plays
// are chosen explicitly, there is no death reel, and no two clash.
func (h *Handlers) validateMultiPOVSelection(w http.ResponseWriter, j job.Job, ids []string, deathReel bool) bool {
	if j.KillPlan == nil || !j.KillPlan.IsMultiPOV() {
		return true
	}
	msg := ""
	switch {
	case deathReel:
		msg = "a multi-POV video has no death reel"
	case len(ids) == 0:
		msg = "a multi-POV video needs the segment ids of its plays"
	default:
		msg = povClash(*j.KillPlan, ids)
	}
	if msg != "" {
		writeCodedError(w, http.StatusBadRequest, multiPOVSelectionCode, msg)
		return false
	}
	return true
}

// rejectMultiPOVFullDemo answers 409 for a Full Demo on a multi-POV job: the
// whole-match program follows one player.
func rejectMultiPOVFullDemo(w http.ResponseWriter, j job.Job) bool {
	if !j.IsMultiPOV() {
		return false
	}
	writeCodedError(w, http.StatusConflict, multiPOVFullDemoCode, "Full Demo follows one player; pick a single player for it")
	return true
}

const (
	multiPOVSelectionCode = "multi_pov_selection"
	multiPOVFullDemoCode  = "multi_pov_full_demo"
)

// writeMultiPOVError answers a pick mistake with 400 and anything else as an
// internal error.
func writeMultiPOVError(w http.ResponseWriter, err error) {
	if _, ok := errors.AsType[*multiPOVError](err); ok {
		writeCodedError(w, http.StatusBadRequest, multiPOVSelectionCode, err.Error())
		return
	}
	internalError(w, "resolve multi-POV players", err)
}
