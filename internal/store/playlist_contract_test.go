package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// TestPlaylistRepositoryContract runs the saved playlists against both repositories.
func TestPlaylistRepositoryContract(t *testing.T) {
	ct, tside := side{clan: "FUT", score: 13}, side{clan: "OG", score: 8}
	cases := []struct {
		name string
		run  func(t *testing.T, f libraryFixture)
	}{
		{
			name: "playlists keep their rounds and comments in order, filter, and a star keeps the list order",
			run: func(t *testing.T, f libraryFixture) {
				mirage := f.analyze(f.demo("mirage", 0), "de_mirage", ct, tside)
				nuke := f.analyze(f.demo("nuke", 1), "de_nuke", ct, tside)
				rivals := &library.Playlist{Name: "FUT T", Folder: "Rivales", Author: "mopoz", Rounds: []library.PlaylistRound{
					{MatchID: mirage.ID, Round: 3, Map: "de_mirage", Comment: "humo tarde"},
					{MatchID: nuke.ID, Round: 5, Map: "de_nuke"},
				}}
				if err := f.repo.CreatePlaylist(f.ctx, rivals); err != nil {
					t.Fatal(err)
				}
				if len(rivals.Rounds) != 2 || rivals.Rounds[0].Comment != "humo tarde" || rivals.Rounds[1].Round != 5 || rivals.Rounds[0].Missing ||
					len(rivals.Maps) != 2 || rivals.Maps[0] != "de_mirage" || rivals.CreatedAt.IsZero() {
					t.Fatalf("created playlist = %+v", rivals)
				}
				again := *rivals
				if err := f.repo.CreatePlaylist(f.ctx, &again); !errors.Is(err, library.ErrConflict) {
					t.Fatalf("CreatePlaylist(same id) = %v, want ErrConflict", err)
				}
				time.Sleep(2 * time.Millisecond)
				pistols := &library.Playlist{Name: "Pistolas", Rounds: []library.PlaylistRound{{MatchID: mirage.ID, Round: 1, Map: "de_mirage"}}}
				if err := f.repo.CreatePlaylist(f.ctx, pistols); err != nil {
					t.Fatal(err)
				}
				list, err := f.repo.ListPlaylists(f.ctx, library.PlaylistFilter{})
				if err != nil || len(list) != 2 || list[0].ID != pistols.ID || len(list[1].Rounds) != 2 {
					t.Fatalf("ListPlaylists = %+v, %v; want newest first with rounds", list, err)
				}
				for _, tc := range []struct {
					filter library.PlaylistFilter
					want   int
				}{
					{library.PlaylistFilter{Query: "MOPOZ"}, 1}, {library.PlaylistFilter{Query: "pistolas"}, 1}, {library.PlaylistFilter{Query: "z\nr"}, 0},
					{library.PlaylistFilter{Folder: "rivales"}, 1}, {library.PlaylistFilter{NoFolder: true, Folder: "Rivales"}, 1},
					{library.PlaylistFilter{Map: "de_nuke"}, 1}, {library.PlaylistFilter{Map: "de_mirage"}, 2}, {library.PlaylistFilter{Favorite: true}, 0},
					{library.PlaylistFilter{Limit: 1, Offset: 1}, 1},
				} {
					if got, err := f.repo.ListPlaylists(f.ctx, tc.filter); err != nil || len(got) != tc.want {
						t.Fatalf("ListPlaylists(%+v) = %d, %v; want %d", tc.filter, len(got), err, tc.want)
					}
				}
				star := true
				starred, err := f.repo.UpdatePlaylist(f.ctx, rivals.ID, library.PlaylistPatch{Favorite: &star})
				if err != nil || !starred.Favorite || !starred.UpdatedAt.Equal(rivals.UpdatedAt) {
					t.Fatalf("star = %+v, %v; want favourite and the same updated_at", starred, err)
				}
				if got, err := f.repo.ListPlaylists(f.ctx, library.PlaylistFilter{Favorite: true}); err != nil || len(got) != 1 || got[0].ID != rivals.ID {
					t.Fatalf("favourites = %+v, %v", got, err)
				}
				time.Sleep(2 * time.Millisecond)
				name, rounds := "FUT T Ancient", []library.PlaylistRound{{MatchID: nuke.ID, Round: 5, Map: "de_nuke", Comment: "rotan pronto"}, rivals.Rounds[0]}
				edited, err := f.repo.UpdatePlaylist(f.ctx, rivals.ID, library.PlaylistPatch{Name: &name, Rounds: &rounds})
				if err != nil || edited.Name != name || edited.Rounds[0].Comment != "rotan pronto" || edited.Rounds[1].Round != 3 ||
					edited.Maps[0] != "de_nuke" || !edited.UpdatedAt.After(rivals.UpdatedAt) || !edited.Favorite || edited.Author != "mopoz" {
					t.Fatalf("UpdatePlaylist = %+v, %v", edited, err)
				}
				if got, err := f.repo.ListPlaylists(f.ctx, library.PlaylistFilter{}); err != nil || got[0].ID != rivals.ID {
					t.Fatalf("after an edit ListPlaylists = %+v, %v; want the edited one first", got, err)
				}
				if err := f.repo.DeletePlaylist(f.ctx, pistols.ID); err != nil {
					t.Fatal(err)
				}
				if _, err := f.repo.GetPlaylist(f.ctx, pistols.ID); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("GetPlaylist(deleted) = %v, want ErrNotFound", err)
				}
				if err := f.repo.DeletePlaylist(f.ctx, pistols.ID); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("second DeletePlaylist = %v, want ErrNotFound", err)
				}
				if _, err := f.repo.UpdatePlaylist(f.ctx, pistols.ID, library.PlaylistPatch{Name: &name}); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("UpdatePlaylist(deleted) = %v, want ErrNotFound", err)
				}
			},
		},
		{
			name: "a round whose match is gone reads as missing, and its demo's hash finds the match again",
			run: func(t *testing.T, f libraryFixture) {
				mirageDemo := f.demo("mirage", 0)
				mirage := f.analyze(mirageDemo, "de_mirage", ct, tside)
				foreign := uuid.New()
				drawing := library.PlaylistNote{ID: uuid.New(), Tick: 640, Text: "rotación", Strokes: []library.Stroke{
					{Color: "accent", Width: 3, Arrow: true, Points: [][2]float64{{-120, 40}, {300, 812.5}}},
				}}
				p := &library.Playlist{Name: "De otro PC", Rounds: []library.PlaylistRound{
					{MatchID: mirage.ID, Round: 2, Map: "de_mirage", DemoSHA256: "mirage", Notes: []library.PlaylistNote{drawing}},
					{MatchID: foreign, Round: 4, Map: "de_inferno", DemoSHA256: "inferno", Comment: "su ejecución"},
				}}
				if err := f.repo.CreatePlaylist(f.ctx, p); err != nil {
					t.Fatal(err)
				}
				if p.Rounds[0].Missing || !p.Rounds[1].Missing || p.Rounds[1].Map != "de_inferno" || p.Rounds[1].MatchID != foreign {
					t.Fatalf("rounds = %+v; want the foreign round missing with its map", p.Rounds)
				}
				if p.Rounds[1].Notes == nil || len(p.Rounds[1].Notes) != 0 {
					t.Fatalf("notes of a round without any = %#v; want an empty list", p.Rounds[1].Notes)
				}
				inferno := f.analyze(f.demo("inferno", 1), "de_inferno", ct, tside)
				got, err := f.repo.GetPlaylist(f.ctx, p.ID)
				if err != nil || got.Rounds[1].Missing || got.Rounds[1].MatchID != inferno.ID || got.Rounds[1].Comment != "su ejecución" {
					t.Fatalf("after the demo came = %+v, %v; want the round on the imported match", got.Rounds, err)
				}
				if err := f.repo.DeleteDemo(f.ctx, mirageDemo.ID); err != nil {
					t.Fatal(err)
				}
				got, err = f.repo.GetPlaylist(f.ctx, p.ID)
				if err != nil || len(got.Rounds) != 2 || !got.Rounds[0].Missing || got.Rounds[0].MatchID != mirage.ID || got.Rounds[0].Map != "de_mirage" {
					t.Fatalf("after the delete = %+v, %v; want the round kept and missing", got.Rounds, err)
				}
				if notes := got.Rounds[0].Notes; len(notes) != 1 || notes[0].ID != drawing.ID || notes[0].Tick != 640 || len(notes[0].Strokes) != 1 ||
					!notes[0].Strokes[0].Arrow || notes[0].Strokes[0].Points[1] != [2]float64{300, 812.5} {
					t.Fatalf("notes after the demo went = %+v; want the drawing kept with its round", notes)
				}
				if list, err := f.repo.ListPlaylists(f.ctx, library.PlaylistFilter{Map: "de_mirage"}); err != nil || len(list) != 1 {
					t.Fatalf("map filter over a missing round = %d, %v; want the stored map to count", len(list), err)
				}
			},
		},
	}
	for _, tc := range cases {
		for name, repo := range libraryRepositoriesUnderTest(t) {
			t.Run(tc.name+"/"+name, func(t *testing.T) {
				tc.run(t, libraryFixture{t: t, ctx: context.Background(), repo: repo})
			})
		}
	}
}
