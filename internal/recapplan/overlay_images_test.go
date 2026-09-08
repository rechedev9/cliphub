package recapplan

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestScreenshotPlanRequiresEveryEnabledImageAndPreservesCapture(t *testing.T) {
	o := fixtureOptions()
	o.Overlays.Roster, o.Overlays.Scoreboard = true, true
	generated, err := Plan(fixtureFacts(), o, VoiceEvidence{Availability: "not_requested"}, nil, "facts")
	if err != nil {
		t.Fatal(err)
	}
	o.Overlays.Mode = "screenshots"
	missing, err := Plan(fixtureFacts(), o, VoiceEvidence{Availability: "not_requested"}, nil, "facts")
	if err != nil || len(missing.Blockers) != 3 {
		t.Fatalf("missing images: %+v %v", missing.Blockers, err)
	}
	refs := []AssetRef{{uuid.NewString(), strings.Repeat("1", 64)}, {uuid.NewString(), strings.Repeat("2", 64)}, {uuid.NewString(), strings.Repeat("3", 64)}}
	o.Overlays.Team1Image, o.Overlays.Team2Image, o.Overlays.ScoreboardImage = &refs[0], &refs[1], &refs[2]
	assets := []AssetEvidence{}
	for _, ref := range refs {
		assets = append(assets, AssetEvidence{Ref: ref, HasImage: true, Title: "captura.png"})
	}
	d, err := Plan(fixtureFacts(), o, VoiceEvidence{Availability: "not_requested"}, assets, "facts")
	if err != nil || len(d.Blockers) != 0 {
		t.Fatalf("image plan: %+v %v", d.Blockers, err)
	}
	before, _ := generated.CaptureHash()
	after, _ := d.CaptureHash()
	if before != after || !reflect.DeepEqual(generated.Timeline, d.Timeline) || d.PlanHash == generated.PlanHash {
		t.Fatal("image choice must change rendering, preserve capture and timeline")
	}
	if !reflect.DeepEqual(o.AssetReferences(), refs) {
		t.Fatalf("references: %+v", o.AssetReferences())
	}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var restored Document
	if err := decodeStrict(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if err := restored.Validate(); err != nil {
		t.Fatal(err)
	}
	if restored.PlanHash != d.PlanHash || !reflect.DeepEqual(restored.Options.Overlays, o.Overlays) {
		t.Fatal("saved plan lost image choices")
	}
	assets[1].HasImage = false
	assets[1].HasVideo = true
	invalid, err := Plan(fixtureFacts(), o, VoiceEvidence{Availability: "not_requested"}, assets, "facts")
	if err != nil || len(invalid.Blockers) == 0 {
		t.Fatal("accepted video as team screenshot")
	}
	o.Overlays.Roster = false
	if refs := o.AssetReferences(); len(refs) != 1 || refs[0] != *o.Overlays.ScoreboardImage {
		t.Fatalf("disabled intro requires images: %+v", refs)
	}
	o.Overlays.Mode = "generated"
	if len(o.AssetReferences()) != 0 {
		t.Fatal("inactive screenshots leaked into generated plan")
	}
}

func TestOldOverlayOptionsKeepTheirHashAndRejectUnknownModes(t *testing.T) {
	o := fixtureOptions()
	before, _ := HashValue(o)
	raw, _ := json.Marshal(o)
	overlayRaw, _ := json.Marshal(o.Overlays)
	if strings.Contains(string(overlayRaw), `"mode"`) || strings.Contains(string(overlayRaw), `"team1_image"`) {
		t.Fatal("legacy options acquired new fields")
	}
	var restored Options
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	after, _ := HashValue(restored)
	if before != after {
		t.Fatal("legacy approval hash changed")
	}
	restored.Overlays.Mode = "unknown"
	if restored.Validate() == nil {
		t.Fatal("unknown mode accepted")
	}
}
