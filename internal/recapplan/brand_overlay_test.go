package recapplan

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/brandkit"
)

func TestBrandOverlayIsARenderOnlyImageChoice(t *testing.T) {
	o := fixtureOptions()
	plain, err := Plan(fixtureFacts(), o, VoiceEvidence{Availability: "not_requested"}, nil, "facts")
	if err != nil {
		t.Fatal(err)
	}
	logo := brandkit.ImageRef{ID: "66666666-6666-4666-8666-666666666666", SHA256: strings.Repeat("6", 64)}
	banner := brandkit.ImageRef{ID: "77777777-7777-4777-8777-777777777777", SHA256: strings.Repeat("7", 64)}
	o.Overlays.Brand = BrandOverlay{
		Watermark: &brandkit.Watermark{Image: logo, Corner: brandkit.CornerBottomLeft, Opacity: 0.6},
		Banner:    &brandkit.Banner{Image: banner, StartSeconds: 30, EndSeconds: 45},
	}
	missing, err := Plan(fixtureFacts(), o, VoiceEvidence{Availability: "not_requested"}, nil, "facts")
	if err != nil || len(missing.Blockers) != 2 {
		t.Fatalf("brand images without evidence: %+v %v", missing.Blockers, err)
	}
	assets := []AssetEvidence{
		{Ref: AssetRef{ID: logo.ID, SHA256: logo.SHA256}, HasImage: true, Title: "logo.png"},
		{Ref: AssetRef{ID: banner.ID, SHA256: banner.SHA256}, HasImage: true, Title: "banner.png"},
	}
	d, err := Plan(fixtureFacts(), o, VoiceEvidence{Availability: "not_requested"}, assets, "facts")
	if err != nil || len(d.Blockers) != 0 {
		t.Fatalf("brand plan: %+v %v", d.Blockers, err)
	}
	before, _ := plain.CaptureHash()
	after, _ := d.CaptureHash()
	if before != after || d.PlanHash == plain.PlanHash || d.Options.Overlays.Brand.Watermark == nil {
		t.Fatal("the brand kit must change the render and keep the capture")
	}
	if refs := d.Options.AssetReferences(); len(refs) != 2 || !d.Options.IsOverlayImage(refs[0]) {
		t.Fatalf("brand images are not overlay image references: %+v", refs)
	}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var restored Document
	if err := decodeStrict(raw, &restored); err != nil || restored.Options.Overlays.Brand.Banner.EndSeconds != 45 {
		t.Fatalf("round trip: %v %+v", err, restored.Options.Overlays.Brand)
	}
	o.Overlays.Brand.Watermark.Opacity = 0
	if _, err := Plan(fixtureFacts(), o, VoiceEvidence{Availability: "not_requested"}, assets, "facts"); err == nil {
		t.Fatal("a transparent watermark was accepted")
	}
}
