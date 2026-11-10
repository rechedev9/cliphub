package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// TestLibretaRepositoryContract runs the Libreta's records against both repositories.
func TestLibretaRepositoryContract(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T, f libraryFixture)
	}{
		{
			name: "lineups keep their detail, filter by map, grenade, status and tag, and patch in place",
			run: func(t *testing.T, f libraryFixture) {
				landing := [3]float64{7, 8, 9}
				source := &tacticalplan.Evidence{MatchID: uuid.New(), Round: 4, Tick: 900}
				smoke := &library.Lineup{Title: "Ventana", Map: "de_mirage", Side: "T", Grenade: "smoke", Status: "active",
					Tags: []string{"ct"}, Origin: [3]float64{1, 2, 3}, Pitch: -20, Yaw: 45, Landing: &landing,
					ThrowCallout: "TSpawn", LandingCallout: "Window", Movement: "standing", Jump: true, Click: "left",
					TechniqueEstimated: true, Source: source}
				if err := f.repo.CreateLineup(f.ctx, smoke); err != nil {
					t.Fatal(err)
				}
				if smoke.Landing == nil || *smoke.Landing != landing || smoke.Source == nil || smoke.Source.Round != 4 ||
					!smoke.Jump || !smoke.TechniqueEstimated || smoke.Yaw != 45 || smoke.LandingCallout != "Window" {
					t.Fatalf("created lineup = %+v", smoke)
				}
				again := *smoke
				if err := f.repo.CreateLineup(f.ctx, &again); !errors.Is(err, library.ErrConflict) {
					t.Fatalf("CreateLineup(same id) = %v, want ErrConflict", err)
				}
				time.Sleep(2 * time.Millisecond)
				flash := &library.Lineup{Title: "Pop B", Map: "de_nuke", Side: "CT", Grenade: "flash", Status: "archived",
					Origin: [3]float64{4, 5, 6}, Movement: "running", Click: "both"}
				if err := f.repo.CreateLineup(f.ctx, flash); err != nil {
					t.Fatal(err)
				}
				if flash.Landing != nil || flash.Source != nil || flash.Tags == nil {
					t.Fatalf("hand-written lineup = %+v, want no landing, no source and empty tags", flash)
				}
				list, err := f.repo.ListLineups(f.ctx, library.LineupFilter{})
				if err != nil || len(list) != 2 || list[0].ID != flash.ID {
					t.Fatalf("ListLineups = %+v, %v; want newest first", list, err)
				}
				for _, tc := range []struct {
					filter library.LineupFilter
					want   int
				}{
					{library.LineupFilter{Map: "de_mirage"}, 1},
					{library.LineupFilter{Grenade: "flash"}, 1},
					{library.LineupFilter{Side: "T"}, 1},
					{library.LineupFilter{ExcludeArchived: true}, 1},
					{library.LineupFilter{Status: "archived"}, 1},
					{library.LineupFilter{Tag: "CT"}, 1},
					{library.LineupFilter{Limit: 1, Offset: 1}, 1},
				} {
					if got, err := f.repo.ListLineups(f.ctx, tc.filter); err != nil || len(got) != tc.want {
						t.Fatalf("ListLineups(%+v) = %d, %v; want %d", tc.filter, len(got), err, tc.want)
					}
				}
				movement, estimated, title := "walking", false, "Ventana desde T"
				patched, err := f.repo.UpdateLineup(f.ctx, smoke.ID, library.LineupPatch{Title: &title, Movement: &movement, TechniqueEstimated: &estimated})
				if err != nil || patched.Title != title || patched.Movement != "walking" || patched.TechniqueEstimated || patched.Landing == nil {
					t.Fatalf("UpdateLineup = %+v, %v", patched, err)
				}
				if err := f.repo.DeleteLineup(f.ctx, smoke.ID); err != nil {
					t.Fatal(err)
				}
				if _, err := f.repo.GetLineup(f.ctx, smoke.ID); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("GetLineup(deleted) = %v, want ErrNotFound", err)
				}
				if err := f.repo.DeleteLineup(f.ctx, smoke.ID); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("second DeleteLineup = %v, want ErrNotFound", err)
				}
				if _, err := f.repo.UpdateLineup(f.ctx, smoke.ID, library.LineupPatch{Title: &title}); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("UpdateLineup(deleted) = %v, want ErrNotFound", err)
				}
			},
		},
		{
			name: "team role sheets are saved per map and follow a merge, where into keeps its own",
			run: func(t *testing.T, f libraryFixture) {
				vit, mouz := &library.Team{Name: "Vitality"}, &library.Team{Name: "MOUZ"}
				for _, team := range []*library.Team{vit, mouz} {
					if err := f.repo.CreateTeam(f.ctx, team); err != nil {
						t.Fatal(err)
					}
				}
				empty, err := f.repo.GetTeamRoles(f.ctx, vit.ID, "de_mirage")
				if err != nil || empty.T == nil || empty.CT == nil || len(empty.T) != 0 || !empty.UpdatedAt.IsZero() {
					t.Fatalf("GetTeamRoles(none saved) = %+v, %v", empty, err)
				}
				if _, err := f.repo.GetTeamRoles(f.ctx, uuid.New(), "de_mirage"); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("GetTeamRoles(missing team) = %v, want ErrNotFound", err)
				}
				save := func(team uuid.UUID, mapName, name string) {
					t.Helper()
					sheet := &library.TeamRoleSheet{TeamID: team, Map: mapName, T: []library.RoleSlot{{Name: name, SteamID64: "765", Task: "abre B"}},
						CT: []library.RoleSlot{{Name: "Ancla", Callout: "BombsiteB"}}}
					if err := f.repo.SetTeamRoles(f.ctx, sheet); err != nil || sheet.UpdatedAt.IsZero() {
						t.Fatalf("SetTeamRoles = %+v, %v", sheet, err)
					}
				}
				save(vit.ID, "de_mirage", "Entry")
				save(vit.ID, "de_mirage", "AWP")
				save(mouz.ID, "de_mirage", "Lurker")
				save(mouz.ID, "de_nuke", "Apoyo")
				got, err := f.repo.GetTeamRoles(f.ctx, vit.ID, "de_mirage")
				if err != nil || len(got.T) != 1 || got.T[0].Name != "AWP" || got.T[0].Task != "abre B" || got.CT[0].Callout != "BombsiteB" {
					t.Fatalf("GetTeamRoles = %+v, %v; want the second save", got, err)
				}
				if _, err := f.repo.MergeTeams(f.ctx, mouz.ID, vit.ID); err != nil {
					t.Fatal(err)
				}
				for mapName, want := range map[string]string{"de_mirage": "AWP", "de_nuke": "Apoyo"} {
					if got, err := f.repo.GetTeamRoles(f.ctx, vit.ID, mapName); err != nil || len(got.T) != 1 || got.T[0].Name != want {
						t.Fatalf("after merge %s = %+v, %v; want %s", mapName, got, err, want)
					}
				}
			},
		},
		{
			name: "team profiles replace, list only live teams and follow a merge, where into keeps its own",
			run: func(t *testing.T, f libraryFixture) {
				vit, mouz, faze := &library.Team{Name: "Vitality"}, &library.Team{Name: "MOUZ"}, &library.Team{Name: "FaZe"}
				for _, team := range []*library.Team{vit, mouz, faze} {
					if err := f.repo.CreateTeam(f.ctx, team); err != nil {
						t.Fatal(err)
					}
				}
				if err := f.repo.SetTeamProfile(f.ctx, &library.TeamProfile{TeamID: uuid.New(), Own: true}); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("SetTeamProfile(missing team) = %v, want ErrNotFound", err)
				}
				for _, p := range []library.TeamProfile{{TeamID: vit.ID, HLTVRank: 3}, {TeamID: vit.ID, Own: true, HLTVRank: 1},
					{TeamID: mouz.ID, HLTVRank: 7}, {TeamID: faze.ID, HLTVRank: 9}} {
					if err := f.repo.SetTeamProfile(f.ctx, &p); err != nil || p.UpdatedAt.IsZero() {
						t.Fatalf("SetTeamProfile = %+v, %v", p, err)
					}
				}
				if _, err := f.repo.MergeTeams(f.ctx, mouz.ID, vit.ID); err != nil {
					t.Fatal(err)
				}
				if _, err := f.repo.MergeTeams(f.ctx, faze.ID, mouz.ID); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("MergeTeams(into a merged team) = %v, want ErrNotFound", err)
				}
				got, err := f.repo.ListTeamProfiles(f.ctx)
				if err != nil || len(got) != 2 {
					t.Fatalf("ListTeamProfiles = %+v, %v; want Vitality and FaZe", got, err)
				}
				for _, p := range got {
					if p.TeamID == vit.ID && (!p.Own || p.HLTVRank != 1) {
						t.Fatalf("Vitality after merge = %+v, want its own profile", p)
					}
				}
				// One own team: marking FaZe takes the flag from Vitality.
				if err := f.repo.SetTeamProfile(f.ctx, &library.TeamProfile{TeamID: faze.ID, Own: true, HLTVRank: 9}); err != nil {
					t.Fatal(err)
				}
				if got, _ = f.repo.ListTeamProfiles(f.ctx); got[0].Own == got[1].Own {
					t.Fatalf("profiles = %+v, want one own team", got)
				}
				// Merging the own team into another makes that one own, with its own rank.
				if _, err := f.repo.MergeTeams(f.ctx, faze.ID, vit.ID); err != nil {
					t.Fatal(err)
				}
				if got, err = f.repo.ListTeamProfiles(f.ctx); err != nil || len(got) != 1 || !got[0].Own || got[0].HLTVRank != 1 {
					t.Fatalf("after merging the own team = %+v, %v; want Vitality own, rank 1", got, err)
				}
			},
		},
		{
			name: "pinned players keep their order, replace whole and read back empty as a list",
			run: func(t *testing.T, f libraryFixture) {
				if got, err := f.repo.ListPinnedPlayers(f.ctx); err != nil || got == nil || len(got) != 0 {
					t.Fatalf("ListPinnedPlayers(none) = %+v, %v; want an empty list", got, err)
				}
				first := []library.PinnedPlayer{{SteamID64: "76561198000000002", Name: "ZywOo"}, {SteamID64: "76561198000000001", Name: "donk"}}
				if err := f.repo.SetPinnedPlayers(f.ctx, first); err != nil {
					t.Fatal(err)
				}
				got, err := f.repo.ListPinnedPlayers(f.ctx)
				if err != nil || len(got) != 2 || got[0] != first[0] || got[1] != first[1] {
					t.Fatalf("ListPinnedPlayers = %+v, %v; want the saved order", got, err)
				}
				got[0].Name = "changed"
				if again, _ := f.repo.ListPinnedPlayers(f.ctx); again[0].Name != "ZywOo" {
					t.Fatalf("a caller edit reached the store: %+v", again)
				}
				if err := f.repo.SetPinnedPlayers(f.ctx, first[1:]); err != nil {
					t.Fatal(err)
				}
				if got, err = f.repo.ListPinnedPlayers(f.ctx); err != nil || len(got) != 1 || got[0].Name != "donk" {
					t.Fatalf("after replace = %+v, %v; want donk only", got, err)
				}
			},
		},
		{
			name: "calls list by name, a side keeps the calls of both, and callout names replace and clear",
			run: func(t *testing.T, f libraryFixture) {
				for _, c := range []*library.Call{
					{Name: "rotate", Side: "CT", Meaning: "a B"},
					{Name: "Bait", Side: "both", Tags: []string{"eco"}},
					{Name: "Lento", Side: "T", Status: "archived"},
				} {
					if err := f.repo.CreateCall(f.ctx, c); err != nil || c.Status == "" || c.Tags == nil || c.CreatedAt.IsZero() {
						t.Fatalf("CreateCall = %+v, %v", c, err)
					}
				}
				all, err := f.repo.ListCalls(f.ctx, library.CallFilter{})
				if err != nil || len(all) != 3 || all[0].Name != "Bait" || all[2].Name != "rotate" {
					t.Fatalf("ListCalls = %+v, %v; want by name, case-insensitive", all, err)
				}
				again := all[0]
				if err := f.repo.CreateCall(f.ctx, &again); !errors.Is(err, library.ErrConflict) {
					t.Fatalf("CreateCall(same id) = %v, want ErrConflict", err)
				}
				for _, tc := range []struct {
					filter library.CallFilter
					want   int
				}{
					{library.CallFilter{Side: "T"}, 2}, {library.CallFilter{Side: "CT", ExcludeArchived: true}, 2},
					{library.CallFilter{Status: "archived"}, 1}, {library.CallFilter{Tag: "ECO"}, 1}, {library.CallFilter{Limit: 2, Offset: 2}, 1},
				} {
					if got, err := f.repo.ListCalls(f.ctx, tc.filter); err != nil || len(got) != tc.want {
						t.Fatalf("ListCalls(%+v) = %d, %v; want %d", tc.filter, len(got), err, tc.want)
					}
				}
				meaning := "a B por mid"
				patched, err := f.repo.UpdateCall(f.ctx, all[2].ID, library.CallPatch{Meaning: &meaning})
				if err != nil || patched.Meaning != meaning || patched.Name != "rotate" {
					t.Fatalf("UpdateCall = %+v, %v", patched, err)
				}
				if err := f.repo.DeleteCall(f.ctx, all[2].ID); err != nil {
					t.Fatal(err)
				}
				if _, err := f.repo.GetCall(f.ctx, all[2].ID); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("GetCall(deleted) = %v, want ErrNotFound", err)
				}

				for _, c := range []library.CalloutName{{Map: "de_mirage", Token: "BombsiteB", Name: "B"}, {Map: "de_mirage", Token: "BombsiteB", Name: "Bé"},
					{Map: "de_mirage", Token: "Apartments", Name: "Pisos"}, {Map: "de_nuke", Token: "Ramp", Name: "Rampa"}} {
					if err := f.repo.SetCalloutName(f.ctx, &c); err != nil || c.UpdatedAt.IsZero() {
						t.Fatalf("SetCalloutName = %+v, %v", c, err)
					}
				}
				names, err := f.repo.ListCalloutNames(f.ctx, "de_mirage")
				if err != nil || len(names) != 2 || names[0].Token != "Apartments" || names[1].Name != "Bé" {
					t.Fatalf("ListCalloutNames = %+v, %v", names, err)
				}
				if every, err := f.repo.ListCalloutNames(f.ctx, ""); err != nil || len(every) != 3 {
					t.Fatalf("ListCalloutNames(all maps) = %+v, %v", every, err)
				}
				if err := f.repo.DeleteCalloutName(f.ctx, "de_mirage", "BombsiteB"); err != nil {
					t.Fatal(err)
				}
				if err := f.repo.DeleteCalloutName(f.ctx, "de_mirage", "BombsiteB"); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("second DeleteCalloutName = %v, want ErrNotFound", err)
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
