package mapgeo

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
)

// DXGI formats the CS2 render meshes use for the attributes Studio reads.
const (
	dxgiR32G32B32A32Float = 2
	dxgiR32G32B32Float    = 6
	dxgiR32G32Float       = 16
	dxgiR8G8B8A8Unorm     = 28
	dxgiR16G16Float       = 34
	dxgiR16G16Unorm       = 35
	dxgiR16G16Snorm       = 37
	dxgiR32Uint           = 42
)

type layoutField struct {
	semantic string
	index    int
	format   int
	offset   int
}

type gpuBuffer struct {
	count, stride int
	data          []byte
	fields        []layoutField
}

// DrawCall is one material's triangles of a render mesh, in model space.
type DrawCall struct {
	Material  string
	Positions []float32
	Normals   []float32
	UVs       []float32
	// UV2 is the lightmap chart coordinate, when the mesh has one.
	UV2 []float32
	// Colors is the vertex paint tint, RGBA.
	Colors []float32
	// Blend is the vertex paint blend of layered materials, RGBA.
	Blend   []float32
	Indices []uint32
	Tint    [3]float32
	Alpha   float32
	// Mesh and Index address the call as a world node's material override does: the model's mesh and its draw call in it.
	Mesh, Index int
}

// ModelDrawCalls decodes every embedded render mesh of a compiled model
// (vmdl_c) into draw calls. Models without embedded meshes return none.
func ModelDrawCalls(data []byte) ([]DrawCall, error) {
	blocks, err := ResourceBlockList(data)
	if err != nil {
		return nil, err
	}
	var ctrl []byte
	for _, b := range blocks {
		if b.Kind == "CTRL" {
			ctrl = b.Data
		}
	}
	if ctrl == nil {
		return nil, nil
	}
	root, err := decodeKV3(ctrl)
	if err != nil {
		return nil, fmt.Errorf("model ctrl: %w", err)
	}
	lod0 := lodZeroMeshes(blocks)
	var calls []DrawCall
	for _, item := range list(object(root)["embedded_meshes"]) {
		mesh := object(item)
		if mesh == nil {
			continue
		}
		if index := meshInt(mesh, "m_nMeshIndex", "mesh_index"); lod0 != nil && index < len(lod0) && !lod0[index] {
			continue
		}
		got, err := embeddedMeshCalls(blocks, mesh)
		if err != nil {
			return nil, err
		}
		calls = append(calls, got...)
	}
	return calls, nil
}

// lodZeroMeshes reports per mesh index whether the model's first LOD draws it;
// nil when the model has no LOD masks.
func lodZeroMeshes(blocks []ResourceBlock) []bool {
	for _, b := range blocks {
		if b.Kind != "DATA" {
			continue
		}
		root, err := decodeKV3(b.Data)
		if err != nil {
			return nil
		}
		masks := floats(object(root)["m_refLODGroupMasks"])
		if len(masks) == 0 {
			return nil
		}
		out := make([]bool, len(masks))
		for i, m := range masks {
			out[i] = uint64(m)&1 != 0 || m == 0
		}
		return out
	}
	return nil
}

func blockAt(blocks []ResourceBlock, index int) ([]byte, error) {
	if index < 0 || index >= len(blocks) {
		return nil, fmt.Errorf("block index %d outside %d blocks", index, len(blocks))
	}
	return blocks[index].Data, nil
}

func embeddedMeshCalls(blocks []ResourceBlock, mesh map[string]any) ([]DrawCall, error) {
	dataBlock, err := blockAt(blocks, meshInt(mesh, "m_nDataBlock", "data_block"))
	if err != nil {
		return nil, err
	}
	meshData, err := decodeKV3(dataBlock)
	if err != nil {
		return nil, fmt.Errorf("mesh data: %w", err)
	}
	vertexBuffers, indexBuffers, err := meshBuffers(blocks, mesh)
	if err != nil {
		return nil, err
	}
	var calls []DrawCall
	meshIndex := meshInt(mesh, "m_nMeshIndex", "mesh_index")
	for _, so := range list(object(meshData)["m_sceneObjects"]) {
		for _, dc := range list(object(so)["m_drawCalls"]) {
			call, ok, err := drawCall(object(dc), vertexBuffers, indexBuffers)
			if err != nil {
				return nil, err
			}
			// Aggregates address draw calls by position, so a skipped one keeps its slot.
			if !ok {
				call = DrawCall{}
			}
			call.Mesh, call.Index = meshIndex, len(calls)
			calls = append(calls, call)
		}
	}
	return calls, nil
}

