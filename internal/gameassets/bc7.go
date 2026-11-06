package gameassets

import "encoding/binary"

// bc7Mode is one row of the BC7 mode table (Direct3D 11 BC7 format spec).
type bc7Mode struct {
	subsets, partitionBits, rotationBits, indexSelBits int
	colorBits, alphaBits                               int
	endpointPBits, sharedPBits                         bool
	indexBits, index2Bits                              int
}

var bc7Modes = [8]bc7Mode{
	{subsets: 3, partitionBits: 4, colorBits: 4, endpointPBits: true, indexBits: 3},
	{subsets: 2, partitionBits: 6, colorBits: 6, sharedPBits: true, indexBits: 3},
	{subsets: 3, partitionBits: 6, colorBits: 5, indexBits: 2},
	{subsets: 2, partitionBits: 6, colorBits: 7, endpointPBits: true, indexBits: 2},
	{subsets: 1, rotationBits: 2, indexSelBits: 1, colorBits: 5, alphaBits: 6, indexBits: 2, index2Bits: 3},
	{subsets: 1, rotationBits: 2, colorBits: 7, alphaBits: 8, indexBits: 2, index2Bits: 2},
	{subsets: 1, colorBits: 7, alphaBits: 7, endpointPBits: true, indexBits: 4},
	{subsets: 2, partitionBits: 6, colorBits: 5, alphaBits: 5, endpointPBits: true, indexBits: 2},
}

// bc7Partition2 holds the two-subset shapes as masks: bit i set puts pixel i in subset 1.
var bc7Partition2 = [64]uint16{
	0xcccc, 0x8888, 0xeeee, 0xecc8, 0xc880, 0xfeec, 0xfec8, 0xec80,
	0xc800, 0xffec, 0xfe80, 0xe800, 0xffe8, 0xff00, 0xfff0, 0xf000,
	0xf710, 0x008e, 0x7100, 0x08ce, 0x008c, 0x7310, 0x3100, 0x8cce,
	0x088c, 0x3110, 0x6666, 0x366c, 0x17e8, 0x0ff0, 0x718e, 0x399c,
	0xaaaa, 0xf0f0, 0x5a5a, 0x33cc, 0x3c3c, 0x55aa, 0x9696, 0xa55a,
	0x73ce, 0x13c8, 0x324c, 0x3bdc, 0x6996, 0xc33c, 0x9966, 0x0660,
	0x0272, 0x04e4, 0x4e40, 0x2720, 0xc936, 0x936c, 0x39c6, 0x639c,
	0x9336, 0x9cc6, 0x817e, 0xe718, 0xccf0, 0x0fcc, 0x7744, 0xee22,
}

// bc7Partition3 holds the three-subset shapes, one digit per pixel.
var bc7Partition3 = [64]string{
	"0011001102212222", "0001001122112221", "0000200122112211", "0222002200110111",
	"0000000011221122", "0011001100220022", "0022002211111111", "0011001122112211",
	"0000000011112222", "0000111111112222", "0000111122222222", "0012001200120012",
	"0112011201120112", "0122012201220122", "0011011211221222", "0011200122002220",
	"0001001101121122", "0111001120012200", "0000112211221122", "0022002200221111",
	"0111011102220222", "0001000122212221", "0000001101220122", "0000110022102210",
	"0122012200110000", "0012001211222222", "0110122112210110", "0000011012211221",
	"0022110211020022", "0110011020022222", "0011012201220011", "0000200022112221",
	"0000000211221222", "0222002200120011", "0011001200220222", "0120012001200120",
	"0000111122220000", "0120120120120120", "0120201212010120", "0011220011220011",
	"0011112222000011", "0101010122222222", "0000000021212121", "0022112200221122",
	"0022001100220011", "0220122102201221", "0101222222220101", "0000212121212121",
	"0101010101012222", "0222011102220111", "0002111200021112", "0000211221122112",
	"0222011101110222", "0002111211120002", "0110011001102222", "0000000021122112",
	"0110011022222222", "0022001100110022", "0022112211220022", "0000000000002112",
	"0002000100020001", "0222122202221222", "0101222222222222", "0111201122012220",
}

// Anchor pixels of subset 1 in two-subset shapes, and of subsets 1 and 2 in
// three-subset shapes. Subset 0 always anchors at pixel 0.
var (
	bc7Anchor2 = [64]uint8{
		15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15,
		15, 2, 8, 2, 2, 8, 8, 15, 2, 8, 2, 2, 8, 8, 2, 2,
		15, 15, 6, 8, 2, 8, 15, 15, 2, 8, 2, 2, 2, 15, 15, 6,
		6, 2, 6, 8, 15, 15, 2, 2, 15, 15, 15, 15, 15, 2, 2, 15,
	}
	bc7Anchor3a = [64]uint8{
		3, 3, 15, 15, 8, 3, 15, 15, 8, 8, 6, 6, 6, 5, 3, 3,
		3, 3, 8, 15, 3, 3, 6, 10, 5, 8, 8, 6, 8, 5, 15, 15,
		8, 15, 3, 5, 6, 10, 8, 15, 15, 3, 15, 5, 15, 15, 15, 15,
		3, 15, 5, 5, 5, 8, 5, 10, 5, 10, 8, 13, 15, 12, 3, 3,
	}
	bc7Anchor3b = [64]uint8{
		15, 8, 8, 3, 15, 15, 3, 8, 15, 15, 15, 15, 15, 15, 15, 8,
		15, 8, 15, 3, 15, 8, 15, 8, 3, 15, 6, 10, 15, 15, 10, 8,
		15, 3, 15, 10, 10, 8, 9, 10, 6, 15, 8, 15, 3, 6, 6, 8,
		15, 3, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 3, 15, 15, 8,
	}
)

