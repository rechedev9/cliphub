package mapgeo

import "fmt"

const (
	kv3Signature     = 0x4b563300 // "\x003VK" with the version in the low byte
	kv3Uncompressed  = 0
	kv3HeaderGUIDLen = 16
)

// skipKV3 moves r past one binary KeyValues3 block without decoding it. The
// nav file embeds a few (movable mesh settings, game data) that carry nothing
// a floor mesh needs. Layout reference: ValveResourceFormat's BinaryKV3 reader.
func skipKV3(r *byteReader) error {
	r.pos = (r.pos + 7) &^ 7 // blocks start 8-byte aligned from the file start
	magic := r.u32()
	version := magic & 0xff
	if r.err != nil {
		return r.err
	}
	if magic&0xffffff00 != kv3Signature || version < 2 || version > 5 {
		return fmt.Errorf("unsupported kv3 block (magic %#x)", magic)
	}
	r.take(kv3HeaderGUIDLen)
	method := r.u32()
	r.u16() // compression dictionary id
	r.u16() // compression frame size
	r.take(4 * 4)
	r.u16() // object count
	r.u16() // array count
	uncompressedTotal := r.u32()
	compressedTotal := r.u32()
	blocks := r.u32()
	r.u32() // binary blob bytes
	if version >= 4 {
		r.take(2 * 4)
	}
	size := compressedTotal
	if method == kv3Uncompressed {
		size = uncompressedTotal
	}
	if version >= 5 {
		uncompressed1, compressed1 := r.u32(), r.u32()
		uncompressed2, compressed2 := r.u32(), r.u32()
		r.take(8 * 4)
		size = compressed1 + compressed2
		if method == kv3Uncompressed {
			size = uncompressed1 + uncompressed2
		}
	}
	if r.err != nil {
		return r.err
	}
	if blocks != 0 {
		return fmt.Errorf("kv3 block with %d binary blobs is not supported", blocks)
	}
	r.take(int(size))
	return r.err
}
