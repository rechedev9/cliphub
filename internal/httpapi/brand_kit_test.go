package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"net/http"
	"reflect"
	"testing"

	"github.com/rechedev9/cliphub/internal/brandkit"
	"github.com/rechedev9/cliphub/internal/overlayassets"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/store"
)

func storeBrandImage(t *testing.T, env libraryTestEnv) brandkit.ImageRef {
	t.Helper()
	var body bytes.Buffer
	if err := png.Encode(&body, image.NewNRGBA(image.Rect(0, 0, 40, 20))); err != nil {
		t.Fatal(err)
	}
	asset, err := overlayassets.Store(context.Background(), env.storage, &body, "logo.png")
	if err != nil {
		t.Fatal(err)
	}
	return brandkit.ImageRef{ID: asset.ID.String(), SHA256: asset.SHA256}
}

func TestBrandKitSaveChecksUploadedImages(t *testing.T) {
	env := newLibraryTestEnv(t)
	env.h.brandKit = store.NewMemoryBrandKitRepository()
	var got brandKitBody
	env.json(http.MethodGet, "/api/brand-kit", nil, http.StatusOK, &got)
	if got.Logo != nil || got.WatermarkCorner != brandkit.DefaultCorner || len(got.Corners) != 4 {
		t.Fatalf("unsaved kit = %+v; want the default kit and the corners", got)
	}
	logo := storeBrandImage(t, env)
	env.json(http.MethodPut, "/api/brand-kit", map[string]any{"logo": logo, "watermark_corner": "bottom-left", "watermark_opacity": 0.5, "banner_start_seconds": 1}, http.StatusOK, &got)
	if got.Logo == nil || *got.Logo != logo || got.WatermarkCorner != brandkit.CornerBottomLeft || got.UpdatedAt.IsZero() {
		t.Fatalf("saved kit = %+v", got)
	}
	unknown := brandkit.ImageRef{ID: "0b5d1b6e-8a3c-4f0e-9d8a-1c2b3d4e5f60", SHA256: logo.SHA256}
	for _, bad := range []map[string]any{
		{"logo": unknown, "watermark_corner": "top-left", "watermark_opacity": 1},
		{"watermark_corner": "middle", "watermark_opacity": 1},
		{"watermark_corner": "top-left", "watermark_opacity": 0},
		{"watermark_corner": "top-left", "watermark_opacity": 1, "banner_start_seconds": 5, "banner_end_seconds": 4},
	} {
		env.json(http.MethodPut, "/api/brand-kit", bad, http.StatusBadRequest, nil)
	}
}

func TestRenderBrandKitChoices(t *testing.T) {
	env := newLibraryTestEnv(t)
	repo := store.NewMemoryBrandKitRepository()
	env.h.brandKit = repo
	on, off := &brandChoice[brandkit.Watermark]{On: true}, &brandChoice[brandkit.Watermark]{}
	bannerOn, bannerOff := &brandChoice[brandkit.Banner]{On: true}, &brandChoice[brandkit.Banner]{}
	edit := renderplan.DefaultEditRequest()
	if err := env.h.applyBrandKit(context.Background(), renderEditRequest{BrandWatermark: on}, &edit); !errors.Is(err, brandkit.ErrMissing) {
		t.Fatalf("watermark without a logo = %v; want ErrMissing", err)
	}
	logo := storeBrandImage(t, env)
	if _, err := repo.SaveBrandKit(context.Background(), brandkit.Kit{Logo: &logo, WatermarkCorner: brandkit.CornerTopLeft, WatermarkOpacity: 0.4, Banner: &logo, BannerStartSeconds: 2, BannerEndSeconds: 6}); err != nil {
		t.Fatal(err)
	}
	if err := env.h.applyBrandKit(context.Background(), renderEditRequest{BrandWatermark: on, BrandBanner: bannerOn}, &edit); err != nil {
		t.Fatal(err)
	}
	if edit.BrandWatermark == nil || edit.BrandWatermark.Opacity != 0.4 || edit.BrandWatermark.Image != logo || edit.BrandBanner == nil || edit.BrandBanner.EndSeconds != 6 {
		t.Fatalf("edit = %+v %+v; want the kit copied", edit.BrandWatermark, edit.BrandBanner)
	}
	if err := edit.Validate(); err != nil {
		t.Fatalf("branded edit is invalid: %v", err)
	}
	// A later kit edit does not reach an edit already made; an absent choice keeps it.
	if _, err := repo.SaveBrandKit(context.Background(), brandkit.Kit{WatermarkCorner: brandkit.CornerTopRight, WatermarkOpacity: 1}); err != nil {
		t.Fatal(err)
	}
	if err := env.h.applyBrandKit(context.Background(), renderEditRequest{}, &edit); err != nil || edit.BrandWatermark == nil {
		t.Fatalf("absent choices changed the edit: %v %+v", err, edit.BrandWatermark)
	}
	// A retry sends back the effective edit; its copy decodes and renders as it was.
	raw, err := json.Marshal(map[string]any{"brand_watermark": edit.BrandWatermark, "brand_banner": edit.BrandBanner})
	if err != nil {
		t.Fatal(err)
	}
	var retry renderEditRequest
	if err := json.Unmarshal(raw, &retry); err != nil {
		t.Fatal(err)
	}
	again := renderplan.DefaultEditRequest()
	if err := env.h.applyBrandKit(context.Background(), retry, &again); err != nil || !reflect.DeepEqual(again.BrandWatermark, edit.BrandWatermark) || !reflect.DeepEqual(again.BrandBanner, edit.BrandBanner) {
		t.Fatalf("retry = %v %+v; want the saved copy kept", err, again)
	}
	if err := json.Unmarshal([]byte(`{"brand_watermark":{"image":{},"nope":1}}`), &retry); err == nil {
		t.Fatal("accepted an unknown field in a brand copy")
	}
	if err := env.h.applyBrandKit(context.Background(), renderEditRequest{BrandWatermark: off, BrandBanner: bannerOff}, &edit); err != nil || edit.BrandWatermark != nil || edit.BrandBanner != nil {
		t.Fatalf("off choices kept the kit: %v %+v", err, edit)
	}
}
