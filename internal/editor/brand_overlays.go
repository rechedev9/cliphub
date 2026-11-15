package editor

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // brand images are PNG or JPG uploads
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"github.com/rechedev9/cliphub/internal/brandkit"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

const (
	brandFadeSeconds = 0.3
	// Full Demo banners sit under the custom HUD's top strip (it ends at y≈206) and the native scoreboard.
	fullDemoBannerTop       = 224
	fullDemoBannerMaxWidth  = 600
	fullDemoBannerMaxHeight = 96
)

// BrandWatermark is the creator's logo pinned to Corner for the whole Short.
// Width and Height are the image's own size, read when it is prepared.
type BrandWatermark struct {
	Path    string  `json:"path"`
	Corner  string  `json:"corner"`
	Opacity float64 `json:"opacity"`
	Width   int     `json:"width,omitempty"`
	Height  int     `json:"height,omitempty"`
}

// SponsorBanner is the creator's own sponsor image; EndSeconds 0 keeps it to the end.
type SponsorBanner struct {
	Path         string  `json:"path"`
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds,omitempty"`
	Width        int     `json:"width,omitempty"`
	Height       int     `json:"height,omitempty"`
}

// prepareBrandOverlays checks the brand flags and rewrites each image as a
// PNG in outDir, with the watermark's opacity applied to its alpha.
func prepareBrandOverlays(watermark *BrandWatermark, banner *SponsorBanner, outDir string) (*BrandWatermark, *SponsorBanner, error) {
	var outWatermark *BrandWatermark
	if watermark != nil {
		if err := brandkit.ValidateCorner(watermark.Corner); err != nil {
			return nil, nil, err
		}
		if err := brandkit.ValidateOpacity(watermark.Opacity); err != nil {
			return nil, nil, err
		}
		prepared := *watermark
		path := filepath.Join(outDir, "brand-watermark.png")
		w, h, err := bakeBrandImage(watermark.Path, path, watermark.Opacity)
		if err != nil {
			return nil, nil, fmt.Errorf("watermark image: %w", err)
		}
		prepared.Path, prepared.Width, prepared.Height = path, w, h
		outWatermark = &prepared
	}
	var outBanner *SponsorBanner
	if banner != nil {
		if err := brandkit.ValidateWindow(banner.StartSeconds, banner.EndSeconds); err != nil {
			return nil, nil, err
		}
		prepared := *banner
		path := filepath.Join(outDir, "brand-banner.png")
		w, h, err := bakeBrandImage(banner.Path, path, 1)
		if err != nil {
			return nil, nil, fmt.Errorf("sponsor banner image: %w", err)
		}
		prepared.Path, prepared.Width, prepared.Height = path, w, h
		outBanner = &prepared
	}
	return outWatermark, outBanner, nil
}

// bakeBrandImage writes src as a PNG at dst with its alpha scaled by opacity and returns its size.
func bakeBrandImage(src, dst string, opacity float64) (int, int, error) {
	// #nosec G304 -- src is a brand image the render worker materialized for this job.
	in, err := os.Open(src)
	if err != nil {
		return 0, 0, err
	}
	defer in.Close()
	decoded, _, err := image.Decode(in)
	if err != nil {
		return 0, 0, fmt.Errorf("decode: %w", err)
	}
	bounds := decoded.Bounds()
	baked := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.NRGBAModel.Convert(decoded.At(x, y)).(color.NRGBA)
			c.A = uint8(math.Round(float64(c.A) * opacity))
			baked.SetNRGBA(x-bounds.Min.X, y-bounds.Min.Y, c)
		}
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return 0, 0, err
	}
	out, err := os.Create(dst)
	if err != nil {
		return 0, 0, err
	}
	if err := png.Encode(out, baked); err != nil {
		_ = out.Close()
		return 0, 0, err
	}
	return bounds.Dx(), bounds.Dy(), out.Close()
}

// fitBox scales w×h to fit inside maxW×maxH, keeping the aspect and even sides for yuv420p.
func fitBox(w, h, maxW, maxH int) (int, int) {
	if w <= 0 || h <= 0 {
		return maxW &^ 1, maxH &^ 1
	}
	scale := math.Min(float64(maxW)/float64(w), float64(maxH)/float64(h))
	fw := max(2, int(math.Round(float64(w)*scale))&^1)
	fh := max(2, int(math.Round(float64(h)*scale))&^1)
	return fw, fh
}

// watermarkPlacement sizes the logo and places it in corner of a frameW×frameH frame.
// A 9:16 Short keeps its bottom corners above the Shorts player's caption band.
func watermarkPlacement(corner string, imageW, imageH, frameW, frameH int) (w, h, x, y int) {
	short := min(frameW, frameH)
	w, h = fitBox(imageW, imageH, short*16/100, short*10/100)
	margin := short * 4 / 100
	bottom := margin
	if frameH > frameW {
		bottom = frameH * 14 / 100
	}
	x, y = margin, margin
	if corner == brandkit.CornerTopRight || corner == brandkit.CornerBottomRight {
		x = frameW - w - margin
	}
	if corner == brandkit.CornerBottomLeft || corner == brandkit.CornerBottomRight {
		y = frameH - h - bottom
	}
	return w, h, x, y
}

