package mapgeo

import (
	"math"
	"sort"
)

// Reflections are the map's baked reflection probes, as ValveResourceFormat reads
// them: one cube array texture, and per probe the box it lights and the box its
// reflections are projected on, in the probe's own axes.
type Reflections struct {
	// Texture is the scene texture of the cube array; its faces are 6 per cube.
	Texture int     `json:"texture"`
	Probes  []Probe `json:"probes"`
}

// Probe is one reflection probe. Probes are listed in the order CS2 takes them:
// the higher indoor-outdoor level first, then the smaller volume.
type Probe struct {
	// Cube is the probe's index in the cube array.
	Cube int `json:"cube"`
	// WorldToLocal is a 3x4 row-major transform into the probe's axes.
	WorldToLocal [12]float64 `json:"worldToLocal"`
	// Min and Max bound the volume the probe lights, grown by boundsExtend.
	Min [3]float64 `json:"min"`
	Max [3]float64 `json:"max"`
	// ProjectMin and ProjectMax bound the box reflections are projected on; Box is false for a sphere, which projects on none.
	ProjectMin [3]float64 `json:"projectMin"`
	ProjectMax [3]float64 `json:"projectMax"`
	Box        bool       `json:"box"`
	// InvFade is the reciprocal of the edge fade distance on each axis; a large value is a hard edge.
	InvFade [3]float64 `json:"invFade"`
	// Normalization is the luminance of the probe's radiance as an L1 harmonic, for a dot with (normal, 1).
	Normalization [4]float64 `json:"normalization"`
}

// boundsExtend grows every probe box on each side, as CS2 tests fragments against the grown box.
const boundsExtend = 0.1

// hardEdge stands in for the reciprocal of a zero fade distance.
const hardEdge = 1e6

type probeEntity struct {
	texture                string
	cube, level            int
	origin, angles         [3]float64
	min, max, fade         [3]float64
	projectMin, projectMax [3]float64
	box, project           bool
}

func probeFromEntity(class string, vals map[string]any) (probeEntity, bool) {
	texture, _ := vals["cubemaptexture"].(string)
	if custom, _ := vals["customcubemaptexture"].(string); texture == "" || custom != "" {
		return probeEntity{}, false
	}
	p := probeEntity{texture: texture, cube: int(numberValue(vals["array_index"], 0)), level: int(numberValue(vals["indoor_outdoor_level"], 0)), box: class != "env_cubemap"}
	p.origin, _ = vec3Value(vals["origin"])
	p.angles, _ = vec3Value(vals["angles"])
	if p.box {
		p.min, _ = vec3Value(vals["box_mins"])
		p.max, _ = vec3Value(vals["box_maxs"])
		p.fade, _ = vec3Value(vals["edge_fade_dists"])
	} else {
		r := numberValue(vals["influenceradius"], 512)
		p.min, p.max = [3]float64{-r, -r, -r}, [3]float64{r, r, r}
		f := numberValue(vals["edge_fade_dist"], 0)
		p.fade = [3]float64{f, f, f}
	}
	p.projectMin, p.projectMax = p.min, p.max
	if lo, ok := vec3Value(vals["boxproject_mins"]); ok {
		if hi, ok := vec3Value(vals["boxproject_maxs"]); ok {
			p.projectMin, p.projectMax = lo, hi
		}
	}
	return p, true
}

// addReflections reads the cube array most probes name and keeps the probes in it.
func (b *sceneBuilder) addReflections() {
	if len(b.probes) == 0 || b.opts.ReflectionSize <= 0 {
		return
	}
	uses := map[string]int{}
	for _, p := range b.probes {
		uses[p.texture]++
	}
	texture := ""
	for path, n := range uses {
		if n > uses[texture] || n == uses[texture] && path < texture {
			texture = path
		}
	}
	file, err := b.files.read(texture + "_c")
	if err != nil {
		return
	}
	tex, err := ReadCubeArrayTexture(file, b.opts.ReflectionSize)
	if err != nil {
		b.warnings++
		return
	}
	cubes := tex.Faces / 6
	var kept []probeEntity
	for _, p := range b.probes {
		if p.texture == texture && p.cube >= 0 && p.cube < cubes {
			kept = append(kept, p)
		}
	}
	if len(kept) == 0 {
		return
	}
	sort.SliceStable(kept, func(i, j int) bool {
		if kept[i].level != kept[j].level {
			return kept[i].level > kept[j].level
		}
		return boxVolume(kept[i]) < boxVolume(kept[j])
	})
	b.scene.Textures = append(b.scene.Textures, SceneTexture{Path: texture, Tex: tex})
	r := &Reflections{Texture: len(b.scene.Textures) - 1}
	for _, p := range kept {
		r.Probes = append(r.Probes, sceneProbe(p, tex.RadianceSH))
	}
	b.scene.Reflections = r
}

func boxVolume(p probeEntity) float64 {
	return (p.max[0] - p.min[0]) * (p.max[1] - p.min[1]) * (p.max[2] - p.min[2])
}

func sceneProbe(p probeEntity, radiance []float32) Probe {
	at := angleAffine(p.angles, p.origin, [3]float64{1, 1, 1})
	out := Probe{Cube: p.cube, Box: p.box, ProjectMin: p.projectMin, ProjectMax: p.projectMax}
	// The inverse of a rotation and a move: the transposed rotation, then minus the moved origin.
	for i := 0; i < 3; i++ {
		t := 0.0
		for j := 0; j < 3; j++ {
			out.WorldToLocal[i*4+j] = at[j][i]
			t -= at[j][i] * p.origin[j]
		}
		out.WorldToLocal[i*4+3] = t
	}
	for k := 0; k < 3; k++ {
		out.Min[k] = p.min[k] - boundsExtend
		out.Max[k] = p.max[k] + boundsExtend
		out.InvFade[k] = hardEdge
		if p.fade[k] > 0 {
			out.InvFade[k] = 1 / p.fade[k]
		}
	}
	out.Normalization = normalizationSH(radiance, p.cube, at)
	return out
}

// normalizationSH folds a probe's L2 radiance down to its luminance as an L1 harmonic,
// turned into world axes, as ValveResourceFormat's CalculateNormalizationSH does. Zero means none.
func normalizationSH(radiance []float32, cube int, at Affine) [4]float64 {
	const perChannel, perProbe = 9, 27
	const constantBand, linearBand, maxLinear = 0.282095, 0.325735, 0.99
	offset := cube * perProbe
	if cube < 0 || offset+perProbe > len(radiance) {
		return [4]float64{}
	}
	var luma [4]float64
	for band := range luma {
		r, g, b := float64(radiance[offset+band]), float64(radiance[offset+perChannel+band]), float64(radiance[offset+2*perChannel+band])
		luma[band] = 0.2126*r + 0.7152*g + 0.0722*b
	}
	constant := luma[0] * constantBand
	if constant <= 0 {
		return [4]float64{0, 0, 0, 0.01}
	}
	// The linear band is stored y, z, x, with y and x of opposite sign to the normal's axes.
	local := [3]float64{-luma[3] * linearBand, -luma[1] * linearBand, luma[2] * linearBand}
	var world [3]float64
	for i := 0; i < 3; i++ {
		world[i] = at[i][0]*local[0] + at[i][1]*local[1] + at[i][2]*local[2]
	}
	length := math.Sqrt(world[0]*world[0] + world[1]*world[1] + world[2]*world[2] + constant*constant)
	scale := math.Min(constant*maxLinear/length, 1)
	return [4]float64{world[0] * scale, world[1] * scale, world[2] * scale, constant}
}
