package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/hudthemes"
)

func hudThemeRepositoriesUnderTest(t *testing.T) map[string]HUDThemeRepository {
	t.Helper()
	return map[string]HUDThemeRepository{
		"memory": NewMemoryHUDThemeRepository(),
		"sqlite": NewSQLiteHUDThemeRepository(newTestSQLiteRepo(t).db),
	}
}

func TestHUDThemeRepositoryContract(t *testing.T) {
	for name, repo := range hudThemeRepositoriesUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if got, err := repo.ListHUDThemes(ctx); err != nil || got == nil || len(got) != 0 {
				t.Fatalf("ListHUDThemes(none) = %+v, %v; want an empty list", got, err)
			}
			radius := 6
			first := hudthemes.Theme{Base: "arena", Style: hudthemes.Style{Name: "Mío", CT: "112233", T: "445566", Background: "000000", Radius: &radius}}
			if err := repo.CreateHUDTheme(ctx, &first); err != nil || first.ID == uuid.Nil || first.CreatedAt.IsZero() {
				t.Fatalf("CreateHUDTheme = %+v, %v", first, err)
			}
			second := hudthemes.Theme{Base: "apex", Style: hudthemes.Style{Name: "Sin radio", CT: "AAAAAA", T: "BBBBBB", Background: "CCCCCC"}}
			if err := repo.CreateHUDTheme(ctx, &second); err != nil {
				t.Fatal(err)
			}
			got, err := repo.ListHUDThemes(ctx)
			if err != nil || len(got) != 2 || got[0].ID != first.ID || got[1].Radius != nil || *got[0].Radius != 6 || got[0].Base != "arena" || !got[0].CreatedAt.Equal(first.CreatedAt) {
				t.Fatalf("ListHUDThemes = %+v, %v; want both, oldest first, with the radius kept", got, err)
			}
			*got[0].Radius = 15
			if again, _ := repo.GetHUDTheme(ctx, first.ID); *again.Radius != 6 {
				t.Fatalf("a caller edit reached the store: %+v", again)
			}
			square := 0
			updated, err := repo.UpdateHUDTheme(ctx, second.ID, hudthemes.Style{Name: "Recto", CT: "AAAAAA", T: "BBBBBB", Background: "111111", Radius: &square})
			if err != nil || updated.Name != "Recto" || updated.Base != "apex" || *updated.Radius != 0 || updated.ID != second.ID {
				t.Fatalf("UpdateHUDTheme = %+v, %v", updated, err)
			}
			missing := uuid.New()
			if _, err := repo.UpdateHUDTheme(ctx, missing, first.Style); !errors.Is(err, hudthemes.ErrNotFound) {
				t.Fatalf("UpdateHUDTheme(missing) = %v", err)
			}
			if _, err := repo.GetHUDTheme(ctx, missing); !errors.Is(err, hudthemes.ErrNotFound) {
				t.Fatalf("GetHUDTheme(missing) = %v", err)
			}
			for len(got) < hudthemes.MaxThemes {
				extra := hudthemes.Theme{Base: "arena", Style: first.Style}
				if err := repo.CreateHUDTheme(ctx, &extra); err != nil {
					t.Fatal(err)
				}
				got = append(got, extra)
			}
			over := hudthemes.Theme{Base: "arena", Style: first.Style}
			if err := repo.CreateHUDTheme(ctx, &over); !errors.Is(err, hudthemes.ErrLimit) {
				t.Fatalf("CreateHUDTheme past the limit = %v, want ErrLimit", err)
			}
			if err := repo.DeleteHUDTheme(ctx, first.ID); err != nil {
				t.Fatal(err)
			}
			if err := repo.DeleteHUDTheme(ctx, first.ID); !errors.Is(err, hudthemes.ErrNotFound) {
				t.Fatalf("DeleteHUDTheme(twice) = %v", err)
			}
			if err := repo.CreateHUDTheme(ctx, &over); err != nil {
				t.Fatalf("a delete did not free a slot: %v", err)
			}
		})
	}
}