// generatedBrandEffects draws a Short's watermark and sponsor banner; a Full Demo draws them per item.
func generatedBrandEffects(short ShortEdit) []Effect {
	if short.FullDemo != nil || short.DurationSeconds <= 0 {
		return nil
	}
	frameW, frameH := outputDimensions(short)
	var effects []Effect
	if wm := short.Overlays.Watermark; wm != nil && wm.Path != "" {
		w, h, x, y := watermarkPlacement(wm.Corner, wm.Width, wm.Height, frameW, frameH)
		effects = append(effects, Effect{
			Type: EffectImage, Path: wm.Path, X: strconv.Itoa(x), Y: strconv.Itoa(y), Width: w, Height: h,
			StartSeconds: 0, EndSeconds: short.DurationSeconds, FadeInSeconds: brandFadeSeconds, Source: "brand-watermark",
		})
	}
	if banner := short.Overlays.Banner; banner != nil && banner.Path != "" {
		start, end, ok := brandkit.Banner{StartSeconds: banner.StartSeconds, EndSeconds: banner.EndSeconds}.Window(short.DurationSeconds)
		if ok {
			maxW, maxH := frameW*80/100, frameH*12/100
			if frameW > frameH {
				maxW = frameW * 45 / 100
			}
			w, h := fitBox(banner.Width, banner.Height, maxW, maxH)
			// Centred where the affiliate plate sits, which a Short with this banner leaves out.
			y := int(math.Round(float64(frameH)*0.86)) - h/2
			effects = append(effects, Effect{
				Type: EffectImage, Path: banner.Path, X: strconv.Itoa((frameW - w) / 2), Y: strconv.Itoa(min(y, frameH-h)), Width: w, Height: h,
				StartSeconds: start, EndSeconds: end, FadeInSeconds: brandFadeSeconds, FadeOutSeconds: brandFadeSeconds, Source: "brand-banner",
			})
		}
	}
	return effects
}

// fullDemoBrandOverlays draws the plan's watermark and sponsor banner on a
// round item; the creator's own bumper clips stay as uploaded. The banner
// window is on the program clock, so it is shifted to the item's local time.
func fullDemoBrandOverlays(short ShortEdit, item recapplan.TimelineItem, command []string, video string) ([]string, string, error) {
	brand := short.FullDemo.Effective.Options.Overlays.Brand
	if item.Role == "bumper" || (brand.Watermark == nil && brand.Banner == nil) {
		return command, video, nil
	}
	const frameW, frameH = 1920, 1080
	if wm := brand.Watermark; wm != nil {
		path, err := short.fullDemo.execution.assetPath(recapplan.AssetRef{ID: wm.Image.ID, SHA256: wm.Image.SHA256})
		if err != nil {
			return nil, "", err
		}
		width, height, err := brandImageSize(path)
		if err != nil {
			return nil, "", err
		}
		w, h, x, y := watermarkPlacement(wm.Corner, width, height, frameW, frameH)
		index := fullDemoInputCount(command)
		command = append(command, "-i", path)
		video += fmt.Sprintf("[brandwmbase];[%d:v]format=rgba,scale=%d:%d:flags=lanczos,colorchannelmixer=aa=%s,setsar=1[brandwm];[brandwmbase][brandwm]overlay=x=%d:y=%d:eof_action=repeat:repeatlast=1:format=auto,format=yuv420p",
			index, w, h, strconv.FormatFloat(wm.Opacity, 'f', 3, 64), x, y)
	}
	if banner := brand.Banner; banner != nil {
		itemStart := float64(item.StartFrame) / recapplan.OutputFPS
		itemEnd := float64(item.EndFrame) / recapplan.OutputFPS
		start, end, ok := banner.Window(math.Inf(1))
		if !ok || end <= itemStart || start >= itemEnd {
			return command, video, nil
		}
		path, err := short.fullDemo.execution.assetPath(recapplan.AssetRef{ID: banner.Image.ID, SHA256: banner.Image.SHA256})
		if err != nil {
			return nil, "", err
		}
		width, height, err := brandImageSize(path)
		if err != nil {
			return nil, "", err
		}
		w, h := fitBox(width, height, fullDemoBannerMaxWidth, fullDemoBannerMaxHeight)
		index := fullDemoInputCount(command)
		command = append(command, "-i", path)
		video += fmt.Sprintf("[brandbnbase];[%d:v]format=rgba,scale=%d:%d:flags=lanczos,setsar=1[brandbn];[brandbnbase][brandbn]overlay=x=%d:y=%d:eof_action=repeat:repeatlast=1:format=auto:enable='%s',format=yuv420p",
			index, w, h, (frameW-w)/2, fullDemoBannerTop, betweenExpression(max(0, start-itemStart), math.Min(end, itemEnd)-itemStart))
	}
	return command, video, nil
}

func brandImageSize(path string) (int, int, error) {
	// #nosec G304 -- path is a verified Full Demo asset materialized for this render.
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, fmt.Errorf("brand image: %w", err)
	}
	return cfg.Width, cfg.Height, nil
}

// singleClipBrandImages are the brand images of a one-clip Short. That path
// draws no other image effect, so only the brand kit is added to it.
func singleClipBrandImages(short ShortEdit) []Effect {
	var out []Effect
	for _, effect := range imageEffects(short.Effects) {
		if effect.Source == "brand-watermark" || effect.Source == "brand-banner" {
			out = append(out, effect)
		}
	}
	return out
}

func appendBrandImageInputs(command []string, short ShortEdit) []string {
	for _, effect := range singleClipBrandImages(short) {
		command = append(command, "-i", effect.Path)
	}
	return command
}

// singleClipImageInputStart is the first brand image input: after the clip and the looped music.
func singleClipImageInputStart(short ShortEdit) int {
	if short.MusicPath != "" {
		return 2
	}
	return 1
}
