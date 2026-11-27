package renderplan

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/brandkit"
)

func TestEditRequestBrandKitRules(t *testing.T) {
	image := brandkit.ImageRef{ID: "0b5d1b6e-8a3c-4f0e-9d8a-1c2b3d4e5f60", SHA256: strings.Repeat("a", 64)}
	edit := DefaultEditRequest()
	edit.BrandWatermark = &brandkit.Watermark{Image: image, Corner: brandkit.CornerBottomRight, Opacity: 0.7}
	edit.BrandBanner = &brandkit.Banner{Image: image, StartSeconds: 1}
	if err := edit.Validate(); err != nil {
		t.Fatalf("a branded Short was rejected: %v", err)
	}
	doc := NewEditDocument(NewEditDocumentOptions{JobID: uuid.New(), Edit: edit})
	if !doc.Layers.Watermark {
		t.Fatal("the edit document does not record the watermark layer")
	}
	if NewEditDocument(NewEditDocumentOptions{JobID: uuid.New(), Edit: DefaultEditRequest()}).Layers.Watermark {
		t.Fatal("an unbranded Short records a watermark layer")
	}
	withPlate := edit
	withPlate.KeyDropStyle = "classic"
	if err := withPlate.Validate(); err == nil {
		t.Fatal("the sponsor banner and the affiliate plate were accepted on one Short")
	}
	recap := edit
	recap.MatchRecap = true
	if err := recap.Validate(); err == nil {
		t.Fatal("a match recap took the brand kit outside its approved plan")
	}
	bad := edit
	bad.BrandWatermark = &brandkit.Watermark{Image: image, Corner: "center", Opacity: 0.7}
	if err := bad.Validate(); err == nil {
		t.Fatal("an unknown watermark corner was accepted")
	}
}

func TestEditRequestSubscribeBannerRules(t *testing.T) {
	logo := brandkit.ImageRef{ID: "0b5d1b6e-8a3c-4f0e-9d8a-1c2b3d4e5f60", SHA256: strings.Repeat("a", 64)}
	edit := DefaultEditRequest()
	for _, card := range []brandkit.Subscribe{{}, {ChannelName: "ClipHub RSN"}, {Logo: &logo, ChannelName: "ClipHub RSN"}} {
		edit.SubscribeBanner = &card
		if err := edit.Validate(); err != nil {
			t.Fatalf("subscribe banner %+v was rejected: %v", card, err)
		}
	}
	recap := edit
	recap.MatchRecap = true
	if err := recap.Validate(); err == nil {
		t.Fatal("a match recap took the subscribe banner outside its approved plan")
	}
	bad := edit
	bad.SubscribeBanner = &brandkit.Subscribe{ChannelName: " ClipHub"}
	if err := bad.Validate(); err == nil {
		t.Fatal("an untrimmed channel name was accepted")
	}
}
