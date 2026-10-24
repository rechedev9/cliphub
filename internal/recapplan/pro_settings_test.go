package recapplan

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCanonicalNewOptionsResolveProSettingsFromTheCatalog(t *testing.T) {
	options := DefaultOptions()
	// A stale or edited client value must not reach the capture.
	options.Capture.ProSettings = ProSettings{PlayerID: "donk", Name: "fake", Width: 640, Height: 480, ViewmodelFOV: 90}
	canonical, err := CanonicalNewOptions(options)
	if err != nil {
		t.Fatal(err)
	}
	got := canonical.Capture.ProSettings
	if got.Name != "donk" || got.Width != 1280 || got.Height != 960 || got.CaptureWidth != 1440 || got.CaptureHeight != 1080 || got.ViewmodelFOV != 68 {
		t.Fatalf("pro settings not taken from the catalog: %+v", got)
	}
	if err := canonical.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := canonical.ValidateCurrentFullDemoPolicy(); err != nil {
		t.Fatal(err)
	}
	if w, h := canonical.Capture.CaptureSize(); w != 1440 || h != 1080 || !canonical.Capture.StretchToFrame() {
		t.Fatalf("capture %dx%d stretch=%v", w, h, canonical.Capture.StretchToFrame())
	}
}

func TestCanonicalNewOptionsRejectUnknownProPlayer(t *testing.T) {
	options := DefaultOptions()
	options.Capture.ProSettings.PlayerID = "nobody"
	if _, err := CanonicalNewOptions(options); err == nil || !strings.Contains(err.Error(), "nobody") {
		t.Fatalf("unknown pro accepted: %v", err)
	}
}

// Documents without a pro keep their wire shape, so approved hashes stay valid.
func TestCaptureWithoutProSettingsKeepsItsWireShape(t *testing.T) {
	options := DefaultOptions()
	b, err := json.Marshal(options.Capture)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "pro_settings") {
		t.Fatalf("empty pro settings serialized: %s", b)
	}
	if w, h := options.Capture.CaptureSize(); w != 1920 || h != 1080 || options.Capture.StretchToFrame() {
		t.Fatalf("standard capture %dx%d", w, h)
	}
}

func TestValidateRejectsInconsistentProCaptureSize(t *testing.T) {
	settings, err := ResolveProSettings("zywoo")
	if err != nil {
		t.Fatal(err)
	}
	settings.CaptureWidth = 1920
	if err := settings.Validate(); err == nil {
		t.Fatal("a 4:3 pro captured at 16:9 was accepted")
	}
}
