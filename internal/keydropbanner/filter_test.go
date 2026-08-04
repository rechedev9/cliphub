package keydropbanner

import (
	"strings"
	"testing"
)

func TestBuildOverlayFilterCoversCodeAndDrawsLabel(t *testing.T) {
	t.Parallel()
	style, ok := Lookup(StyleOperator)
	if !ok {
		t.Fatal("operator style missing")
	}
	filter, err := BuildOverlayFilter(OverlayParams{
		Style:           style,
		Code:            "TESTCODE",
		FontPath:        `C:\Fonts\Montserrat.ttf`,
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
		"drawbox=",
		"drawtext=",
		"CODE\\: TESTCODE",
		"[content][kdplate]overlay=",
		"[keydropped]",
		`C\:/Fonts/Montserrat.ttf`,
	} {
		if !strings.Contains(filter, want) {
			t.Fatalf("filter missing %q\n%s", want, filter)
		}
	}
}

func TestBuildOverlayFilterSlideUsesEvalFrame(t *testing.T) {
	t.Parallel()
	style, _ := Lookup(StyleClassic)
	filter, err := BuildOverlayFilter(OverlayParams{
		Style:           style,
		Code:            "ZACKCSGO",
		FontPath:        "/usr/share/fonts/test.ttf",
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
	style, _ := Lookup(StyleOperator)
	_, err := BuildOverlayFilter(OverlayParams{
		Style:           style,
		FontPath:        "",
		OutputWidth:     1080,
		OutputHeight:    1920,
		DurationSeconds: 1,
		ContentLabel:    "c",
		OutputLabel:     "o",
	})
	if err == nil || !strings.Contains(err.Error(), "font path") {
		t.Fatalf("error = %v, want font path", err)
	}
}