// meshInt reads an embedded mesh's field under its current name or the one older models use.
func meshInt(mesh map[string]any, name, legacy string) int {
	if v, ok := mesh[name]; ok {
		return int(toFloat(v))
	}
	return int(toFloat(mesh[legacy]))
}

// meshBuffers reads a mesh's GPU buffers, from its own blocks or from the binary VBIB block older models use.
func meshBuffers(blocks []ResourceBlock, mesh map[string]any) (vertex, index []gpuBuffer, err error) {
	if _, legacy := mesh["vbib_block"]; legacy {
		block, err := blockAt(blocks, int(toFloat(mesh["vbib_block"])))
		if err != nil {
			return nil, nil, err
		}
		return readVBIB(block)
	}
	if vertex, err = readBuffers(blocks, list(mesh["m_vertexBuffers"]), true); err != nil {
		return nil, nil, err
	}
	if index, err = readBuffers(blocks, list(mesh["m_indexBuffers"]), false); err != nil {
		return nil, nil, err
	}
	return vertex, index, nil
}

func readBuffers(blocks []ResourceBlock, items []any, vertex bool) ([]gpuBuffer, error) {
	out := make([]gpuBuffer, 0, len(items))
	for _, item := range items {
		b := object(item)
		buf := gpuBuffer{count: int(toFloat(b["m_nElementCount"])), stride: int(toFloat(b["m_nElementSizeInBytes"]))}
		for _, f := range list(b["m_inputLayoutFields"]) {
			field := object(f)
			name, _ := field["m_pSemanticName"].(string)
			if blob, ok := field["m_pSemanticName"].([]byte); ok {
				name = strings.TrimRight(string(blob), "\x00")
			}
			buf.fields = append(buf.fields, layoutField{
				semantic: strings.ToUpper(name),
				index:    int(toFloat(field["m_nSemanticIndex"])),
				format:   int(toFloat(field["m_Format"])),
				offset:   int(toFloat(field["m_nOffset"])),
			})
		}
		raw, err := blockAt(blocks, int(toFloat(b["m_nBlockIndex"])))
		if err != nil {
			return nil, err
		}
		if truthy(b["m_bCompressedZSTD"]) {
			if raw, err = zstdDecode(raw, buf.count*buf.stride+(1<<16)); err != nil {
				return nil, fmt.Errorf("zstd buffer: %w", err)
			}
		}
		switch {
		case !truthy(b["m_bMeshoptCompressed"]):
			buf.data = raw
		case vertex:
			buf.data, err = DecodeMeshoptVertices(buf.count, buf.stride, raw)
		default:
			buf.data, err = DecodeMeshoptIndices(buf.count, buf.stride, raw)
		}
		if err != nil {
			return nil, err
		}
		if len(buf.data) < buf.count*buf.stride {
			return nil, errors.New("gpu buffer shorter than its elements")
		}
		out = append(out, buf)
	}
	return out, nil
}

func truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case nil:
		return false
	}
	return toFloat(v) != 0
}

func handle(v any) (int, int) {
	o := object(v)
	return int(toFloat(o["m_hBuffer"])), int(toFloat(o["m_nBindOffsetBytes"]))
}

// binding is one vertex stream a draw call reads, starting at vertex first.
type binding struct {
	buf   gpuBuffer
	first int
}

