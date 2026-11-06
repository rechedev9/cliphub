package mapgeo

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/bits"
)

// meshoptimizer vertex and index codecs (versions 0 and 1), as CS2 packs its
// render meshes. Port of zeux/meshoptimizer vertexcodec.cpp and indexcodec.cpp
// (MIT) by way of ValveResourceFormat's C# decoders (MIT).
const (
	meshoptVertexHeader = 0xa0
	meshoptIndexHeader  = 0xe0
	vertexBlockBytes    = 8192
	vertexBlockMax      = 256
	byteGroupSize       = 16
	byteGroupLimit      = 24
	tailMinV0           = 32
	tailMinV1           = 24
)

var (
	meshoptBitsV0 = []int{0, 2, 4, 8}
	meshoptBitsV1 = []int{0, 1, 2, 4, 8}
	errMeshopt    = errors.New("malformed meshopt buffer")
)

func vertexBlockSize(vertexSize int) int {
	n := (vertexBlockBytes / vertexSize) &^ (byteGroupSize - 1)
	return min(n, vertexBlockMax)
}

// decodeBytesGroup fills out[:16] from one byte group of the given bit width
// and returns the unread rest of data.
func decodeBytesGroup(data, out []byte, width int) ([]byte, error) {
	switch width {
	case 0:
		clear(out[:byteGroupSize])
		return data, nil
	case 8:
		if len(data) < byteGroupSize {
			return nil, errMeshopt
		}
		copy(out, data[:byteGroupSize])
		return data[byteGroupSize:], nil
	}
	headerBytes := width * 2
	if len(data) < headerBytes {
		return nil, errMeshopt
	}
	extra := headerBytes
	perByte := 8 / width
	sentinel := byte(1<<width - 1)
	for i := 0; i < byteGroupSize; i++ {
		b := data[i/perByte]
		if width == 1 {
			// 1-bit groups store their bits in the reverse order of the wider ones.
			b = bits.Reverse8(b)
		}
		shift := 8 - width*(i%perByte+1)
		v := (b >> shift) & sentinel
		if v == sentinel {
			if extra >= len(data) {
				return nil, errMeshopt
			}
			v = data[extra]
			extra++
		}
		out[i] = v
	}
	return data[extra:], nil
}

func decodeBytes(data, out []byte, widths []int) ([]byte, error) {
	groups := len(out) / byteGroupSize
	headerSize := (groups + 3) / 4
	if len(data) < headerSize {
		return nil, errMeshopt
	}
	header := data[:headerSize]
	data = data[headerSize:]
	for g := 0; g < groups; g++ {
		if len(data) < byteGroupLimit {
			return nil, errMeshopt
		}
		k := int(header[g/4]>>((g%4)*2)) & 3
		if k >= len(widths) {
			return nil, errMeshopt
		}
		var err error
		if data, err = decodeBytesGroup(data, out[g*byteGroupSize:], widths[k]); err != nil {
			return nil, err
		}
	}
	return data, nil
}

func unzigzag(v uint32, size int) uint32 {
	switch size {
	case 1:
		b := uint8(v)
		return uint32(uint8(-(b & 1)) ^ (b >> 1))
	default:
		s := uint16(v)
		return uint32(uint16(-(s & 1)) ^ (s >> 1))
	}
}

// decodeDeltas undoes one 4-byte column of a block: `size` 1 and 2 are
// zigzag deltas, 4 a rotated xor.
func decodeDeltas(opts deltaOpts) {
	buffer, lastVertex := opts.buffer, opts.last
	n := opts.count
	for k := 0; k < 4; k += opts.size {
		offset := k
		var p uint32
		for j := 0; j < opts.size; j++ {
			p |= uint32(lastVertex[j]) << (8 * j)
		}
		for i := 0; i < n; i++ {
			var v uint32
			for j := 0; j < opts.size; j++ {
				v |= uint32(buffer[i+n*j]) << (8 * j)
			}
			switch opts.size {
			case 4:
				v = bits.RotateLeft32(v, opts.rot) ^ p
			default:
				v = (unzigzag(v, opts.size) + p) & (1<<(8*opts.size) - 1)
			}
			for j := 0; j < opts.size; j++ {
				opts.out[offset+j] = byte(v >> (8 * j))
			}
			p = v
			offset += opts.stride
		}
		buffer = buffer[n*opts.size:]
		lastVertex = lastVertex[opts.size:]
	}
}

type deltaOpts struct {
	buffer, out, last []byte
	count, stride     int
	size, rot         int
}

type vertexBlock struct {
	out, last, channels []byte
	count, size         int
	version             int
}

