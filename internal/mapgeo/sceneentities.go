package mapgeo

import (
	"math"
	"strconv"
	"strings"
)

// Affine is a 3x4 row-major transform: world = R*p + T, R in the first three
// columns of each row and T in the fourth.
type Affine [3][4]float64

var identityAffine = Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}

func (a Affine) point(x, y, z float64) (float64, float64, float64) {
	return a[0][0]*x + a[0][1]*y + a[0][2]*z + a[0][3],
		a[1][0]*x + a[1][1]*y + a[1][2]*z + a[1][3],
		a[2][0]*x + a[2][1]*y + a[2][2]*z + a[2][3]
}

// normal rotates a normal by the inverse transpose of R, renormalised.
func (a Affine) normal(x, y, z float64) (float64, float64, float64) {
	m := a.inverseTransposeR()
	nx := m[0][0]*x + m[0][1]*y + m[0][2]*z
	ny := m[1][0]*x + m[1][1]*y + m[1][2]*z
	nz := m[2][0]*x + m[2][1]*y + m[2][2]*z
	l := math.Sqrt(nx*nx + ny*ny + nz*nz)
	if l == 0 {
		return 0, 0, 1
	}
	return nx / l, ny / l, nz / l
}

func (a Affine) inverseTransposeR() [3][3]float64 {
	r := [3][3]float64{{a[0][0], a[0][1], a[0][2]}, {a[1][0], a[1][1], a[1][2]}, {a[2][0], a[2][1], a[2][2]}}
	det := r[0][0]*(r[1][1]*r[2][2]-r[1][2]*r[2][1]) - r[0][1]*(r[1][0]*r[2][2]-r[1][2]*r[2][0]) + r[0][2]*(r[1][0]*r[2][1]-r[1][1]*r[2][0])
	if det == 0 {
		return [3][3]float64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
	}
	// The cofactor matrix over the determinant is the inverse transpose.
	var out [3][3]float64
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			a1, a2 := (i+1)%3, (i+2)%3
			b1, b2 := (j+1)%3, (j+2)%3
			out[i][j] = (r[a1][b1]*r[a2][b2] - r[a1][b2]*r[a2][b1]) / det
		}
	}
	return out
}

func (a Affine) mirrored() bool {
	r := a.inverseTransposeR()
	det := r[0][0]*(r[1][1]*r[2][2]-r[1][2]*r[2][1]) - r[0][1]*(r[1][0]*r[2][2]-r[1][2]*r[2][0]) + r[0][2]*(r[1][0]*r[2][1]-r[1][1]*r[2][0])
	return det < 0
}

func affineFromKV(v any) (Affine, bool) {
	rows := list(v)
	if len(rows) == 3 {
		var a Affine
		for i, row := range rows {
			f := floats(row)
			if len(f) < 4 {
				return identityAffine, false
			}
			copy(a[i][:], f[:4])
		}
		return a, true
	}
	if f := floats(v); len(f) >= 12 {
		var a Affine
		for i := 0; i < 3; i++ {
			copy(a[i][:], f[i*4:i*4+4])
		}
		return a, true
	}
	return identityAffine, false
}

// angleAffine builds Source's AngleMatrix (pitch, yaw, roll in degrees), scaled and moved to origin.
func angleAffine(angles, origin, scales [3]float64) Affine {
	rad := math.Pi / 180
	sp, cp := math.Sincos(angles[0] * rad)
	sy, cy := math.Sincos(angles[1] * rad)
	sr, cr := math.Sincos(angles[2] * rad)
	r := [3][3]float64{
		{cp * cy, sr*sp*cy - cr*sy, cr*sp*cy + sr*sy},
		{cp * sy, sr*sp*sy + cr*cy, cr*sp*sy - sr*cy},
		{-sp, sr * cp, cr * cp},
	}
	var a Affine
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			a[i][j] = r[i][j] * scales[j]
		}
		a[i][3] = origin[i]
	}
	return a
}

// Sun is the map's light_environment: where the light comes from and its colour.
type Sun struct {
	// Direction points from the ground toward the sun.
	Direction [3]float64 `json:"direction"`
	// Color is linear RGB times brightness.
	Color [3]float64 `json:"color"`
	// Sky is the sky ambient colour times its intensity, linear RGB.
	Sky [3]float64 `json:"sky"`
	// ShadowChannel is the lightmap shadow channel the sun bakes into, -1 for none.
	ShadowChannel int `json:"shadowChannel"`
}

// entityModel is a model a map entity places, such as a door or a func_brush.
type entityModel struct {
	model string
	at    Affine
	tint  [3]float32
}

type entityLump struct {
	sun    *Sun
	models []entityModel
	fog    *Fog
	sky    *skyEntity
	// skyDisabled marks a sky that starts off.
	skyDisabled bool
}

// skyEntity is the map's enabled env_sky: its material and how it is turned and lit.
type skyEntity struct {
	material   string
	brightness float64
	tint       [3]float64
	yaw        float64
}

// Sky is the map's sky: a cube texture of the scene, its brightness and tint, and its turn about z in degrees.
type Sky struct {
	Texture    int        `json:"texture"`
	Brightness float64    `json:"brightness"`
	Tint       [3]float64 `json:"tint"`
	Yaw        float64    `json:"yaw"`
}

// Fog is the map's distance fog, from env_cubemap_fog or env_gradient_fog.
type Fog struct {
	Start float64    `json:"start"`
	End   float64    `json:"end"`
	Color [3]float64 `json:"color"`
	Max   float64    `json:"max"`
}

