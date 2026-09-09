package customhud

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/mediafont"
)

// RasterizePreview recovers straight RGBA from the export renderer over two
// opaque mattes. FFmpeg's ass alpha=1 blends the alpha plane as a color channel:
// a 50% panel otherwise has 25% alpha and dark edges in a browser. Opaque YUV
// inputs also match the export's color conversion, unlike RGB subtitle inputs.
func RasterizePreview(ctx context.Context, ffmpeg, ass string) (*image.NRGBA, error) {
	dir, err := os.MkdirTemp("", "cliphub-hud-preview-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "preview.ass")
	if err := os.WriteFile(file, []byte(ass), 0600); err != nil {
		return nil, err
	}
	fonts, err := mediafont.MaterializeHUD()
	if err != nil {
		return nil, err
	}
	filter := ASSFilter(file, fonts, false)
	graph := "[0:v]" + filter + "[black];[1:v]" + filter + "[white];[black][white]hstack=inputs=2,format=rgb24[out]"
	cmd := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=black:s=1920x1080:r=60,format=yuv444p",
		"-f", "lavfi", "-i", "color=white:s=1920x1080:r=60,format=yuv444p",
		"-filter_complex", graph, "-map", "[out]", "-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	raw, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("rasterize HUD preview: %w: %s", err, stderr.String())
	}
	if len(raw) != Width*Height*6 {
		return nil, fmt.Errorf("invalid HUD preview raster size %d", len(raw))
	}
	return previewMattes(raw), nil
}

func previewMattes(raw []byte) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, Width, Height))
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			black := (y*Width*2 + x) * 3
			white := black + Width*3
			delta := 0
			for c := 0; c < 3; c++ {
				delta += int(raw[white+c]) - int(raw[black+c])
			}
			alpha := max(0, min(255, 255-(delta+1)/3))
			pixel := y*img.Stride + x*4
			img.Pix[pixel+3] = uint8(alpha)
			if alpha == 0 {
				continue
			}
			for c := 0; c < 3; c++ {
				img.Pix[pixel+c] = uint8(min(255, (int(raw[black+c])*255+alpha/2)/alpha))
			}
		}
	}
	return img
}
