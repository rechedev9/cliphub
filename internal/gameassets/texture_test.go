package gameassets

import (
	"bytes"
	"encoding/binary"
	"image"
	"strings"
	"testing"
)

type le struct{ bytes.Buffer }

func (b *le) u8(v uint8)   { b.WriteByte(v) }
func (b *le) u16(v uint16) { _ = binary.Write(&b.Buffer, binary.LittleEndian, v) }
func (b *le) u32(v uint32) { _ = binary.Write(&b.Buffer, binary.LittleEndian, v) }

// buildResource lays out a compiled resource holding one DATA block, followed
// by tail (the texture bytes). The header size field marks where tail starts.
func buildResource(data, tail []byte) []byte {
	var out le
	out.u32(uint32(28 + len(data)))
	out.u16(12)
	out.u16(1)
	out.u32(8) // block table right after this field pair
	out.u32(1)
	out.WriteString("DATA")
	out.u32(8) // block right after the table entry
	out.u32(uint32(len(data)))
	out.Write(data)
	out.Write(tail)
	return out.Bytes()
}

type textureSpec struct {
	width, height  int
	format, mips   int
	compressedMips []int // stored size per level, largest first
}

func textureData(spec textureSpec) []byte {
	var b le
	b.u16(1)
	b.u16(0)
	b.Write(make([]byte, 16))
	b.u16(uint16(spec.width))
	b.u16(uint16(spec.height))
	b.u16(1)
	b.u8(uint8(spec.format))
	b.u8(uint8(spec.mips))
	b.u32(0)
	if spec.compressedMips == nil {
		b.u32(0)
		b.u32(0)
		return b.Bytes()
	}
	b.u32(8) // extra data entries start right after the count
	b.u32(1)
	b.u32(extraCompressedMip)
	b.u32(8) // header follows this entry
	b.u32(12)
	b.u32(1)
	b.u32(8)
	b.u32(uint32(len(spec.compressedMips)))
	for _, size := range spec.compressedMips {
		b.u32(uint32(size))
	}
	return b.Bytes()
}

// lz4Literals is a valid LZ4 block made of one literal run.
func lz4Literals(data []byte) []byte {
	var b le
	n := len(data)
	if n < 15 {
		b.u8(uint8(n << 4))
	} else {
		b.u8(0xf0)
		for rest := n - 15; ; rest -= 255 {
			if rest < 255 {
				b.u8(uint8(rest))
				break
			}
			b.u8(255)
		}
	}
	b.Write(data)
	return b.Bytes()
}

func nrgbaPixel(t *testing.T, img image.Image, x, y int) [4]byte {
	t.Helper()
	n, ok := img.(*image.NRGBA)
	if !ok {
		t.Fatalf("image is %T, want *image.NRGBA", img)
	}
	at := n.PixOffset(x, y)
	return [4]byte(n.Pix[at : at+4])
}

var bgra2x2 = []byte{
	10, 20, 30, 255, 40, 50, 60, 255,
	70, 80, 90, 128, 1, 2, 3, 0,
}

func TestDecodeTextureSkipsSmallerMipsAndSwapsBGRA(t *testing.T) {
	tail := append([]byte{9, 9, 9, 9}, bgra2x2...) // the 1x1 mip comes first
	res := buildResource(textureData(textureSpec{width: 2, height: 2, format: formatBGRA8888, mips: 2}), tail)
	img, format, err := DecodeTexture(res)
	if err != nil {
		t.Fatal(err)
	}
	if format != formatBGRA8888 || img.Bounds().Dx() != 2 || img.Bounds().Dy() != 2 {
		t.Fatalf("decoded %s %v", FormatName(format), img.Bounds())
	}
	if got := nrgbaPixel(t, img, 0, 0); got != [4]byte{30, 20, 10, 255} {
		t.Errorf("pixel (0,0) = %v, want BGRA swapped to {30 20 10 255}", got)
	}
	if got := nrgbaPixel(t, img, 0, 1); got != [4]byte{90, 80, 70, 128} {
		t.Errorf("pixel (0,1) = %v", got)
	}
}

func TestDecodeTextureDecompressesLZ4Mips(t *testing.T) {
	mip0 := lz4Literals(bgra2x2)
	tail := append([]byte{9, 9, 9, 9}, mip0...) // a mip stored at its raw size is not compressed
	spec := textureSpec{width: 2, height: 2, format: formatRGBA8888, mips: 2, compressedMips: []int{len(mip0), 4}}
	img, _, err := DecodeTexture(buildResource(textureData(spec), tail))
	if err != nil {
		t.Fatal(err)
	}
	if got := nrgbaPixel(t, img, 1, 0); got != [4]byte{40, 50, 60, 255} {
		t.Errorf("pixel (1,0) = %v, want {40 50 60 255}", got)
	}
}

func TestDecodeTextureBlockFormats(t *testing.T) {
	dxt1 := []byte{0x00, 0xf8, 0x1f, 0x00, 0, 0, 0, 0} // solid red
	img, _, err := DecodeTexture(buildResource(textureData(textureSpec{width: 4, height: 4, format: formatDXT1, mips: 1}), dxt1))
	if err != nil {
		t.Fatal(err)
	}
	if got := nrgbaPixel(t, img, 3, 3); got != [4]byte{255, 0, 0, 255} {
		t.Errorf("DXT1 pixel = %v, want red", got)
	}
	// A 6x2 BC7 texture needs two blocks; the second is reserved mode, so transparent.
	bc7 := make([]byte, 32)
	bc7[0] = 1 << 6
	img, _, err = DecodeTexture(buildResource(textureData(textureSpec{width: 6, height: 2, format: formatBC7, mips: 1}), bc7))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 6 || nrgbaPixel(t, img, 5, 1) != [4]byte{} {
		t.Errorf("BC7 image %v, pixel (5,1) = %v", img.Bounds(), nrgbaPixel(t, img, 5, 1))
	}
}

func TestDecodeTextureRejectsBadInput(t *testing.T) {
	cases := map[string][]byte{
		"not a resource": []byte("nope"),
		"unsupported":    buildResource(textureData(textureSpec{width: 2, height: 2, format: 19, mips: 1}), make([]byte, 64)),
		"truncated":      buildResource(textureData(textureSpec{width: 2, height: 2, format: formatBGRA8888, mips: 1}), make([]byte, 8)),
		"zero size":      buildResource(textureData(textureSpec{width: 0, height: 2, format: formatBGRA8888, mips: 1}), nil),
		"bad lz4":        buildResource(textureData(textureSpec{width: 2, height: 2, format: formatBGRA8888, mips: 1, compressedMips: []int{3}}), []byte{0xf0, 1, 2}),
		"mip count":      buildResource(textureData(textureSpec{width: 2, height: 2, format: formatBGRA8888, mips: 2, compressedMips: []int{3}}), nil),
	}
	for name, res := range cases {
		if _, _, err := DecodeTexture(res); err == nil {
			t.Errorf("%s: DecodeTexture accepted it", name)
		}
	}
	if !strings.Contains(FormatName(19), "19") {
		t.Errorf("FormatName(19) = %q", FormatName(19))
	}
}

func TestUndoYCoCg(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	copy(img.Pix, []byte{148, 138, 0, 128}) // Co +20, Cg +10, scale 1, Y 128
	undoYCoCg(img)
	if got := [4]byte(img.Pix); got != [4]byte{138, 138, 98, 255} {
		t.Errorf("YCoCg pixel = %v, want {138 138 98 255}", got)
	}
}