func decodeVertexBlock(data []byte, b vertexBlock) ([]byte, error) {
	aligned := (b.count + byteGroupSize - 1) &^ (byteGroupSize - 1)
	controlSize := 0
	if b.version != 0 {
		controlSize = b.size / 4
	}
	if len(data) < controlSize {
		return nil, errMeshopt
	}
	control := data[:controlSize]
	data = data[controlSize:]
	buffer := make([]byte, vertexBlockMax*4)
	transposed := make([]byte, vertexBlockBytes)
	for k := 0; k < b.size; k += 4 {
		var ctrlByte byte
		if b.version != 0 {
			ctrlByte = control[k/4]
		}
		for j := 0; j < 4; j++ {
			lane := buffer[j*b.count:]
			switch ctrl := int(ctrlByte>>(j*2)) & 3; ctrl {
			case 3:
				if len(data) < b.count {
					return nil, errMeshopt
				}
				copy(lane[:b.count], data[:b.count])
				data = data[b.count:]
			case 2:
				clear(lane[:b.count])
			default:
				widths := meshoptBitsV0
				if b.version != 0 {
					widths = meshoptBitsV1[ctrl:]
				}
				scratch := make([]byte, aligned)
				var err error
				if data, err = decodeBytes(data, scratch, widths); err != nil {
					return nil, err
				}
				copy(lane[:b.count], scratch[:b.count])
			}
		}
		channel := 0
		if b.version != 0 {
			channel = int(b.channels[k/4])
		}
		opts := deltaOpts{buffer: buffer, out: transposed[k:], last: b.last[k:], count: b.count, stride: b.size}
		switch channel & 3 {
		case 0:
			opts.size = 1
		case 1:
			opts.size = 2
		case 2:
			opts.size, opts.rot = 4, (32-(channel>>4))&31
		default:
			return nil, errMeshopt
		}
		decodeDeltas(opts)
	}
	copy(b.out, transposed[:b.count*b.size])
	copy(b.last, transposed[b.size*(b.count-1):b.size*b.count])
	return data, nil
}

// DecodeMeshoptVertices decodes a meshopt vertex buffer of count vertices of
// size bytes each.
func DecodeMeshoptVertices(count, size int, buf []byte) ([]byte, error) {
	if size <= 0 || size > 256 || size%4 != 0 {
		return nil, fmt.Errorf("meshopt vertex size %d", size)
	}
	if len(buf) < 1 || buf[0]&0xf0 != meshoptVertexHeader {
		return nil, errors.New("not a meshopt vertex buffer")
	}
	version := int(buf[0] & 0x0f)
	if version > 1 {
		return nil, fmt.Errorf("meshopt vertex version %d", version)
	}
	buf = buf[1:]
	tail := size
	tailMin := tailMinV0
	if version != 0 {
		tail += size / 4
		tailMin = tailMinV1
	}
	if len(buf) < max(tail, tailMin) {
		return nil, errMeshopt
	}
	out := make([]byte, count*size)
	last := append([]byte(nil), buf[len(buf)-tail:len(buf)-tail+size]...)
	var channels []byte
	if version != 0 {
		channels = buf[len(buf)-tail+size : len(buf)-tail+size+size/4]
	}
	step := vertexBlockSize(size)
	var err error
	for offset := 0; offset < count; offset += step {
		n := min(step, count-offset)
		buf, err = decodeVertexBlock(buf, vertexBlock{out: out[offset*size:], last: last, channels: channels, count: n, size: size, version: version})
		if err != nil {
			return nil, err
		}
	}
	if len(buf) != max(tail, tailMin) {
		return nil, errors.New("meshopt vertex tail size")
	}
	return out, nil
}

type edge struct{ a, b uint32 }

// indexDecoder holds the edge and vertex FIFOs of the index codec.
type indexDecoder struct {
	edges     [16]edge
	verts     [16]uint32
	edgeAt    int
	vertAt    int
	data      []byte
	pos       int
	next      uint32
	last      uint32
	truncated bool
}

func (d *indexDecoder) pushEdge(a, b uint32) {
	d.edges[d.edgeAt] = edge{a, b}
	d.edgeAt = (d.edgeAt + 1) & 15
}

func (d *indexDecoder) pushVertex(v uint32, advance bool) {
	d.verts[d.vertAt] = v
	if advance {
		d.vertAt = (d.vertAt + 1) & 15
	}
}

func (d *indexDecoder) vertex(back int) uint32 {
	return d.verts[(d.vertAt-back)&15]
}

