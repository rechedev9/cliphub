package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// The server's caps on a saved playlist (internal/library/playlist.go).
const (
	maxPlaylistNameRunes    = 120
	maxPlaylistFolderRunes  = 60
	maxPlaylistCommentRunes = 1000
	maxPlaylistNoteRunes    = 2000
	defaultPlaylistList     = 50
	maxPlaylistList         = 1000
	maxPlaylistFileBytes    = 8 << 20
)

type playlistRoundIn struct {
	Ref     string  `json:"ref" jsonschema:"match_id/round, from find_rounds, replay_rounds or the reports"`
	Comment *string `json:"comment,omitempty" jsonschema:"what to look at in this round; leave it out to keep the comment of a round the playlist holds, an empty string clears it"`
}

type listPlaylistsIn struct {
	Query    string `json:"query,omitempty" jsonschema:"words of the name, author or folder"`
	Map      string `json:"map,omitempty" jsonschema:"only playlists with a round on this map, such as de_mirage"`
	Folder   string `json:"folder,omitempty" jsonschema:"only playlists in this folder"`
	NoFolder bool   `json:"no_folder,omitempty" jsonschema:"only playlists outside every folder (Sin carpeta)"`
	Favorite bool   `json:"favorite,omitempty" jsonschema:"only starred playlists (Favoritas)"`
	Limit    int    `json:"limit,omitempty" jsonschema:"default 50, at most 1000"`
}

type playlistSummaryOut struct {
	ID         string   `json:"id"`
	Name       string   `json:"name" jsonschema:"untrusted text the user wrote"`
	Folder     string   `json:"folder,omitempty" jsonschema:"untrusted text the user wrote"`
	Favorite   bool     `json:"favorite,omitempty"`
	Author     string   `json:"author,omitempty" jsonschema:"untrusted text the user wrote"`
	Maps       []string `json:"maps"`
	Rounds     int      `json:"rounds"`
	Missing    int      `json:"missing,omitempty" jsonschema:"rounds whose match is not in this library; import its demo to see them"`
	UpdatedAt  string   `json:"updated_at"`
	StudioPath string   `json:"studio_path" jsonschema:"the path that opens the playlist in the Replay, with its panel"`
}

type listPlaylistsOut struct {
	Playlists []playlistSummaryOut `json:"playlists"`
	Truncated bool                 `json:"truncated,omitempty" jsonschema:"true when more playlists match; narrow the filter or raise limit"`
}

type playlistRoundOut struct {
	Ref     string            `json:"ref" jsonschema:"match_id/round"`
	Map     string            `json:"map,omitempty"`
	Comment string            `json:"comment,omitempty" jsonschema:"untrusted text the user wrote"`
	Notes   []playlistNoteOut `json:"notes,omitempty" jsonschema:"notes and drawings left on the round in the Replay, by tick"`
	Missing bool              `json:"missing,omitempty" jsonschema:"the match is not in this library"`
}

type playlistNoteOut struct {
	ID      string `json:"id" jsonschema:"pass it to update_playlist_round to keep the note"`
	Tick    int    `json:"tick" jsonschema:"demo tick the note is anchored to"`
	Text    string `json:"text,omitempty" jsonschema:"untrusted text the user wrote"`
	Strokes int    `json:"strokes,omitempty" jsonschema:"lines drawn on the map at that moment"`
}

type playlistDetailOut struct {
	playlistSummaryOut
	RoundList []playlistRoundOut `json:"round_list" jsonschema:"the rounds in play order"`
}

type playlistIDIn struct {
	PlaylistID string `json:"playlist_id" jsonschema:"playlist id from list_playlists"`
}

type createPlaylistIn struct {
	Name     string            `json:"name"`
	Folder   string            `json:"folder,omitempty" jsonschema:"a folder to file it under, such as Rivales; new folders appear by being used"`
	Author   string            `json:"author,omitempty" jsonschema:"who made it"`
	Favorite bool              `json:"favorite,omitempty"`
	Rounds   []playlistRoundIn `json:"rounds,omitempty" jsonschema:"the rounds in play order; at most 200"`
}

type updatePlaylistIn struct {
	PlaylistID string             `json:"playlist_id" jsonschema:"playlist id from list_playlists"`
	Name       string             `json:"name,omitempty" jsonschema:"new name; empty keeps it"`
	Folder     *string            `json:"folder,omitempty" jsonschema:"new folder; an empty string takes it out of its folder"`
	Author     *string            `json:"author,omitempty"`
	Favorite   *bool              `json:"favorite,omitempty" jsonschema:"star or unstar it"`
	Rounds     *[]playlistRoundIn `json:"rounds,omitempty" jsonschema:"the full new round list in play order: it reorders and removes rounds; a round without comment keeps its own. Start from get_playlist's round_list"`
}

