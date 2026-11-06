package gameassets

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/png"

	"github.com/rechedev9/cliphub/internal/mapgeo"
)

// Texture formats of a compiled .vtex_c (ValveResourceFormat's VTexFormat).
const (
	formatDXT1        = 1
	formatDXT5        = 2
	formatRGBA8888    = 4
	formatPNGRGBA8888 = 16
	formatPNGDXT5     = 18
	formatBC7         = 20
	formatBGRA8888    = 28
)

const (
	texFlagCube        = 0x10
	texFlagVolume      = 0x20
	texFlagArray       = 0x40
	extraCompressedMip = 4
	// maxTextureSide bounds the decoded image so a corrupt header cannot ask
	// for gigabytes.
	maxTextureSide = 8192
)

// textureInfo is the header of a texture's DATA block.
type textureInfo struct {
	Width, Height int
	Format        int
	Mips          int
	flags         int
	compressed    []int // LZ4 size of each mip by level, largest first; nil when raw
}

// FormatName names a texture format for logs and reports.
func FormatName(format int) string {
	switch format {
	case formatDXT1:
		return "DXT1"
	case formatDXT5:
		return "DXT5"
	case formatRGBA8888:
		return "RGBA8888"
	case formatPNGRGBA8888:
		return "PNG_RGBA8888"
	case formatPNGDXT5:
		return "PNG_DXT5"
	case formatBC7:
		return "BC7"
	case formatBGRA8888:
		return "BGRA8888"
	}
	return fmt.Sprintf("format %d", format)
}

// parseTextureInfo reads the DATA block layout from ValveResourceFormat's
// Texture: version, flags, reflectivity, size, format, mips, then extra data.
func parseTextureInfo(block []byte) (textureInfo, error) {
	if len(block) < 40 {
		return textureInfo{}, errors.New("texture header is too short")
	}
	if v := binary.LittleEndian.Uint16(block); v != 1 {
		return textureInfo{}, fmt.Errorf("unsupported texture version %d", v)
	}
	info := textureInfo{
		flags:  int(binary.LittleEndian.Uint16(block[2:])),
		Width:  int(binary.LittleEndian.Uint16(block[20:])),
		Height: int(binary.LittleEndian.Uint16(block[22:])),
		Format: int(block[26]),
		Mips:   int(block[27]),
	}
	depth := int(binary.LittleEndian.Uint16(block[24:]))
	if info.flags&(texFlagCube|texFlagVolume|texFlagArray) != 0 || depth > 1 {
		return textureInfo{}, errors.New("cube, volume and array textures are not supported")
	}
	if info.Width == 0 || info.Height == 0 || info.Width > maxTextureSide || info.Height > maxTextureSide || info.Mips == 0 {
		return textureInfo{}, fmt.Errorf("bad texture size %dx%d with %d mips", info.Width, info.Height, info.Mips)
	}
	extraAt := 32 + int(binary.LittleEndian.Uint32(block[32:]))
	count := int(binary.LittleEndian.Uint32(block[36:]))
	for i := 0; i < count; i++ {
		at := extraAt + i*12
		if at < 0 || at+12 > len(block) {
			return textureInfo{}, errors.New("texture extra data outside the block")
		}
		kind := binary.LittleEndian.Uint32(block[at:])
		start := at + 4 + int(binary.LittleEndian.Uint32(block[at+4:]))
		size := int(binary.LittleEndian.Uint32(block[at+8:]))
		if start < 0 || size < 0 || start > len(block) || size > len(block)-start {
			return textureInfo{}, errors.New("texture extra data outside the block")
		}
		if kind == extraCompressedMip {
			// The entry's size covers the 12-byte header; the sizes follow it.
			sizes, err := compressedMipSizes(block[start:])
			if err != nil {
				return textureInfo{}, err
			}
			info.compressed = sizes
		}
	}
	if info.compressed != nil && len(info.compressed) != info.Mips {
		return textureInfo{}, fmt.Errorf("%d compressed mip sizes for %d mips", len(info.compressed), info.Mips)
	}
	return info, nil
}

// compressedMipSizes reads the COMPRESSED_MIP_SIZE extra data: a flag that is
// 1 when mips are LZ4 blocks, an offset to the sizes, and their count.
func compressedMipSizes(data []byte) ([]int, error) {
	if len(data) < 12 {
		return nil, errors.New("compressed mip table is too short")
	}
	if binary.LittleEndian.Uint32(data) != 1 {
		return nil, nil
	}
	at := 4 + int(binary.LittleEndian.Uint32(data[4:]))
	count := int(binary.LittleEndian.Uint32(data[8:]))
	if count < 0 || count > 32 || at < 0 || at+count*4 > len(data) {
		return nil, errors.New("compressed mip table outside the block")
	}
	sizes := make([]int, count)
	for i := range sizes {
		sizes[i] = int(binary.LittleEndian.Uint32(data[at+i*4:]))
	}
	return sizes, nil
}