// modelEntityClasses place a visible model in the world.
var modelEntityClasses = []string{"prop_dynamic", "prop_physics", "prop_door_rotating", "func_brush", "func_door", "func_breakable", "prop_static", "func_rotating", "func_movelinear"}

func vec3Value(v any) ([3]float64, bool) {
	if s, ok := v.(string); ok {
		f := strings.Fields(s)
		if len(f) < 3 {
			return [3]float64{}, false
		}
		var out [3]float64
		for i := range out {
			out[i] = parseFloatOr(f[i], 0)
		}
		return out, true
	}
	f := floats(v)
	if len(f) < 3 {
		return [3]float64{}, false
	}
	return [3]float64{f[0], f[1], f[2]}, true
}

func parseFloatOr(s string, fallback float64) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return fallback
	}
	return f
}

func numberValue(v any, fallback float64) float64 {
	if s, ok := v.(string); ok {
		return parseFloatOr(s, fallback)
	}
	if v == nil {
		return fallback
	}
	return toFloat(v)
}

func srgbToLinear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func parseEntityLump(file []byte) (entityLump, error) {
	var lump entityLump
	blocks, err := ResourceBlocks(file)
	if err != nil {
		return lump, err
	}
	root, err := decodeKV3(blocks["DATA"])
	if err != nil {
		return lump, err
	}
	for _, e := range list(object(root)["m_entityKeyValues"]) {
		vals := object(object(object(e)["keyValues3Data"])["values"])
		if vals == nil {
			continue
		}
		class, _ := vals["classname"].(string)
		switch {
		case class == "light_environment":
			lump.sun = sunFromEntity(vals)
		case class == "env_cubemap_fog" && lump.fog == nil:
			lump.fog = fogFromEntity(vals)
		case class == "env_sky":
			// Some maps only ship skies a script turns on; one of those stands in for an enabled one.
			if sky := skyFromEntity(vals); sky != nil && (lump.sky == nil || lump.skyDisabled && !truthy(vals["StartDisabled"])) {
				lump.sky, lump.skyDisabled = sky, truthy(vals["StartDisabled"])
			}
		default:
			if m, ok := modelFromEntity(class, vals); ok {
				lump.models = append(lump.models, m)
			}
		}
	}
	return lump, nil
}

func sunFromEntity(vals map[string]any) *Sun {
	angles, _ := vec3Value(vals["angles"])
	rad := math.Pi / 180
	sp, cp := math.Sincos(angles[0] * rad)
	sy, cy := math.Sincos(angles[1] * rad)
	sun := &Sun{Direction: [3]float64{-cp * cy, -cp * sy, sp}, ShadowChannel: -1}
	brightness := numberValue(vals["brightness"], 1)
	if c, ok := vec3Value(vals["color"]); ok {
		for i := range c {
			sun.Color[i] = srgbToLinear(c[i]/255) * brightness
		}
	}
	intensity := numberValue(vals["skyintensity"], 1)
	if c, ok := vec3Value(vals["skycolor"]); ok {
		for i := range c {
			sun.Sky[i] = srgbToLinear(c[i]/255) * intensity
		}
	}
	if idx, ok := vals["bakedshadowindex"]; ok {
		sun.ShadowChannel = int(numberValue(idx, -1))
	}
	return sun
}

func skyFromEntity(vals map[string]any) *skyEntity {
	name, _ := vals["skyname"].(string)
	if name == "" {
		return nil
	}
	sky := &skyEntity{material: name, brightness: numberValue(vals["brightnessscale"], 1), tint: [3]float64{1, 1, 1}}
	if c, ok := vec3Value(vals["tint_color"]); ok {
		for i := range c {
			sky.tint[i] = srgbToLinear(c[i] / 255)
		}
	}
	if a, ok := vec3Value(vals["angles"]); ok {
		sky.yaw = a[1]
	}
	return sky
}

func fogFromEntity(vals map[string]any) *Fog {
	f := &Fog{
		Start: numberValue(vals["cubemapfogstartdistance"], 0),
		End:   numberValue(vals["cubemapfogenddistance"], 0),
		Max:   numberValue(vals["cubemapfogmaxopacity"], 1),
	}
	if f.End <= f.Start {
		return nil
	}
	f.Color = [3]float64{0.7, 0.75, 0.8}
	return f
}

func modelFromEntity(class string, vals map[string]any) (entityModel, bool) {
	known := false
	for _, prefix := range modelEntityClasses {
		if strings.HasPrefix(class, prefix) {
			known = true
		}
	}
	model, _ := vals["model"].(string)
	if !known || model == "" || !strings.HasSuffix(model, ".vmdl") {
		return entityModel{}, false
	}
	if mode := numberValue(vals["rendermode"], 0); mode == 10 {
		return entityModel{}, false
	}
	if amt, ok := vals["renderamt"]; ok && numberValue(amt, 255) == 0 {
		return entityModel{}, false
	}
	angles, _ := vec3Value(vals["angles"])
	origin, _ := vec3Value(vals["origin"])
	scales, ok := vec3Value(vals["scales"])
	if !ok || scales == [3]float64{} {
		scales = [3]float64{1, 1, 1}
	}
	if s, ok := vals["modelscale"]; ok {
		k := numberValue(s, 1)
		scales = [3]float64{scales[0] * k, scales[1] * k, scales[2] * k}
	}
	m := entityModel{model: model, at: angleAffine(angles, origin, scales), tint: [3]float32{1, 1, 1}}
	if c, ok := vec3Value(vals["rendercolor"]); ok && c != [3]float64{} {
		m.tint = [3]float32{float32(c[0] / 255), float32(c[1] / 255), float32(c[2] / 255)}
	}
	return m, true
}