func (d *indexDecoder) vbyte() uint32 {
	if d.pos >= len(d.data) {
		d.truncated = true
		return 0
	}
	lead := uint32(d.data[d.pos])
	d.pos++
	if lead < 128 {
		return lead
	}
	result, shift := lead&127, 7
	for i := 0; i < 4; i++ {
		if d.pos >= len(d.data) {
			d.truncated = true
			return 0
		}
		group := uint32(d.data[d.pos])
		d.pos++
		result |= (group & 127) << shift
		shift += 7
		if group < 128 {
			break
		}
	}
	return result
}

func (d *indexDecoder) freeIndex() uint32 {
	v := d.vbyte()
	d.last += (v >> 1) ^ -(v & 1)
	return d.last
}

// DecodeMeshoptIndices decodes a meshopt triangle index buffer into count
// indices of size 2 or 4 bytes, little endian.
func DecodeMeshoptIndices(count, size int, buf []byte) ([]byte, error) {
	if count%3 != 0 || (size != 2 && size != 4) {
		return nil, fmt.Errorf("meshopt index count %d size %d", count, size)
	}
	dataOffset := 1 + count/3
	if len(buf) < dataOffset+16 || buf[0]&0xf0 != meshoptIndexHeader {
		return nil, errors.New("not a meshopt index buffer")
	}
	version := int(buf[0] & 0x0f)
	if version > 1 {
		return nil, fmt.Errorf("meshopt index version %d", version)
	}
	fecmax := 15
	if version >= 1 {
		fecmax = 13
	}
	code := buf[1:dataOffset]
	data := buf[dataOffset:]
	safeEnd := len(data) - 16
	codeaux := data[safeEnd:]
	d := &indexDecoder{data: data[:safeEnd+16]}
	out := make([]byte, count*size)
	w := 0
	write := func(a, b, c uint32) {
		if size == 2 {
			binary.LittleEndian.PutUint16(out[w:], uint16(a))
			binary.LittleEndian.PutUint16(out[w+2:], uint16(b))
			binary.LittleEndian.PutUint16(out[w+4:], uint16(c))
			w += 6
			return
		}
		binary.LittleEndian.PutUint32(out[w:], a)
		binary.LittleEndian.PutUint32(out[w+4:], b)
		binary.LittleEndian.PutUint32(out[w+8:], c)
		w += 12
	}
	for _, tri := range code {
		if d.pos > safeEnd {
			return nil, errMeshopt
		}
		switch {
		case tri < 0xf0:
			e := d.edges[(d.edgeAt-1-int(tri>>4))&15]
			a, b := e.a, e.b
			fec := int(tri & 15)
			var c uint32
			if fec < fecmax {
				if fec == 0 {
					c = d.next
					d.next++
				} else {
					c = d.vertex(1 + fec)
				}
				d.pushVertex(c, fec == 0)
			} else {
				if fec != 15 {
					d.last += uint32(fec*2 - 27)
					c = d.last
				} else {
					c = d.freeIndex()
				}
				d.pushVertex(c, true)
			}
			d.pushEdge(c, b)
			d.pushEdge(a, c)
			write(a, b, c)
		case tri < 0xfe:
			aux := codeaux[tri&15]
			feb, fec := int(aux>>4), int(aux&15)
			a := d.next
			d.next++
			b := d.vertex(feb)
			if feb == 0 {
				b = d.next
				d.next++
			}
			c := d.vertex(fec)
			if fec == 0 {
				c = d.next
				d.next++
			}
			write(a, b, c)
			d.pushVertex(a, true)
			d.pushVertex(b, feb == 0)
			d.pushVertex(c, fec == 0)
			d.pushEdge(b, a)
			d.pushEdge(c, b)
			d.pushEdge(a, c)
		default:
			aux := d.data[d.pos]
			d.pos++
			feb, fec := int(aux>>4), int(aux&15)
			if aux == 0 {
				d.next = 0
			}
			var a, b, c uint32
			if tri == 0xfe {
				a = d.next
				d.next++
			}
			if feb == 0 {
				b = d.next
				d.next++
			} else {
				b = d.vertex(feb)
			}
			if fec == 0 {
				c = d.next
				d.next++
			} else {
				c = d.vertex(fec)
			}
			if tri != 0xfe {
				a = d.freeIndex()
			}
			if feb == 15 {
				b = d.freeIndex()
			}
			if fec == 15 {
				c = d.freeIndex()
			}
			write(a, b, c)
			d.pushVertex(a, true)
			d.pushVertex(b, feb == 0 || feb == 15)
			d.pushVertex(c, fec == 0 || fec == 15)
			d.pushEdge(b, a)
			d.pushEdge(c, b)
			d.pushEdge(a, c)
		}
		if d.truncated {
			return nil, errMeshopt
		}
	}
	if d.pos != safeEnd {
		return nil, errors.New("meshopt index data length")
	}
	return out, nil
}
