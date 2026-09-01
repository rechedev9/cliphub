package keydropbanner

import (
	"strings"
	"testing"
)

func TestBuildOverlayFilterScalesPrecompositedPlate(t *testing.T) {
	t.Parallel()
	style, ok := Lookup(FamilyKeyDrop, StyleOperator)
	if !ok {
		t.Fatal("operator style missing")
	}
	filter, err := BuildOverlayFilter(OverlayParams{
		Style:           style,
		OutputWidth:     1080,
		OutputHeight:    1920,
		PositionY:       0.86,
		DurationSeconds: 8,
		ContentLabel:    "content",
		OutputLabel:     "keydropped",
		InputIndex:      1,
	})
	if err != nil {
		t.Fatalf("BuildOverlayFilter: %v", err)
	}
	for _, want := range []string{
		"[1:v]format=rgba,scale=",
		"[content][kdplate]overlay=",
		"[keydropped]",
		"enable='between(t\\,0.000000\\,8.000000)'",
	} {
		if !strings.Contains(filter, want) {
			t.Fatalf("filter missing %q\n%s", want, filter)
		}
	}
	// Code is burned into the PNG by CompositeWithCode; the overlay filter
	// must not re-draw text (that path ignored plan code changes on some hosts).
	for _, banned := range []string{"drawtext=", "drawbox=", "CODE\\:"} {
		if strings.Contains(filter, banned) {
			t.Fatalf("filter unexpectedly contains %q\n%s", banned, filter)
		}
	}
}

func TestBuildOverlayFilterHonorsVisibilityWindow(t *testing.T) {
	t.Parallel()
	style, _ := Lookup(FamilyKeyDrop, StyleOperator)
	filter, err := BuildOverlayFilter(OverlayParams{
		Style:           style,
		OutputWidth:     1080,
		OutputHeight:    1920,
		DurationSeconds: 15,
		StartSeconds:    1.5,
		EndSeconds:      5,
		ContentLabel:    "content",
		OutputLabel:     "out",
		InputIndex:      1,
	})
	if err != nil {
		t.Fatalf("BuildOverlayFilter: %v", err)
	}
	if !strings.Contains(filter, "enable='between(t\\,1.500000\\,5.000000)'") {
		t.Fatalf("filter missing window enable: %s", filter)
	}
}

func TestBuildOverlayFilterSlideUsesEvalFrame(t *testing.T) {
	t.Parallel()
	style, _ := Lookup(FamilyKeyDrop, StyleClassic)
	filter, err := BuildOverlayFilter(OverlayParams{
		Style:           style,
		OutputWidth:     1920,
		OutputHeight:    1080,
		SlideEnabled:    true,
		DurationSeconds: 5,
		ContentLabel:    "bannered",
		OutputLabel:     "out",
		InputIndex:      2,
	})
	if err != nil {
		t.Fatalf("BuildOverlayFilter: %v", err)
	}
	if !strings.Contains(filter, "eval=frame") {
		t.Fatalf("slide filter missing eval=frame: %s", filter)
	}
	if !strings.Contains(filter, "if(lt(t") {
		t.Fatalf("slide filter missing temporal x expr: %s", filter)
	}
}

func TestBuildOverlayFilterRejectsBadInput(t *testing.T) {
	t.Parallel()
	_, err := BuildOverlayFilter(OverlayParams{
		// Empty style is invalid even when every other field is set.
		OutputWidth:     1080,
		OutputHeight:    1920,
		DurationSeconds: 1,
		ContentLabel:    "c",
		OutputLabel:     "o",
		InputIndex:      1,
	})
	if err == nil || !strings.Contains(err.Error(), "style is required") {
		t.Fatalf("error = %v, want style is required", err)
	}
}
