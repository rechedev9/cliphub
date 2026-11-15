package main

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/brandkit"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestMergeBrandKitKeepsLeftOutFields(t *testing.T) {
	logo := brandkit.ImageRef{ID: "0b5d1b6e-8a3c-4f0e-9d8a-1c2b3d4e5f60", SHA256: strings.Repeat("a", 64)}
	saved := brandkit.Kit{Logo: &logo, WatermarkCorner: brandkit.CornerTopLeft, WatermarkOpacity: 0.8, Banner: &logo, BannerStartSeconds: 3}
	opacity, end := 45, 9.0
	kit, err := mergeBrandKit(saved, saveBrandKitIn{WatermarkOpacity: &opacity, Banner: noneChoice, BannerEndSeconds: &end})
	if err != nil {
		t.Fatal(err)
	}
	if kit.Logo == nil || kit.WatermarkCorner != brandkit.CornerTopLeft || kit.WatermarkOpacity != 0.45 || kit.Banner != nil || kit.BannerStartSeconds != 3 || kit.BannerEndSeconds != 9 {
		t.Fatalf("merged = %+v", kit)
	}
	if saved.Banner == nil {
		t.Fatal("the merge changed the saved kit")
	}
	tooLow := 5
	if _, err := mergeBrandKit(saved, saveBrandKitIn{WatermarkOpacity: &tooLow}); err == nil {
		t.Fatal("accepted 5 % opacity")
	}
	if _, err := mergeBrandKit(saved, saveBrandKitIn{WatermarkCorner: "centre"}); err == nil {
		t.Fatal("accepted an unknown corner")
	}
}

func TestBrandChoicesOnShortsAndFullDemo(t *testing.T) {
	on := true
	edit, err := resolveEdit(shortChoices{BrandWatermark: &on, BrandBanner: &on})
	if err != nil || !edit.BrandWatermark || !edit.BrandBanner {
		t.Fatalf("resolveEdit = %+v, %v", edit, err)
	}
	if _, err := resolveEdit(shortChoices{BrandBanner: &on, BannerStyle: "classic"}); err == nil {
		t.Fatal("accepted the brand banner with the affiliate banner")
	}
	if !strings.Contains(describeShortRequest(tuiclient.GenerateRequest{Edit: edit}), "brand_watermark") {
		t.Fatal("the approval summary leaves out the brand kit")
	}
	logo := brandkit.ImageRef{ID: "0b5d1b6e-8a3c-4f0e-9d8a-1c2b3d4e5f60", SHA256: strings.Repeat("a", 64)}
	in := brandChoiceIn{BrandWatermark: &on, watermark: &brandkit.Watermark{Image: logo, Corner: brandkit.CornerBottomRight, Opacity: 1}}
	options := recapplan.DefaultOptions()
	applyBrandChoice(&options, in)
	if options.Overlays.Brand.Watermark == nil || options.Overlays.Brand.Banner != nil || describeBrand(options.Overlays.Brand) != " Con tu marca de agua." {
		t.Fatalf("brand = %+v", options.Overlays.Brand)
	}
	off := false
	applyBrandChoice(&options, brandChoiceIn{BrandWatermark: &off})
	if options.Overlays.Brand.Watermark != nil {
		t.Fatal("brand_watermark false kept the watermark")
	}
}
