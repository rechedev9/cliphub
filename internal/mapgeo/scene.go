package mapgeo

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
)

// maxChunkVertices keeps every chunk addressable by 16-bit indices.
const maxChunkVertices = 65535

// SceneOptions sets the texture budget of an extracted scene.
type SceneOptions struct {
	// ColorSize caps the side of albedo textures; LightmapSize that of the lightmaps.
	ColorSize, LightmapSize int
}

// DefaultSceneOptions keeps a big map's scene near 150 MB.
var DefaultSceneOptions = SceneOptions{ColorSize: 512, LightmapSize: 4096}

// SceneTexture is a texture of the scene, still in its GPU block format.
type SceneTexture struct {
	Path string
	Tex  Texture
}

// SceneMaterial is a material with its textures resolved to scene indices (-1 for none).
type SceneMaterial struct {
	MaterialInfo
	ColorTex, Color2Tex, BlendTex int
	// Average is the albedo's average colour, for draws without a texture.
	Average [3]float32
}

// SceneChunk is up to 65535 vertices of one material, in world space.
type SceneChunk struct {
	Material  int
	Lightmap  bool
	Positions []float32
	Normals   []float32
	UVs       []float32
	UV2       []float32
	Colors    []float32
	Blend     []float32
	Indices   []uint16
	Min, Max  [3]float32
}

// Lightmap points at the baked lighting textures of the scene.
type Lightmap struct {
	Irradiance  int        `json:"irradiance"`
	Directional int        `json:"directional"`
	Shadows     int        `json:"shadows"`
	UVScale     [2]float64 `json:"uvScale"`
	// Average is the irradiance's average linear colour, the ambient of surfaces without a lightmap.
	Average [3]float32 `json:"average"`
}

// Scene is a map's render geometry with materials, textures and baked light.
type Scene struct {
	Map, Build string
	Textures   []SceneTexture
	Materials  []SceneMaterial
	Chunks     []SceneChunk
	Lightmap   *Lightmap
	Sun        *Sun
	Fog        *Fog
	Sky        *Sky
	Min, Max   [3]float32
}

// sceneFiles reads a path from the map package first, then the game's pak01.
type sceneFiles struct {
	maps, pak *VPK
}

func (f sceneFiles) read(path string) ([]byte, error) {
	if b, err := f.maps.ReadFile(path); err == nil || !errors.Is(err, ErrNotInVPK) {
		return b, err
	}
	if f.pak == nil {
		return nil, fmt.Errorf("%w: %s", ErrNotInVPK, path)
	}
	return f.pak.ReadFile(path)
}

type sceneBuilder struct {
	files     sceneFiles
	opts      SceneOptions
	scene     *Scene
	models    map[string][]DrawCall
	materials map[string]int
	textures  map[string]int
	open      map[chunkKey]int
	warnings  int
}

type chunkKey struct {
	material int
	lightmap bool
}

// ExtractScene reads the map's render meshes, materials, textures, lightmaps
// and sun from the install.
func (s Source) ExtractScene(opts SceneOptions) (*Scene, error) {
	maps, err := s.open()
	if err != nil {
		return nil, err
	}
	defer maps.Close()
	pak, err := OpenVPK(filepath.Join(s.csgoDir(), "pak01_dir.vpk"))
	if err != nil {
		pak = nil
	} else {
		defer pak.Close()
	}
	b := &sceneBuilder{
		files: sceneFiles{maps: maps, pak: pak}, opts: opts,
		scene:  &Scene{Map: s.Map, Build: s.build()},
		models: map[string][]DrawCall{}, materials: map[string]int{}, textures: map[string]int{}, open: map[chunkKey]int{},
	}
	if err := b.build(); err != nil {
		return nil, fmt.Errorf("%s scene: %w", s.Map, err)
	}
	return b.scene, nil
}

func (b *sceneBuilder) build() error {
	world, err := b.files.read("maps/" + b.scene.Map + "/world.vwrld_c")
	if err != nil {
		return err
	}
	blocks, err := ResourceBlocks(world)
	if err != nil {
		return err
	}
	root, err := decodeKV3(blocks["DATA"])
	if err != nil {
		return fmt.Errorf("world: %w", err)
	}
	wo := object(root)
	for _, node := range b.worldNodes() {
		if err := b.addNode(node); err != nil {
			return err
		}
	}
	for _, lumpPath := range stringList(wo["m_entityLumps"]) {
		b.addEntities(lumpPath, 0)
	}
	b.addLightmap(object(wo["m_worldLightingInfo"]))
	if len(b.scene.Chunks) == 0 {
		return errors.New("no render geometry")
	}
	b.bounds()
	return nil
}

