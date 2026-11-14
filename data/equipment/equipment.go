// Package equipmentdata embeds CS2's equipment icons as sanitized SVG, so the
// replay and the grenade lists draw them without a CS2 install.
package equipmentdata

import "embed"

const ContentTypeSVG = "image/svg+xml"

//go:embed icons/*.svg
var iconFiles embed.FS

// Icon returns the bundled SVG for an equipment name such as smokegrenade,
// or nil when the bundle has none.
func Icon(name string) []byte {
	data, err := iconFiles.ReadFile("icons/" + name + ".svg")
	if err != nil {
		return nil
	}
	return data
}
