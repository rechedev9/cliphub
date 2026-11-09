package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// maxPlaylistRoundNotes is the server's cap on the notes of one playlist round.
const maxPlaylistRoundNotes = 20

type playlistStrokeIn struct {
	Color  string       `json:"color" jsonschema:"accent, ct, t or mark"`
	Width  int          `json:"width" jsonschema:"line width from 1 to 12, in radar pixels at 1024"`
	Arrow  bool         `json:"arrow,omitempty" jsonschema:"the line ends in an arrowhead"`
	Points [][2]float64 `json:"points" jsonschema:"2 to 2000 points as world x and y, the map's own coordinates"`
}

type playlistNoteIn struct {
	ID      string              `json:"id,omitempty" jsonschema:"a note get_playlist listed, to keep it; leave it out for a new note"`
	Tick    *int                `json:"tick,omitempty" jsonschema:"demo tick the note is anchored to; required for a new note"`
	Text    *string             `json:"text,omitempty" jsonschema:"the note's text"`
	Strokes *[]playlistStrokeIn `json:"strokes,omitempty" jsonschema:"lines drawn on the map at that moment; a kept note without it keeps its drawing"`
}

type updatePlaylistRoundIn struct {
	PlaylistID string            `json:"playlist_id" jsonschema:"playlist id from list_playlists"`
	Ref        string            `json:"ref" jsonschema:"match_id/round of a round of the playlist, from get_playlist's round_list"`
	Comment    *string           `json:"comment,omitempty" jsonschema:"the round's new comment; an empty string clears it"`
	Notes      *[]playlistNoteIn `json:"notes,omitempty" jsonschema:"the round's full note list: notes left out are removed. Start from get_playlist's notes"`
	Position   *int              `json:"position,omitempty" jsonschema:"move the round here: 0 is the start"`
}

// playlistNote turns one requested note into the note sent, over the one it keeps.
func playlistNote(in playlistNoteIn, held []tuiclient.PlaylistNote) (tuiclient.PlaylistNote, error) {
	var n tuiclient.PlaylistNote
	if in.ID != "" {
		i := slices.IndexFunc(held, func(h tuiclient.PlaylistNote) bool { return h.ID == in.ID })
		if i < 0 {
			return n, fmt.Errorf("note %s is not on this round; read it again with get_playlist", cleanName(in.ID))
		}
		n = held[i]
	} else if in.Tick == nil {
		return n, errors.New("a new note needs its tick")
	}
	if in.Tick != nil {
		if *in.Tick < 0 {
			return n, errors.New("tick must not be negative")
		}
		n.Tick = *in.Tick
	}
	if in.Text != nil {
		text, err := checkCallText("text", *in.Text, maxPlaylistNoteRunes, false)
		if err != nil {
			return n, err
		}
		n.Text = text
	}
	if in.Strokes != nil {
		n.Strokes = make([]json.RawMessage, 0, len(*in.Strokes))
		for _, s := range *in.Strokes {
			raw, err := json.Marshal(s)
			if err != nil {
				return n, err
			}
			n.Strokes = append(n.Strokes, raw)
		}
	}
	if n.Text == "" && len(n.Strokes) == 0 {
		return n, errors.New("a note needs text or a drawing")
	}
	return n, nil
}

func (in updatePlaylistRoundIn) edit(held []tuiclient.PlaylistNote) (tuiclient.PlaylistRoundEdit, error) {
	matchID, round, err := parseRoundRef(in.Ref)
	if err != nil {
		return tuiclient.PlaylistRoundEdit{}, err
	}
	edit := tuiclient.PlaylistRoundEdit{PlaylistID: in.PlaylistID, MatchID: matchID, Round: round, Position: in.Position}
	if in.Position != nil && *in.Position < 0 {
		return edit, errors.New("position must not be negative")
	}
	if in.Comment != nil {
		comment, err := checkCallText("comment", *in.Comment, maxPlaylistCommentRunes, false)
		if err != nil {
			return edit, err
		}
		edit.Comment = &comment
	}
	if in.Notes != nil {
		if len(*in.Notes) > maxPlaylistRoundNotes {
			return edit, fmt.Errorf("a round holds at most %d notes", maxPlaylistRoundNotes)
		}
		notes := make([]tuiclient.PlaylistNote, 0, len(*in.Notes))
		for _, n := range *in.Notes {
			note, err := playlistNote(n, held)
			if err != nil {
				return edit, err
			}
			notes = append(notes, note)
		}
		edit.Notes = &notes
	}
	return edit, nil
}

func registerPlaylistRoundTool(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "update_playlist_round", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Edit one round of a saved playlist, as its panel in the Replay does: its comment, its notes and drawings (each at a demo tick, " +
			"as the Replay's pencil leaves them; the list replaces the round's notes, so pass back the ids of the ones to keep), or its place in the playlist."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in updatePlaylistRoundIn) (*mcp.CallToolResult, playlistDetailOut, error) {
			if err := checkUUID("playlist_id", in.PlaylistID); err != nil {
				return nil, playlistDetailOut{}, err
			}
			if in.Comment == nil && in.Notes == nil && in.Position == nil {
				return nil, playlistDetailOut{}, errors.New("set comment, notes or position")
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, playlistDetailOut{}, toolError(err)
			}
			var held []tuiclient.PlaylistNote
			if in.Notes != nil {
				matchID, round, err := parseRoundRef(in.Ref)
				if err != nil {
					return nil, playlistDetailOut{}, err
				}
				p, err := c.GetPlaylist(ctx, in.PlaylistID)
				if err != nil {
					return nil, playlistDetailOut{}, toolError(err)
				}
				i := slices.IndexFunc(p.Rounds, func(r tuiclient.PlaylistRound) bool { return r.MatchID == matchID && r.Round == round })
				if i < 0 {
					return nil, playlistDetailOut{}, fmt.Errorf("round %s is not in this playlist", cleanName(in.Ref))
				}
				held = p.Rounds[i].Notes
			}
			edit, err := in.edit(held)
			if err != nil {
				return nil, playlistDetailOut{}, err
			}
			p, err := c.UpdatePlaylistRound(ctx, edit)
			if err != nil {
				return nil, playlistDetailOut{}, toolError(err)
			}
			return nil, playlistDetailOf(p), nil
		})
}
