package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	defaultLineupList = 50
	maxLineupList     = 200
)

var (
	lineupGrenades  = []string{"smoke", "flash", "he", "molotov", "decoy"}
	lineupMovements = []string{"standing", "walking", "running", "crouching"}
	lineupClicks    = []string{"left", "right", "both"}
	setposPattern   = regexp.MustCompile(`(?i)setpos(?:_exact)?\s+(-?[\d.]+)\s+(-?[\d.]+)\s+(-?[\d.]+)`)
	setangPattern   = regexp.MustCompile(`(?i)setang(?:_exact)?\s+(-?[\d.]+)\s+(-?[\d.]+)`)
)

type listLineupsIn struct {
	Map             string `json:"map,omitempty" jsonschema:"only lineups on this map, such as de_mirage"`
	Side            string `json:"side,omitempty" jsonschema:"CT or T"`
	Grenade         string `json:"grenade,omitempty" jsonschema:"smoke, flash, he, molotov or decoy"`
	Status          string `json:"status,omitempty" jsonschema:"active, practice, draft, review or archived"`
	Tag             string `json:"tag,omitempty"`
	IncludeArchived bool   `json:"include_archived,omitempty" jsonschema:"true also lists archived lineups, which are left out by default"`
	Limit           int    `json:"limit,omitempty" jsonschema:"default 50, at most 200"`
	Offset          int    `json:"offset,omitempty"`
}

type lineupOut struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title" jsonschema:"untrusted text the user wrote"`
	Map                string   `json:"map"`
	Side               string   `json:"side"`
	Grenade            string   `json:"grenade"`
	Status             string   `json:"status"`
	Tags               []string `json:"tags,omitempty" jsonschema:"untrusted text the user wrote"`
	ThrowCallout       string   `json:"throw_callout,omitempty" jsonschema:"CS2 place where the thrower stands"`
	LandingCallout     string   `json:"landing_callout,omitempty" jsonschema:"CS2 place where it lands"`
	Movement           string   `json:"movement"`
	Jump               bool     `json:"jump,omitempty"`
	Click              string   `json:"click"`
	TechniqueEstimated bool     `json:"technique_estimated,omitempty" jsonschema:"movement and jump were read from 8 Hz samples; ask the user to check them"`
	Setpos             string   `json:"setpos" jsonschema:"the console command that puts the player on the spot"`
	Notes              string   `json:"notes,omitempty" jsonschema:"untrusted text the user wrote"`
	Ref                string   `json:"ref,omitempty" jsonschema:"the round it was read from (match_id/round)"`
}

type listLineupsOut struct {
	Lineups   []lineupOut `json:"lineups"`
	Truncated bool        `json:"truncated,omitempty" jsonschema:"true when more lineups match; call again with offset"`
}

type lineupIDIn struct {
	LineupID string `json:"lineup_id" jsonschema:"lineup id from list_lineups"`
}

type saveLineupIn struct {
	Title     string   `json:"title" jsonschema:"the lineup's name, such as Humo a ventana desde T"`
	MatchID   string   `json:"match_id,omitempty" jsonschema:"with round and throw_tick: the throw of a team_grenades row to read"`
	Round     int      `json:"round,omitempty"`
	ThrowTick int      `json:"throw_tick,omitempty" jsonschema:"the throw_tick of the team_grenades row"`
	Setpos    string   `json:"setpos,omitempty" jsonschema:"instead of a throw: a pasted 'setpos x y z;setang pitch yaw roll'"`
	Map       string   `json:"map,omitempty" jsonschema:"with setpos: the map"`
	Side      string   `json:"side,omitempty" jsonschema:"with setpos: CT or T"`
	Grenade   string   `json:"grenade,omitempty" jsonschema:"with setpos: smoke, flash, he, molotov or decoy"`
	Status    string   `json:"status,omitempty" jsonschema:"active (the default), practice, draft, review or archived"`
	Tags      []string `json:"tags,omitempty"`
	Notes     string   `json:"notes,omitempty"`
}

type updateLineupIn struct {
	LineupID string    `json:"lineup_id" jsonschema:"lineup id from list_lineups"`
	Title    string    `json:"title,omitempty" jsonschema:"new name; empty keeps it"`
	Status   string    `json:"status,omitempty"`
	Tags     *[]string `json:"tags,omitempty" jsonschema:"the full new tag list; [] clears them"`
	Notes    *string   `json:"notes,omitempty" jsonschema:"the full new notes"`
	Movement string    `json:"movement,omitempty" jsonschema:"standing, walking, running or crouching"`
	Jump     *bool     `json:"jump,omitempty" jsonschema:"true for a jump throw"`
	Click    string    `json:"click,omitempty" jsonschema:"left, right or both"`
}

