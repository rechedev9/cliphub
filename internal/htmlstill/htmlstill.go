// Package htmlstill draws a self-contained HTML page into one transparent PNG
// with Studio's bundled Chromium, and wraps such a page as an SVG preview.
package htmlstill

import (
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Size is the viewport of a page and the pixel size of its PNG.
type Size struct {
	Width, Height int
}

// Landscape is the Full Demo frame and Portrait the 9:16 Short frame; the
// renderer (desktop/src/overlay-render-request.ts) admits only these two.
var (
	Landscape = Size{Width: 1920, Height: 1080}
	Portrait  = Size{Width: 1080, Height: 1920}
)

// Render writes the page as a PNG of exactly size through the renderer named by ZV_OVERLAY_RENDERER_PATH.
func Render(html []byte, outPath string, size Size) error {
	renderer := strings.TrimSpace(os.Getenv("ZV_OVERLAY_RENDERER_PATH"))
	if renderer == "" {
		return fmt.Errorf("neon overlay requires the Studio Chromium renderer (ZV_OVERLAY_RENDERER_PATH)")
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0750); err != nil {
		return err
	}
	dir, err := os.MkdirTemp(filepath.Dir(outPath), ".neon-overlay-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	htmlPath, err := filepath.Abs(filepath.Join(dir, "overlay.html"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(htmlPath, html, 0600); err != nil {
		return err
	}
	outputPath, err := filepath.Abs(outPath)
	if err != nil {
		return err
	}
	requestPath, err := filepath.Abs(filepath.Join(dir, "request.json"))
	if err != nil {
		return err
	}
	body, err := json.Marshal(struct {
		HTMLPath   string `json:"html_path"`
		OutputPath string `json:"output_path"`
		Width      int    `json:"width"`
		Height     int    `json:"height"`
	}{htmlPath, outputPath, size.Width, size.Height})
	if err != nil {
		return err
	}
	if err := os.WriteFile(requestPath, body, 0600); err != nil {
		return err
	}
	args := []string{}
	if app := strings.TrimSpace(os.Getenv("ZV_OVERLAY_RENDERER_APP")); app != "" {
		args = append(args, app)
	}
	args = append(args, "--cliphub-render-overlay", requestPath)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, renderer, args...)
	for _, env := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(env), "ELECTRON_RUN_AS_NODE=") {
			cmd.Env = append(cmd.Env, env)
		}
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Chromium overlay render: %w: %s", err, strings.TrimSpace(string(out)))
	}
	f, err := os.Open(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()
	config, err := png.DecodeConfig(f)
	if err != nil {
		return fmt.Errorf("Chromium overlay is not a PNG: %w", err)
	}
	if config.Width != size.Width || config.Height != size.Height {
		return fmt.Errorf("Chromium overlay is %dx%d, want %dx%d", config.Width, config.Height, size.Width, size.Height)
	}
	return nil
}

// The embedded fonts would make each thumbnail about 2 MB, so previews draw with the fallback font.
var pageFontFaces = regexp.MustCompile(`@font-face\{[^}]*\}`)

// PreviewSVG wraps a page as a scalable SVG image, so an <img> shows the
// renderer's own page. The page body must be well-formed XHTML.
func PreviewSVG(page []byte, size Size) ([]byte, error) {
	html := pageFontFaces.ReplaceAllString(string(page), "")
	styleStart, styleEnd := strings.Index(html, "<style>"), strings.Index(html, "</style>")
	bodyStart, bodyEnd := strings.Index(html, "<body"), strings.LastIndex(html, "</body>")
	if styleStart < 0 || styleEnd < styleStart || bodyStart < styleEnd || bodyEnd < bodyStart {
		return nil, fmt.Errorf("overlay page has no style and body to preview")
	}
	var out strings.Builder
	fmt.Fprintf(&out, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d"><foreignObject width="%d" height="%d"><html xmlns="http://www.w3.org/1999/xhtml"><head>`,
		size.Width, size.Height, size.Width, size.Height, size.Width, size.Height)
	out.WriteString(html[styleStart : styleEnd+len("</style>")])
	out.WriteString("</head>")
	out.WriteString(html[bodyStart : bodyEnd+len("</body>")])
	out.WriteString("</html></foreignObject></svg>")
	return []byte(out.String()), nil
}
