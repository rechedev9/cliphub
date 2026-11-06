package mapgeo

import (
	"errors"
	"fmt"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// Binary KeyValues3 container. Layout reference: ValveResourceFormat's
// BinaryKV3 reader (versions 2 to 5); kv3value.go walks the decoded tree.
const (
	kv3Signature     = 0x4b563300 // "\x003VK" with the version in the low byte
	kv3Uncompressed  = 0
	kv3LZ4           = 1
	kv3Zstd          = 2
	kv3HeaderGUIDLen = 16
	kv3Trailer       = 0xffeedd00
	// maxKV3Bytes caps any decompressed buffer so a corrupt size cannot exhaust memory.
	maxKV3Bytes = 512 << 20
)

type kv3Header struct {
	version                         int
	method                          uint32
	bytes1, bytes2, bytes4, bytes8  int
	types                           int
	uncompressedTotal, compressedTo int
	blocks, blobBytes               int
	uncompressed1, compressed1      int
	uncompressed2, compressed2      int
	b2Bytes1, b2Bytes2, b2Bytes4    int
	b2Bytes8, b2Objects             int
}

// readKV3Header reads the header after aligning r to 8 bytes from the start
// of the data, which is where nav files place their blocks.
func readKV3Header(r *byteReader) (kv3Header, error) {
	magic := r.u32()
	h := kv3Header{version: int(magic & 0xff)}
	if r.err != nil {
		return h, r.err
	}
	if magic&0xffffff00 != kv3Signature || h.version < 2 || h.version > 5 {
		return h, fmt.Errorf("unsupported kv3 block (magic %#x)", magic)
	}
	r.take(kv3HeaderGUIDLen)
	h.method = r.u32()
	if r.u16() != 0 {
		return h, errors.New("kv3 compression dictionaries are not supported")
	}
	r.u16() // compression frame size
	h.bytes1, h.bytes4, h.bytes8, h.types = r.i32(), r.i32(), r.i32(), r.i32()
	r.u16() // object count
	r.u16() // array count
	h.uncompressedTotal, h.compressedTo = r.i32(), r.i32()
	h.blocks, h.blobBytes = r.i32(), r.i32()
	if h.version >= 4 {
		h.bytes2 = r.i32()
		r.u32() // block compressed sizes bytes
	}
	h.uncompressed1, h.compressed1 = h.uncompressedTotal, h.compressedTo
	if h.version >= 5 {
		h.uncompressed1, h.compressed1 = r.i32(), r.i32()
		h.uncompressed2, h.compressed2 = r.i32(), r.i32()
		h.b2Bytes1, h.b2Bytes2, h.b2Bytes4, h.b2Bytes8 = r.i32(), r.i32(), r.i32(), r.i32()
		r.u32() // node count
		h.b2Objects = r.i32()
		r.take(2 * 4) // array and array element counts
	}
	if r.err != nil {
		return h, r.err
	}
	if h.method > kv3Zstd {
		return h, fmt.Errorf("unknown kv3 compression method %d", h.method)
	}
	for _, n := range []int{h.bytes1, h.bytes2, h.bytes4, h.bytes8, h.types, h.blocks, h.blobBytes,
		h.uncompressed1, h.compressed1, h.uncompressed2, h.compressed2, h.b2Bytes1, h.b2Bytes2, h.b2Bytes4, h.b2Bytes8, h.b2Objects} {
		if n < 0 || n > maxKV3Bytes {
			return h, errors.New("kv3 header size out of range")
		}
	}
	return h, nil
}

// skipKV3 moves r past one 8-byte aligned KV3 block without decoding it. The
// nav file embeds a few that carry nothing a floor mesh needs.
func skipKV3(r *byteReader) error {
	r.pos = (r.pos + 7) &^ 7
	h, err := readKV3Header(r)
	if err != nil {
		return err
	}
	if h.blocks != 0 {
		return fmt.Errorf("kv3 block with %d binary blobs is not supported here", h.blocks)
	}
	size := h.compressed1 + h.compressed2
	if h.method == kv3Uncompressed {
		size = h.uncompressed1 + h.uncompressed2
	}
	r.take(size)
	return r.err
}

// kv3Lanes are the typed value streams of one buffer.
type kv3Lanes struct {
	b1, b2, b4, b8 []byte
}

// DecodeKV3 decodes a binary KV3 block, such as a resource's RED2 edit info,
// into the Go values decodeKV3 documents.
func DecodeKV3(data []byte) (any, error) {
	return decodeKV3(data)
}

// decodeKV3 decodes a whole binary KV3 block into Go values: map[string]any,
// []any, string, bool, nil, int64, uint64, float64, []byte for blobs and
// []float64 for packed numeric arrays.
func decodeKV3(data []byte) (any, error) {
	r := &byteReader{data: data}
	h, err := readKV3Header(r)
	if err != nil {
		return nil, err
	}
	d := &kv3Decoder{version: h.version}
	first, err := h.readBuffer(r, h.compressed1, h.firstBufferSize())
	if err != nil {
		return nil, fmt.Errorf("kv3 buffer 1: %w", err)
	}
	blobSizes, err := d.splitBuffers(r, h, first)
	if err != nil {
		return nil, err
	}
	if h.blocks > 0 {
		if err := d.readBlobs(r, h, first, blobSizes); err != nil {
			return nil, fmt.Errorf("kv3 blobs: %w", err)
		}
	}
	root, err := d.root()
	if err != nil {
		return nil, fmt.Errorf("kv3 values: %w", err)
	}
	return root, nil
}

// firstBufferSize is the decompressed size of buffer 1; before version 5 zstd
// also packs the binary blobs into that frame.
func (h kv3Header) firstBufferSize() int {
	if h.version < 5 && h.method == kv3Zstd {
		return h.uncompressed1 + h.blobBytes
	}
	return h.uncompressed1
}

func (h kv3Header) readBuffer(r *byteReader, compressed, size int) ([]byte, error) {
	switch h.method {
	case kv3Uncompressed:
		out := r.take(size)
		return out, r.err
	case kv3LZ4:
		in := r.take(compressed)
		if r.err != nil {
			return nil, r.err
		}
		out := make([]byte, size)
		n, err := lz4DecodeBlock(in, out, 0)
		if err != nil || n != size {
			return nil, errLZ4
		}
		return out, nil
	default:
		in := r.take(compressed)
		if r.err != nil {
			return nil, r.err
		}
		return zstdDecode(in, size)
	}
}

var zstdDecoder = sync.OnceValues(func() (*zstd.Decoder, error) {
	return zstd.NewReader(nil, zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(maxKV3Bytes))
})

func zstdDecode(in []byte, size int) ([]byte, error) {
	dec, err := zstdDecoder()
	if err != nil {
		return nil, err
	}
	out, err := dec.DecodeAll(in, make([]byte, 0, size))
	if err != nil {
		return nil, err
	}
	if len(out) != size {
		return nil, fmt.Errorf("zstd gave %d bytes, want %d", len(out), size)
	}
	return out, nil
}

// splitBuffers cuts the decompressed buffers into lanes, strings and types,
// and returns the region holding the blob sizes.
func (d *kv3Decoder) splitBuffers(r *byteReader, h kv3Header, first []byte) (*byteReader, error) {
	b1 := &byteReader{data: first[:min(len(first), h.uncompressed1)]}
	aux := readLanes(b1, h.bytes1, h.bytes2, h.bytes4, h.bytes8, h.version < 5)
	if b1.err != nil {
		return nil, fmt.Errorf("kv3 buffer 1 lanes: %w", b1.err)
	}
	stringCount := laneI32(&aux.b4)
	if stringCount < 0 || stringCount > len(first) {
		return nil, errors.New("kv3 string count out of range")
	}
	d.aux = aux
	if h.version >= 5 {
		strs, rest, err := readCStrings(aux.b1, stringCount)
		if err != nil {
			return nil, err
		}
		d.strings, d.aux.b1 = strs, rest
		second, err := h.readBuffer(r, h.compressed2, h.uncompressed2)
		if err != nil {
			return nil, fmt.Errorf("kv3 buffer 2: %w", err)
		}
		b2 := &byteReader{data: second}
		d.objectLengths = b2.take(h.b2Objects * 4)
		d.buf = readLanes(b2, h.b2Bytes1, h.b2Bytes2, h.b2Bytes4, h.b2Bytes8, false)
		d.types = b2.take(h.types)
		return b2, b2.err
	}
	strs, rest, err := readCStrings(b1.data[b1.pos:], stringCount)
	if err != nil {
		return nil, err
	}
	d.strings, d.buf = strs, aux
	stringBytes := len(b1.data) - b1.pos - len(rest)
	rr := &byteReader{data: rest}
	d.types = rr.take(h.types - stringBytes)
	return rr, rr.err
}

func readLanes(r *byteReader, n1, n2, n4, n8 int, alignEmpty8 bool) kv3Lanes {
	var l kv3Lanes
	l.b1 = r.take(n1)
	if n2 > 0 {
		r.pos = (r.pos + 1) &^ 1
		l.b2 = r.take(n2 * 2)
	}
	if n4 > 0 {
		r.pos = (r.pos + 3) &^ 3
		l.b4 = r.take(n4 * 4)
	}
	if n8 > 0 || alignEmpty8 {
		r.pos = (r.pos + 7) &^ 7
		l.b8 = r.take(n8 * 8)
	}
	if r.pos > len(r.data) && r.err == nil {
		r.err = errTruncated
	}
	return l
}

func readCStrings(b []byte, count int) ([]string, []byte, error) {
	out := make([]string, count)
	r := &byteReader{data: b}
	for i := range out {
		out[i] = r.cstring()
	}
	if r.err != nil {
		return nil, nil, fmt.Errorf("kv3 strings: %w", r.err)
	}
	return out, b[r.pos:], nil
}

// readBlobs reads the blob lengths from sizes, then the blob bytes from r.
func (d *kv3Decoder) readBlobs(r *byteReader, h kv3Header, first []byte, sizes *byteReader) error {
	lengths := make([]int, h.blocks)
	total := 0
	for i := range lengths {
		lengths[i] = int(sizes.u32())
		total += lengths[i]
	}
	if sizes.u32() != kv3Trailer || sizes.err != nil {
		return errors.New("bad blob size table")
	}
	if total != h.blobBytes {
		return fmt.Errorf("blob lengths add to %d, want %d", total, h.blobBytes)
	}
	var all []byte
	switch {
	case h.method == kv3Uncompressed:
		all = r.take(total)
	case h.method == kv3LZ4:
		all = make([]byte, total)
		if err := decodeLZ4Frames(r, sizes, all, lengths); err != nil {
			return err
		}
	case h.version >= 5:
		in := r.take(h.compressedTo - h.compressed1 - h.compressed2)
		if r.err != nil {
			return r.err
		}
		var err error
		if all, err = zstdDecode(in, total); err != nil {
			return err
		}
	default:
		all = first[h.uncompressed1:]
	}
	if r.u32() != kv3Trailer || r.err != nil {
		return errors.New("missing blob trailer")
	}
	d.blobs = make([][]byte, len(lengths))
	for i, n := range lengths {
		d.blobs[i], all = all[:n:n], all[n:]
	}
	return nil
}

// decodeLZ4Frames fills out blob by blob; every frame size is a u16 in sizes,
// and frames chain so a match can reach into earlier blobs.
func decodeLZ4Frames(r, sizes *byteReader, out []byte, lengths []int) error {
	pos := 0
	for _, n := range lengths {
		end := pos + n
		for pos < end {
			in := r.take(int(sizes.u16()))
			if r.err != nil || sizes.err != nil {
				return errTruncated
			}
			written, err := lz4DecodeBlock(in, out[:end], pos)
			if err != nil || written == 0 {
				return errLZ4
			}
			pos += written
		}
	}
	return nil
}