// worldNodes lists the map's world node resources, in name order.
func (b *sceneBuilder) worldNodes() []string {
	prefix := "maps/" + b.scene.Map + "/worldnodes/"
	var out []string
	for name := range b.files.maps.entries {
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".vwnod_c") {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func (b *sceneBuilder) modelCalls(path string) []DrawCall {
	if calls, ok := b.models[path]; ok {
		return calls
	}
	data, err := b.files.read(path + "_c")
	var calls []DrawCall
	if err == nil {
		calls, err = ModelDrawCalls(data)
	}
	if err != nil {
		b.warnings++
		calls = nil
	}
	b.models[path] = calls
	return calls
}

func tintOf(v any, scale float64) [3]float32 {
	f := floats(v)
	if len(f) < 3 || (len(f) >= 4 && f[3] == 0 && scale == 1) {
		return [3]float32{1, 1, 1}
	}
	return [3]float32{float32(f[0] / scale), float32(f[1] / scale), float32(f[2] / scale)}
}

func (b *sceneBuilder) addNode(path string) error {
	data, err := b.files.read(path)
	if err != nil {
		return err
	}
	blocks, err := ResourceBlocks(data)
	if err != nil {
		return err
	}
	root, err := decodeKV3(blocks["DATA"])
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	node := object(root)
	for _, so := range list(node["m_sceneObjects"]) {
		o := object(so)
		model, _ := o["m_renderableModel"].(string)
		if model == "" {
			continue
		}
		at, _ := affineFromKV(o["m_vTransform"])
		tint := tintOf(o["m_vTintColor"], 1)
		for _, call := range b.modelCalls(model) {
			b.addCall(call, at, tint)
		}
	}
	for _, agg := range list(node["m_aggregateSceneObjects"]) {
		b.addAggregate(object(agg))
	}
	return nil
}

func (b *sceneBuilder) addAggregate(agg map[string]any) {
	model, _ := agg["m_renderableModel"].(string)
	calls := b.modelCalls(model)
	transforms := list(agg["m_fragmentTransforms"])
	next := 0
	for _, item := range list(agg["m_aggregateMeshes"]) {
		mesh := object(item)
		at := identityAffine
		if truthy(mesh["m_bHasTransform"]) {
			if next < len(transforms) {
				at, _ = affineFromKV(transforms[next])
			}
			next++
		}
		mask := uint64(toFloat(mesh["m_nLODGroupMask"]))
		if setup := int(toFloat(mesh["m_nLODSetupIndex"])); setup >= 0 && mask != 0 && mask&1 == 0 {
			continue
		}
		index := int(toFloat(mesh["m_nDrawCallIndex"]))
		if index < 0 || index >= len(calls) {
			continue
		}
		b.addCall(calls[index], at, tintOf(mesh["m_vTintColor"], 255))
	}
}

func (b *sceneBuilder) addEntities(path string, depth int) {
	data, err := b.files.read(path + "_c")
	if err != nil || depth > 4 {
		return
	}
	lump, err := parseEntityLump(data)
	if err != nil {
		b.warnings++
		return
	}
	if lump.sun != nil && b.scene.Sun == nil {
		b.scene.Sun = lump.sun
	}
	if lump.fog != nil && b.scene.Fog == nil {
		b.scene.Fog = lump.fog
	}
	if lump.sky != nil && b.scene.Sky == nil {
		b.addSky(lump.sky)
	}
	for _, m := range lump.models {
		for _, call := range b.modelCalls(m.model) {
			b.addCall(call, m.at, m.tint)
		}
	}
	blocks, _ := ResourceBlocks(data)
	if root, err := decodeKV3(blocks["DATA"]); err == nil {
		for _, child := range stringList(object(root)["m_childLumps"]) {
			b.addEntities(child, depth+1)
		}
	}
}

// skySize caps a face of the sky cube.
const skySize = 1024

func (b *sceneBuilder) addSky(sky *skyEntity) {
	data, err := b.files.read(sky.material + "_c")
	if err != nil {
		return
	}
	info, err := ParseMaterial(sky.material, data)
	if err != nil || info.Sky == "" {
		return
	}
	file, err := b.files.read(info.Sky + "_c")
	if err != nil {
		return
	}
	tex, err := ReadCubeTexture(file, skySize)
	if err != nil {
		b.warnings++
		return
	}
	b.scene.Textures = append(b.scene.Textures, SceneTexture{Path: info.Sky, Tex: tex})
	r := tex.Reflectivity
	b.scene.Sky = &Sky{Texture: len(b.scene.Textures) - 1, Brightness: sky.brightness, Tint: sky.tint, Yaw: sky.yaw,
		Average: [3]float64{float64(r[0]), float64(r[1]), float64(r[2])}}
}

func (b *sceneBuilder) addLightmap(info map[string]any) {
	if info == nil || !truthy(info["m_bHasLightmaps"]) {
		return
	}
	lm := &Lightmap{Irradiance: -1, Directional: -1, Shadows: -1, UVScale: [2]float64{1, 1}}
	if s := floats(info["m_vLightmapUvScale"]); len(s) >= 2 {
		lm.UVScale = [2]float64{s[0], s[1]}
	}
	// The sun's direct light is not in the irradiance: it is masked by its channel of the shadow map.
	for _, path := range stringList(info["m_lightMaps"]) {
		switch base := filepath.Base(path); {
		case strings.HasPrefix(base, "irradiance"):
			lm.Irradiance = b.texture(path, b.opts.LightmapSize)
		case strings.HasPrefix(base, "direct_light_shadows"):
			lm.Shadows = b.texture(path, b.opts.LightmapSize)
		}
	}
	if lm.Irradiance >= 0 {
		r := b.scene.Textures[lm.Irradiance].Tex.Reflectivity
		lm.Average = [3]float32{r[0], r[1], r[2]}
		b.scene.Lightmap = lm
	}
}

func (b *sceneBuilder) texture(path string, size int) int {
	if path == "" {
		return -1
	}
	if i, ok := b.textures[path]; ok {
		return i
	}
	i := -1
	if data, err := b.files.read(path + "_c"); err == nil {
		if tex, err := ReadTexture(data, size); err == nil {
			i = len(b.scene.Textures)
			b.scene.Textures = append(b.scene.Textures, SceneTexture{Path: path, Tex: tex})
		}
	}
	b.textures[path] = i
	return i
}

// material returns the scene index of a material, or -1 when it is not drawn.
func (b *sceneBuilder) material(path string) int {
	if i, ok := b.materials[path]; ok {
		return i
	}
	i := -1
	if data, err := b.files.read(path + "_c"); err == nil {
		if info, err := ParseMaterial(path, data); err == nil && !info.Skip {
			m := SceneMaterial{MaterialInfo: info}
			m.ColorTex = b.texture(info.Color, b.opts.ColorSize)
			m.Color2Tex = b.texture(info.Color2, b.opts.ColorSize)
			m.BlendTex = b.texture(info.BlendMod, b.opts.ColorSize)
			m.Average = [3]float32{0.6, 0.6, 0.6}
			if m.ColorTex >= 0 {
				r := b.scene.Textures[m.ColorTex].Tex.Reflectivity
				// Reflectivity is linear; the renderer wants the sRGB average.
				for k := 0; k < 3; k++ {
					m.Average[k] = float32(math.Pow(float64(r[k]), 1/2.2))
				}
			}
			i = len(b.scene.Materials)
			b.scene.Materials = append(b.scene.Materials, m)
		}
	}
	b.materials[path] = i
	return i
}

func (b *sceneBuilder) addCall(call DrawCall, at Affine, tint [3]float32) {
	if len(call.Indices) < 3 || len(call.Positions) == 0 {
		return
	}
	mat := b.material(call.Material)
	if mat < 0 {
		return
	}
	key := chunkKey{material: mat, lightmap: call.UV2 != nil}
	flip := at.mirrored()
	vertexCount := len(call.Positions) / 3
	if vertexCount <= maxChunkVertices {
		b.appendRange(key, rangeCall{call: call, at: at, tint: tint, flip: flip, tris: allTriangles(len(call.Indices))})
		return
	}
	// Split an oversized call into runs of triangles that fit one chunk each.
	var run []int
	seen := map[uint32]bool{}
	for t := 0; t+2 < len(call.Indices); t += 3 {
		fresh := 0
		for k := 0; k < 3; k++ {
			if !seen[call.Indices[t+k]] {
				fresh++
			}
		}
		if len(seen)+fresh > maxChunkVertices {
			b.appendRange(key, rangeCall{call: call, at: at, tint: tint, flip: flip, tris: run})
			run, seen = nil, map[uint32]bool{}
		}
		for k := 0; k < 3; k++ {
			seen[call.Indices[t+k]] = true
		}
		run = append(run, t)
	}
	b.appendRange(key, rangeCall{call: call, at: at, tint: tint, flip: flip, tris: run})
}

type rangeCall struct {
	call DrawCall
	at   Affine
	tint [3]float32
	flip bool
	// tris are the first index of each triangle to take.
	tris []int
}

func allTriangles(indices int) []int {
	out := make([]int, 0, indices/3)
	for t := 0; t+2 < indices; t += 3 {
		out = append(out, t)
	}
	return out
}

func (b *sceneBuilder) chunkFor(key chunkKey, vertices int) *SceneChunk {
	if i, ok := b.open[key]; ok && len(b.scene.Chunks[i].Positions)/3+vertices <= maxChunkVertices {
		return &b.scene.Chunks[i]
	}
	b.scene.Chunks = append(b.scene.Chunks, SceneChunk{Material: key.material, Lightmap: key.lightmap})
	b.open[key] = len(b.scene.Chunks) - 1
	return &b.scene.Chunks[len(b.scene.Chunks)-1]
}

func (b *sceneBuilder) appendRange(key chunkKey, r rangeCall) {
	if len(r.tris) == 0 {
		return
	}
	remap := map[uint32]uint16{}
	var order []uint32
	for _, t := range r.tris {
		for k := 0; k < 3; k++ {
			v := r.call.Indices[t+k]
			if _, ok := remap[v]; !ok {
				remap[v] = uint16(len(order))
				order = append(order, v)
			}
		}
	}
	c := b.chunkFor(key, len(order))
	base := len(c.Positions) / 3
	call := r.call
	tint := [3]float32{r.tint[0] * call.Tint[0], r.tint[1] * call.Tint[1], r.tint[2] * call.Tint[2]}
	tinted := tint != [3]float32{1, 1, 1} || call.Colors != nil
	if tinted && c.Colors == nil {
		c.Colors = filled(base*4, 1)
	}
	if call.Blend != nil && c.Blend == nil {
		c.Blend = filled(base*4, 0)
	}
	for _, v := range order {
		i := int(v)
		x, y, z := r.at.point(float64(call.Positions[i*3]), float64(call.Positions[i*3+1]), float64(call.Positions[i*3+2]))
		c.Positions = append(c.Positions, float32(x), float32(y), float32(z))
		if call.Normals != nil {
			nx, ny, nz := r.at.normal(float64(call.Normals[i*3]), float64(call.Normals[i*3+1]), float64(call.Normals[i*3+2]))
			c.Normals = append(c.Normals, float32(nx), float32(ny), float32(nz))
		} else {
			c.Normals = append(c.Normals, 0, 0, 1)
		}
		c.UVs = appendPair(c.UVs, call.UVs, i)
		if key.lightmap {
			c.UV2 = appendPair(c.UV2, call.UV2, i)
		}
		if c.Colors != nil {
			col := [4]float32{1, 1, 1, 1}
			if call.Colors != nil {
				copy(col[:], call.Colors[i*4:i*4+4])
			}
			c.Colors = append(c.Colors, col[0]*tint[0], col[1]*tint[1], col[2]*tint[2], col[3])
		}
		if c.Blend != nil {
			if call.Blend != nil {
				c.Blend = append(c.Blend, call.Blend[i*4:i*4+4]...)
			} else {
				c.Blend = append(c.Blend, 0, 0, 0, 0)
			}
		}
	}
	for _, t := range r.tris {
		a, bb, cc := remap[call.Indices[t]], remap[call.Indices[t+1]], remap[call.Indices[t+2]]
		if r.flip {
			bb, cc = cc, bb
		}
		c.Indices = append(c.Indices, uint16(base)+a, uint16(base)+bb, uint16(base)+cc)
	}
}

func appendPair(dst, src []float32, i int) []float32 {
	if src == nil || i*2+1 >= len(src) {
		return append(dst, 0, 0)
	}
	return append(dst, src[i*2], src[i*2+1])
}

func filled(n int, v float32) []float32 {
	out := make([]float32, n)
	for i := range out {
		out[i] = v
	}
	return out
}

func (b *sceneBuilder) bounds() {
	first := true
	for ci := range b.scene.Chunks {
		c := &b.scene.Chunks[ci]
		c.Min = [3]float32{math.MaxFloat32, math.MaxFloat32, math.MaxFloat32}
		c.Max = [3]float32{-math.MaxFloat32, -math.MaxFloat32, -math.MaxFloat32}
		for i := 0; i+2 < len(c.Positions); i += 3 {
			for k := 0; k < 3; k++ {
				c.Min[k] = min(c.Min[k], c.Positions[i+k])
				c.Max[k] = max(c.Max[k], c.Positions[i+k])
			}
		}
		if first {
			b.scene.Min, b.scene.Max = c.Min, c.Max
			first = false
			continue
		}
		for k := 0; k < 3; k++ {
			b.scene.Min[k] = min(b.scene.Min[k], c.Min[k])
			b.scene.Max[k] = max(b.scene.Max[k], c.Max[k])
		}
	}
}

// SceneCacheKey names the cached scene file of this map, game build and options.
func (s Source) SceneCacheKey(opts SceneOptions) (string, error) {
	info, err := s.locate()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("mapgeo/scene/v%d/%s-%s-%d-%d-c%d-l%d.chms",
		SceneFileVersion, s.Map, s.build(), info.Size(), info.ModTime().Unix(), opts.ColorSize, opts.LightmapSize), nil
}
