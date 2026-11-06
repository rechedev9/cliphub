package mapgeo

import (
	"errors"
	"fmt"
)

// Texture formats (VTexFormat) Studio reads from compiled textures.
const (
	TexDXT1     = 1
	TexDXT5     = 2
	TexRGBA8888 = 4
	TexBC6H     = 19
	TexBC7      = 20
	TexATI2N    = 21
	TexBGRA8888 = 28
	TexATI1N    = 27
)

const (
	vtexCubeFlag           = 1 << 4
	vtexVolumeFlag         = 1 << 5
	vtexExtraCompressedMip = 4
	mipCompressionLZ4      = 1
)

// Texture is a mip chain of a compiled 2D or cube texture, still in its GPU format.
type Texture struct {
	Format        int
	Width, Height int
	// Faces is 6 for a cube texture, whose every level holds its faces in D3D order (+X -X +Y -Y +Z -Z).
	Faces int
	// Data is the mip chain from Width x Height down, in 4x4 blocks for the
	// block compressed formats; Levels holds each level's length in bytes.
	Data   []byte
	Levels []int
	// Mips is how many levels the source holds; Level is the first one in Data.
	Mips, Level int
	// Reflectivity is the texture's average linear colour, as the compiler stored it.
	Reflectivity [4]float32
}

type vtexHeader struct {
	flags                int
	reflectivity         [4]float32
	width, height, depth int
	format, mips         int
	compressedMips       []int
	lz4                  bool
	dataOffset           int
	faces                int
}

func blockBytes(format int) (int, bool) {
	switch format {
	case TexDXT1, TexATI1N:
		return 8, true
	case TexDXT5, TexBC6H, TexBC7, TexATI2N:
		return 16, true
	case TexRGBA8888, TexBGRA8888:
		return 4, false
	}
	return 0, false
}

func mipSize(size, level int) int {
	return max(1, size>>level)
}

func (h vtexHeader) levelBytes(level int) int {
	w, ht := mipSize(h.width, level), mipSize(h.height, level)
	bpp, compressed := blockBytes(h.format)
	faces := max(1, h.faces)
	if compressed {
		return ((w + 3) / 4) * ((ht + 3) / 4) * bpp * faces
	}
	return w * ht * bpp * faces
}

func readVtexHeader(file []byte) (vtexHeader, error) {
	blocks, err := ResourceBlockList(file)
	if err != nil {
		return vtexHeader{}, err
	}
	var data *ResourceBlock
	for i := range blocks {
		if blocks[i].Kind == "DATA" {
			data = &blocks[i]
		}
	}
	if data == nil {
		return vtexHeader{}, errors.New("texture without DATA block")
	}
	r := &byteReader{data: data.Data}
	if v := r.u16(); v != 1 && r.err == nil {
		return vtexHeader{}, fmt.Errorf("vtex version %d", v)
	}
	h := vtexHeader{flags: int(r.u16())}
	for i := range h.reflectivity {
		h.reflectivity[i] = r.f32()
	}
	h.width, h.height, h.depth = int(r.u16()), int(r.u16()), int(r.u16())
	h.format, h.mips = int(r.u8()), int(r.u8())
	r.u32() // picmip0 resolution
	extraStart := r.pos
	extraOffset, extraCount := int(r.u32()), int(r.u32())
	if r.err != nil {
		return h, r.err
	}
	at := extraStart + extraOffset
	for i := 0; i < extraCount; i++ {
		e := &byteReader{data: data.Data, pos: at + 12*i}
		kind := e.u32()
		fieldAt := e.pos
		offset, size := int(e.u32()), int(e.u32())
		if e.err != nil {
			return h, e.err
		}
		if kind != vtexExtraCompressedMip {
			continue
		}
		m := &byteReader{data: data.Data, pos: fieldAt + offset}
		method := m.u32()
		mipsAt := m.pos
		mipsOffset, count := int(m.u32()), int(m.u32())
		m.pos = mipsAt + mipsOffset
		if count > size || count > 64 {
			return h, errors.New("vtex compressed mip table")
		}
		h.compressedMips = make([]int, count)
		for j := range h.compressedMips {
			h.compressedMips[j] = m.i32()
		}
		if m.err != nil {
			return h, m.err
		}
		h.lz4 = method == mipCompressionLZ4
	}
	h.dataOffset = data.Offset + len(data.Data)
	return h, nil
}

// ReadTexture returns a compiled 2D texture from the largest mip level no
// wider or taller than maxSize down to its smallest level. Cube and volume
// textures are refused.
func ReadTexture(file []byte, maxSize int) (Texture, error) {
	return readTexture(file, maxSize, false)
}

// ReadCubeTexture is ReadTexture for a cube texture, such as a sky.
func ReadCubeTexture(file []byte, maxSize int) (Texture, error) {
	return readTexture(file, maxSize, true)
}

func readTexture(file []byte, maxSize int, cube bool) (Texture, error) {
	h, err := readVtexHeader(file)
	if err != nil {
		return Texture{}, err
	}
	if (h.flags&vtexCubeFlag != 0) != cube || h.flags&vtexVolumeFlag != 0 || h.depth > 1 {
		return Texture{}, errors.New("texture is not of the expected kind")
	}
	if cube {
		h.faces = 6
	}
	if _, ok := blockBytes(h.format); !ok {
		return Texture{}, fmt.Errorf("texture format %d is not supported", h.format)
	}
	mips := max(h.mips, 1)
	level := 0
	for level < mips-1 && (mipSize(h.width, level) > maxSize || mipSize(h.height, level) > maxSize) {
		level++
	}
	// Mips are stored smallest first; the result lists them largest first.
	levels := make([][]byte, mips)
	pos := h.dataOffset
	for j := mips - 1; j >= level; j-- {
		stored, want := h.storedBytes(j), h.levelBytes(j)
		if pos < 0 || stored < 0 || pos+stored > len(file) {
			return Texture{}, errors.New("texture data outside the file")
		}
		raw := file[pos : pos+stored]
		pos += stored
		if stored < want {
			if !h.lz4 {
				return Texture{}, errors.New("compressed mip with an unsupported method")
			}
			if raw, err = DecodeLZ4(raw, want); err != nil {
				return Texture{}, err
			}
		}
		levels[j] = raw
	}
	tex := Texture{Format: h.format, Width: mipSize(h.width, level), Height: mipSize(h.height, level), Faces: max(1, h.faces), Mips: mips, Level: level, Reflectivity: h.reflectivity}
	for j := level; j < mips; j++ {
		tex.Data = append(tex.Data, levels[j]...)
		tex.Levels = append(tex.Levels, len(levels[j]))
	}
	return tex, nil
}

func (h vtexHeader) storedBytes(level int) int {
	size := h.levelBytes(level)
	if level < len(h.compressedMips) && h.compressedMips[level] < size {
		return h.compressedMips[level]
	}
	return size
}
