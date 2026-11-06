package mapgeo

import (
	"fmt"
	"strings"
)

// MaterialInfo is what Studio's renderer needs from a compiled CS2 material:
// the colour layers, how they blend and how the surface lets light through.
type MaterialInfo struct {
	Name   string
	Shader string
	// Color is the albedo texture path; Color2 the second layer of a blend.
	Color, Color2 string
	// BlendMod is the blend modulation texture of a two layer material.
	BlendMod string
	// Normal is the normal and roughness texture, a hemi-octahedral normal in RG and the roughness in B; Normal2 the second layer's.
	Normal, Normal2 string
	// Metalness is the material's constant metalness.
	Metalness float32
	// Tint multiplies the albedo; Tint2 the second layer.
	Tint, Tint2 [3]float32
	// Scale and offset of the layer texture coordinates, as the material sets them.
	UVScale, UVOffset   [2]float32
	UV2Scale, UV2Offset [2]float32
	BlendSoftness       float32
	// BlendMode is F_FANCY_BLENDING: 0 vertex blend only, 1 softness in the modulation red, 2 and 3 a fixed softness.
	BlendMode   int
	AlphaTest   bool
	AlphaRef    float32
	Translucent bool
	DoubleSided bool
	// Unlit draws the colour as is, as black unlit and sky shaders do.
	Unlit bool
	// Overlay is a decal mesh laid on a surface, blended over it.
	Overlay bool
	// Skip marks tool, effect and sky materials, which CS2 never draws as world surfaces.
	Skip bool
	// Sky is the cube texture of a sky material; SkyExposureBias brightens it by that many stops.
	Sky             string
	SkyExposureBias float32
}

// skippedShaders are drawn by CS2 as effects, decals or sky, not as surfaces.
var skippedShaders = map[string]bool{
	"csgo_effects.vfx":          true,
	"csgo_sky.vfx":              true,
	"sky.vfx":                   true,
	"csgo_decalmodulate.vfx":    true,
	"csgo_projected_decals.vfx": true,
	"spritecard.vfx":            true,
	"cables.vfx":                true,
}

func skippedMaterialPath(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "materials/tools/") || strings.HasPrefix(lower, "materials/dev/") ||
		strings.Contains(lower, "/toolsblack") || strings.Contains(lower, "nodraw")
}

// ParseMaterial reads a compiled material (vmat_c).
func ParseMaterial(name string, file []byte) (MaterialInfo, error) {
	m := MaterialInfo{
		Name: name, Tint: [3]float32{1, 1, 1}, Tint2: [3]float32{1, 1, 1},
		UVScale: [2]float32{1, 1}, UV2Scale: [2]float32{1, 1}, AlphaRef: 0.5, BlendSoftness: 0.5,
	}
	blocks, err := ResourceBlocks(file)
	if err != nil {
		return m, err
	}
	data, ok := blocks["DATA"]
	if !ok {
		return m, fmt.Errorf("material %s without DATA", name)
	}
	root, err := decodeKV3(data)
	if err != nil {
		return m, fmt.Errorf("material %s: %w", name, err)
	}
	o := object(root)
	m.Shader, _ = o["m_shaderName"].(string)
	ints := map[string]float64{}
	for _, p := range list(o["m_intParams"]) {
		po := object(p)
		key, _ := po["m_name"].(string)
		ints[key] = toFloat(po["m_nValue"])
	}
	for _, p := range list(o["m_floatParams"]) {
		po := object(p)
		switch po["m_name"] {
		case "g_flAlphaTestReference":
			m.AlphaRef = float32(toFloat(po["m_flValue"]))
		case "g_flBlendSoftness":
			m.BlendSoftness = float32(toFloat(po["m_flValue"]))
		case "g_flMetalness", "g_flMetalness1":
			m.Metalness = float32(toFloat(po["m_flValue"]))
		case "g_flBrightnessExposureBias", "g_flRenderOnlyExposureBias":
			// CS2 draws the sky, and colours its fog, this many stops brighter.
			m.SkyExposureBias += float32(toFloat(po["m_flValue"]))
		}
	}
	for _, p := range list(o["m_vectorParams"]) {
		po := object(p)
		v := floats(po["m_value"])
		if len(v) < 3 {
			continue
		}
		switch po["m_name"] {
		case "g_vColorTint", "g_vLayer1Tint":
			m.Tint = [3]float32{float32(v[0]), float32(v[1]), float32(v[2])}
		case "g_vLayer2Tint":
			m.Tint2 = [3]float32{float32(v[0]), float32(v[1]), float32(v[2])}
		case "g_vTexCoordScale", "g_vLayer1TexCoordScale":
			m.UVScale = [2]float32{float32(v[0]), float32(v[1])}
		case "g_vTexCoordOffset", "g_vLayer1TexCoordOffset":
			m.UVOffset = [2]float32{float32(v[0]), float32(v[1])}
		case "g_vLayer2TexCoordScale":
			m.UV2Scale = [2]float32{float32(v[0]), float32(v[1])}
		case "g_vLayer2TexCoordOffset":
			m.UV2Offset = [2]float32{float32(v[0]), float32(v[1])}
		}
	}
	for _, p := range list(o["m_textureParams"]) {
		po := object(p)
		path, _ := po["m_pValue"].(string)
		switch po["m_name"] {
		case "g_tColor", "g_tColor1", "g_tLayer1Color":
			if m.Color == "" {
				m.Color = path
			}
		case "g_tLayer2Color", "g_tColor2":
			m.Color2 = path
		case "g_tBlendModulation":
			m.BlendMod = path
		case "g_tNormal", "g_tNormalRoughness", "g_tLayer1NormalRoughness", "g_tNormal1":
			if m.Normal == "" {
				m.Normal = path
			}
		case "g_tLayer2NormalRoughness", "g_tNormal2", "g_tNormalRoughness2":
			m.Normal2 = path
		case "g_tSkyTexture":
			m.Sky = path
		}
	}
	m.BlendMode = int(ints["F_FANCY_BLENDING"])
	m.AlphaTest = ints["F_ALPHA_TEST"] != 0
	m.Translucent = ints["F_TRANSLUCENT"] != 0 || ints["F_BLEND_MODE"] != 0
	m.DoubleSided = ints["F_RENDER_BACKFACES"] != 0 || strings.Contains(m.Shader, "foliage")
	m.Unlit = strings.Contains(m.Shader, "unlit")
	m.Overlay = strings.Contains(m.Shader, "static_overlay")
	m.Skip = skippedShaders[m.Shader] || skippedMaterialPath(name) || strings.Contains(m.Shader, "decal")
	return m, nil
}
