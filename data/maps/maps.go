// Package mapdata embeds ClipHub's own map radars and shots so Studio shows
// them without reading the CS2 install: cleaner images that the app controls.
package mapdata

import (
	"embed"
	"strings"
)

const (
	ContentTypePNG  = "image/png"
	ContentTypeWebP = "image/webp"
)

//go:embed radars/*.png
var radarFiles embed.FS

//go:embed shots/*.webp
var shotFiles embed.FS

// Radar returns the bundled overview radar for a map and layer, or nil when
// the bundle has none. Layer is "upper" or "lower". Every radar is square and
// spans the map's radarmap calibration square, at twice CS2's 1024 pixels.
func Radar(mapName, layer string) []byte {
	base := strings.TrimPrefix(mapName, "de_")
	base = strings.TrimPrefix(base, "cs_")
	base = strings.TrimPrefix(base, "ar_")
	var candidates []string
	if layer == "lower" {
		candidates = []string{base + "_lower.png"}
	} else {
		candidates = []string{base + "_upper.png", base + ".png"}
	}
	for _, name := range candidates {
		data, err := radarFiles.ReadFile("radars/" + name)
		if err == nil {
			return data
		}
	}
	return nil
}

// Shot returns the bundled loading-screen picture for a map, or nil when the
// bundle has none.
func Shot(mapName string) []byte {
	data, err := shotFiles.ReadFile("shots/" + mapName + ".webp")
	if err == nil {
		return data
	}
	return nil
}