type addPlaylistRoundsIn struct {
	PlaylistID string            `json:"playlist_id" jsonschema:"playlist id from list_playlists"`
	Rounds     []playlistRoundIn `json:"rounds" jsonschema:"rounds to add; ones the playlist holds stay where they are"`
	Position   *int              `json:"position,omitempty" jsonschema:"where they go: 0 is the start; the end without it"`
}

type deletePlaylistIn struct {
	PlaylistID string `json:"playlist_id" jsonschema:"playlist id from list_playlists"`
	Approve    string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type exportPlaylistsIn struct {
	Path        string   `json:"path" jsonschema:"absolute path of the new .json file on a local drive; an existing file is not overwritten"`
	PlaylistIDs []string `json:"playlist_ids,omitempty" jsonschema:"the playlists to export; without them every playlist the filters keep"`
	listPlaylistsIn
}

type importPlaylistsIn struct {
	Path string `json:"path" jsonschema:"absolute path of a ClipHub playlist .json file on a local drive"`
}

type importPlaylistsOut struct {
	Imported       int                  `json:"imported"`
	Duplicates     int                  `json:"duplicates" jsonschema:"playlists already here"`
	Rejected       []string             `json:"rejected" jsonschema:"playlists that were not stored, with the reason; at most 50 listed"`
	Playlists      []playlistSummaryOut `json:"playlists" jsonschema:"the imported playlists, at most 50 listed"`
	MissingMatches []string             `json:"missing_matches" jsonschema:"matches the imported rounds point at that this library lacks: import these demos to see them"`
}

func savedPlaylistPath(id string) string { return "/replay?p=" + url.QueryEscape(id) }

// parseRoundRef reads a match_id/round ref.
func parseRoundRef(ref string) (string, int, error) {
	matchID, round, ok := strings.Cut(strings.TrimSpace(ref), "/")
	n, err := strconv.Atoi(round)
	if !ok || err != nil || n < 1 || n > maxPlayRound {
		return "", 0, fmt.Errorf("%q is not a round ref (match_id/round)", cleanName(ref))
	}
	if err := checkUUID("match_id", matchID); err != nil {
		return "", 0, err
	}
	return strings.ToLower(matchID), n, nil
}

func playlistRoundRefs(in []playlistRoundIn) ([]tuiclient.PlaylistRoundRef, error) {
	if len(in) > maxPlaylistRounds {
		return nil, fmt.Errorf("a playlist holds at most %d rounds", maxPlaylistRounds)
	}
	out := make([]tuiclient.PlaylistRoundRef, 0, len(in))
	for _, r := range in {
		matchID, round, err := parseRoundRef(r.Ref)
		if err != nil {
			return nil, err
		}
		ref := tuiclient.PlaylistRoundRef{MatchID: matchID, Round: round}
		if r.Comment != nil {
			comment, err := checkCallText("comment", *r.Comment, maxPlaylistCommentRunes, false)
			if err != nil {
				return nil, err
			}
			ref.Comment = &comment
		}
		out = append(out, ref)
	}
	return out, nil
}

func checkPlaylistName(raw string) (string, error) {
	name, err := checkCallText("name", raw, maxPlaylistNameRunes, true)
	if err == nil && strings.ContainsRune(name, '\n') {
		err = errors.New("name must be one line")
	}
	return name, err
}

// checkPlaylistLine checks a folder or an author: one line, spaces collapsed.
func checkPlaylistLine(field, raw string) (string, error) {
	return checkCallText(field, strings.Join(strings.Fields(raw), " "), maxPlaylistFolderRunes, false)
}

func (in listPlaylistsIn) query() (tuiclient.PlaylistQuery, error) {
	if in.Limit < 0 || in.Limit > maxPlaylistList {
		return tuiclient.PlaylistQuery{}, fmt.Errorf("limit must be from 1 to %d", maxPlaylistList)
	}
	if in.Map != "" && !mapNamePattern.MatchString(in.Map) {
		return tuiclient.PlaylistQuery{}, errors.New("map must be a map name such as de_mirage")
	}
	query, err := checkCallText("query", in.Query, maxPlaylistNameRunes, false)
	if err != nil {
		return tuiclient.PlaylistQuery{}, err
	}
	folder, err := checkPlaylistLine("folder", in.Folder)
	if err != nil {
		return tuiclient.PlaylistQuery{}, err
	}
	return tuiclient.PlaylistQuery{Query: query, Map: strings.ToLower(in.Map), Folder: folder, NoFolder: in.NoFolder, Favorite: in.Favorite}, nil
}

func (in createPlaylistIn) patch() (tuiclient.PlaylistPatch, error) {
	name, err := checkPlaylistName(in.Name)
	if err != nil {
		return tuiclient.PlaylistPatch{}, err
	}
	folder, err := checkPlaylistLine("folder", in.Folder)
	if err != nil {
		return tuiclient.PlaylistPatch{}, err
	}
	author, err := checkPlaylistLine("author", in.Author)
	if err != nil {
		return tuiclient.PlaylistPatch{}, err
	}
	rounds, err := playlistRoundRefs(in.Rounds)
	if err != nil {
		return tuiclient.PlaylistPatch{}, err
	}
	return tuiclient.PlaylistPatch{Name: &name, Folder: &folder, Author: &author, Favorite: &in.Favorite, Rounds: &rounds}, nil
}

func (in updatePlaylistIn) patch() (tuiclient.PlaylistPatch, error) {
	var p tuiclient.PlaylistPatch
	if err := checkUUID("playlist_id", in.PlaylistID); err != nil {
		return p, err
	}
	if in.Name == "" && in.Folder == nil && in.Author == nil && in.Favorite == nil && in.Rounds == nil {
		return p, errors.New("set name, folder, author, favorite or rounds")
	}
	if in.Name != "" {
		name, err := checkPlaylistName(in.Name)
		if err != nil {
			return p, err
		}
		p.Name = &name
	}
	for _, f := range []struct {
		field string
		raw   *string
		dst   **string
	}{{"folder", in.Folder, &p.Folder}, {"author", in.Author, &p.Author}} {
		if f.raw != nil {
			text, err := checkPlaylistLine(f.field, *f.raw)
			if err != nil {
				return p, err
			}
			*f.dst = &text
		}
	}
	p.Favorite = in.Favorite
	if in.Rounds != nil {
		rounds, err := playlistRoundRefs(*in.Rounds)
		if err != nil {
			return p, err
		}
		p.Rounds = &rounds
	}
	return p, nil
}

func playlistSummaryOf(p tuiclient.Playlist) playlistSummaryOut {
	out := playlistSummaryOut{ID: p.ID, Name: cleanName(p.Name), Folder: cleanName(p.Folder), Favorite: p.Favorite, Author: cleanName(p.Author),
		Maps: cleanNames(p.Maps), Rounds: len(p.Rounds), UpdatedAt: p.UpdatedAt.UTC().Format(time.RFC3339), StudioPath: savedPlaylistPath(p.ID)}
	for _, r := range p.Rounds {
		if r.Missing {
			out.Missing++
		}
	}
	return out
}

func playlistDetailOf(p tuiclient.Playlist) playlistDetailOut {
	out := playlistDetailOut{playlistSummaryOut: playlistSummaryOf(p), RoundList: []playlistRoundOut{}}
	for _, r := range p.Rounds {
		round := playlistRoundOut{Ref: cleanName(r.MatchID) + "/" + strconv.Itoa(r.Round), Map: cleanName(r.Map),
			Comment: cleanText(r.Comment, maxPlaylistCommentRunes), Missing: r.Missing}
		for _, n := range r.Notes {
			round.Notes = append(round.Notes, playlistNoteOut{ID: cleanName(n.ID), Tick: n.Tick, Text: cleanText(n.Text, maxPlaylistNoteRunes), Strokes: len(n.Strokes)})
		}
		out.RoundList = append(out.RoundList, round)
	}
	return out
}

func playlistMatchLine(m tuiclient.PlaylistMatch) string {
	parts := []string{cleanName(m.Map)}
	if len(m.Teams) > 0 {
		parts = append(parts, strings.Join(cleanNames(m.Teams), " vs "))
	}
	if m.PlayedAt != nil {
		parts = append(parts, m.PlayedAt.UTC().Format(time.DateOnly))
	}
	if m.FileName != "" {
		parts = append(parts, "demo "+quotedName(m.FileName, ""))
	}
	return strings.Join(parts, ", ")
}

func registerPlaylistTools(server *mcp.Server, s *studio) {
	registerPlaylistRoundTool(server, s)
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	detail := func(ctx context.Context, call func(*tuiclient.Client) (tuiclient.Playlist, error)) (*mcp.CallToolResult, playlistDetailOut, error) {
		c, err := s.client(ctx)
		if err != nil {
			return nil, playlistDetailOut{}, toolError(err)
		}
		p, err := call(c)
		if err != nil {
			return nil, playlistDetailOut{}, toolError(err)
		}
		return nil, playlistDetailOf(p), nil
	}

	mcp.AddTool(server, &mcp.Tool{Name: "list_playlists", Annotations: readOnly,
		Description: "List the saved round playlists (Replay > Playlists) by most recent edit, filtered by words, map, folder or star. " +
			"studio_path opens one in the Replay."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listPlaylistsIn) (*mcp.CallToolResult, listPlaylistsOut, error) {
			q, err := in.query()
			if err != nil {
				return nil, listPlaylistsOut{}, err
			}
			limit := cmp.Or(in.Limit, defaultPlaylistList)
			q.Limit = min(limit+1, maxPlaylistList)
			c, err := s.client(ctx)
			if err != nil {
				return nil, listPlaylistsOut{}, toolError(err)
			}
			list, err := c.ListPlaylists(ctx, q)
			if err != nil {
				return nil, listPlaylistsOut{}, toolError(err)
			}
			out := listPlaylistsOut{Playlists: []playlistSummaryOut{}, Truncated: len(list) > limit}
			for _, p := range list[:min(len(list), limit)] {
				out.Playlists = append(out.Playlists, playlistSummaryOf(p))
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_playlist", Annotations: readOnly,
		Description: "Read one saved playlist with its rounds in play order, the comment of each and the notes and drawings the coach left on it in the Replay (by tick), as its panel shows them."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in playlistIDIn) (*mcp.CallToolResult, playlistDetailOut, error) {
			if err := checkUUID("playlist_id", in.PlaylistID); err != nil {
				return nil, playlistDetailOut{}, err
			}
			return detail(ctx, func(c *tuiclient.Client) (tuiclient.Playlist, error) { return c.GetPlaylist(ctx, in.PlaylistID) })
		})

	mcp.AddTool(server, &mcp.Tool{Name: "create_playlist", Annotations: &mcp.ToolAnnotations{},
		Description: "Save a new playlist of rounds from any matches, as Nueva playlist does: a name, an optional folder and author, and round refs in play order, " +
			"each with an optional comment. It stays in Replay > Playlists; studio_path opens it."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in createPlaylistIn) (*mcp.CallToolResult, playlistDetailOut, error) {
			patch, err := in.patch()
			if err != nil {
				return nil, playlistDetailOut{}, err
			}
			return detail(ctx, func(c *tuiclient.Client) (tuiclient.Playlist, error) { return c.CreatePlaylist(ctx, patch) })
		})

	mcp.AddTool(server, &mcp.Tool{Name: "update_playlist", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Rename a saved playlist, move it to a folder, star it, or replace its rounds: the full list reorders and removes them, " +
			"as the playlist's panel in the Replay does. A round it already holds keeps its notes and drawings, and its comment unless the round gives one. " +
			"To edit one round's comment, notes or place use update_playlist_round. If the playlist changed meanwhile it fails: read it again with get_playlist."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in updatePlaylistIn) (*mcp.CallToolResult, playlistDetailOut, error) {
			patch, err := in.patch()
			if err != nil {
				return nil, playlistDetailOut{}, err
			}
			return detail(ctx, func(c *tuiclient.Client) (tuiclient.Playlist, error) {
				return c.UpdatePlaylist(ctx, in.PlaylistID, patch)
			})
		})

	mcp.AddTool(server, &mcp.Tool{Name: "add_playlist_rounds", Annotations: &mcp.ToolAnnotations{},
		Description: "Add rounds to a saved playlist, as Añadir a playlist (B) does in the Replay. Rounds it already holds keep their place and comment."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in addPlaylistRoundsIn) (*mcp.CallToolResult, playlistDetailOut, error) {
			if err := checkUUID("playlist_id", in.PlaylistID); err != nil {
				return nil, playlistDetailOut{}, err
			}
			if len(in.Rounds) == 0 {
				return nil, playlistDetailOut{}, errors.New("rounds must name at least one round")
			}
			if in.Position != nil && *in.Position < 0 {
				return nil, playlistDetailOut{}, errors.New("position must not be negative")
			}
			rounds, err := playlistRoundRefs(in.Rounds)
			if err != nil {
				return nil, playlistDetailOut{}, err
			}
			add := tuiclient.PlaylistAdd{Rounds: rounds, Position: in.Position}
			return detail(ctx, func(c *tuiclient.Client) (tuiclient.Playlist, error) {
				return c.AddPlaylistRounds(ctx, in.PlaylistID, add)
			})
		})

	mcp.AddTool(server, &mcp.Tool{Name: "delete_playlist", Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true)},
		Description: "Delete a saved playlist. Its matches and rounds stay in the library. It cannot be undone, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in deletePlaylistIn) (*mcp.CallToolResult, deleteOut, error) {
			if err := checkUUID("playlist_id", in.PlaylistID); err != nil {
				return nil, deleteOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			p, err := c.GetPlaylist(ctx, in.PlaylistID)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			summary := fmt.Sprintf("Borrar la playlist %s (%d rondas). Las partidas siguen en la biblioteca. No se puede deshacer.", quotedName(p.Name, "sin nombre"), len(p.Rounds))
			return approveAndDelete(req, approvalSpec{Tool: "delete_playlist", JobID: in.PlaylistID, Request: p.UpdatedAt, Summary: summary, Given: in.Approve, Question: "¿Borrar ahora?"},
				func() error { return c.DeletePlaylist(ctx, in.PlaylistID) })
		})

	mcp.AddTool(server, &mcp.Tool{Name: "export_playlists", Annotations: &mcp.ToolAnnotations{},
		Description: "Export saved playlists to a new .json file on this PC, as Exportar al equipo does, to send to teammates. " +
			"The file names each match's demo, so import_playlists on another PC finds the rounds once the same demos are imported."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in exportPlaylistsIn) (*mcp.CallToolResult, exportPlaysOut, error) {
			if err := checkNewLocalFile(in.Path, ".json"); err != nil {
				return nil, exportPlaysOut{}, err
			}
			if len(in.PlaylistIDs) > maxPlaylistList {
				return nil, exportPlaysOut{}, fmt.Errorf("export at most %d playlists", maxPlaylistList)
			}
			for _, id := range in.PlaylistIDs {
				if err := checkUUID("playlist_ids", id); err != nil {
					return nil, exportPlaysOut{}, err
				}
			}
			q, err := in.query()
			if err != nil {
				return nil, exportPlaysOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, exportPlaysOut{}, toolError(err)
			}
			file, err := c.ExportPlaylists(ctx, in.PlaylistIDs, q)
			if err != nil {
				return nil, exportPlaysOut{}, toolError(err)
			}
			if err := writeNewFile(in.Path, file); err != nil {
				return nil, exportPlaysOut{}, err
			}
			return nil, exportPlaysOut{Path: in.Path, Bytes: len(file)}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "import_playlists", Annotations: &mcp.ToolAnnotations{},
		Description: "Import a ClipHub playlist .json file, as Importar does in Replay > Playlists. Each playlist is stored on its own: ones already here count as duplicates. " +
			"Rounds whose demo this library lacks are kept and listed in missing_matches; they appear once that demo is imported."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in importPlaylistsIn) (*mcp.CallToolResult, importPlaylistsOut, error) {
			if !strings.EqualFold(filepath.Ext(in.Path), ".json") {
				return nil, importPlaylistsOut{}, errors.New("path must be a .json playlist file")
			}
			if err := checkLocalFile(in.Path, "playlist", maxPlaylistFileBytes); err != nil {
				return nil, importPlaylistsOut{}, err
			}
			file, err := readJSONFile(in.Path)
			if err != nil {
				return nil, importPlaylistsOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, importPlaylistsOut{}, toolError(err)
			}
			res, err := c.ImportPlaylists(ctx, file)
			if err != nil {
				return nil, importPlaylistsOut{}, toolError(err)
			}
			out := importPlaylistsOut{Imported: res.Imported, Duplicates: res.Duplicates, Rejected: []string{}, Playlists: []playlistSummaryOut{}, MissingMatches: []string{}}
			for _, r := range res.Rejected[:min(len(res.Rejected), maxImportListed)] {
				out.Rejected = append(out.Rejected, fmt.Sprintf("%d %s: %s", r.Index, quotedName(r.Name, "sin nombre"), cleanText(r.Error, maxErrorRunes)))
			}
			for _, p := range res.Playlists[:min(len(res.Playlists), maxImportListed)] {
				out.Playlists = append(out.Playlists, playlistSummaryOf(p))
			}
			for _, m := range res.MissingMatches[:min(len(res.MissingMatches), maxImportListed)] {
				out.MissingMatches = append(out.MissingMatches, playlistMatchLine(m))
			}
			return nil, out, nil
		})
}