func drawCall(dc map[string]any, vbs, ibs []gpuBuffer) (DrawCall, bool, error) {
	if dc == nil {
		return DrawCall{}, false, nil
	}
	if prim, _ := dc["m_nPrimitiveType"].(string); prim != "" && prim != "RENDER_PRIM_TRIANGLES" {
		return DrawCall{}, false, nil
	}
	baseVertex := int(toFloat(dc["m_nBaseVertex"]))
	var streams []binding
	for _, h := range list(dc["m_vertexBuffers"]) {
		index, offset := handle(h)
		if index < 0 || index >= len(vbs) {
			return DrawCall{}, false, errors.New("draw call vertex buffer out of range")
		}
		vb := vbs[index]
		streams = append(streams, binding{buf: vb, first: offset/max(vb.stride, 1) + baseVertex})
	}
	if len(streams) == 0 {
		return DrawCall{}, false, nil
	}
	ibIndex, ibOffset := handle(dc["m_indexBuffer"])
	if ibIndex < 0 || ibIndex >= len(ibs) {
		return DrawCall{}, false, errors.New("draw call index buffer out of range")
	}
	ib := ibs[ibIndex]
	start, count := int(toFloat(dc["m_nStartIndex"])), int(toFloat(dc["m_nIndexCount"]))
	firstIndex := start + ibOffset/max(ib.stride, 1)
	if firstIndex < 0 || count < 0 || firstIndex+count > ib.count {
		return DrawCall{}, false, errors.New("draw call indices outside the buffer")
	}
	material, _ := dc["m_material"].(string)
	call := DrawCall{Material: material, Tint: [3]float32{1, 1, 1}, Alpha: 1}
	if tint := floats(dc["m_vTintColor"]); len(tint) >= 3 {
		call.Tint = [3]float32{float32(tint[0]), float32(tint[1]), float32(tint[2])}
	}
	if a, ok := dc["m_flAlpha"]; ok {
		call.Alpha = float32(toFloat(a))
	}
	// Remap the referenced vertices to a compact range for this call.
	remap := map[uint32]uint32{}
	used := []int{}
	call.Indices = make([]uint32, count)
	for i := 0; i < count; i++ {
		at := (firstIndex + i) * ib.stride
		var idx uint32
		if ib.stride == 2 {
			idx = uint32(binary.LittleEndian.Uint16(ib.data[at:]))
		} else {
			idx = binary.LittleEndian.Uint32(ib.data[at:])
		}
		for _, st := range streams {
			if v := int(idx) + st.first; v < 0 || v >= st.buf.count {
				return DrawCall{}, false, errors.New("draw call vertex outside the buffer")
			}
		}
		local, ok := remap[idx]
		if !ok {
			local = uint32(len(used))
			remap[idx] = local
			used = append(used, int(idx))
		}
		call.Indices[i] = local
	}
	if err := fillAttributes(&call, streams, used); err != nil {
		return DrawCall{}, false, err
	}
	return call, true, nil
}

func floats(v any) []float64 {
	switch x := v.(type) {
	case []float64:
		return x
	case []any:
		out := make([]float64, len(x))
		for i, e := range x {
			out[i] = toFloat(e)
		}
		return out
	}
	return nil
}

// vertexAttr is a layout field located in one of the draw call's streams.
type vertexAttr struct {
	f  layoutField
	st binding
}

func (a vertexAttr) at(v int) []byte {
	return a.st.buf.data[(v+a.st.first)*a.st.buf.stride+a.f.offset:]
}

func findAttribute(streams []binding, semantic string, index int) (vertexAttr, bool) {
	for _, st := range streams {
		for _, f := range st.buf.fields {
			if f.semantic == semantic && f.index == index {
				return vertexAttr{f: f, st: st}, true
			}
		}
	}
	return vertexAttr{}, false
}

func fillAttributes(call *DrawCall, streams []binding, used []int) error {
	pos, ok := findAttribute(streams, "POSITION", 0)
	if !ok || pos.f.format != dxgiR32G32B32Float {
		return errors.New("render mesh without float positions")
	}
	call.Positions = make([]float32, 0, len(used)*3)
	for _, v := range used {
		b := pos.at(v)
		for k := 0; k < 3; k++ {
			call.Positions = append(call.Positions, math.Float32frombits(binary.LittleEndian.Uint32(b[4*k:])))
		}
	}
	if n, ok := findAttribute(streams, "NORMAL", 0); ok {
		call.Normals = make([]float32, 0, len(used)*3)
		for _, v := range used {
			x, y, z := readNormal(n.at(v), n.f.format)
			call.Normals = append(call.Normals, x, y, z)
		}
	}
	if uv, ok := findAttribute(streams, "TEXCOORD", 0); ok {
		call.UVs = readUVs(uv, used)
	}
	// The lightmap chart coordinate is the unnamed texcoord3 stream.
	if uv, ok := findAttribute(streams, "TEXCOORD", 3); ok {
		call.UV2 = readUVs(uv, used)
	}
	if c, ok := findAttribute(streams, "COLOR", 0); ok && c.f.format == dxgiR8G8B8A8Unorm {
		call.Colors = readColors(c, used)
	}
	if c, ok := findAttribute(streams, "TEXCOORD", 4); ok && c.f.format == dxgiR8G8B8A8Unorm {
		call.Blend = readColors(c, used)
	}
	return nil
}

