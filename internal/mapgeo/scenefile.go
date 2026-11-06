package mapgeo

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
)

// The scene file: "CHMS", a uint32 format version, a uint32 header length, the
// JSON header, zero padding to 16 bytes, then the binary arrays the header
// points into by byte offset from the start of that binary section. Every
// array starts on a 4-byte boundary.
const (
	sceneMagic = "CHMS"
	// SceneFileVersion versions the encoding; bump it when the header or arrays change.
	SceneFileVersion = 8
)

type sceneRange struct {
	Offset int `json:"offset"`
	Length int `json:"length"`
}

type sceneTextureHeader struct {
	Format string `json:"format"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Faces  int    `json:"faces"`
	// Levels are the byte lengths of the mip chain, largest first, laid end to end.
	Levels []int `json:"levels"`
	sceneRange
}

type sceneMaterialHeader struct {
	Name          string     `json:"name"`
	Shader        string     `json:"shader"`
	Color         int        `json:"color"`
	Color2        int        `json:"color2"`
	Blend         int        `json:"blend"`
	Tint          [3]float32 `json:"tint"`
	Tint2         [3]float32 `json:"tint2"`
	UVScale       [2]float32 `json:"uvScale"`
	UVOffset      [2]float32 `json:"uvOffset"`
	UV2Scale      [2]float32 `json:"uv2Scale"`
	UV2Offset     [2]float32 `json:"uv2Offset"`
	BlendSoftness float32    `json:"blendSoftness"`
	BlendMode     int        `json:"blendMode"`
	AlphaTest     bool       `json:"alphaTest"`
	AlphaRef      float32    `json:"alphaRef"`
	Translucent   bool       `json:"translucent"`
	DoubleSided   bool       `json:"doubleSided"`
	Unlit         bool       `json:"unlit"`
	Overlay       bool       `json:"overlay"`
	Average       [3]float32 `json:"average"`
}

// sceneChunkHeader locates one chunk's arrays. Positions are uint16 triples
// normalised to [min, max]; normals int8 triples; uv float16 pairs; uv2 uint16
// pairs normalised to [0, 1]; color and blend uint8 quads; indices uint16.
type sceneChunkHeader struct {
	Material int         `json:"material"`
	Vertices int         `json:"vertices"`
	Indices  int         `json:"indices"`
	Min      [3]float32  `json:"min"`
	Max      [3]float32  `json:"max"`
	Position sceneRange  `json:"position"`
	Normal   sceneRange  `json:"normal"`
	UV       sceneRange  `json:"uv"`
	UV2      *sceneRange `json:"uv2,omitempty"`
	Color    *sceneRange `json:"color,omitempty"`
	Blend    *sceneRange `json:"blend,omitempty"`
	Index    sceneRange  `json:"index"`
}

type sceneHeader struct {
	Version   int                   `json:"version"`
	Map       string                `json:"map"`
	Build     string                `json:"build"`
	Min       [3]float32            `json:"min"`
	Max       [3]float32            `json:"max"`
	Sun       *Sun                  `json:"sun"`
	Fog       *Fog                  `json:"fog"`
	Sky       *Sky                  `json:"sky"`
	Lightmap  *Lightmap             `json:"lightmap"`
	Post      *scenePostHeader      `json:"post"`
	Textures  []sceneTextureHeader  `json:"textures"`
	Materials []sceneMaterialHeader `json:"materials"`
	Chunks    []sceneChunkHeader    `json:"chunks"`
}

// scenePostHeader is the post processing with its LUT in the binary section.
type scenePostHeader struct {
	Post
	LUT *sceneRange `json:"lut"`
}

var textureFormatNames = map[int]string{
	TexDXT1: "bc1", TexDXT5: "bc3", TexBC6H: "bc6h", TexBC7: "bc7",
	TexATI1N: "bc4", TexATI2N: "bc5", TexRGBA8888: "rgba8", TexBGRA8888: "bgra8",
}

type blobWriter struct {
	buf bytes.Buffer
}

func (w *blobWriter) add(data []byte) sceneRange {
	for w.buf.Len()%4 != 0 {
		w.buf.WriteByte(0)
	}
	r := sceneRange{Offset: w.buf.Len(), Length: len(data)}
	w.buf.Write(data)
	return r
}

func quantize(v, lo, hi float32) uint16 {
	if hi <= lo {
		return 0
	}
	t := (v - lo) / (hi - lo)
	return uint16(math.Round(float64(max(0, min(1, t)) * 65535)))
}

func unorm8(v float32) byte {
	return byte(math.Round(float64(max(0, min(1, v)) * 255)))
}

func snorm8(v float32) byte {
	return byte(int8(math.Round(float64(max(-1, min(1, v)) * 127))))
}

func floatToHalf(f float32) uint16 {
	bits := math.Float32bits(f)
	sign := uint16(bits>>16) & 0x8000
	exp := int((bits>>23)&0xff) - 127 + 15
	mant := bits & 0x7fffff
	switch {
	case exp <= 0:
		if exp < -10 {
			return sign
		}
		mant |= 0x800000
		shift := uint(14 - exp)
		return sign | uint16(mant>>shift)
	case exp >= 31:
		return sign | 0x7bff
	}
	return sign | uint16(exp)<<10 | uint16(mant>>13)
}

func encodeChunk(w *blobWriter, c SceneChunk) sceneChunkHeader {
	n := len(c.Positions) / 3
	h := sceneChunkHeader{Material: c.Material, Vertices: n, Indices: len(c.Indices), Min: c.Min, Max: c.Max}
	pos := make([]byte, n*6)
	for i := 0; i < n; i++ {
		for k := 0; k < 3; k++ {
			binary.LittleEndian.PutUint16(pos[i*6+k*2:], quantize(c.Positions[i*3+k], c.Min[k], c.Max[k]))
		}
	}
	h.Position = w.add(pos)
	nrm := make([]byte, n*3)
	for i := range nrm {
		nrm[i] = snorm8(c.Normals[i])
	}
	h.Normal = w.add(nrm)
	uv := make([]byte, n*4)
	for i := 0; i < n*2; i++ {
		binary.LittleEndian.PutUint16(uv[i*2:], floatToHalf(c.UVs[i]))
	}
	h.UV = w.add(uv)
	if c.UV2 != nil {
		uv2 := make([]byte, n*4)
		for i := 0; i < n*2; i++ {
			binary.LittleEndian.PutUint16(uv2[i*2:], quantize(c.UV2[i], 0, 1))
		}
		r := w.add(uv2)
		h.UV2 = &r
	}
	for _, src := range []struct {
		values []float32
		dst    **sceneRange
	}{{c.Colors, &h.Color}, {c.Blend, &h.Blend}} {
		if src.values == nil {
			continue
		}
		out := make([]byte, len(src.values))
		for i, v := range src.values {
			out[i] = unorm8(v)
		}
		r := w.add(out)
		*src.dst = &r
	}
	idx := make([]byte, len(c.Indices)*2)
	for i, v := range c.Indices {
		binary.LittleEndian.PutUint16(idx[i*2:], v)
	}
	h.Index = w.add(idx)
	return h
}

// WriteScene encodes a scene into the scene file format.
func WriteScene(out io.Writer, s *Scene) error {
	w := &blobWriter{}
	h := sceneHeader{Version: SceneFileVersion, Map: s.Map, Build: s.Build, Min: s.Min, Max: s.Max, Sun: s.Sun, Fog: s.Fog, Sky: s.Sky, Lightmap: s.Lightmap}
	for _, t := range s.Textures {
		name, ok := textureFormatNames[t.Tex.Format]
		if !ok {
			return fmt.Errorf("texture %s has format %d", t.Path, t.Tex.Format)
		}
		h.Textures = append(h.Textures, sceneTextureHeader{Format: name, Width: t.Tex.Width, Height: t.Tex.Height, Faces: t.Tex.Faces, Levels: t.Tex.Levels, sceneRange: w.add(t.Tex.Data)})
	}
	for _, m := range s.Materials {
		h.Materials = append(h.Materials, sceneMaterialHeader{
			Name: m.Name, Shader: m.Shader, Color: m.ColorTex, Color2: m.Color2Tex, Blend: m.BlendTex,
			Tint: m.Tint, Tint2: m.Tint2, UVScale: m.UVScale, UVOffset: m.UVOffset, UV2Scale: m.UV2Scale, UV2Offset: m.UV2Offset,
			BlendSoftness: m.BlendSoftness, BlendMode: m.BlendMode, AlphaTest: m.AlphaTest, AlphaRef: m.AlphaRef, Translucent: m.Translucent,
			DoubleSided: m.DoubleSided, Unlit: m.Unlit, Overlay: m.Overlay, Average: m.Average,
		})
	}
	for _, c := range s.Chunks {
		h.Chunks = append(h.Chunks, encodeChunk(w, c))
	}
	if s.Post != nil {
		h.Post = &scenePostHeader{Post: *s.Post}
		if len(s.Post.LUT) > 0 {
			r := w.add(s.Post.LUT)
			h.Post.LUT = &r
		}
	}
	header, err := json.Marshal(h)
	if err != nil {
		return err
	}
	var head bytes.Buffer
	head.WriteString(sceneMagic)
	_ = binary.Write(&head, binary.LittleEndian, uint32(SceneFileVersion))
	_ = binary.Write(&head, binary.LittleEndian, uint32(len(header)))
	head.Write(header)
	for head.Len()%16 != 0 {
		head.WriteByte(0)
	}
	if _, err := out.Write(head.Bytes()); err != nil {
		return err
	}
	_, err = out.Write(w.buf.Bytes())
	return err
}

// SceneSummary is what a scene file holds, without its arrays.
type SceneSummary struct {
	Map       string `json:"map"`
	Build     string `json:"build"`
	Triangles int    `json:"triangles"`
	Chunks    int    `json:"chunks"`
	Materials int    `json:"materials"`
	Textures  int    `json:"textures"`
	Lightmap  bool   `json:"lightmap"`
	Sun       bool   `json:"sun"`
	Fog       bool   `json:"fog"`
	Sky       bool   `json:"sky"`
	Post      bool   `json:"post"`
}

// maxSceneHeader bounds the JSON header a summary reads.
const maxSceneHeader = 64 << 20

// ReadSceneSummary reads the header of a scene file.
func ReadSceneSummary(r io.Reader) (SceneSummary, error) {
	var head [12]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return SceneSummary{}, err
	}
	if string(head[:4]) != sceneMagic {
		return SceneSummary{}, fmt.Errorf("not a scene file")
	}
	if v := binary.LittleEndian.Uint32(head[4:]); v != SceneFileVersion {
		return SceneSummary{}, fmt.Errorf("scene file version %d", v)
	}
	n := binary.LittleEndian.Uint32(head[8:])
	if n > maxSceneHeader {
		return SceneSummary{}, fmt.Errorf("scene header of %d bytes", n)
	}
	raw := make([]byte, n)
	if _, err := io.ReadFull(r, raw); err != nil {
		return SceneSummary{}, err
	}
	var h sceneHeader
	if err := json.Unmarshal(raw, &h); err != nil {
		return SceneSummary{}, err
	}
	s := SceneSummary{Map: h.Map, Build: h.Build, Chunks: len(h.Chunks), Materials: len(h.Materials), Textures: len(h.Textures),
		Lightmap: h.Lightmap != nil, Sun: h.Sun != nil, Fog: h.Fog != nil, Sky: h.Sky != nil, Post: h.Post != nil}
	for _, c := range h.Chunks {
		s.Triangles += c.Indices / 3
	}
	return s, nil
}
