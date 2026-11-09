package tuiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// PlaylistRound is one round of a saved playlist. Missing marks a round whose
// match is not in this library; it keeps its map and demo hash.
type PlaylistRound struct {
	MatchID    string         `json:"match_id"`
	Round      int            `json:"round"`
	Map        string         `json:"map,omitempty"`
	DemoSHA256 string         `json:"demo_sha256,omitempty"`
	Comment    string         `json:"comment"`
	Notes      []PlaylistNote `json:"notes,omitempty"`
	Missing    bool           `json:"missing,omitempty"`
}

// PlaylistNote is a note and drawing a coach left on a playlist round, at a demo tick.
type PlaylistNote struct {
	ID      string            `json:"id,omitempty"`
	Tick    int               `json:"tick"`
	Text    string            `json:"text"`
	Strokes []json.RawMessage `json:"strokes"`
}

// Playlist is a saved, ordered list of rounds to watch in the Replay.
type Playlist struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Folder    string          `json:"folder"`
	Favorite  bool            `json:"favorite"`
	Author    string          `json:"author"`
	Maps      []string        `json:"maps"`
	Rounds    []PlaylistRound `json:"rounds"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// PlaylistRoundRef names a round to put in a playlist; the server reads its map
// and demo. A nil Comment keeps the comment of a round the playlist holds.
type PlaylistRoundRef struct {
	MatchID string  `json:"match_id"`
	Round   int     `json:"round"`
	Comment *string `json:"comment,omitempty"`
}

// PlaylistRoundEdit edits one round of a playlist; nil fields are left as they
// are, and Notes replaces the round's whole list.
type PlaylistRoundEdit struct {
	PlaylistID string          `json:"-"`
	MatchID    string          `json:"-"`
	Round      int             `json:"-"`
	Comment    *string         `json:"comment,omitempty"`
	Notes      *[]PlaylistNote `json:"notes,omitempty"`
	Position   *int            `json:"position,omitempty"`
}

// UpdatePlaylistRound edits one round's comment, notes and drawings, or place.
func (c *Client) UpdatePlaylistRound(ctx context.Context, edit PlaylistRoundEdit) (Playlist, error) {
	var out Playlist
	path := playlistPath(edit.PlaylistID) + "/rounds/" + url.PathEscape(edit.MatchID) + "/" + strconv.Itoa(edit.Round)
	err := c.doJSON(ctx, http.MethodPatch, path, edit, &out)
	return out, err
}

// PlaylistQuery filters the playlists; zero fields are unset and NoFolder wins over Folder.
type PlaylistQuery struct {
	Query    string
	Map      string
	Folder   string
	NoFolder bool
	Favorite bool
	Limit    int
}

func (q PlaylistQuery) values() url.Values {
	v := url.Values{}
	for k, s := range map[string]string{"q": q.Query, "map": q.Map, "folder": q.Folder} {
		if s != "" {
			v.Set(k, s)
		}
	}
	if q.NoFolder {
		v.Set("no_folder", "true")
	}
	if q.Favorite {
		v.Set("favorite", "true")
	}
	if q.Limit > 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	return v
}

func playlistPath(id string) string { return "/api/library/playlists/" + url.PathEscape(id) }

// ListPlaylists lists the saved playlists by most recent edit.
func (c *Client) ListPlaylists(ctx context.Context, q PlaylistQuery) ([]Playlist, error) {
	var out struct {
		Playlists []Playlist `json:"playlists"`
	}
	err := c.getJSON(ctx, "/api/library/playlists"+encodeValues(q.values()), &out)
	return out.Playlists, err
}

// GetPlaylist returns one playlist with its rounds in order.
func (c *Client) GetPlaylist(ctx context.Context, id string) (Playlist, error) {
	var out Playlist
	err := c.getJSON(ctx, playlistPath(id), &out)
	return out, err
}

// PlaylistPatch is a playlist as sent: nil fields are not set, and Rounds replaces the list.
type PlaylistPatch struct {
	Name     *string             `json:"name,omitempty"`
	Folder   *string             `json:"folder,omitempty"`
	Author   *string             `json:"author,omitempty"`
	Favorite *bool               `json:"favorite,omitempty"`
	Rounds   *[]PlaylistRoundRef `json:"rounds,omitempty"`
}

// CreatePlaylist stores a new playlist.
func (c *Client) CreatePlaylist(ctx context.Context, p PlaylistPatch) (Playlist, error) {
	var out Playlist
	err := c.doJSON(ctx, http.MethodPost, "/api/library/playlists", p, &out)
	return out, err
}

// UpdatePlaylist edits a playlist's name, folder, author, star or rounds.
func (c *Client) UpdatePlaylist(ctx context.Context, id string, patch PlaylistPatch) (Playlist, error) {
	var out Playlist
	err := c.doJSON(ctx, http.MethodPatch, playlistPath(id), patch, &out)
	return out, err
}

// PlaylistAdd adds rounds at Position (0 is the start, nil the end).
type PlaylistAdd struct {
	Rounds   []PlaylistRoundRef `json:"rounds"`
	Position *int               `json:"position,omitempty"`
}

// AddPlaylistRounds adds rounds to a playlist; rounds it holds stay where they are.
func (c *Client) AddPlaylistRounds(ctx context.Context, id string, add PlaylistAdd) (Playlist, error) {
	var out Playlist
	err := c.doJSON(ctx, http.MethodPost, playlistPath(id)+"/rounds", add, &out)
	return out, err
}

// DeletePlaylist removes a playlist.
func (c *Client) DeletePlaylist(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, playlistPath(id), nil, nil)
}

// ExportPlaylists returns the playlist file of the given ids, or of the query without ids.
func (c *Client) ExportPlaylists(ctx context.Context, ids []string, q PlaylistQuery) (json.RawMessage, error) {
	v := url.Values{"id": ids}
	if len(ids) == 0 {
		q.Limit = 0
		v = q.values()
	}
	var out json.RawMessage
	err := c.getJSON(ctx, "/api/library/playlists/export"+encodeValues(v), &out)
	return out, err
}

// PlaylistMatch names a match of a playlist file so a library without it knows which demo to import.
type PlaylistMatch struct {
	MatchID    string     `json:"match_id"`
	DemoSHA256 string     `json:"demo_sha256,omitempty"`
	Map        string     `json:"map"`
	PlayedAt   *time.Time `json:"played_at,omitempty"`
	Teams      []string   `json:"teams"`
	FileName   string     `json:"file_name,omitempty"`
}

// PlaylistImport is the outcome of importing a playlist file.
type PlaylistImport struct {
	Imported   int `json:"imported"`
	Duplicates int `json:"duplicates"`
	Rejected   []struct {
		Index int    `json:"index"`
		Name  string `json:"name"`
		Error string `json:"error"`
	} `json:"rejected"`
	Playlists      []Playlist      `json:"playlists"`
	MissingMatches []PlaylistMatch `json:"missing_matches"`
}

// ImportPlaylists stores the playlists of a playlist file, each on its own.
func (c *Client) ImportPlaylists(ctx context.Context, file json.RawMessage) (PlaylistImport, error) {
	var out PlaylistImport
	err := c.doJSON(ctx, http.MethodPost, "/api/library/playlists/import", file, &out)
	return out, err
}
