package mapgeo

import "math"

// Fog is the map's fog as CS2 layers it: a gradient fog of one colour, then a
// cubemap fog coloured by the blurred sky. Each fades in with the distance from
// the camera and with the height, both raised to their exponents, as
// ValveResourceFormat's fog shader reads env_gradient_fog and env_cubemap_fog.
type Fog struct {
	Gradient *GradientFog `json:"gradient"`
	Cube     *CubeFog     `json:"cube"`
}

// FogRamp is one fade of a fog, raised to Exponent: over distance it is 0 at
// Start and 1 at End; over height 1 at Start and 0 at End, and an equal Start
// and End keep every height in the fog.
type FogRamp struct {
	Start    float64 `json:"start"`
	End      float64 `json:"end"`
	Exponent float64 `json:"exponent"`
}

// GradientFog is an env_gradient_fog; Color is linear and already times its strength.
type GradientFog struct {
	Distance FogRamp    `json:"distance"`
	Height   FogRamp    `json:"height"`
	Color    [3]float64 `json:"color"`
	Max      float64    `json:"max"`
}

// CubeFog is an env_cubemap_fog coloured by the map's sky; LodBias sharpens the sky where the fog is thick.
type CubeFog struct {
	Distance FogRamp `json:"distance"`
	Height   FogRamp `json:"height"`
	Max      float64 `json:"max"`
	LodBias  float64 `json:"lodBias"`
}

func gradientFogFromEntity(vals map[string]any) *GradientFog {
	f := &GradientFog{
		Distance: FogRamp{Start: numberValue(vals["fogstart"], 0), End: numberValue(vals["fogend"], 0), Exponent: numberValue(vals["fogfalloffexponent"], 1)},
		Max:      numberValue(vals["fogmaxopacity"], 1),
	}
	if f.Distance.End <= f.Distance.Start {
		return nil
	}
	// Height fog turned off leaves the height term at 1, as an equal start and end with a zero exponent do.
	if _, has := vals["heightfog"]; !has || truthy(vals["heightfog"]) {
		f.Height = FogRamp{Start: numberValue(vals["fogstartheight"], 0), End: numberValue(vals["fogendheight"], 0), Exponent: numberValue(vals["fogverticalexponent"], 1)}
	}
	strength := numberValue(vals["fogstrength"], 1)
	c, ok := vec3Value(vals["fogcolor"])
	if !ok {
		c = [3]float64{255, 255, 255}
	}
	for i := range c {
		f.Color[i] = srgbToLinear(c[i]/255) * strength
	}
	return f
}

func cubeFogFromEntity(vals map[string]any) *CubeFog {
	f := &CubeFog{
		Distance: FogRamp{Start: numberValue(vals["cubemapfogstartdistance"], 0), End: numberValue(vals["cubemapfogenddistance"], 0), Exponent: numberValue(vals["cubemapfogfalloffexponent"], 1)},
		Max:      numberValue(vals["cubemapfogmaxopacity"], 1),
		LodBias:  numberValue(vals["cubemapfoglodbiase"], 0),
	}
	if f.Distance.End <= f.Distance.Start {
		return nil
	}
	_, hasExponent := vals["cubemapfogheightexponent"]
	if v, has := vals["cubemapheightfog"]; has && !truthy(v) || !has && !hasExponent {
		return f
	}
	start := numberValue(vals["cubemapfogheightstart"], 0)
	end, hasEnd := vals["cubemapfogheightend"]
	f.Height = FogRamp{Start: start, End: numberValue(end, 0), Exponent: numberValue(vals["cubemapfogheightexponent"], 1)}
	if !hasEnd {
		f.Height.End = start + numberValue(vals["cubemapfogheightwidth"], 0)
	}
	return f
}

// exposureScale is 2 to the power of a material's exposure bias, as CS2 brightens a sky by it.
func exposureScale(bias float32) float64 {
	return math.Exp2(float64(bias))
}
