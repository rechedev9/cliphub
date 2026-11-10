package store

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// TestPatternRepositoryContract runs the saved patterns against both repositories.
func TestPatternRepositoryContract(t *testing.T) {
	for name, repo := range libraryRepositoriesUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			team := uuid.New()
			area := &tacticalplan.PatternArea{MinX: -100, MinY: 0, MaxX: 300, MaxY: 400, Level: "lower"}
			smoke := &library.Pattern{Name: "Humo CT en dobles", Map: "de_ancient", TeamID: &team,
				Pattern: tacticalplan.Pattern{Side: tacticalplan.SideCT, Blocks: []tacticalplan.PatternBlock{
					{Kind: tacticalplan.PatternUtility, Grenade: tacticalplan.EventSmoke, Area: area, To: 20},
					{Kind: tacticalplan.PatternPlayer, Players: []string{"76561198000000001"}, Invert: true},
				}},
				Scope: library.PatternScope{Matches: 3, Buy: tacticalplan.BuyFull}}
			if err := repo.CreatePattern(ctx, smoke); err != nil || smoke.ID == uuid.Nil || smoke.CreatedAt.IsZero() {
				t.Fatalf("CreatePattern = %+v, %v", smoke, err)
			}
			again := *smoke
			if err := repo.CreatePattern(ctx, &again); !errors.Is(err, library.ErrConflict) {
				t.Fatalf("CreatePattern(same id) = %v, want ErrConflict", err)
			}
			got, err := repo.GetPattern(ctx, smoke.ID)
			if err != nil || got.TeamID == nil || *got.TeamID != team || len(got.Pattern.Blocks) != 2 ||
				got.Pattern.Blocks[0].Area == nil || !reflect.DeepEqual(*got.Pattern.Blocks[0].Area, *area) || !got.Pattern.Blocks[1].Invert ||
				got.Pattern.Blocks[1].Players[0] != "76561198000000001" || got.Scope.Matches != 3 || got.Scope.Buy != tacticalplan.BuyFull {
				t.Fatalf("GetPattern = %+v, %v", got, err)
			}
			got.Pattern.Blocks[0].Area.MinX = 999
			if again, _ := repo.GetPattern(ctx, smoke.ID); again.Pattern.Blocks[0].Area.MinX != -100 {
				t.Fatal("a read shares its area with the store")
			}
			anyTeam := &library.Pattern{Name: "A Finish", Map: "de_ancient",
				Pattern: tacticalplan.Pattern{Side: tacticalplan.SideT, Blocks: []tacticalplan.PatternBlock{{Kind: tacticalplan.PatternPlant}}}}
			nuke := &library.Pattern{Name: "Rampa", Map: "de_nuke", TeamID: &team,
				Pattern: tacticalplan.Pattern{Side: tacticalplan.SideT, Blocks: []tacticalplan.PatternBlock{{Kind: tacticalplan.PatternPlayer}}}}
			for _, p := range []*library.Pattern{anyTeam, nuke} {
				if err := repo.CreatePattern(ctx, p); err != nil {
					t.Fatal(err)
				}
			}
			other := uuid.New()
			for _, tc := range []struct {
				filter library.PatternFilter
				want   []string
			}{
				{library.PatternFilter{}, []string{"A Finish", "Humo CT en dobles", "Rampa"}},
				{library.PatternFilter{Map: "de_ancient"}, []string{"A Finish", "Humo CT en dobles"}},
				{library.PatternFilter{Map: "de_ancient", TeamID: &other}, []string{"A Finish"}},
				{library.PatternFilter{TeamID: &team, Limit: 1, Offset: 1}, []string{"Humo CT en dobles"}},
			} {
				list, err := repo.ListPatterns(ctx, tc.filter)
				if err != nil || len(list) != len(tc.want) {
					t.Fatalf("ListPatterns(%+v) = %+v, %v", tc.filter, list, err)
				}
				for i, p := range list {
					if p.Name != tc.want[i] {
						t.Fatalf("ListPatterns(%+v)[%d] = %q, want %q", tc.filter, i, p.Name, tc.want[i])
					}
				}
			}
			name := "Humo en dobles"
			patched, err := repo.UpdatePattern(ctx, smoke.ID, library.PatternPatch{Name: &name, ClearTeam: true})
			if err != nil || patched.Name != name || patched.TeamID != nil || len(patched.Pattern.Blocks) != 2 {
				t.Fatalf("UpdatePattern = %+v, %v", patched, err)
			}
			if err := repo.DeletePattern(ctx, smoke.ID); err != nil {
				t.Fatal(err)
			}
			if err := repo.DeletePattern(ctx, smoke.ID); !errors.Is(err, library.ErrNotFound) {
				t.Fatalf("second DeletePattern = %v, want ErrNotFound", err)
			}
			if _, err := repo.UpdatePattern(ctx, smoke.ID, library.PatternPatch{Name: &name}); !errors.Is(err, library.ErrNotFound) {
				t.Fatalf("UpdatePattern(deleted) = %v, want ErrNotFound", err)
			}
		})
	}
}
