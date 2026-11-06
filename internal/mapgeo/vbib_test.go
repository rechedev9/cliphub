package mapgeo

import (
	"encoding/binary"
	"math"
	"testing"
)

// vbibBlock lays out one raw vertex buffer (POSITION, float3) and one raw uint16 index buffer.
func vbibBlock(positions []float32, indices []uint16) []byte {
	le := binary.LittleEndian
	vertexData := make([]byte, 4*len(positions))
	for i, p := range positions {
		le.PutUint32(vertexData[i*4:], math.Float32bits(p))
	}
	indexData := make([]byte, 2*len(indices))
	for i, v := range indices {
		le.PutUint16(indexData[i*2:], v)
	}
	field := make([]byte, vbibFieldSize)
	copy(field, "POSITION")
	le.PutUint32(field[36:], 6) // DXGI_FORMAT_R32G32B32_FLOAT
	const headerAt, vbAt, ibAt = 0, 16, 40
	fieldsAt := ibAt + 24
	vdataAt := fieldsAt + len(field)
	idataAt := vdataAt + len(vertexData)
	block := make([]byte, idataAt+len(indexData))
	le.PutUint32(block[headerAt:], vbAt)
	le.PutUint32(block[headerAt+4:], 1)
	le.PutUint32(block[headerAt+8:], ibAt-8)
	le.PutUint32(block[headerAt+12:], 1)
	header := func(at, count, stride, fields, nfields, data, size int) {
		le.PutUint32(block[at:], uint32(count))
		le.PutUint32(block[at+4:], uint32(stride)|vbibRawFlag)
		le.PutUint32(block[at+8:], uint32(fields-(at+8)))
		le.PutUint32(block[at+12:], uint32(nfields))
		le.PutUint32(block[at+16:], uint32(data-(at+16)))
		le.PutUint32(block[at+20:], uint32(size))
	}
	header(vbAt, len(positions)/3, 12, fieldsAt, 1, vdataAt, len(vertexData))
	header(ibAt, len(indices), 2, fieldsAt, 0, idataAt, len(indexData))
	copy(block[fieldsAt:], field)
	copy(block[vdataAt:], vertexData)
	copy(block[idataAt:], indexData)
	return block
}

func TestReadVBIBReadsRawBuffers(t *testing.T) {
	vertex, index, err := readVBIB(vbibBlock([]float32{0, 0, 0, 1, 0, 0, 0, 1, 0}, []uint16{0, 1, 2}))
	if err != nil {
		t.Fatal(err)
	}
	if len(vertex) != 1 || len(index) != 1 {
		t.Fatalf("buffers = %d vertex, %d index", len(vertex), len(index))
	}
	vb := vertex[0]
	if vb.count != 3 || vb.stride != 12 || len(vb.fields) != 1 || vb.fields[0].semantic != "POSITION" || vb.fields[0].format != 6 {
		t.Fatalf("vertex buffer = %+v", vb)
	}
	if got := binary.LittleEndian.Uint32(vb.data[12:]); got != math.Float32bits(1) {
		t.Fatalf("second vertex x bits = %x", got)
	}
	if ib := index[0]; ib.count != 3 || ib.stride != 2 || binary.LittleEndian.Uint16(ib.data[4:]) != 2 {
		t.Fatalf("index buffer = %+v", ib)
	}
}

func TestReadVBIBRejectsHeadersOutsideTheBlock(t *testing.T) {
	block := vbibBlock([]float32{0, 0, 0}, []uint16{0})
	binary.LittleEndian.PutUint32(block[4:], 9)
	if _, _, err := readVBIB(block); err == nil {
		t.Fatal("want an error for buffer headers past the block")
	}
}

func TestReadVBIBRejectsAHugeBufferCount(t *testing.T) {
	block := vbibBlock([]float32{0, 0, 0}, []uint16{0})
	binary.LittleEndian.PutUint32(block[4:], math.MaxUint32)
	if _, _, err := readVBIB(block); err == nil {
		t.Fatal("want an error for a buffer count the block cannot hold")
	}
}
