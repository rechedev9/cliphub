// Package radarmap holds the per-map radar calibration CS2 ships in
// resource/overviews/<map>.txt and the world -> radar-pixel transform every
// 2D view of a demo depends on.
//
// The package carries numbers, never art: no radar image is bundled, and the
// transform is defined so that a caller who later supplies an official
// overview image gets pixel-exact alignment for free. Maps with no calibration
// are not an error; DeriveCalibration builds an equivalent transform from the
// positions actually observed in the demo, at the cost of a framing that is
// only stable within that demo.
package radarmap

import (
	"fmt"
	"math"
	"sort"
)

// Level names the vertical section a world position belongs to. CS2 draws
// split-level maps as two separate overview images selected by altitude.
const (
	LevelDefault = "default"
	LevelLower   = "lower"
)

// SourceOverview marks a calibration transcribed from the map's shipped
// overview file; SourceDerived marks one computed from observed positions.
const (
	SourceOverview = "overview"
	SourceDerived  = "derived"
)

// Calibration is one map's overview transform. PosX/PosY are the world
// coordinate of the radar image's upper-left corner and Scale is world units
// per native radar pixel, exactly as CS2 defines them, so
// (world-pos)/scale yields native radar pixels.
type Calibration struct {
	Map    string  `json:"map"`
	Source string  `json:"source"`
	PosX   float64 `json:"pos_x"`
	PosY   float64 `json:"pos_y"`
	Scale  float64 `json:"scale"`
	// Size is the native radar resolution in pixels. It is square for every
	// map CS2 ships; since the May 2025 update some maps are 2048 rather than
	// 1024, always with a halved Scale, so the two fields must be read together.
	Size int `json:"size"`
	// LowerAltitudeMax is the AltitudeMax of the lower vertical section for
	// split-level maps. Nil means the map has a single level.
	LowerAltitudeMax *float64 `json:"lower_altitude_max,omitempty"`
}

// calibrations holds the values from CS2's resource/overviews/<map>.txt for the
// competitive map pool. rotate/zoom are deliberately absent: they only affect
// the in-game HUD radar, not the flat overview, so applying them would skew
// every plotted position.
var calibrations = map[string]Calibration{
	"de_mirage":   {Map: "de_mirage", PosX: -3230, PosY: 1713, Scale: 5.0, Size: 1024},
	"de_inferno":  {Map: "de_inferno", PosX: -2087, PosY: 3870, Scale: 4.9, Size: 1024},
	"de_dust2":    {Map: "de_dust2", PosX: -2476, PosY: 3239, Scale: 4.4, Size: 1024},
	"de_ancient":  {Map: "de_ancient", PosX: -2953, PosY: 2164, Scale: 5.0, Size: 1024},
	"de_anubis":   {Map: "de_anubis", PosX: -2796, PosY: 3328, Scale: 5.22, Size: 1024},
	"de_overpass": {Map: "de_overpass", PosX: -4831, PosY: 1781, Scale: 5.2, Size: 1024},
	"de_nuke":     {Map: "de_nuke", PosX: -3453, PosY: 2887, Scale: 7.0, Size: 1024, LowerAltitudeMax: altitude(-495)},
	"de_train":    {Map: "de_train", PosX: -2308, PosY: 2078, Scale: 4.082077, Size: 1024, LowerAltitudeMax: altitude(-50)},
	"de_vertigo":  {Map: "de_vertigo", PosX: -3168, PosY: 1762, Scale: 4.0, Size: 1024, LowerAltitudeMax: altitude(11700)},
}

func altitude(v float64) *float64 { return &v }

// Lookup returns the shipped calibration for a map name. The second result
// reports whether the map is calibrated; callers must not fall back to an
// identity transform, which would silently misplace every position.
func Lookup(mapName string) (Calibration, bool) {
	c, ok := calibrations[mapName]
	if !ok {
		return Calibration{}, false
	}
	c.Source = SourceOverview
	return c, true
}

// Maps returns the calibrated map names, sorted.
func Maps() []string {
	names := make([]string, 0, len(calibrations))
	for name := range calibrations {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Bounds is an axis-aligned world-space rectangle.
type Bounds struct {
	MinX float64 `json:"min_x"`
	MinY float64 `json:"min_y"`
	MaxX float64 `json:"max_x"`
	MaxY float64 `json:"max_y"`
}

// Empty reports whether the bounds cover no area, which is what an
// uninitialised or single-point accumulation looks like.
func (b Bounds) Empty() bool {
	return !(b.MaxX > b.MinX && b.MaxY > b.MinY)
}

// DeriveCalibration builds a calibration that frames bounds inside a size×size
// radar with the given world-unit margin on every side. It exists for maps CS2
// has not published an overview for (workshop maps, new releases): the
// resulting framing is stable for a fixed bounds input but is not comparable
// with another demo's, so callers must record Source and warn.
func DeriveCalibration(mapName string, b Bounds, size int, margin float64) (Calibration, error) {
	if b.Empty() {
		return Calibration{}, fmt.Errorf("derive calibration for %q: no observed positions", mapName)
	}
	if size <= 0 {
		return Calibration{}, fmt.Errorf("derive calibration for %q: size %d must be positive", mapName, size)
	}
	if margin < 0 {
		return Calibration{}, fmt.Errorf("derive calibration for %q: margin %v must not be negative", mapName, margin)
	}
	width := (b.MaxX - b.MinX) + 2*margin
	height := (b.MaxY - b.MinY) + 2*margin
	// One scale for both axes keeps the map square, the way every official
	// overview is drawn; the shorter axis is centred in the leftover space.
	scale := math.Max(width, height) / float64(size)
	covered := scale * float64(size)
	return Calibration{
		Map:    mapName,
		Source: SourceDerived,
		PosX:   b.MinX - margin - (covered-width)/2,
		PosY:   b.MaxY + margin + (covered-height)/2,
		Scale:  scale,
		Size:   size,
	}, nil
}

// WorldToPixel converts world coordinates to native radar pixels. The Y axis
// inverts: world Y grows north, radar Y grows down.
func (c Calibration) WorldToPixel(worldX, worldY float64) (float64, float64) {
	return (worldX - c.PosX) / c.Scale, (c.PosY - worldY) / c.Scale
}

// PixelToWorld is the inverse of WorldToPixel.
func (c Calibration) PixelToWorld(pixelX, pixelY float64) (float64, float64) {
	return pixelX*c.Scale + c.PosX, c.PosY - pixelY*c.Scale
}

// Level reports which vertical section a world altitude belongs to. Maps
// without a lower section always answer LevelDefault.
func (c Calibration) Level(worldZ float64) string {
	if c.LowerAltitudeMax != nil && worldZ <= *c.LowerAltitudeMax {
		return LevelLower
	}
	return LevelDefault
}

// MultiLevel reports whether the map is drawn as two overview images.
func (c Calibration) MultiLevel() bool { return c.LowerAltitudeMax != nil }

// Valid reports whether the calibration can be used for a transform.
func (c Calibration) Valid() bool { return c.Scale > 0 && c.Size > 0 }
