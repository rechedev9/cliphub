package mapgeo

import (
	"math"
)

// Post is the map's post processing, as its master post_processing_volume
// sets it: the range auto exposure keeps to, the filmic tonemap curve and the
// colour grading LUT applied after it.
type Post struct {
	ExposureMin float64 `json:"exposureMin"`
	ExposureMax float64 `json:"exposureMax"`
	Tonemap     Tonemap `json:"tonemap"`
	// LUTSize is the side of the colour LUT, 0 for none; LUT holds its RGBA texels, red fastest.
	LUTSize int    `json:"lutSize"`
	LUT     []byte `json:"-"`
}

// Tonemap is the Uncharted filmic curve of a .vpost; WhitePoint is in stops.
type Tonemap struct {
	ExposureBias   float64 `json:"exposureBias"`
	Shoulder       float64 `json:"shoulder"`
	LinearStrength float64 `json:"linearStrength"`
	LinearAngle    float64 `json:"linearAngle"`
	ToeStrength    float64 `json:"toeStrength"`
	ToeNum         float64 `json:"toeNum"`
	ToeDenom       float64 `json:"toeDenom"`
	WhitePoint     float64 `json:"whitePoint"`
}

// defaultTonemap is the engine's curve when a volume has no tonemap of its own.
var defaultTonemap = Tonemap{Shoulder: 0.15, LinearStrength: 0.5, LinearAngle: 0.1, ToeStrength: 0.2, ToeNum: 0.02, ToeDenom: 0.3, WhitePoint: 4}

// postEntity is a post_processing_volume of the entity lump.
type postEntity struct {
	vpost    string
	min, max float64
	exposure bool
	master   bool
}

func postFromEntity(vals map[string]any) *postEntity {
	vpost, _ := vals["postprocessing"].(string)
	return &postEntity{
		vpost:    vpost,
		min:      exposureBound(vals, "minexposure", "minlogexposure", 0.25),
		max:      exposureBound(vals, "maxexposure", "maxlogexposure", 8),
		exposure: truthy(vals["enableexposure"]),
		master:   truthy(vals["master"]),
	}
}

// exposureBound reads a linear exposure key, or its log2 form, as Source 2 accepts both.
func exposureBound(vals map[string]any, linear, log2 string, fallback float64) float64 {
	if v, ok := vals[linear]; ok {
		return numberValue(v, fallback)
	}
	if v, ok := vals[log2]; ok {
		return math.Exp2(numberValue(v, math.Log2(fallback)))
	}
	return fallback
}

// addPost reads the volume's .vpost; a volume without auto exposure keeps the exposure at 1.
func (b *sceneBuilder) addPost(e *postEntity) {
	post := &Post{ExposureMin: 1, ExposureMax: 1, Tonemap: defaultTonemap}
	if e.exposure {
		post.ExposureMin, post.ExposureMax = e.min, e.max
	}
	b.scene.Post = post
	data, err := b.files.read(e.vpost + "_c")
	if err != nil {
		b.warnings++
		return
	}
	blocks, err := ResourceBlocks(data)
	if err != nil {
		b.warnings++
		return
	}
	root, err := decodeKV3(blocks["DATA"])
	if err != nil {
		b.warnings++
		return
	}
	vpost := object(root)
	if truthy(vpost["m_bHasTonemapParams"]) {
		t := object(vpost["m_toneMapParams"])
		post.Tonemap = Tonemap{
			ExposureBias: toFloat(t["m_flExposureBias"]), Shoulder: toFloat(t["m_flShoulderStrength"]),
			LinearStrength: toFloat(t["m_flLinearStrength"]), LinearAngle: toFloat(t["m_flLinearAngle"]),
			ToeStrength: toFloat(t["m_flToeStrength"]), ToeNum: toFloat(t["m_flToeNum"]), ToeDenom: toFloat(t["m_flToeDenom"]),
			WhitePoint: toFloat(t["m_flWhitePoint"]),
		}
	}
	size := int(toFloat(vpost["m_nColorCorrectionVolumeDim"]))
	lut := lutBytes(vpost["m_colorCorrectionVolumeData"])
	if truthy(vpost["m_bHasColorCorrection"]) && size > 1 && size <= 64 && len(lut) == size*size*size*4 {
		post.LUTSize, post.LUT = size, lut
	}
}

func lutBytes(v any) []byte {
	if raw, ok := v.([]byte); ok {
		return raw
	}
	values := list(v)
	out := make([]byte, len(values))
	for i, x := range values {
		out[i] = byte(toFloat(x))
	}
	return out
}
