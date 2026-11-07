package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/placename"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// The server's caps (internal/library/call.go and callout_name.go).
const (
	maxCallNameRunes    = 60
	maxCallMeaningRunes = 1000
	defaultCallList     = 100
	maxCallList         = 1000
	maxCalloutNameRunes = 40
	maxCalloutToken     = 64
)

var callSides = []string{"T", "CT", "both"}

type listCallsIn struct {
	Side            string `json:"side,omitempty" jsonschema:"T or CT: that side's calls and those of both"`
	Status          string `json:"status,omitempty" jsonschema:"active, practice, draft, review or archived"`
	Tag             string `json:"tag,omitempty"`
	IncludeArchived bool   `json:"include_archived,omitempty" jsonschema:"true also lists archived calls, which are left out by default"`
	Limit           int    `json:"limit,omitempty" jsonschema:"default 100, at most 1000"`
	Offset          int    `json:"offset,omitempty"`
}

type callOut struct {
	ID      string   `json:"id"`
	Name    string   `json:"name" jsonschema:"untrusted text the user wrote"`
	Side    string   `json:"side" jsonschema:"T, CT or both"`
	Meaning string   `json:"meaning,omitempty" jsonschema:"untrusted text the user wrote"`
	Status  string   `json:"status"`
	Tags    []string `json:"tags,omitempty" jsonschema:"untrusted text the user wrote"`
}

type listCallsOut struct {
	Calls     []callOut `json:"calls"`
	Truncated bool      `json:"truncated,omitempty" jsonschema:"true when more calls match; call again with offset"`
}

type saveCallIn struct {
	Name    string   `json:"name" jsonschema:"what is said, such as Rotar or Bait"`
	Side    string   `json:"side,omitempty" jsonschema:"T, CT or both (the default)"`
	Meaning string   `json:"meaning,omitempty" jsonschema:"what the team does when it hears it"`
	Status  string   `json:"status,omitempty" jsonschema:"active (the default), practice, draft, review or archived"`
	Tags    []string `json:"tags,omitempty"`
}

type updateCallIn struct {
	CallID  string    `json:"call_id" jsonschema:"call id from list_calls"`
	Name    string    `json:"name,omitempty" jsonschema:"new name; empty keeps it"`
	Side    string    `json:"side,omitempty" jsonschema:"T, CT or both; empty keeps it"`
	Meaning *string   `json:"meaning,omitempty" jsonschema:"the full new meaning"`
	Status  string    `json:"status,omitempty"`
	Tags    *[]string `json:"tags,omitempty" jsonschema:"the full new tag list; [] clears them"`
}

