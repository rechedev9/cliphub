package gameassets

import (
	"encoding/binary"
	"fmt"
	"image"
)

// blockDecoder writes one 4x4 block as 16 RGBA pixels, row by row.
type blockDecoder func(block []byte, out *[64]byte)

// decodeBlocks expands a block-compressed surface of width x height pixels.
func decodeBlocks(data []byte, width, height, blockBytes int, decode blockDecoder) (*image.NRGBA, error) {
	bw, bh := (width+3)/4, (height+3)/4
	if need := bw * bh * blockBytes; len(data) < need {
		return nil, fmt.Errorf("block data is %d bytes, want %d", len(data), need)
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	var px [64]byte
	for by := 0; by < bh; by++ {
		for bx := 0; bx < bw; bx++ {
			at := (by*bw + bx) * blockBytes
			decode(data[at:at+blockBytes], &px)
			for y := 0; y < 4 && by*4+y < height; y++ {
				for x := 0; x < 4 && bx*4+x < width; x++ {
					dst := img.PixOffset(bx*4+x, by*4+y)
					copy(img.Pix[dst:dst+4], px[(y*4+x)*4:])
				}
			}
		}
	}
	return img, nil
}

func expand565(c uint16) [3]int {
	r, g, b := int(c>>11&31), int(c>>5&63), int(c&31)
	return [3]int{r<<3 | r>>2, g<<2 | g>>4, b<<3 | b>>2}
}

// decodeBC1Color reads a DXT1 color block: two RGB565 endpoints and 2-bit
// indices. When the first endpoint is not greater, index 3 is transparent
// black; BC3 color blocks always use the four-color mode.
func decodeBC1Color(block []byte, out *[64]byte, fourColor bool) {
	c0, c1 := binary.LittleEndian.Uint16(block), binary.LittleEndian.Uint16(block[2:])
	e0, e1 := expand565(c0), expand565(c1)
	var palette [4][4]byte
	for ch := 0; ch < 3; ch++ {
		palette[0][ch], palette[1][ch] = byte(e0[ch]), byte(e1[ch])
		if fourColor || c0 > c1 {
			palette[2][ch] = byte((2*e0[ch] + e1[ch]) / 3)
			palette[3][ch] = byte((e0[ch] + 2*e1[ch]) / 3)
		} else {
			palette[2][ch] = byte((e0[ch] + e1[ch]) / 2)
		}
	}
	palette[0][3], palette[1][3], palette[2][3] = 255, 255, 255
	if fourColor || c0 > c1 {
		palette[3][3] = 255
	}
	indices := binary.LittleEndian.Uint32(block[4:])
	for i := 0; i < 16; i++ {
		copy(out[i*4:], palette[indices>>(2*i)&3][:])
	}
}

func decodeBC1(block []byte, out *[64]byte) {
	decodeBC1Color(block, out, false)
}

// decodeBC3 is DXT5: an 8-byte interpolated alpha block, then a BC1 color block.
func decodeBC3(block []byte, out *[64]byte) {
	decodeBC1Color(block[8:], out, true)
	a0, a1 := int(block[0]), int(block[1])
	var alpha [8]int
	alpha[0], alpha[1] = a0, a1
	if a0 > a1 {
		for i := 1; i <= 6; i++ {
			alpha[i+1] = ((7-i)*a0 + i*a1) / 7
		}
	} else {
		for i := 1; i <= 4; i++ {
			alpha[i+1] = ((5-i)*a0 + i*a1) / 5
		}
		alpha[6], alpha[7] = 0, 255
	}
	var bits uint64
	for i := 0; i < 6; i++ {
		bits |= uint64(block[2+i]) << (8 * i)
	}
	for i := 0; i < 16; i++ {
		out[i*4+3] = byte(alpha[bits>>(3*i)&7])
	}
}