func readColors(c vertexAttr, used []int) []float32 {
	out := make([]float32, 0, len(used)*4)
	for _, v := range used {
		b := c.at(v)
		for k := 0; k < 4; k++ {
			out = append(out, float32(b[k])/255)
		}
	}
	return out
}

func readUVs(a vertexAttr, used []int) []float32 {
	out := make([]float32, 0, len(used)*2)
	for _, v := range used {
		b := a.at(v)
		var u, w float32
		switch a.f.format {
		case dxgiR32G32Float, dxgiR32G32B32Float, dxgiR32G32B32A32Float:
			u = math.Float32frombits(binary.LittleEndian.Uint32(b))
			w = math.Float32frombits(binary.LittleEndian.Uint32(b[4:]))
		case dxgiR16G16Float:
			u, w = halfToFloat(binary.LittleEndian.Uint16(b)), halfToFloat(binary.LittleEndian.Uint16(b[2:]))
		case dxgiR16G16Unorm:
			u, w = float32(binary.LittleEndian.Uint16(b))/65535, float32(binary.LittleEndian.Uint16(b[2:]))/65535
		case dxgiR16G16Snorm:
			u = float32(int16(binary.LittleEndian.Uint16(b))) / 32767
			w = float32(int16(binary.LittleEndian.Uint16(b[2:]))) / 32767
		}
		out = append(out, u, w)
	}
	return out
}

func halfToFloat(h uint16) float32 {
	sign := uint32(h>>15) << 31
	exp := uint32(h>>10) & 0x1f
	mant := uint32(h) & 0x3ff
	switch {
	case exp == 0 && mant == 0:
		return math.Float32frombits(sign)
	case exp == 0:
		for mant&0x400 == 0 {
			mant <<= 1
			exp--
		}
		exp++
		mant &= 0x3ff
	case exp == 31:
		return math.Float32frombits(sign | 0x7f800000 | mant<<13)
	}
	return math.Float32frombits(sign | (exp+112)<<23 | mant<<13)
}

// readNormal unpacks a vertex normal; R32_UINT is the CS2 packed tangent
// frame (ValveResourceFormat DecompressNormalTangents2).
func readNormal(b []byte, format int) (float32, float32, float32) {
	switch format {
	case dxgiR32G32B32Float, dxgiR32G32B32A32Float:
		return math.Float32frombits(binary.LittleEndian.Uint32(b)),
			math.Float32frombits(binary.LittleEndian.Uint32(b[4:])),
			math.Float32frombits(binary.LittleEndian.Uint32(b[8:]))
	case dxgiR32Uint:
		packed := binary.LittleEndian.Uint32(b)
		x := float64((packed>>12)&0x3ff)/1023*2 - 1
		y := float64((packed>>22)&0x3ff)/1023*2 - 1
		return octNormal(x, y)
	case dxgiR8G8B8A8Unorm:
		x := float64(b[0])/255*2 - 1
		y := float64(b[1])/255*2 - 1
		return octNormal(x, y)
	}
	return 0, 0, 1
}

func octNormal(x, y float64) (float32, float32, float32) {
	z := 1 - math.Abs(x) - math.Abs(y)
	neg := math.Max(0, math.Min(1, -z))
	if x >= 0 {
		x -= neg
	} else {
		x += neg
	}
	if y >= 0 {
		y -= neg
	} else {
		y += neg
	}
	l := math.Sqrt(x*x + y*y + z*z)
	if l == 0 {
		return 0, 0, 1
	}
	return float32(x / l), float32(y / l), float32(z / l)
}