var bc7Weights = [5][]int{
	2: {0, 21, 43, 64},
	3: {0, 9, 18, 27, 37, 46, 55, 64},
	4: {0, 4, 9, 13, 17, 21, 26, 30, 34, 38, 43, 47, 51, 55, 60, 64},
}

// bitStream reads a 128-bit block least significant bit first.
type bitStream struct {
	lo, hi uint64
	pos    int
}

func (b *bitStream) read(n int) int {
	var v uint64
	for i := 0; i < n; i++ {
		p := b.pos + i
		var bit uint64
		if p < 64 {
			bit = b.lo >> p & 1
		} else {
			bit = b.hi >> (p - 64) & 1
		}
		v |= bit << i
	}
	b.pos += n
	return int(v)
}

// bc7Subset returns the subset of pixel i for a shape.
func bc7Subset(subsets, partition, i int) int {
	switch subsets {
	case 2:
		return int(bc7Partition2[partition] >> i & 1)
	case 3:
		return int(bc7Partition3[partition][i] - '0')
	}
	return 0
}

func bc7IsAnchor(subsets, partition, i int) bool {
	switch {
	case i == 0:
		return true
	case subsets == 2:
		return i == int(bc7Anchor2[partition])
	case subsets == 3:
		return i == int(bc7Anchor3a[partition]) || i == int(bc7Anchor3b[partition])
	}
	return false
}

// decodeBC7 decodes one BC7 block. A reserved mode (first byte zero) decodes
// to transparent black, as the format requires.
func decodeBC7(block []byte, out *[64]byte) {
	bits := &bitStream{lo: binary.LittleEndian.Uint64(block), hi: binary.LittleEndian.Uint64(block[8:])}
	mode := 0
	for mode < 8 && bits.read(1) == 0 {
		mode++
	}
	if mode == 8 {
		*out = [64]byte{}
		return
	}
	m := bc7Modes[mode]
	partition := bits.read(m.partitionBits)
	rotation := bits.read(m.rotationBits)
	indexSel := bits.read(m.indexSelBits)

	var ep [3][2][4]int // subset, endpoint, channel
	for ch := 0; ch < 3; ch++ {
		for s := 0; s < m.subsets; s++ {
			ep[s][0][ch], ep[s][1][ch] = bits.read(m.colorBits), bits.read(m.colorBits)
		}
	}
	for s := 0; s < m.subsets; s++ {
		ep[s][0][3], ep[s][1][3] = bits.read(m.alphaBits), bits.read(m.alphaBits)
	}
	colorBits, alphaBits := m.colorBits, m.alphaBits
	if m.endpointPBits || m.sharedPBits {
		var pbits [3][2]int
		for s := 0; s < m.subsets; s++ {
			if m.sharedPBits {
				p := bits.read(1)
				pbits[s] = [2]int{p, p}
			} else {
				pbits[s] = [2]int{bits.read(1), bits.read(1)}
			}
		}
		for s := 0; s < m.subsets; s++ {
			for e := 0; e < 2; e++ {
				for ch := 0; ch < 3; ch++ {
					ep[s][e][ch] = ep[s][e][ch]<<1 | pbits[s][e]
				}
				if m.alphaBits > 0 {
					ep[s][e][3] = ep[s][e][3]<<1 | pbits[s][e]
				}
			}
		}
		colorBits++
		if alphaBits > 0 {
			alphaBits++
		}
	}
	for s := 0; s < m.subsets; s++ {
		for e := 0; e < 2; e++ {
			for ch := 0; ch < 3; ch++ {
				ep[s][e][ch] = unquantize(ep[s][e][ch], colorBits)
			}
			if alphaBits > 0 {
				ep[s][e][3] = unquantize(ep[s][e][3], alphaBits)
			} else {
				ep[s][e][3] = 255
			}
		}
	}

	var index, index2 [16]int
	for i := 0; i < 16; i++ {
		n := m.indexBits
		if bc7IsAnchor(m.subsets, partition, i) {
			n--
		}
		index[i] = bits.read(n)
	}
	if m.index2Bits > 0 {
		for i := 0; i < 16; i++ {
			n := m.index2Bits
			if i == 0 {
				n--
			}
			index2[i] = bits.read(n)
		}
	}

	for i := 0; i < 16; i++ {
		s := bc7Subset(m.subsets, partition, i)
		colorIdx, colorW := index[i], bc7Weights[m.indexBits]
		alphaIdx, alphaW := index[i], bc7Weights[m.indexBits]
		if m.index2Bits > 0 {
			alphaIdx, alphaW = index2[i], bc7Weights[m.index2Bits]
			if indexSel == 1 {
				colorIdx, colorW, alphaIdx, alphaW = alphaIdx, alphaW, colorIdx, colorW
			}
		}
		var px [4]int
		for ch := 0; ch < 3; ch++ {
			px[ch] = interpolate(ep[s][0][ch], ep[s][1][ch], colorW[colorIdx])
		}
		px[3] = interpolate(ep[s][0][3], ep[s][1][3], alphaW[alphaIdx])
		if rotation > 0 {
			px[3], px[rotation-1] = px[rotation-1], px[3]
		}
		for ch := 0; ch < 4; ch++ {
			out[i*4+ch] = byte(px[ch])
		}
	}
}

// unquantize widens an n-bit endpoint to 8 bits by replicating its high bits.
func unquantize(v, n int) int {
	v <<= 8 - n
	return v | v>>n
}

func interpolate(e0, e1, w int) int {
	return ((64-w)*e0 + w*e1 + 32) >> 6
}
