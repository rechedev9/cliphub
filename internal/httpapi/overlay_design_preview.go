package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/hudthemes"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

// GetOverlayDesignPreview handles GET /api/full-demo/overlay-designs/preview:
// the example match in the neon or hud intro design (outro=1 for the final
// scoreboard), as an SVG of the renderer's own page. The hud design reads the
// HUD like the HUD preview does; without base it is the native CS2 HUD.
func (h *Handlers) GetOverlayDesignPreview(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	o := recapplan.OverlayOptions{}
	switch q.Get("design") {
	case "neon":
		o.Theme = recapplan.OverlayThemeNeon
	case "hud":
		o.Theme = recapplan.OverlayThemeHUD
	default:
		writeError(w, http.StatusBadRequest, "design must be neon or hud")
		return
	}
	if base := q.Get("base"); base != "" {
		if _, ok := customhud.Lookup(base); !ok {
			writeError(w, http.StatusBadRequest, "base must be a HUD design id from the catalog")
			return
		}
		style, err := hudPreviewStyle(q)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		o.HUDTheme, o.HUDCustom = base, style
	}
	doc := demooverlay.PreviewDocument()
	doc.Theme, doc.Palette = demooverlay.NormalizeTheme(o.Theme), demooverlay.DesignPalette(o)
	page, err := demooverlay.DesignHTML(doc, demooverlay.DesignView{Outro: q.Get("outro") == "1"})
	if err != nil {
		internalError(w, "overlay design preview", err)
		return
	}
	svg, err := demooverlay.PreviewSVG(page)
	if err != nil {
		internalError(w, "overlay design preview", err)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; font-src data:; img-src data:")
	_, _ = w.Write(svg)
}

// hudPreviewStyle reads a user theme's colours and radius from a preview query; nil without colours.
func hudPreviewStyle(q url.Values) (*hudthemes.Style, error) {
	if !q.Has("ct") && !q.Has("t") && !q.Has("background") {
		return nil, nil
	}
	style := hudthemes.Style{Name: "Vista previa", CT: q.Get("ct"), T: q.Get("t"), Background: q.Get("background")}
	if raw := q.Get("radius"); raw != "" {
		radius, err := strconv.Atoi(raw)
		if err != nil {
			return nil, errors.New("radius must be a number")
		}
		style.Radius = &radius
	}
	clean, err := style.Clean()
	if err != nil {
		return nil, err
	}
	return &clean, nil
}