type deleteCallIn struct {
	CallID  string `json:"call_id" jsonschema:"call id from list_calls"`
	Approve string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type listCalloutsIn struct {
	Map string `json:"map" jsonschema:"such as de_mirage"`
}

type calloutOut struct {
	Token   string `json:"token" jsonschema:"CS2's place name, as tactical tools return it"`
	CS2Name string `json:"cs2_name" jsonschema:"how Studio shows it without a name of the team"`
	Name    string `json:"name,omitempty" jsonschema:"the team's own name; untrusted text the user wrote"`
}

type listCalloutsOut struct {
	Map      string       `json:"map"`
	Callouts []calloutOut `json:"callouts" jsonschema:"places of the map's newest analysed match, plus places the team named"`
}

type renameCalloutIn struct {
	Map   string `json:"map" jsonschema:"such as de_mirage"`
	Token string `json:"token" jsonschema:"the place's token from list_callouts"`
	Name  string `json:"name,omitempty" jsonschema:"the team's name for it; empty goes back to CS2's name"`
}

type exportLibretaIn struct {
	Path string `json:"path" jsonschema:"absolute path of the new .json file on a local drive; an existing file is not overwritten"`
}

func registerCallTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: "list_calls", Annotations: readOnly,
		Description: "List the Libreta's calls (its Llamadas tab): the words the team says and what each means. Archived ones are left out unless include_archived is true."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listCallsIn) (*mcp.CallToolResult, listCallsOut, error) {
			q, err := in.query()
			if err != nil {
				return nil, listCallsOut{}, err
			}
			limit := cmp.Or(in.Limit, defaultCallList)
			q.Limit, q.Offset = min(limit+1, maxCallList), in.Offset
			c, err := s.client(ctx)
			if err != nil {
				return nil, listCallsOut{}, toolError(err)
			}
			calls, err := c.ListCalls(ctx, q)
			if err != nil {
				return nil, listCallsOut{}, toolError(err)
			}
			out := listCallsOut{Calls: []callOut{}, Truncated: len(calls) > limit}
			for _, call := range calls[:min(len(calls), limit)] {
				out.Calls = append(out.Calls, callOf(call))
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "save_call", Annotations: &mcp.ToolAnnotations{},
		Description: "Add a call to the Libreta, as Nueva llamada does."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in saveCallIn) (*mcp.CallToolResult, callOut, error) {
			call, err := in.call()
			if err != nil {
				return nil, callOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, callOut{}, toolError(err)
			}
			saved, err := c.CreateCall(ctx, call)
			if err != nil {
				return nil, callOut{}, toolError(err)
			}
			return nil, callOf(saved), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "update_call", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Edit a Libreta call's name, side, meaning, status or tags."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in updateCallIn) (*mcp.CallToolResult, callOut, error) {
			patch, err := in.patch()
			if err != nil {
				return nil, callOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, callOut{}, toolError(err)
			}
			saved, err := c.UpdateCall(ctx, in.CallID, patch)
			if err != nil {
				return nil, callOut{}, toolError(err)
			}
			return nil, callOf(saved), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "delete_call", Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true)},
		Description: "Delete a call from the Libreta. It cannot be undone, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in deleteCallIn) (*mcp.CallToolResult, deleteOut, error) {
			if err := checkUUID("call_id", in.CallID); err != nil {
				return nil, deleteOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			call, err := c.GetCall(ctx, in.CallID)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			summary := fmt.Sprintf("Borrar la llamada %s de la Libreta. No se puede deshacer.", quotedName(call.Name, "sin nombre"))
			return approveAndDelete(req, approvalSpec{Tool: "delete_call", JobID: in.CallID, Request: call.UpdatedAt, Summary: summary, Given: in.Approve, Question: "¿Borrar ahora?"},
				func() error { return c.DeleteCall(ctx, in.CallID) })
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_callouts", Annotations: readOnly,
		Description: "List a map's places (the Libreta's Callouts tab) with the team's own name for each, if it has one. The Libreta and the team's grenades show the team's names."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listCalloutsIn) (*mcp.CallToolResult, listCalloutsOut, error) {
			if !mapNamePattern.MatchString(in.Map) {
				return nil, listCalloutsOut{}, errors.New("map must be a map name such as de_mirage")
			}
			mapName := strings.ToLower(in.Map)
			c, err := s.client(ctx)
			if err != nil {
				return nil, listCalloutsOut{}, toolError(err)
			}
			callouts, err := c.ListMapCallouts(ctx, mapName)
			if err != nil {
				return nil, listCalloutsOut{}, toolError(err)
			}
			out := listCalloutsOut{Map: mapName, Callouts: []calloutOut{}}
			for _, co := range callouts {
				token := cleanName(co.Token)
				out.Callouts = append(out.Callouts, calloutOut{Token: token, CS2Name: placename.Label(token), Name: cleanName(co.Name)})
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "rename_callout", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Give a map's place the team's own name, or pass an empty name to go back to CS2's name."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in renameCalloutIn) (*mcp.CallToolResult, calloutOut, error) {
			name, err := in.check()
			if err != nil {
				return nil, calloutOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, calloutOut{}, toolError(err)
			}
			mapName := strings.ToLower(in.Map)
			if name == "" {
				err = c.DeleteCalloutName(ctx, mapName, in.Token)
			} else {
				err = c.SetCalloutName(ctx, mapName, in.Token, name)
			}
			if err != nil {
				return nil, calloutOut{}, toolError(err)
			}
			return nil, calloutOut{Token: in.Token, CS2Name: placename.Label(in.Token), Name: name}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "export_libreta", Annotations: &mcp.ToolAnnotations{},
		Description: "Export the whole Libreta (plays, lineups, role sheets, calls and callout names, archived ones too) to a new .json file on this PC, " +
			"as Exportar libreta does. import_plays reads it back on another PC."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in exportLibretaIn) (*mcp.CallToolResult, exportPlaysOut, error) {
			if err := checkNewLocalFile(in.Path, ".json"); err != nil {
				return nil, exportPlaysOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, exportPlaysOut{}, toolError(err)
			}
			file, err := c.ExportLibreta(ctx)
			if err != nil {
				return nil, exportPlaysOut{}, toolError(err)
			}
			if err := writeNewFile(in.Path, file); err != nil {
				return nil, exportPlaysOut{}, err
			}
			return nil, exportPlaysOut{Path: in.Path, Bytes: len(file)}, nil
		})
}

