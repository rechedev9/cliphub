package tuiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Call is one word of the team's vocabulary and what it means.
type Call struct {
	ID        string    `json:"id,omitempty"`
	Name      string    `json:"name"`
	Side      string    `json:"side"`
	Meaning   string    `json:"meaning"`
	Status    string    `json:"status,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`
}

// CallQuery filters the Libreta's calls; zero fields are unset.
type CallQuery struct {
	Side            string
	Status          string
	Tag             string
	IncludeArchived bool
	Limit           int
	Offset          int
}

// CallPatch edits a call; nil fields stay.
type CallPatch struct {
	Name    *string   `json:"name,omitempty"`
	Side    *string   `json:"side,omitempty"`
	Meaning *string   `json:"meaning,omitempty"`
	Status  *string   `json:"status,omitempty"`
	Tags    *[]string `json:"tags,omitempty"`
}

// MapCallout is a CS2 place of a map with the team's own name, if any.
type MapCallout struct {
	Token string `json:"token"`
	Name  string `json:"name,omitempty"`
}

func (q CallQuery) encode() string {
	params := map[string]string{"side": q.Side, "status": q.Status, "tag": q.Tag}
	if q.IncludeArchived {
		params["include_archived"] = "true"
	}
	if q.Limit > 0 {
		params["limit"] = strconv.Itoa(q.Limit)
	}
	if q.Offset > 0 {
		params["offset"] = strconv.Itoa(q.Offset)
	}
	return query(params)
}

func callPath(id string) string { return "/api/library/calls/" + url.PathEscape(id) }

// ListCalls lists the Libreta's calls by name.
func (c *Client) ListCalls(ctx context.Context, q CallQuery) ([]Call, error) {
	var out struct {
		Calls []Call `json:"calls"`
	}
	err := c.getJSON(ctx, "/api/library/calls"+q.encode(), &out)
	return out.Calls, err
}

// GetCall returns one call.
func (c *Client) GetCall(ctx context.Context, id string) (Call, error) {
	var out Call
	err := c.getJSON(ctx, callPath(id), &out)
	return out, err
}

// CreateCall stores a call.
func (c *Client) CreateCall(ctx context.Context, call Call) (Call, error) {
	var out Call
	err := c.doJSON(ctx, http.MethodPost, "/api/library/calls", CallPatch{Name: &call.Name, Side: &call.Side, Meaning: &call.Meaning,
		Status: &call.Status, Tags: &call.Tags}, &out)
	return out, err
}

// UpdateCall edits a call.
func (c *Client) UpdateCall(ctx context.Context, id string, patch CallPatch) (Call, error) {
	var out Call
	err := c.doJSON(ctx, http.MethodPatch, callPath(id), patch, &out)
	return out, err
}

// DeleteCall removes a call from the Libreta.
func (c *Client) DeleteCall(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, callPath(id), nil, nil)
}

func calloutPath(mapName, token string) string {
	return "/api/library/maps/" + url.PathEscape(mapName) + "/callouts/" + url.PathEscape(token)
}

// ListMapCallouts lists a map's CS2 places with the team's names.
func (c *Client) ListMapCallouts(ctx context.Context, mapName string) ([]MapCallout, error) {
	var out struct {
		Callouts []MapCallout `json:"callouts"`
	}
	err := c.getJSON(ctx, "/api/library/maps/"+url.PathEscape(mapName)+"/callouts", &out)
	return out.Callouts, err
}

// SetCalloutName gives a place of a map the team's own name.
func (c *Client) SetCalloutName(ctx context.Context, mapName, token, name string) error {
	return c.doJSON(ctx, http.MethodPut, calloutPath(mapName, token), map[string]string{"name": name}, nil)
}

// DeleteCalloutName gives a place back its CS2 name.
func (c *Client) DeleteCalloutName(ctx context.Context, mapName, token string) error {
	return c.doJSON(ctx, http.MethodDelete, calloutPath(mapName, token), nil, nil)
}

// ExportLibreta reads the whole Libreta as a file.
func (c *Client) ExportLibreta(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.getJSON(ctx, "/api/library/libreta/export", &out)
	return out, err
}
