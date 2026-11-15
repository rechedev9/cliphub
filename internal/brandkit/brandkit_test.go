package brandkit

import (
	"errors"
	"strings"
	"testing"
)

var testImage = ImageRef{ID: "0b5d1b6e-8a3c-4f0e-9d8a-1c2b3d4e5f60", SHA256: strings.Repeat("a", 64)}

func TestKitValidateAndCopies(t *testing.T) {
	kit := Default()
	if err := kit.Validate(); err != nil {
		t.Fatalf("default kit: %v", err)
	}
	if _, err := kit.Watermark(); !errors.Is(err, ErrMissing) {
		t.Fatalf("watermark without a logo = %v; want ErrMissing", err)
	}
	if _, err := kit.SponsorBanner(); !errors.Is(err, ErrMissing) {
		t.Fatalf("banner without an image = %v; want ErrMissing", err)
	}
	logo := testImage
	kit.Logo, kit.Banner, kit.BannerStartSeconds, kit.BannerEndSeconds = &logo, &logo, 3, 8
	wm, err := kit.Watermark()
	if err != nil || wm.Corner != DefaultCorner || wm.Opacity != DefaultOpacity || wm.Validate() != nil {
		t.Fatalf("watermark = %+v, %v", wm, err)
	}
	copied := kit.Copy()
	copied.Logo.ID = "other"
	if kit.Logo.ID != testImage.ID {
		t.Fatal("Copy shares the logo pointer")
	}
	for name, bad := range map[string]Kit{
		"corner":   {WatermarkCorner: "center", WatermarkOpacity: 1},
		"opacity":  {WatermarkCorner: CornerTopLeft, WatermarkOpacity: 0.05},
		"window":   {WatermarkCorner: CornerTopLeft, WatermarkOpacity: 1, BannerStartSeconds: 5, BannerEndSeconds: 5},
		"negative": {WatermarkCorner: CornerTopLeft, WatermarkOpacity: 1, BannerStartSeconds: -1},
		"image":    {WatermarkCorner: CornerTopLeft, WatermarkOpacity: 1, Logo: &ImageRef{ID: "x", SHA256: testImage.SHA256}},
		"hash":     {WatermarkCorner: CornerTopLeft, WatermarkOpacity: 1, Banner: &ImageRef{ID: testImage.ID, SHA256: "ABC"}},
	} {
		if err := bad.Validate(); err == nil {
			t.Errorf("%s: accepted %+v", name, bad)
		}
	}
}

func TestBannerWindowClampsToTheVideo(t *testing.T) {
	for _, tc := range []struct {
		banner     Banner
		duration   float64
		start, end float64
		ok         bool
	}{
		{Banner{StartSeconds: 2}, 10, 2, 10, true},
		{Banner{StartSeconds: 2, EndSeconds: 6}, 10, 2, 6, true},
		{Banner{StartSeconds: 2, EndSeconds: 60}, 10, 2, 10, true},
		{Banner{StartSeconds: 12}, 10, 12, 10, false},
	} {
		start, end, ok := tc.banner.Window(tc.duration)
		if start != tc.start || end != tc.end || ok != tc.ok {
			t.Errorf("%+v over %g = %g..%g %v; want %g..%g %v", tc.banner, tc.duration, start, end, ok, tc.start, tc.end, tc.ok)
		}
	}
}
