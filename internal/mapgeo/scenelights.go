package mapgeo

import "math"

// Light is a light CS2 shades at runtime, with its shadows baked into a channel
// of the lightmap: a barn light (a spot shaped by a frustum) or an omni2 (a
// point light). The irradiance lightmap holds only their bounce. The fields are
// the constants ValveResourceFormat gives its barn light shader.
type Light struct {
	// Omni is true for a point light; a barn light is lit through its frustum.
	Omni bool `json:"omni"`
	// Color is linear, times the intensity that falls off as Falloff over the squared distance from Eye.
	Color   [3]float64 `json:"color"`
	Eye     [3]float64 `json:"eye"`
	Falloff float64    `json:"falloff"`
	// Forward, Right and Up are the light's axes; a barn light looks along Forward.
	Forward [3]float64 `json:"forward"`
	Right   [3]float64 `json:"right"`
	Up      [3]float64 `json:"up"`
	// Frustum is the half width and height of the barn frustum at Near, then its centre; Near 0 is an orthographic frustum.
	Frustum [4]float64 `json:"frustum"`
	Near    float64    `json:"near"`
	Far     float64    `json:"far"`
	// Skirt fades the frustum in from its near plane and out toward its far plane, as rates; 0 is no fade.
	Skirt [2]float64 `json:"skirt"`
	// Fade is the distance fade's offset and rate, Angle the angle fade's bias and scale.
	Fade  [2]float64 `json:"fade"`
	Angle [2]float64 `json:"angle"`
	// Shadow is the lightmap shadow channel, -1 for none.
	Shadow int `json:"shadow"`
	// Min and Max bound what the light reaches.
	Min [3]float64 `json:"min"`
	Max [3]float64 `json:"max"`
}

// linearBrightnessDistance is how far in front of its luminaire a light reaches its linear brightness.
const linearBrightnessDistance = 100.0

// lightEntityClasses are the lights CS2 can shade at runtime.
var lightEntityClasses = map[string]bool{"light_barn": true, "light_omni2": true}

// lightFromEntity reads a dynamic or stationary barn or omni2 light; baked and disabled lights are skipped.
func lightFromEntity(class string, vals map[string]any) (Light, bool) {
	direct := int(numberValue(vals["directlight"], 2))
	enabled, has := vals["enabled"]
	if !lightEntityClasses[class] || direct < 2 || entityFlag(vals["StartDisabled"]) || has && !entityFlag(enabled) {
		return Light{}, false
	}
	angles, _ := vec3Value(vals["angles"])
	origin, _ := vec3Value(vals["origin"])
	at := angleAffine(angles, origin, [3]float64{1, 1, 1})
	l := Light{Omni: class == "light_omni2", Shadow: int(numberValue(vals["bakedshadowindex"], -1))}
	for i := 0; i < 3; i++ {
		l.Forward[i], l.Up[i] = at[i][0], at[i][2]
	}
	l.Right = normalize3(cross3(l.Up, l.Forward))
	color := lightColor(vals)
	intensity := math.Exp2(numberValue(vals["brightness"], 0)) * numberValue(vals["brightnessscale"], 1)
	rng := numberValue(vals["range"], 512)
	skirt := numberValue(vals["skirt"], 0.1)
	if l.Omni {
		l.Eye, l.Falloff = origin, 1
		intensity *= linearBrightnessDistance * linearBrightnessDistance
		l.Fade = distanceFade(rng, skirt)
		l.Angle = angleFade(numberValue(vals["outer_angle"], 180), numberValue(vals["inner_angle"], 0))
		for i := 0; i < 3; i++ {
			l.Min[i], l.Max[i] = origin[i]-rng, origin[i]+rng
		}
	} else if !barnGeometry(&l, vals, origin, rng, skirt) {
		return Light{}, false
	} else if l.Falloff > 0 {
		ref := math.Sqrt(l.Falloff) + linearBrightnessDistance
		intensity *= ref * ref / l.Falloff
	}
	for i := range color {
		l.Color[i] = color[i] * intensity
	}
	return l, intensity > 0
}

// entityFlag reads a boolean key, which entity lumps hold as a bool, a number or text.
func entityFlag(v any) bool {
	if s, ok := v.(string); ok {
		return s == "true" || parseFloatOr(s, 0) != 0
	}
	return truthy(v)
}

// lightColor is the light's linear colour, from its colour temperature when it is set by one.
func lightColor(vals map[string]any) [3]float64 {
	c, ok := vec3Value(vals["color"])
	if !ok {
		c = [3]float64{255, 255, 255}
	}
	srgb := [3]float64{c[0] / 255, c[1] / 255, c[2] / 255}
	if numberValue(vals["colormode"], 0) != 0 {
		srgb = colorTemperature(numberValue(vals["colortemperature"], 6600))
	}
	return [3]float64{srgbToLinear(srgb[0]), srgbToLinear(srgb[1]), srgbToLinear(srgb[2])}
}

