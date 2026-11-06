package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/prosettings"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

func TestCheckFullDemoExtras(t *testing.T) {
	dir := t.TempDir()
	portrait := filepath.Join(dir, "me.png")
	clip := filepath.Join(dir, "intro.mp4")
	for _, f := range []string{portrait, clip} {
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pro := prosettings.Catalog()[0].ID
	ok := []planFullDemoIn{
		{fullDemoExtrasIn: fullDemoExtrasIn{ProSettings: pro, IntroBumper: clip, OutroBumper: noneChoice}},
		{HUD: "focus", fullDemoExtrasIn: fullDemoExtrasIn{HUDPortrait: portrait}},
		{fullDemoExtrasIn: fullDemoExtrasIn{ProSettings: noneChoice, HUDPortrait: noneChoice}},
	}
	for _, in := range ok {
		if err := checkFullDemoExtras(in, recapplan.DefaultOptions()); err != nil {
			t.Errorf("%+v refused: %v", in.fullDemoExtrasIn, err)
		}
	}
	bad := map[string]planFullDemoIn{
		"portrait without focus": {HUD: "native", fullDemoExtrasIn: fullDemoExtrasIn{HUDPortrait: portrait}},
		"unknown pro":            {fullDemoExtrasIn: fullDemoExtrasIn{ProSettings: "nobody"}},
		"bumper not mp4":         {fullDemoExtrasIn: fullDemoExtrasIn{SponsorBumper: portrait}},
		"missing bumper":         {fullDemoExtrasIn: fullDemoExtrasIn{IntroBumper: filepath.Join(dir, "gone.mp4")}},
		"relative portrait":      {HUD: "focus", fullDemoExtrasIn: fullDemoExtrasIn{HUDPortrait: "me.png"}},
		"bad round with a clip":  {Rounds: []int{0}, fullDemoExtrasIn: fullDemoExtrasIn{IntroBumper: clip}},
		"bad level with a clip":  {GameLevel: ptrTo(500), fullDemoExtrasIn: fullDemoExtrasIn{OutroBumper: clip}},
	}
	for name, in := range bad {
		if err := checkFullDemoExtras(in, recapplan.DefaultOptions()); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestApplyFullDemoExtrasWithoutUploads(t *testing.T) {
	pro := prosettings.Catalog()[0]
	clip := &recapplan.AssetRef{ID: "a", SHA256: strings.Repeat("0", 64)}
	options := recapplan.Options{Bumpers: &recapplan.BumperOptions{
		Intro:   recapplan.BumperSlot{Enabled: true, Video: clip},
		Sponsor: &recapplan.BumperSlot{Enabled: true, Video: clip},
	}}
	in := planFullDemoIn{fullDemoExtrasIn: fullDemoExtrasIn{ProSettings: pro.ID, SponsorBumper: noneChoice}}
	if err := applyFullDemoExtras(context.Background(), nil, &options, in); err != nil {
		t.Fatal(err)
	}
	gotPro, _, bumpers := describeExtras(options)
	if gotPro != pro.Name || !slices.Equal(bumpers, []string{"intro"}) || options.Bumpers.Sponsor != nil {
		t.Errorf("pro %q, bumpers %v, sponsor %+v", gotPro, bumpers, options.Bumpers.Sponsor)
	}
	if err := applyFullDemoExtras(context.Background(), nil, &options, planFullDemoIn{fullDemoExtrasIn: fullDemoExtrasIn{ProSettings: noneChoice}}); err != nil {
		t.Fatal(err)
	}
	if options.Capture.ProSettings.PlayerID != "" {
		t.Errorf("pro settings not cleared: %+v", options.Capture.ProSettings)
	}
}