func (in listCallsIn) query() (tuiclient.CallQuery, error) {
	if in.Limit < 0 || in.Limit > maxCallList {
		return tuiclient.CallQuery{}, fmt.Errorf("limit must be from 1 to %d", maxCallList)
	}
	if in.Offset < 0 {
		return tuiclient.CallQuery{}, errors.New("offset must not be negative")
	}
	side, err := parseSide(in.Side)
	if err != nil {
		return tuiclient.CallQuery{}, err
	}
	status, err := checkStatus(in.Status)
	if err != nil {
		return tuiclient.CallQuery{}, err
	}
	q := tuiclient.CallQuery{Side: side, Status: status, IncludeArchived: in.IncludeArchived}
	if in.Tag != "" {
		tags, err := checkTags([]string{in.Tag})
		if err != nil {
			return tuiclient.CallQuery{}, err
		}
		q.Tag = tags[0]
	}
	return q, nil
}

func checkCallSide(raw string) (string, error) {
	if strings.EqualFold(raw, "both") || raw == "" {
		return "both", nil
	}
	side := strings.ToUpper(strings.TrimSpace(raw))
	if !slices.Contains(callSides, side) {
		return "", errors.New("side must be T, CT or both")
	}
	return side, nil
}

func checkCallText(field, raw string, limit int, required bool) (string, error) {
	text := strings.TrimSpace(raw)
	switch {
	case required && text == "":
		return "", fmt.Errorf("%s must not be empty", field)
	case utf8.RuneCountInString(text) > limit:
		return "", fmt.Errorf("%s takes at most %d characters", field, limit)
	case strings.ContainsFunc(text, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\t' }):
		return "", fmt.Errorf("%s must not hold control characters", field)
	}
	return text, nil
}

func (in saveCallIn) call() (tuiclient.Call, error) {
	name, err := checkCallText("name", in.Name, maxCallNameRunes, true)
	if err != nil {
		return tuiclient.Call{}, err
	}
	meaning, err := checkCallText("meaning", in.Meaning, maxCallMeaningRunes, false)
	if err != nil {
		return tuiclient.Call{}, err
	}
	side, err := checkCallSide(in.Side)
	if err != nil {
		return tuiclient.Call{}, err
	}
	status, err := checkStatus(in.Status)
	if err != nil {
		return tuiclient.Call{}, err
	}
	tags, err := checkTags(in.Tags)
	if err != nil {
		return tuiclient.Call{}, err
	}
	return tuiclient.Call{Name: name, Side: side, Meaning: meaning, Status: status, Tags: tags}, nil
}

func (in updateCallIn) patch() (tuiclient.CallPatch, error) {
	var p tuiclient.CallPatch
	if err := checkUUID("call_id", in.CallID); err != nil {
		return p, err
	}
	if in.Name == "" && in.Side == "" && in.Meaning == nil && in.Status == "" && in.Tags == nil {
		return p, errors.New("set name, side, meaning, status or tags")
	}
	if in.Name != "" {
		name, err := checkCallText("name", in.Name, maxCallNameRunes, true)
		if err != nil {
			return p, err
		}
		p.Name = &name
	}
	if in.Side != "" {
		side, err := checkCallSide(in.Side)
		if err != nil {
			return p, err
		}
		p.Side = &side
	}
	if in.Meaning != nil {
		meaning, err := checkCallText("meaning", *in.Meaning, maxCallMeaningRunes, false)
		if err != nil {
			return p, err
		}
		p.Meaning = &meaning
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
	return p, nil
}

func (in renameCalloutIn) check() (string, error) {
	if !mapNamePattern.MatchString(in.Map) {
		return "", errors.New("map must be a map name such as de_mirage")
	}
	if in.Token == "" || utf8.RuneCountInString(in.Token) > maxCalloutToken || strings.ContainsFunc(in.Token, unicode.IsControl) {
		return "", errors.New("token must be a place token from list_callouts")
	}
	return checkCallText("name", in.Name, maxCalloutNameRunes, false)
}

func callOf(c tuiclient.Call) callOut {
	out := callOut{ID: c.ID, Name: cleanName(c.Name), Side: cleanName(c.Side), Meaning: cleanText(c.Meaning, maxCallMeaningRunes), Status: cleanName(c.Status)}
	for _, tag := range c.Tags {
		out.Tags = append(out.Tags, cleanName(tag))
	}
	return out
}