// mipBytes is the uncompressed size of one mip level.
func (t textureInfo) mipBytes(level int) (int, error) {
	w, h := max(1, t.Width>>level), max(1, t.Height>>level)
	switch t.Format {
	case formatDXT1:
		return ((w + 3) / 4) * ((h + 3) / 4) * 8, nil
	case formatDXT5, formatBC7:
		return ((w + 3) / 4) * ((h + 3) / 4) * 16, nil
	case formatRGBA8888, formatBGRA8888:
		return w * h * 4, nil
	}
	return 0, fmt.Errorf("unsupported texture format %s", FormatName(t.Format))
}

// storedBytes is the size a mip level takes in the file.
func (t textureInfo) storedBytes(level int) (int, error) {
	if t.compressed != nil {
		return t.compressed[level], nil
	}
	return t.mipBytes(level)
}

// DecodeTexture decodes the largest mip of a compiled .vtex_c and returns it
// with the texture's format. Mips follow the resource header smallest first.
func DecodeTexture(resource []byte) (image.Image, int, error) {
	blocks, err := mapgeo.ResourceBlocks(resource)
	if err != nil {
		return nil, 0, err
	}
	block, ok := blocks["DATA"]
	if !ok {
		return nil, 0, errors.New("texture has no DATA block")
	}
	info, err := parseTextureInfo(block)
	if err != nil {
		return nil, 0, err
	}
	start := int(binary.LittleEndian.Uint32(resource))
	if start <= 0 || start > len(resource) {
		return nil, 0, errors.New("texture data outside the file")
	}
	if info.Format == formatPNGRGBA8888 || info.Format == formatPNGDXT5 {
		img, err := png.Decode(bytes.NewReader(resource[start:]))
		return img, info.Format, err
	}
	raw, err := largestMip(resource, start, info)
	if err != nil {
		return nil, 0, err
	}
	img, err := decodeSurface(raw, info)
	if err != nil {
		return nil, 0, err
	}
	if info.Format == formatDXT5 && hasSpecialDependency(blocks["RED2"], "Texture Compiler Version Image YCoCg Conversion") {
		undoYCoCg(img)
	}
	return img, info.Format, nil
}

// largestMip returns mip 0, decompressed. The texture bytes start after the
// resource header (its declared size) and hold the smaller mips first.
func largestMip(resource []byte, at int, info textureInfo) ([]byte, error) {
	for level := info.Mips - 1; level > 0; level-- {
		n, err := info.storedBytes(level)
		if err != nil {
			return nil, err
		}
		at += n
	}
	stored, err := info.storedBytes(0)
	if err != nil {
		return nil, err
	}
	if at < 0 || stored < 0 || at > len(resource) || stored > len(resource)-at {
		return nil, errors.New("texture mip outside the file")
	}
	raw := resource[at : at+stored]
	size, err := info.mipBytes(0)
	if err != nil {
		return nil, err
	}
	if info.compressed != nil && stored != size {
		if raw, err = mapgeo.DecodeLZ4(raw, size); err != nil {
			return nil, fmt.Errorf("decompress texture: %w", err)
		}
	}
	return raw, nil
}

// hasSpecialDependency reports whether the resource's edit info lists a
// compiler step, such as the YCoCg color transform of some DXT5 textures.
func hasSpecialDependency(editInfo []byte, name string) bool {
	if editInfo == nil {
		return false
	}
	root, err := mapgeo.DecodeKV3(editInfo)
	if err != nil {
		return false
	}
	fields, _ := root.(map[string]any)
	deps, _ := fields["m_SpecialDependencies"].([]any)
	for _, dep := range deps {
		if entry, ok := dep.(map[string]any); ok && entry["m_String"] == name {
			return true
		}
	}
	return false
}

// undoYCoCg converts scaled YCoCg-DXT5 pixels (Y in alpha, the scale in blue)
// back to opaque RGB, as the texture compiler stored them.
func undoYCoCg(img *image.NRGBA) {
	for i := 0; i < len(img.Pix); i += 4 {
		p := img.Pix[i : i+4]
		scale := int(p[2]>>3) + 1
		co, cg, y := (int(p[0])-128)/scale, (int(p[1])-128)/scale, int(p[3])
		p[0], p[1], p[2], p[3] = clampByte(y+co-cg), clampByte(y+cg), clampByte(y-co-cg), 255
	}
}

func clampByte(v int) byte {
	return byte(min(255, max(0, v)))
}

func decodeSurface(raw []byte, info textureInfo) (*image.NRGBA, error) {
	switch info.Format {
	case formatDXT1:
		return decodeBlocks(raw, info.Width, info.Height, 8, decodeBC1)
	case formatDXT5:
		return decodeBlocks(raw, info.Width, info.Height, 16, decodeBC3)
	case formatBC7:
		return decodeBlocks(raw, info.Width, info.Height, 16, decodeBC7)
	case formatRGBA8888, formatBGRA8888:
		img := image.NewNRGBA(image.Rect(0, 0, info.Width, info.Height))
		if len(raw) < len(img.Pix) {
			return nil, errors.New("texture pixels are truncated")
		}
		copy(img.Pix, raw)
		if info.Format == formatBGRA8888 {
			for i := 0; i < len(img.Pix); i += 4 {
				img.Pix[i], img.Pix[i+2] = img.Pix[i+2], img.Pix[i]
			}
		}
		return img, nil
	}
	return nil, fmt.Errorf("unsupported texture format %s", FormatName(info.Format))
}
