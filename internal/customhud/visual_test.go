package customhud

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/mediafont"
)

func isolatedSceneASS(t *testing.T, nodes []Node) string {
	t.Helper()
	r, err := NewRenderer("arena")
	if err != nil {
		t.Fatal(err)
	}
	ass, err := r.ASS(testTimeline(), Window{StartTick: 100, Frames: 60})
	if err != nil {
		t.Fatal(err)
	}
	header, _, ok := strings.Cut(ass, "Dialogue:")
	if !ok {
		t.Fatal("missing ASS events")
	}
	for _, node := range nodes {
		header += fmt.Sprintf("Dialogue: %d,0:00:00.00,0:00:01.00,HUD,,0,0,0,,%s\n", node.Layer, assNode(node))
	}
	return header
}

func TestPreviewPreservesOpacityAndInteriorIconCutouts(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg unavailable")
	}
	r, err := NewRenderer("arena")
	if err != nil {
		t.Fatal(err)
	}
	s := &scene{r: r}
	s.rect("half", 20, 20, 100, 100, "FFFFFF", 1)
	s.fade(.5)
	s.icon("skull", "status/icon_skull_default", 160, 20, 128, 128, "FFFFFF", false)
	s.typeText("text", "HUD", 350, 70, 52, 300, 500, "FFFFFF", "left")
	s.fade(.5)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	frame, err := RasterizePreview(ctx, ffmpeg, isolatedSceneASS(t, s.nodes))
	if err != nil {
		t.Fatal(err)
	}
	pixel := frame.NRGBAAt(50, 50)
	if pixel.A < 126 || pixel.A > 129 || pixel.R < 250 || pixel.G < 250 || pixel.B < 250 {
		t.Fatalf("50%% white became dark or lost opacity: %+v", pixel)
	}
	// Coordinates lie inside the original silhouette's forehead and eye.
	if frame.NRGBAAt(224, 50).A < 250 || frame.NRGBAAt(200, 75).A > 2 {
		t.Fatal("skull silhouette lost its opaque outline or transparent eye")
	}
	maxAlpha := uint8(0)
	for y := 25; y < 110; y++ {
		for x := 345; x < 500; x++ {
			maxAlpha = max(maxAlpha, frame.NRGBAAt(x, y).A)
		}
	}
	if maxAlpha < 126 || maxAlpha > 129 {
		t.Fatalf("text opacity=%d", maxAlpha)
	}
	if frame.NRGBAAt(800, 400).A != 0 {
		t.Fatal("preview painted the gameplay area")
	}
}

func TestGrenadeSilhouettesStayInsideTheirSlots(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg unavailable")
	}
	r, err := NewRenderer("arena")
	if err != nil {
		t.Fatal(err)
	}
	s := &scene{r: r}
	names := []string{"flashbang", "hegrenade", "smokegrenade"}
	var slots []image.Rectangle
	for i, name := range names {
		x := 100 + i*160
		if !s.icon(name, "weapons/"+name, x, 100, 64, 64, "FFFFFF", false) {
			t.Fatalf("missing %s", name)
		}
		slots = append(slots, image.Rect(x-1, 99, x+65, 165))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	frame, err := RasterizePreview(ctx, ffmpeg, isolatedSceneASS(t, s.nodes))
	if err != nil {
		t.Fatal(err)
	}
	counts := make([]int, len(slots))
	outside := 0
	for y := range frame.Bounds().Dy() {
		for x := range frame.Bounds().Dx() {
			if frame.NRGBAAt(x, y).A < 8 {
				continue
			}
			inside := false
			for i, slot := range slots {
				if image.Pt(x, y).In(slot) {
					counts[i]++
					inside = true
				}
			}
			if !inside {
				outside++
			}
		}
	}
	for i, count := range counts {
		if count < 200 {
			t.Errorf("%s has only %d visible pixels inside its slot", names[i], count)
		}
	}
	if outside > 0 {
		t.Errorf("grenade artwork paints %d pixels outside its assigned slots", outside)
	}
}

