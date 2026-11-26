package demooverlay

import "embed"

const (
	HookLabel       = "MEJOR JUGADA"
	HookLabelLeft   = 344
	HookLabelTop    = 44
	HookLabelHeight = 48
	HookSlidePixels = 420
)

//go:embed hook.html
var hookAssets embed.FS

func HookHTML(palette *Palette) ([]byte, error) {
	fonts, err := loadPageFonts()
	if err != nil {
		return nil, err
	}
	return htmlPage(hookAssets, "hook.html", struct {
		pageFonts
		Palette           *Palette
		Label             string
		Left, Top, Height int
	}{fonts, palette, HookLabel, HookLabelLeft, HookLabelTop, HookLabelHeight})
}

func RenderHookPNG(palette *Palette, outPath string) error {
	html, err := HookHTML(palette)
	if err != nil {
		return err
	}
	return renderHTMLStill(html, outPath)
}