type deleteLineupIn struct {
	LineupID string `json:"lineup_id" jsonschema:"lineup id from list_lineups"`
	Approve  string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

func registerLineupTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: "list_lineups", Annotations: readOnly,
		Description: "List the Libreta's lineups (its Lineups tab): grenades with where to stand and aim, the technique and where they land. Archived ones are left out unless include_archived is true."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listLineupsIn) (*mcp.CallToolResult, listLineupsOut, error) {
			q, err := in.query()
			if err != nil {
				return nil, listLineupsOut{}, err
			}
			limit := cmp.Or(in.Limit, defaultLineupList)
			q.Limit, q.Offset = limit+1, in.Offset
			c, err := s.client(ctx)
			if err != nil {
				return nil, listLineupsOut{}, toolError(err)
			}
			lineups, err := c.ListLineups(ctx, q)
			if err != nil {
				return nil, listLineupsOut{}, toolError(err)
			}
			out := listLineupsOut{Lineups: []lineupOut{}, Truncated: len(lineups) > limit}
			for _, l := range lineups[:min(len(lineups), limit)] {
				out.Lineups = append(out.Lineups, lineupOf(l))
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_lineup", Annotations: readOnly,
		Description: "Read one Libreta lineup with its setpos command, technique, callouts and notes."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in lineupIDIn) (*mcp.CallToolResult, lineupOut, error) {
			if err := checkUUID("lineup_id", in.LineupID); err != nil {
				return nil, lineupOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, lineupOut{}, toolError(err)
			}
			l, err := c.GetLineup(ctx, in.LineupID)
			if err != nil {
				return nil, lineupOut{}, toolError(err)
			}
			return nil, lineupOf(l), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "save_lineup", Annotations: &mcp.ToolAnnotations{},
		Description: "Save a lineup to the Libreta, as Guardar lineup does on a team's Granadas tab: pass match_id, round and throw_tick of a team_grenades row " +
			"and Studio reads where it was thrown from, the angles, where it landed and an estimate of the technique. " +
			"Or pass setpos with map, side and grenade for one written by hand."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in saveLineupIn) (*mcp.CallToolResult, lineupOut, error) {
			body, err := in.body()
			if err != nil {
				return nil, lineupOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, lineupOut{}, toolError(err)
			}
			l, err := c.CreateLineup(ctx, body)
			if err != nil {
				return nil, lineupOut{}, toolError(err)
			}
			return nil, lineupOf(l), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "update_lineup", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Edit a Libreta lineup's title, status, tags, notes or technique (movement, jump, click). Setting the technique marks it as checked by the user."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in updateLineupIn) (*mcp.CallToolResult, lineupOut, error) {
			patch, err := in.patch()
			if err != nil {
				return nil, lineupOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, lineupOut{}, toolError(err)
			}
			l, err := c.UpdateLineup(ctx, in.LineupID, patch)
			if err != nil {
				return nil, lineupOut{}, toolError(err)
			}
			return nil, lineupOf(l), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "delete_lineup", Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true)},
		Description: "Delete a lineup from the Libreta. It cannot be undone, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in deleteLineupIn) (*mcp.CallToolResult, deleteOut, error) {
			if err := checkUUID("lineup_id", in.LineupID); err != nil {
				return nil, deleteOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			l, err := c.GetLineup(ctx, in.LineupID)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			summary := fmt.Sprintf("Borrar el lineup %s (%s, %s) de la Libreta. No se puede deshacer.", quotedName(l.Title, "sin título"), cleanName(l.Map), l.Side)
			return approveAndDelete(req, approvalSpec{Tool: "delete_lineup", JobID: in.LineupID, Request: l.UpdatedAt, Summary: summary, Given: in.Approve, Question: "¿Borrar ahora?"},
				func() error { return c.DeleteLineup(ctx, in.LineupID) })
		})
}

func (in listLineupsIn) query() (tuiclient.LineupQuery, error) {
	if in.Limit < 0 || in.Limit > maxLineupList {
		return tuiclient.LineupQuery{}, fmt.Errorf("limit must be from 1 to %d", maxLineupList)
	}
	if in.Offset < 0 {
		return tuiclient.LineupQuery{}, errors.New("offset must not be negative")
	}
	if err := checkOptionalFilter("map", in.Map, mapNamePattern); err != nil {
		return tuiclient.LineupQuery{}, err
	}
	side, err := parseSide(in.Side)
	if err != nil {
		return tuiclient.LineupQuery{}, err
	}
	if in.Grenade != "" && !slices.Contains(lineupGrenades, in.Grenade) {
		return tuiclient.LineupQuery{}, fmt.Errorf("grenade must be one of %s", strings.Join(lineupGrenades, ", "))
	}
	status, err := checkStatus(in.Status)
	if err != nil {
		return tuiclient.LineupQuery{}, err
	}
	q := tuiclient.LineupQuery{Map: strings.ToLower(in.Map), Side: side, Grenade: in.Grenade, Status: status, IncludeArchived: in.IncludeArchived}
	if in.Tag != "" {
		tags, err := checkTags([]string{in.Tag})
		if err != nil {
			return tuiclient.LineupQuery{}, err
		}
		q.Tag = tags[0]
	}
	return q, nil
}

