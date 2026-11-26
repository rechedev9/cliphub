package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/brandkit"
	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/overlayassets"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

// BrandKitRepository is the store of the creator's one brand kit.
type BrandKitRepository interface {
	GetBrandKit(ctx context.Context) (brandkit.Kit, error)
	SaveBrandKit(ctx context.Context, kit brandkit.Kit) (brandkit.Kit, error)
}

// WithBrandKit enables /api/brand-kit and the brand_watermark and brand_banner render choices.
func WithBrandKit(repo BrandKitRepository) Option {
	return func(h *Handlers) {
		h.brandKit = repo
	}
}

// errBrandKitUnavailable is a render that asks for the kit on a build without one.
var errBrandKitUnavailable = errors.New("the brand kit is not available")

type brandKitBody struct {
	brandkit.Kit
	Corners []string `json:"corners"`
}

func (h *Handlers) brandKitReady(w http.ResponseWriter) bool {
	if h.brandKit == nil {
		writeCodedError(w, http.StatusServiceUnavailable, "not_configured", errBrandKitUnavailable.Error())
		return false
	}
	return true
}

// GetBrandKit handles GET /api/brand-kit: the saved kit, or the default one before the first save.
func (h *Handlers) GetBrandKit(w http.ResponseWriter, r *http.Request) {
	if !h.brandKitReady(w) {
		return
	}
	kit, err := h.brandKit.GetBrandKit(r.Context())
	if err != nil {
		internalError(w, "get brand kit", err)
		return
	}
	writeJSON(w, http.StatusOK, brandKitBody{Kit: kit, Corners: brandkit.Corners})
}

// SaveBrandKit handles PUT /api/brand-kit. Images are uploaded first to /api/full-demo/overlay-images.
func (h *Handlers) SaveBrandKit(w http.ResponseWriter, r *http.Request) {
	if !h.brandKitReady(w) {
		return
	}
	var kit brandkit.Kit
	if err := decodeSingleJSONBody(w, r, &kit, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid brand kit JSON")
		return
	}
	if err := kit.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	for _, ref := range []*brandkit.ImageRef{kit.Logo, kit.Banner} {
		if ref == nil {
			continue
		}
		if err := h.verifyBrandImage(r.Context(), *ref); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	saved, err := h.brandKit.SaveBrandKit(r.Context(), kit)
	if err != nil {
		internalError(w, "save brand kit", err)
		return
	}
	writeJSON(w, http.StatusOK, brandKitBody{Kit: saved, Corners: brandkit.Corners})
}

// verifyBrandImage checks that ref names a stored overlay image whose bytes still hash to it.
func (h *Handlers) verifyBrandImage(ctx context.Context, ref brandkit.ImageRef) error {
	id, err := uuid.Parse(ref.ID)
	if err != nil {
		return err
	}
	image, err := overlayassets.Load(h.storage, id)
	if err != nil || image.SHA256 != ref.SHA256 {
		return fmt.Errorf("%w: image %s is not an uploaded overlay image", brandkit.ErrMissing, ref.ID)
	}
	if err := mediaassets.VerifyContent(ctx, h.storage, overlayassets.MediaKey(id), ref.SHA256, overlayassets.MaxBytes); err != nil {
		return fmt.Errorf("%w: image %s changed: %v", brandkit.ErrMissing, ref.ID, err)
	}
	return nil
}

// brandChoice is a render request's brand field. true or false turns the saved
// kit on or off; an object is the copy an earlier render made, sent back by a retry.
type brandChoice[T any] struct {
	On       bool
	Snapshot *T
}

func (c *brandChoice[T]) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &c.On); err == nil {
		return nil
	}
	var snapshot T
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&snapshot); err != nil {
		return fmt.Errorf("brand choice must be true, false or a saved copy: %w", err)
	}
	c.On, c.Snapshot = true, &snapshot
	return nil
}

// applyBrandKit sets a Short's watermark and banner from the request: a saved
// copy as it is, true from the kit, false off; a field left out keeps the base edit's.
func (h *Handlers) applyBrandKit(ctx context.Context, patch renderEditRequest, edit *renderplan.EditRequest) error {
	var kit *brandkit.Kit
	loadKit := func() (brandkit.Kit, error) {
		if kit != nil {
			return *kit, nil
		}
		if h.brandKit == nil {
			return brandkit.Kit{}, errBrandKitUnavailable
		}
		loaded, err := h.brandKit.GetBrandKit(ctx)
		kit = &loaded
		return loaded, err
	}
	if c := patch.BrandWatermark; c != nil {
		watermark, err := resolveBrandChoice(*c, func() (brandkit.Watermark, error) {
			k, err := loadKit()
			if err != nil {
				return brandkit.Watermark{}, err
			}
			return k.Watermark()
		})
		if err != nil {
			return err
		}
		if watermark != nil {
			if err := h.verifyBrandImage(ctx, watermark.Image); err != nil {
				return err
			}
		}
		edit.BrandWatermark = watermark
	}
	if c := patch.BrandBanner; c != nil {
		banner, err := resolveBrandChoice(*c, func() (brandkit.Banner, error) {
			k, err := loadKit()
			if err != nil {
				return brandkit.Banner{}, err
			}
			return k.SponsorBanner()
		})
		if err != nil {
			return err
		}
		if banner != nil {
			if err := h.verifyBrandImage(ctx, banner.Image); err != nil {
				return err
			}
		}
		edit.BrandBanner = banner
	}
	if c := patch.SubscribeBanner; c != nil {
		card, err := resolveBrandChoice(*c, func() (brandkit.Subscribe, error) {
			k, err := loadKit()
			return k.SubscribeCard(), err
		})
		if err != nil {
			return err
		}
		if card != nil && card.Logo != nil {
			if err := h.verifyBrandImage(ctx, *card.Logo); err != nil {
				return err
			}
		}
		edit.SubscribeBanner = card
	}
	return nil
}

func resolveBrandChoice[T any](c brandChoice[T], fromKit func() (T, error)) (*T, error) {
	if !c.On {
		return nil, nil
	}
	if c.Snapshot != nil {
		return c.Snapshot, nil
	}
	value, err := fromKit()
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func writeBrandKitError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, brandkit.ErrMissing):
		writeCodedError(w, http.StatusConflict, "brand_kit_incomplete", err.Error())
	case errors.Is(err, errBrandKitUnavailable):
		writeCodedError(w, http.StatusServiceUnavailable, "not_configured", err.Error())
	default:
		internalError(w, "read brand kit", err)
	}
}
