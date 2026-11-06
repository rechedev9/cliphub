package mapgeo

import (
	"errors"
	"fmt"
)

const resourceHeaderVersion = 12

// ResourceBlock is one block of a compiled resource, in file order.
type ResourceBlock struct {
	Kind string
	Data []byte
	// Offset is where Data starts in the file.
	Offset int
}

// ResourceBlocks splits a compiled Source 2 resource (a *_c file) into its
// blocks by four-letter type. A type that repeats keeps its last block.
func ResourceBlocks(data []byte) (map[string][]byte, error) {
	list, err := ResourceBlockList(data)
	if err != nil {
		return nil, err
	}
	blocks := make(map[string][]byte, len(list))
	for _, b := range list {
		blocks[b.Kind] = b.Data
	}
	return blocks, nil
}

// ResourceBlockList returns every block in file order; render meshes address
// their MVTX and MIDX buffers by this position. Layout reference:
// ValveResourceFormat's Resource.
func ResourceBlockList(data []byte) ([]ResourceBlock, error) {
	r := &byteReader{data: data}
	r.u32() // file size
	if v := r.u16(); v != resourceHeaderVersion && r.err == nil {
		return nil, fmt.Errorf("not a compiled resource (header version %d)", v)
	}
	r.u16() // resource version
	tableStart := r.pos
	tableOffset := int(r.u32())
	count := int(r.u32())
	if r.err != nil {
		return nil, r.err
	}
	r.pos = tableStart + tableOffset
	if count > r.remaining()/12 {
		return nil, fmt.Errorf("resource block count %d exceeds the file", count)
	}
	blocks := make([]ResourceBlock, 0, count)
	for i := 0; i < count; i++ {
		kind := string(r.take(4))
		at := r.pos
		offset, size := int(r.u32()), int(r.u32())
		if r.err != nil {
			return nil, r.err
		}
		start := at + offset
		if offset < 0 || size < 0 || start > len(data) || size > len(data)-start {
			return nil, errors.New("resource block outside the file")
		}
		blocks = append(blocks, ResourceBlock{Kind: kind, Data: data[start : start+size], Offset: start})
	}
	return blocks, nil
}
