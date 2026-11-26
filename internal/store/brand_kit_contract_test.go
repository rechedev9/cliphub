package store

import (
	"context"
	"reflect"
	"testing"

	"github.com/rechedev9/cliphub/internal/brandkit"
)

func TestBrandKitRepositoryContract(t *testing.T) {
	repos := map[string]BrandKitRepository{
		"memory": NewMemoryBrandKitRepository(),
		"sqlite": NewSQLiteBrandKitRepository(newTestSQLiteRepo(t).db),
	}
	for name, repo := range repos {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if got, err := repo.GetBrandKit(ctx); err != nil || !reflect.DeepEqual(got, brandkit.Default()) {
				t.Fatalf("GetBrandKit(unsaved) = %+v, %v; want the default kit", got, err)
			}
			logo := brandkit.ImageRef{ID: "0b5d1b6e-8a3c-4f0e-9d8a-1c2b3d4e5f60", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
			banner := brandkit.ImageRef{ID: "1c6e2c7f-9b4d-4a1f-8e9b-2d3c4e5f6071", SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
			kit := brandkit.Kit{Logo: &logo, WatermarkCorner: brandkit.CornerBottomLeft, WatermarkOpacity: 0.55, Banner: &banner, BannerStartSeconds: 2, BannerEndSeconds: 9.5, ChannelName: "ClipHub RSN"}
			saved, err := repo.SaveBrandKit(ctx, kit)
			if err != nil || saved.UpdatedAt.IsZero() {
				t.Fatalf("SaveBrandKit = %+v, %v", saved, err)
			}
			got, err := repo.GetBrandKit(ctx)
			if err != nil || !reflect.DeepEqual(got, saved) {
				t.Fatalf("GetBrandKit = %+v, %v; want %+v", got, err, saved)
			}
			got.Logo.ID = "changed"
			if again, _ := repo.GetBrandKit(ctx); again.Logo.ID != logo.ID {
				t.Fatalf("a caller edit reached the store: %+v", again)
			}
			cleared, err := repo.SaveBrandKit(ctx, brandkit.Kit{WatermarkCorner: brandkit.CornerTopLeft, WatermarkOpacity: 1})
			if err != nil {
				t.Fatal(err)
			}
			if got, _ := repo.GetBrandKit(ctx); got.Logo != nil || got.Banner != nil || got.BannerEndSeconds != 0 || got.ChannelName != "" || got.WatermarkCorner != brandkit.CornerTopLeft || !got.UpdatedAt.Equal(cleared.UpdatedAt) {
				t.Fatalf("GetBrandKit after clearing = %+v; want no images", got)
			}
			if _, err := repo.SaveBrandKit(ctx, brandkit.Kit{WatermarkCorner: "middle", WatermarkOpacity: 1}); err == nil {
				t.Fatal("SaveBrandKit accepted an unknown corner")
			}
			if _, err := repo.SaveBrandKit(ctx, brandkit.Kit{WatermarkCorner: brandkit.CornerTopLeft, WatermarkOpacity: 1, ChannelName: "a\nb"}); err == nil {
				t.Fatal("SaveBrandKit accepted a channel name on two lines")
			}
			if got, _ := repo.GetBrandKit(ctx); got.WatermarkCorner != brandkit.CornerTopLeft {
				t.Fatalf("a rejected save changed the kit: %+v", got)
			}
		})
	}
}
