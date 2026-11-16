package tuiclient

import (
	"context"
	"net/http"

	"github.com/rechedev9/cliphub/internal/brandkit"
)

const brandKitPath = "/api/brand-kit"

// BrandKit is the saved brand kit and the watermark corners Studio accepts.
type BrandKit struct {
	brandkit.Kit
	Corners []string `json:"corners"`
}

func (c *Client) GetBrandKit(ctx context.Context) (BrandKit, error) {
	var out BrandKit
	err := c.getJSON(ctx, brandKitPath, &out)
	return out, err
}

// SaveBrandKit replaces the kit; its images come from UploadFullDemoImage.
func (c *Client) SaveBrandKit(ctx context.Context, kit brandkit.Kit) (BrandKit, error) {
	var out BrandKit
	err := c.doJSON(ctx, http.MethodPut, brandKitPath, kit, &out)
	return out, err
}
