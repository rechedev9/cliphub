package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/brandkit"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const maxBrandImageSize = 10 << 20

// brandChoiceIn are the plan_full_demo switches that copy the saved brand kit into the plan.
type brandChoiceIn struct {
	BrandWatermark *bool `json:"brand_watermark,omitempty" jsonschema:"draw the channel logo of the brand kit (get_brand_kit) over the rounds, in its corner and opacity; false removes it; needs a saved logo"`
	BrandBanner    *bool `json:"brand_banner,omitempty" jsonschema:"show the sponsor banner of the brand kit under the scoreboard in its time window of the video; false removes it; needs a saved banner"`
	watermark      *brandkit.Watermark
	banner         *brandkit.Banner
}

type brandKitOut struct {
	HasLogo            bool     `json:"has_logo" jsonschema:"a channel logo is saved, so brand_watermark can be used"`
	WatermarkCorner    string   `json:"watermark_corner"`
	WatermarkOpacity   int      `json:"watermark_opacity_percent"`
	HasBanner          bool     `json:"has_banner" jsonschema:"a sponsor banner is saved, so brand_banner can be used"`
	BannerStartSeconds float64  `json:"banner_start_seconds"`
	BannerEndSeconds   float64  `json:"banner_end_seconds,omitempty" jsonschema:"0 or absent keeps the banner to the end of the video"`
	ChannelName        string   `json:"channel_name,omitempty" jsonschema:"channel name the SUBSCRIBE banner shows beside the logo"`
	Corners            []string `json:"corners" jsonschema:"values for watermark_corner"`
	Summary            string   `json:"summary"`
}

type saveBrandKitIn struct {
	Logo               string   `json:"logo,omitempty" jsonschema:"absolute path of a PNG or JPG up to 10 MB with the channel logo, or none to remove it; left out keeps the saved one"`
	WatermarkCorner    string   `json:"watermark_corner,omitempty" jsonschema:"top-left, top-right, bottom-left or bottom-right; left out keeps the saved one"`
	WatermarkOpacity   *int     `json:"watermark_opacity_percent,omitempty" jsonschema:"watermark opacity in percent, 10 to 100; left out keeps the saved one"`
	Banner             string   `json:"banner,omitempty" jsonschema:"absolute path of a PNG or JPG up to 10 MB with the sponsor banner, or none to remove it; left out keeps the saved one"`
	BannerStartSeconds *float64 `json:"banner_start_seconds,omitempty" jsonschema:"second of the video when the banner appears; left out keeps the saved one"`
	BannerEndSeconds   *float64 `json:"banner_end_seconds,omitempty" jsonschema:"second of the video when the banner leaves; 0 keeps it to the end; left out keeps the saved one"`
	ChannelName        *string  `json:"channel_name,omitempty" jsonschema:"YouTube channel name, up to 30 characters, shown with the logo on the SUBSCRIBE banner (generate_short subscribe_banner, plan_full_demo subscribe_button); empty removes it; left out keeps the saved one"`
}

func describeBrandKit(kit tuiclient.BrandKit) brandKitOut {
	out := brandKitOut{HasLogo: kit.Logo != nil, WatermarkCorner: kit.WatermarkCorner, WatermarkOpacity: int(math.Round(kit.WatermarkOpacity * 100)),
		HasBanner: kit.Banner != nil, BannerStartSeconds: kit.BannerStartSeconds, BannerEndSeconds: kit.BannerEndSeconds, ChannelName: kit.ChannelName, Corners: kit.Corners}
	parts := []string{"Kit de marca: sin logo", "sin banner de sponsor"}
	if out.HasLogo {
		parts[0] = fmt.Sprintf("Kit de marca: logo como marca de agua en la esquina %s al %d %%", out.WatermarkCorner, out.WatermarkOpacity)
	}
	if out.HasBanner {
		window := fmt.Sprintf("desde el segundo %g hasta el final", out.BannerStartSeconds)
		if out.BannerEndSeconds > 0 {
			window = fmt.Sprintf("del segundo %g al %g", out.BannerStartSeconds, out.BannerEndSeconds)
		}
		parts[1] = "banner de sponsor " + window
	}
	if out.ChannelName != "" {
		parts = append(parts, fmt.Sprintf("canal %q en el banner de Subscribe", out.ChannelName))
	}
	out.Summary = strings.Join(parts, ", ") + "."
	return out
}

func registerBrandKitTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "get_brand_kit", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read the creator's saved brand kit: the channel logo drawn as a watermark (corner and opacity), the own sponsor banner (time window) and the channel name. " +
			"generate_short and plan_full_demo apply it with brand_watermark and brand_banner; the logo and channel name also brand generate_short subscribe_banner and plan_full_demo subscribe_button. " +
			"Each render copies the kit, so later edits do not change it."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, brandKitOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, brandKitOut{}, toolError(err)
			}
			kit, err := c.GetBrandKit(ctx)
			if err != nil {
				return nil, brandKitOut{}, toolError(err)
			}
			return nil, describeBrandKit(kit), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "save_brand_kit",
		Description: "Save the creator's brand kit, as Kit de marca in the Studio video options. Logo and banner are local PNG or JPG files that Studio uploads; " +
			"none removes one. Fields left out keep the saved value. Videos already rendered or planned keep their own copy."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in saveBrandKitIn) (*mcp.CallToolResult, brandKitOut, error) {
			for _, f := range []struct{ field, path string }{{"logo", in.Logo}, {"banner", in.Banner}} {
				if f.path != "" && f.path != noneChoice {
					if err := checkUploadFile(f.field, f.path, portraitExtensions, maxBrandImageSize); err != nil {
						return nil, brandKitOut{}, err
					}
				}
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, brandKitOut{}, toolError(err)
			}
			current, err := c.GetBrandKit(ctx)
			if err != nil {
				return nil, brandKitOut{}, toolError(err)
			}
			kit, err := mergeBrandKit(current.Kit, in)
			if err != nil {
				return nil, brandKitOut{}, err
			}
			if (in.Logo != "" && in.Logo != noneChoice) || (in.Banner != "" && in.Banner != noneChoice) {
				uploader, err := s.uploadClient(ctx)
				if err != nil {
					return nil, brandKitOut{}, toolError(err)
				}
				if kit.Logo, err = uploadBrandImage(ctx, uploader, in.Logo, kit.Logo); err != nil {
					return nil, brandKitOut{}, err
				}
				if kit.Banner, err = uploadBrandImage(ctx, uploader, in.Banner, kit.Banner); err != nil {
					return nil, brandKitOut{}, err
				}
			}
			saved, err := c.SaveBrandKit(ctx, kit)
			if err != nil {
				return nil, brandKitOut{}, toolError(err)
			}
			return nil, describeBrandKit(saved), nil
		})
}

// mergeBrandKit applies the fields a save_brand_kit call sets; uploads come after it.
func mergeBrandKit(kit brandkit.Kit, in saveBrandKitIn) (brandkit.Kit, error) {
	kit = kit.Copy()
	if in.Logo == noneChoice {
		kit.Logo = nil
	}
	if in.Banner == noneChoice {
		kit.Banner = nil
	}
	if in.WatermarkCorner != "" {
		kit.WatermarkCorner = strings.ToLower(strings.TrimSpace(in.WatermarkCorner))
	}
	if in.WatermarkOpacity != nil {
		kit.WatermarkOpacity = float64(*in.WatermarkOpacity) / 100
	}
	if in.BannerStartSeconds != nil {
		kit.BannerStartSeconds = *in.BannerStartSeconds
	}
	if in.BannerEndSeconds != nil {
		kit.BannerEndSeconds = *in.BannerEndSeconds
	}
	if in.ChannelName != nil {
		kit.ChannelName = strings.TrimSpace(*in.ChannelName)
	}
	if err := kit.Validate(); err != nil {
		return brandkit.Kit{}, fmt.Errorf("these brand kit values cannot be saved: %w", err)
	}
	return kit, nil
}

func uploadBrandImage(ctx context.Context, c *tuiclient.Client, path string, current *brandkit.ImageRef) (*brandkit.ImageRef, error) {
	if path == "" || path == noneChoice {
		return current, nil
	}
	asset, err := c.UploadFullDemoImage(ctx, path)
	if err != nil {
		return nil, toolError(err)
	}
	return &brandkit.ImageRef{ID: asset.ID, SHA256: asset.SHA256}, nil
}

// resolveBrandKit reads the saved kit when plan_full_demo turns a brand switch on.
func resolveBrandKit(ctx context.Context, c *tuiclient.Client, in *brandChoiceIn) error {
	if !orDefault(in.BrandWatermark, false) && !orDefault(in.BrandBanner, false) {
		return nil
	}
	kit, err := c.GetBrandKit(ctx)
	if err != nil {
		return toolError(err)
	}
	if orDefault(in.BrandWatermark, false) {
		watermark, err := kit.Watermark()
		if err != nil {
			return errors.New("brand_watermark needs a saved channel logo; save one with save_brand_kit")
		}
		in.watermark = &watermark
	}
	if orDefault(in.BrandBanner, false) {
		banner, err := kit.SponsorBanner()
		if err != nil {
			return errors.New("brand_banner needs a saved sponsor banner; save one with save_brand_kit")
		}
		in.banner = &banner
	}
	return nil
}

// applyBrandChoice puts the resolved kit in the plan; a switch left out keeps the plan's value.
func applyBrandChoice(options *recapplan.Options, in brandChoiceIn) {
	if in.BrandWatermark != nil {
		options.Overlays.Brand.Watermark = in.watermark
	}
	if in.BrandBanner != nil {
		options.Overlays.Brand.Banner = in.banner
	}
}

// describeBrand is the plan summary part for the brand kit.
func describeBrand(b recapplan.BrandOverlay) string {
	var parts []string
	if b.Watermark != nil {
		parts = append(parts, "tu marca de agua")
	}
	if b.Banner != nil {
		parts = append(parts, "tu banner de sponsor")
	}
	if len(parts) == 0 {
		return ""
	}
	return " Con " + strings.Join(parts, " y ") + "."
}
