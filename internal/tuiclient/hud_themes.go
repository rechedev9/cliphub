package tuiclient

import (
	"context"
	"net/http"
	"net/url"

	"github.com/rechedev9/cliphub/internal/hudthemes"
)

const hudThemesPath = "/api/full-demo/hud-themes"

// HUDThemes is the user's own HUD themes and how many Studio keeps.
type HUDThemes struct {
	Themes []hudthemes.Theme `json:"themes"`
	Max    int               `json:"max"`
}

func (c *Client) ListHUDThemes(ctx context.Context) (HUDThemes, error) {
	var out HUDThemes
	err := c.getJSON(ctx, hudThemesPath, &out)
	return out, err
}

// CreateHUDTheme duplicates the catalog design base with style.
func (c *Client) CreateHUDTheme(ctx context.Context, base string, style hudthemes.Style) (hudthemes.Theme, error) {
	var out hudthemes.Theme
	err := c.doJSON(ctx, http.MethodPost, hudThemesPath, struct {
		Base string `json:"base"`
		hudthemes.Style
	}{base, style}, &out)
	return out, err
}

func (c *Client) UpdateHUDTheme(ctx context.Context, id string, style hudthemes.Style) (hudthemes.Theme, error) {
	var out hudthemes.Theme
	err := c.doJSON(ctx, http.MethodPut, hudThemesPath+"/"+url.PathEscape(id), style, &out)
	return out, err
}

func (c *Client) DeleteHUDTheme(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, hudThemesPath+"/"+url.PathEscape(id), nil, nil)
}