// colorTemperature is the sRGB colour of a black body, as ValveResourceFormat's ColorTemperatureToSrgb fits it.
func colorTemperature(kelvin float64) [3]float64 {
	t := kelvin / 100
	red, blue := 255.0, 255.0
	green := 99.4708*math.Log(t) - 161.11957
	if t > 66 {
		red = 329.69873 * math.Pow(t-60, -0.13320476)
		green = 288.12216 * math.Pow(t-60, -0.075514846)
	}
	if t < 66 {
		blue = 138.51773*math.Log(t-10) - 305.0448
	}
	channel := func(v float64) float64 {
		if math.IsNaN(v) {
			return 0
		}
		return math.Min(math.Max(v, 0), 255) / 255
	}
	return [3]float64{channel(red), channel(green), channel(blue)}
}

// barnGeometry sets a barn light's frustum, eye and fades; false when its size is not usable.
func barnGeometry(l *Light, vals map[string]any, origin [3]float64, rng, skirt float64) bool {
	size, _ := vec3Value(vals["size_params"])
	shear := floats(vals["shear"])
	var sx, sy float64
	if len(shear) >= 2 {
		sx, sy = shear[0], shear[1]
	}
	if size[0] <= 0 || size[1] <= 0 || rng <= 0 {
		return false
	}
	skirtNear := numberValue(vals["skirt_near"], 0.05)
	l.Fade = [2]float64{1, 0}
	l.Angle = [2]float64{1, 0}
	if size[2] <= 0 {
		// An orthographic barn light shines along its forward axis with no falloff.
		l.Eye, l.Far = origin, rng
		l.Frustum = [4]float64{size[0], size[1], 0, 0}
		if skirtNear > 0 {
			l.Skirt[0] = 1 / skirtNear
		}
		if skirt > 0 {
			l.Skirt[1] = 1 / skirt
		}
	} else {
		near := 1 / size[2]
		cx, cy := sx/rng*near, sy/rng*near
		for i := 0; i < 3; i++ {
			l.Eye[i] = origin[i] - near*l.Forward[i] - cx*l.Right[i] - cy*l.Up[i]
		}
		l.Near, l.Far = near, near+rng
		l.Frustum = [4]float64{size[0], size[1], cx, cy}
		l.Falloff = cx*cx + cy*cy + near*near
		if skirtNear > 0 {
			l.Skirt[0] = (near + skirtNear*rng) / (l.Far * skirtNear)
		}
		if skirt > 0 {
			l.Skirt[1] = (near + (1-skirt)*rng) / (near * skirt)
		}
	}
	l.Min, l.Max = frustumBounds(*l)
	return true
}

// frustumBounds is the world box around a barn light's frustum.
func frustumBounds(l Light) ([3]float64, [3]float64) {
	lo := [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}
	hi := [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for _, depth := range []float64{math.Max(l.Near, 1e-3), l.Far} {
		scale := 1.0
		if l.Near > 0 {
			scale = depth / l.Near
		}
		for _, sx := range []float64{-1, 1} {
			for _, sy := range []float64{-1, 1} {
				x := (l.Frustum[2] + sx*l.Frustum[0]) * scale
				y := (l.Frustum[3] + sy*l.Frustum[1]) * scale
				for i := 0; i < 3; i++ {
					p := l.Eye[i] + l.Forward[i]*depth + l.Right[i]*x + l.Up[i]*y
					lo[i], hi[i] = math.Min(lo[i], p), math.Max(hi[i], p)
				}
			}
		}
	}
	return lo, hi
}

// distanceFade is the offset and rate that fade a light out over the last skirt share of its range.
func distanceFade(rng, skirt float64) [2]float64 {
	start := (1 - skirt) * rng
	end := math.Max(rng, math.Nextafter(start, math.Inf(1)))
	return [2]float64{end / (end - start), -1 / (end - start)}
}

// angleFade is the bias and scale of the fade from the inner to the outer cone, by the cosine of the angle.
func angleFade(outer, inner float64) [2]float64 {
	if outer >= 180 {
		return [2]float64{1, 0}
	}
	if outer <= 0 {
		return [2]float64{0, 0}
	}
	cosOuter := math.Min(math.Cos(outer*math.Pi/180), 1-1e-7)
	cosInner := 1.0
	if inner > 0 {
		cosInner = math.Cos(math.Min(inner, outer-0.001) * math.Pi / 180)
	}
	denom := cosOuter - cosInner
	if denom >= 0 {
		denom = -1e-7
	}
	return [2]float64{cosOuter / denom, 1 / denom}
}

func cross3(a, b [3]float64) [3]float64 {
	return [3]float64{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}

func normalize3(v [3]float64) [3]float64 {
	l := math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
	if l == 0 {
		return v
	}
	return [3]float64{v[0] / l, v[1] / l, v[2] / l}
}
