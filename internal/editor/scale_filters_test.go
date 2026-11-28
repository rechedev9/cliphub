package editor

import (
	"fmt"
	"strings"
	"testing"
)

func requireCropBeforeExactScale(t *testing.T, filter string, width, height int) {
	t.Helper()
	scale := fmt.Sprintf("scale=w=%d:h=%d:eval=init", width, height)
	scaleAt := strings.Index(filter, scale)
	if scaleAt < 0 {
		t.Fatalf("filter does not scale straight to %dx%d:\n%s", width, height, filter)
	}
	cropAt := strings.Index(filter, "crop=")
	if cropAt < 0 || cropAt > scaleAt {
		t.Fatalf("filter scales before it crops, so it upscales pixels the crop throws away:\n%s", filter)
	}
	if strings.Contains(filter, "force_original_aspect_ratio=increase") {
		t.Fatalf("filter still upscales past the output frame:\n%s", filter)
	}
}

func TestBaseGeometryCropsBeforeScalingForEveryFormat(t *testing.T) {
	for _, format := range []string{OutputFormatShort9x16, OutputFormatLandscape16x9} {
		short := ShortEdit{OutputFormat: format, HQFilters: true}
		width, height := outputDimensions(short)
		for _, fullFrame := range []bool{false, true} {
			filter := strings.Join(baseGeometryFilters(short, fmt.Sprintf("%d", height), false, fullFrame), ",")
			requireCropBeforeExactScale(t, filter, width, height)
			if !strings.Contains(filter, "min(iw\\,ih*") || !strings.Contains(filter, "min(ih\\,iw*") {
				t.Fatalf("crop must bound both sides to the output aspect, or a taller or narrower part stretches:\n%s", filter)
			}
			if !strings.Contains(filter, "flags="+hqScaleFlags(short)) {
				t.Fatalf("HQ scale lost its flags:\n%s", filter)
			}
		}
	}
}

func TestBaseGeometryKeepsDynamicScaleForZoom(t *testing.T) {
	filter := strings.Join(baseGeometryFilters(ShortEdit{}, "'1920*(1+0.1*t)'", false, false), ",")
	if !strings.Contains(filter, "eval=frame") || !strings.Contains(filter, "crop=1080:1920") {
		t.Fatalf("zoom geometry must scale per frame and crop to the output:\n%s", filter)
	}
}
