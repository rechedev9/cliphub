package mapgeo

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

// The binary VBIB block older compiled models still ship: vertex and index
// buffer headers with their layouts and data, laid out as ValveResourceFormat's
// VBIB reader documents. Offsets are relative to the field that holds them.
const (
	vbibElementSizeMask = 0x3FFFFFF
	vbibRawFlag         = 0x4000000
	vbibZstdFlag        = 0x8000000
	vbibFieldSize       = 32 + 6*4
	// vbibMaxBytes bounds a decoded buffer, so a damaged header cannot ask for gigabytes.
	vbibMaxBytes = 1 << 28
)

func readVBIB(block []byte) (vertex, index []gpuBuffer, err error) {
	if len(block) < 16 {
		return nil, nil, errors.New("vbib block too short")
	}
	le := binary.LittleEndian
	vbOffset, vbCount := int(le.Uint32(block)), int(le.Uint32(block[4:]))
	ibOffset, ibCount := int(le.Uint32(block[8:])), int(le.Uint32(block[12:]))
	if vertex, err = readVBIBBuffers(block, vbOffset, vbCount, true); err != nil {
		return nil, nil, fmt.Errorf("vbib vertex buffers: %w", err)
	}
	if index, err = readVBIBBuffers(block, 8+ibOffset, ibCount, false); err != nil {
		return nil, nil, fmt.Errorf("vbib index buffers: %w", err)
	}
	return vertex, index, nil
}

func readVBIBBuffers(block []byte, at, count int, vertex bool) ([]gpuBuffer, error) {
	le := binary.LittleEndian
	out := make([]gpuBuffer, 0, count)
	for i := 0; i < count; i++ {
		head := at + i*24
		if head < 0 || head+24 > len(block) {
			return nil, errors.New("buffer header outside the block")
		}
		size := le.Uint32(block[head+4:])
		buf := gpuBuffer{count: int(le.Uint32(block[head:])), stride: int(size & vbibElementSizeMask)}
		refA, refB := head+8, head+16
		fieldsAt, fieldCount := refA+int(le.Uint32(block[refA:])), int(le.Uint32(block[refA+4:]))
		dataAt, dataSize := refB+int(le.Uint32(block[refB:])), int(le.Uint32(block[refB+4:]))
		if buf.count*buf.stride > vbibMaxBytes {
			return nil, errors.New("buffer larger than any model needs")
		}
		if fieldsAt+fieldCount*vbibFieldSize > len(block) || dataAt < 0 || dataAt+dataSize > len(block) {
			return nil, errors.New("buffer layout or data outside the block")
		}
		for f := 0; f < fieldCount; f++ {
			field := block[fieldsAt+f*vbibFieldSize:]
			name, _, _ := bytes.Cut(field[:32], []byte{0})
			buf.fields = append(buf.fields, layoutField{
				semantic: strings.ToUpper(string(name)),
				index:    int(int32(le.Uint32(field[32:]))),
				format:   int(le.Uint32(field[36:])),
				offset:   int(le.Uint32(field[40:])),
			})
		}
		data, err := vbibData(block[dataAt:dataAt+dataSize], buf, size, vertex)
		if err != nil {
			return nil, err
		}
		if len(data) < buf.count*buf.stride {
			return nil, errors.New("gpu buffer shorter than its elements")
		}
		buf.data = data
		out = append(out, buf)
	}
	return out, nil
}

func vbibData(raw []byte, buf gpuBuffer, size uint32, vertex bool) ([]byte, error) {
	want := buf.count * buf.stride
	if want <= len(raw) {
		return raw, nil
	}
	var err error
	if size&vbibZstdFlag != 0 {
		if raw, err = zstdDecode(raw, want+(1<<16)); err != nil {
			return nil, fmt.Errorf("zstd buffer: %w", err)
		}
	}
	switch {
	case size&vbibRawFlag != 0:
		return raw, nil
	case vertex:
		return DecodeMeshoptVertices(buf.count, buf.stride, raw)
	default:
		return DecodeMeshoptIndices(buf.count, buf.stride, raw)
	}
}