func TestHUDWeightsResolveToDistinctBundledFaces(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg unavailable")
	}
	r, err := NewRenderer("arena")
	if err != nil {
		t.Fatal(err)
	}
	s := &scene{r: r}
	for i, weight := range []int{500, 600, 700} {
		s.typeText(fmt.Sprint(weight), "WIDE 1234", 40, 60+i*80, 48, 800, weight, "FFFFFF", "left")
	}
	s.typeText("cyrillic", "Игрок", 40, 320, 40, 800, 600, "FFFFFF", "left")
	if s.nodes[3].Font != mediafont.FamilyName {
		t.Fatal("Cyrillic did not select the bundled fallback")
	}
	file := filepath.Join(t.TempDir(), "font-weights.ass")
	if err := os.WriteFile(file, []byte(isolatedSceneASS(t, s.nodes)), 0600); err != nil {
		t.Fatal(err)
	}
	fonts, err := mediafont.MaterializeHUD()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-loglevel", "verbose", "-f", "lavfi", "-i", "color=black:s=1920x1080,format=yuv444p", "-vf", ASSFilter(file, fonts, false), "-frames:v", "1", "-f", "null", "-")
	var output bytes.Buffer
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("font render: %v\n%s", err, output.String())
	}
	for _, face := range []string{"Medium", "SemiBold", "Bold"} {
		match := regexp.MustCompile(`fontselect: \(Barlow Semi Condensed ` + face + `, [^\n]+\) -> [^\n]*BarlowSemiCondensed-` + face + `\b`)
		if !match.Match(output.Bytes()) {
			t.Fatalf("%s selected a different face:\n%s", face, output.String())
		}
	}
}

func TestWeaponSilhouettesKeepUnknownEquipmentUnknown(t *testing.T) {
	if _, err := NewRenderer("arena"); err != nil {
		t.Fatal(err)
	}
	for name, icon := range map[string]string{"AK-47": "ak47", "M4A1-S": "m4a1_silencer", "M4A1": "m4a1_silencer", "M4A4": "m4a1", "weapon_m4a1": "m4a1", "USP-S": "usp_silencer", "Desert Eagle": "deagle", "Galil AR": "galilar", "Dual Berettas": "elite", "Five-SeveN": "fiveseven", "SG 553": "sg556", "Knife": "knife", "GLOCK-18": "glock", "DECOY GRENADE": "decoy"} {
		if got := weaponIcon(name); got != "weapons/"+icon {
			t.Fatalf("%s silhouette=%s", name, got)
		}
	}
	if weaponIcon("UNKNOWN EQUIPMENT") != "" {
		t.Fatal("unknown weapon got an unrelated icon")
	}
}

func TestDamageFadePreservesSourceClockAcrossTrim(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg unavailable")
	}
	r, err := NewRenderer("arena")
	if err != nil {
		t.Fatal(err)
	}
	timeline := testTimeline()
	var bar Node
	for _, n := range r.Scene(timeline.Snapshots[1], ExampleTarget) {
		if n.ID == "focus/bar" {
			bar = n
		}
	}
	if bar.W == 0 {
		t.Fatal("missing health bar")
	}
	fonts, err := mediafont.MaterializeHUD()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	render := func(name string, offset int64, frames [2]int) []byte {
		t.Helper()
		ass, err := r.ASS(timeline, Window{StartTick: 100, SourceOffsetFrames: offset, Frames: 90})
		if err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(t.TempDir(), name+".ass")
		if err := os.WriteFile(file, []byte(ass), 0600); err != nil {
			t.Fatal(err)
		}
		filter := ASSFilter(file, fonts, false) + fmt.Sprintf(",select='eq(n,%d)+eq(n,%d)',format=rgb24,crop=1:1:%d:%d", frames[0], frames[1], bar.X+bar.W+4, bar.Y+1)
		cmd := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=black:s=1920x1080:r=60,format=yuv444p", "-vf", filter, "-frames:v", "2", "-fps_mode", "vfr", "-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		raw, err := cmd.Output()
		if err != nil || len(raw) != 6 {
			t.Fatalf("damage fade raster: %v, %d bytes\n%s", err, len(raw), stderr.String())
		}
		return raw
	}
	full, trim := render("full", 0, [2]int{65, 69}), render("trim", 65, [2]int{0, 4})
	if int(full[0])-int(full[3]) < 30 {
		t.Fatalf("damage cue did not fade: %v", full)
	}
	for i := range full {
		if delta := int(full[i]) - int(trim[i]); delta < -3 || delta > 3 {
			t.Fatalf("trim shifted damage fade: full=%v trim=%v", full, trim)
		}
	}
}
