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
	}
	for _, tc := range cases {
		for name, repo := range libraryRepositoriesUnderTest(t) {
			t.Run(tc.name+"/"+name, func(t *testing.T) {
				tc.run(t, libraryFixture{t: t, ctx: context.Background(), repo: repo})
			})
		}
	}
}
