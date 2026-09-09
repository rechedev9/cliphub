package customhud

import (
	"context"
	"image"
	"os/exec"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestBroadcastLayoutKeepsOneUpperRosterAndFixedPlayerCorners(t *testing.T) {
	state := Example()
	state.Players[1].Health, state.Players[1].Armor = 21, 94
	state.Players[1].Ammo, state.Players[1].Reserve = 18, 2
	state.Players[1].Name = strings.Repeat("W", 120)
	for _, theme := range Themes() {
		r, err := NewRenderer(theme.ID)
		if err != nil {
			t.Fatal(err)
		}
		nodes := map[string]Node{}
		roster := 0
		for _, n := range r.Scene(state, ExampleTarget) {
			nodes[n.ID] = n
			if strings.Contains(n.ID, "alive/") || n.Text == "ALIVE" || n.Text == "VS" {
				t.Fatalf("%s duplicated alive information: %s", theme.ID, n.ID)
			}
			if strings.HasPrefix(n.ID, "player/") && strings.HasSuffix(n.ID, "/name") {
				roster++
				if n.Y > 94 {
					t.Fatalf("%s roster left the top row: %+v", theme.ID, n)
				}
			}
		}
		if roster != 10 || nodes["focus/name"].X > 330 || nodes["focus/name"].Y < 940 || nodes["focus/ammo"].X < 1550 || nodes["focus/ammo"].Y < 950 {
			t.Fatalf("%s lost the reference composition", theme.ID)
		}
		for id, value := range map[string]string{"focus/health": "21", "focus/armor": "94", "focus/ammo": "18/2"} {
			if nodes[id].Text != value {
				t.Fatalf("%s %s=%q, want target value %q", theme.ID, id, nodes[id].Text, value)
			}
		}
		if !strings.HasSuffix(nodes["focus/name"].Text, "…") {
			t.Fatal("long target name did not fit its own plate")
		}
		// Team deaths and roster order cannot redirect the observed-player HUD.
		state.Players[0], state.Players[9] = state.Players[9], state.Players[0]
		for _, n := range r.Scene(state, ExampleTarget) {
			if strings.HasPrefix(n.ID, "focus/") && n.Text != nodes[n.ID].Text {
				t.Fatalf("%s target followed roster order: %s", theme.ID, n.ID)
			}
		}
	}
}

func TestReferenceLayoutPaintsOnlyTheUpperStripAndLowerCorners(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg unavailable")
	}
	allowed := []image.Rectangle{
		image.Rect(440, 20, 1480, 100),
		image.Rect(24, 935, 332, 1050),
		image.Rect(1548, 946, 1896, 1050),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	for _, theme := range Themes() {
		r, err := NewRenderer(theme.ID)
		if err != nil {
			t.Fatal(err)
		}
		frame, err := RasterizePreview(ctx, ffmpeg, isolatedSceneASS(t, r.Scene(Example(), ExampleTarget)))
		if err != nil {
			t.Fatal(err)
		}
		painted := make([]int, len(allowed))
		for y := range Height {
			for x := range Width {
				if frame.NRGBAAt(x, y).A < 8 {
					continue
				}
				inside := false
				for i, region := range allowed {
					if image.Pt(x, y).In(region) {
						painted[i]++
						inside = true
					}
				}
				if !inside {
					t.Fatalf("%s paints outside the reference composition at %d,%d", theme.ID, x, y)
				}
			}
		}
		for i, count := range painted {
			if count < 10000 {
				t.Fatalf("%s region %d missing: %d pixels", theme.ID, i, count)
			}
		}
	}
}

func TestCatalogTextIsValidUTF8AndAmmoDoesNotInventMissingReserve(t *testing.T) {
	if !utf8.Valid(catalogJSON) {
		t.Fatal("catalog contains invalid UTF-8")
	}
	for _, theme := range Themes() {
		if strings.ContainsRune(theme.Description, '\uFFFD') {
			t.Fatalf("%s description contains replacement characters", theme.ID)
		}
	}
	r, _ := NewRenderer("arena")
	state := Example()
	state.Players[1].Ammo, state.Players[1].Reserve = 18, -1
	for _, n := range r.Scene(state, ExampleTarget) {
		if n.ID == "focus/ammo" && n.Text != "18/—" {
			t.Fatalf("unknown reserve became a factual count: %q", n.Text)
		}
	}
}