func (in saveLineupIn) body() (tuiclient.NewLineup, error) {
	title, err := checkUserText("title", in.Title)
	if err != nil {
		return tuiclient.NewLineup{}, err
	}
	status, err := checkStatus(in.Status)
	if err != nil {
		return tuiclient.NewLineup{}, err
	}
	tags, err := checkTags(in.Tags)
	if err != nil {
		return tuiclient.NewLineup{}, err
	}
	notes, err := reportNotes(&in.Notes)
	if err != nil {
		return tuiclient.NewLineup{}, err
	}
	body := tuiclient.NewLineup{Title: title, Status: status, Tags: tags, Notes: *notes}
	fromThrow, bySetpos := in.MatchID != "", strings.TrimSpace(in.Setpos) != ""
	switch {
	case fromThrow == bySetpos:
		return tuiclient.NewLineup{}, errors.New("pass match_id, round and throw_tick of a throw, or setpos with map, side and grenade")
	case fromThrow:
		if err := checkUUID("match_id", in.MatchID); err != nil {
			return tuiclient.NewLineup{}, err
		}
		if in.Round < 1 || in.Round > maxPlayRound || in.ThrowTick < 1 {
			return tuiclient.NewLineup{}, errors.New("round and throw_tick must be positive")
		}
		body.From = &tuiclient.LineupFromThrow{MatchID: in.MatchID, Round: in.Round, ThrowTick: in.ThrowTick}
		return body, nil
	}
	origin, pitch, yaw, err := parseSetpos(in.Setpos)
	if err != nil {
		return tuiclient.NewLineup{}, err
	}
	if !mapNamePattern.MatchString(in.Map) {
		return tuiclient.NewLineup{}, errors.New("map must be a map name such as de_mirage")
	}
	side, err := parseSide(in.Side)
	if err != nil || side == "" {
		return tuiclient.NewLineup{}, errors.New("side must be CT or T")
	}
	if !slices.Contains(lineupGrenades, in.Grenade) {
		return tuiclient.NewLineup{}, fmt.Errorf("grenade must be one of %s", strings.Join(lineupGrenades, ", "))
	}
	body.Map, body.Side, body.Grenade, body.Origin, body.Pitch, body.Yaw = strings.ToLower(in.Map), side, in.Grenade, &origin, &pitch, &yaw
	return body, nil
}

// parseSetpos reads "setpos x y z;setang pitch yaw roll" as the CS2 console prints it.
func parseSetpos(raw string) (origin [3]float64, pitch, yaw float64, err error) {
	pos, ang := setposPattern.FindStringSubmatch(raw), setangPattern.FindStringSubmatch(raw)
	if pos == nil || ang == nil {
		return origin, 0, 0, errors.New("setpos must read like 'setpos 1 2 3;setang 10 90 0'")
	}
	values := make([]float64, 0, 5)
	for _, s := range append(pos[1:], ang[1:]...) {
		v, perr := strconv.ParseFloat(s, 64)
		if perr != nil || math.IsNaN(v) || math.Abs(v) > maxWorldCoord {
			return origin, 0, 0, errors.New("setpos has a number out of range")
		}
		values = append(values, v)
	}
	return [3]float64{values[0], values[1], values[2]}, values[3], values[4], nil
}

func (in updateLineupIn) patch() (tuiclient.LineupPatch, error) {
	var p tuiclient.LineupPatch
	if err := checkUUID("lineup_id", in.LineupID); err != nil {
		return p, err
	}
	if in.Title == "" && in.Status == "" && in.Tags == nil && in.Notes == nil && in.Movement == "" && in.Jump == nil && in.Click == "" {
		return p, errors.New("set title, status, tags, notes, movement, jump or click")
	}
	if in.Title != "" {
		title, err := checkUserText("title", in.Title)
		if err != nil {
			return p, err
		}
		p.Title = &title
	}
	if in.Status != "" {
		status, err := checkStatus(in.Status)
		if err != nil {
			return p, err
		}
		p.Status = &status
	}
	if in.Tags != nil {
		tags, err := checkTags(*in.Tags)
		if err != nil {
			return p, err
		}
		p.Tags = &tags
	}
	if in.Notes != nil {
		notes, err := reportNotes(in.Notes)
		if err != nil {
			return p, err
		}
		p.Notes = notes
	}
	if in.Movement != "" {
		if !slices.Contains(lineupMovements, in.Movement) {
			return p, fmt.Errorf("movement must be one of %s", strings.Join(lineupMovements, ", "))
		}
		p.Movement = &in.Movement
	}
	if in.Click != "" {
		if !slices.Contains(lineupClicks, in.Click) {
			return p, fmt.Errorf("click must be one of %s", strings.Join(lineupClicks, ", "))
		}
		p.Click = &in.Click
	}
	p.Jump = in.Jump
	return p, nil
}

