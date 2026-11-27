package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/rechedev9/cliphub/internal/brandkit"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

var generateBrandFields = []string{"brand_watermark", "brand_banner", "subscribe_banner"}

type generateEditRequest struct {
	renderplan.EditRequest
	brand renderEditRequest
}

type generateBrandChoices struct {
	BrandWatermark  *brandChoice[brandkit.Watermark] `json:"brand_watermark"`
	BrandBanner     *brandChoice[brandkit.Banner]    `json:"brand_banner"`
	SubscribeBanner *brandChoice[brandkit.Subscribe] `json:"subscribe_banner"`
}

func (g *generateEditRequest) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return g.EditRequest.UnmarshalJSON(data)
	}
	if _, err := renderplan.HasFullDemoJSON(data); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	brandOnly := map[string]json.RawMessage{}
	for _, name := range generateBrandFields {
		if value, ok := fields[name]; ok {
			brandOnly[name] = value
			delete(fields, name)
		}
	}
	rest, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	if err := g.EditRequest.UnmarshalJSON(rest); err != nil {
		return err
	}
	raw, err := json.Marshal(brandOnly)
	if err != nil {
		return err
	}
	var choices generateBrandChoices
	if err := json.Unmarshal(raw, &choices); err != nil {
		return err
	}
	g.brand = renderEditRequest{BrandWatermark: choices.BrandWatermark, BrandBanner: choices.BrandBanner, SubscribeBanner: choices.SubscribeBanner}
	return nil
}

func (h *Handlers) validateGenerateIntent(ctx context.Context, w http.ResponseWriter, edit generateEditRequest, intent *renderplan.GenerateIntent) bool {
	if err := h.applyBrandKit(ctx, edit.brand, &intent.Edit); err != nil {
		writeBrandKitError(w, err)
		return false
	}
	if err := intent.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return false
	}
	return true
}