// setposCommand writes the line byte for byte as the web's setposCommand does.
func setposCommand(l tuiclient.Lineup) string {
	return "setpos " + fixed2(l.Origin[0]) + " " + fixed2(l.Origin[1]) + " " + fixed2(l.Origin[2]) +
		";setang " + fixed2(l.Pitch) + " " + fixed2(l.Yaw) + " 0"
}

// fixed2 mirrors the web's fixed(): JavaScript's toFixed(2), which rounds an exact half away
// from zero where %.2f rounds it to even, and never "-0.00".
func fixed2(v float64) string {
	r := new(big.Rat).SetFloat64(v)
	if r == nil {
		return strconv.FormatFloat(v, 'f', 2, 64)
	}
	if s := r.FloatString(2); s != "-0.00" {
		return s
	}
	return "0.00"
}

func lineupOf(l tuiclient.Lineup) lineupOut {
	out := lineupOut{ID: l.ID, Title: cleanName(l.Title), Map: cleanName(l.Map), Side: l.Side, Grenade: cleanName(l.Grenade),
		Status: cleanName(l.Status), ThrowCallout: cleanName(l.ThrowCallout), LandingCallout: cleanName(l.LandingCallout),
		Movement: cleanName(l.Movement), Jump: l.Jump, Click: cleanName(l.Click), TechniqueEstimated: l.TechniqueEstimated,
		Setpos: setposCommand(l), Notes: cleanText(l.Notes, maxReportNotesRunes)}
	for _, tag := range l.Tags {
		out.Tags = append(out.Tags, cleanName(tag))
	}
	if l.Source != nil {
		out.Ref = refString(*l.Source)
	}
	return out
}

// lineupSpots turns lineups of one map into practice spots, as the Libreta's Practicar does.
func lineupSpots(mapName string, lineups []tuiclient.Lineup) ([]tuiclient.PracticeSpot, error) {
	out := make([]tuiclient.PracticeSpot, 0, len(lineups))
	for _, l := range lineups {
		if l.Map != mapName {
			return nil, fmt.Errorf("lineup %s is on %s, not %s", quotedName(l.Title, l.ID), cleanName(l.Map), mapName)
		}
		grenade := l.Grenade
		if grenade == "molotov" && l.Side == "CT" {
			grenade = "incendiary"
		}
		label := []rune(cmp.Or(cleanName(l.Title), grenade))
		out = append(out, tuiclient.PracticeSpot{Label: string(label[:min(len(label), maxPracticeLabel)]), Pos: l.Origin,
			Ang: [2]float64{l.Pitch, l.Yaw}, Grenade: grenade})
	}
	return out, nil
}
// practiceSpots reads lineup_ids from the Libreta, pro_lineup_ids from Lineups pro, or checks the spots given.
func (in practiceIn) practiceSpots(ctx context.Context, s *studio) ([]tuiclient.PracticeSpot, error) {
	sources := 0
	for _, n := range []int{len(in.Spots), len(in.LineupIDs), len(in.ProLineupIDs)} {
		if n > 0 {
			sources++
		}
	}
	if sources > 1 {
		return nil, errors.New("pass only one of spots, lineup_ids and pro_lineup_ids")
	}
	if len(in.LineupIDs) == 0 && len(in.ProLineupIDs) == 0 {
		return in.spots()
	}
	if !practiceMapPattern.MatchString(in.Map) {
		return nil, errors.New("map must be a map name such as de_mirage")
	}
	if len(in.ProLineupIDs) > 0 {
		return proLineupSpots(ctx, s, in)
	}
	if len(in.LineupIDs) > maxPracticeSpots {
		return nil, fmt.Errorf("lineup_ids takes 1 to %d lineups", maxPracticeSpots)
	}
	for _, id := range in.LineupIDs {
		if err := checkUUID("lineup_ids", id); err != nil {
			return nil, err
		}
	}
	c, err := s.client(ctx)
	if err != nil {
		return nil, toolError(err)
	}
	lineups := make([]tuiclient.Lineup, 0, len(in.LineupIDs))
	for _, id := range in.LineupIDs {
		l, err := c.GetLineup(ctx, id)
		if err != nil {
			return nil, toolError(err)
		}
		lineups = append(lineups, l)
	}
	return lineupSpots(in.Map, lineups)
}
