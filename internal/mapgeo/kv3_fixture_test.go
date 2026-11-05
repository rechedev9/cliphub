package mapgeo

import (
	"math"
	"sort"

	"github.com/klauspost/compress/zstd"
)

// kv3Writer encodes a Go value tree as a version 5 binary KV3 block, the
// layout CS2 compiles physics into, so the decoder is tested without any
// Valve file.
type kv3Writer struct {
	strings  []string
	ids      map[string]int
	types    []byte
	objLens  le
	b1       le
	b4       le
	b8       le
	auxB1    le
	blobs    [][]byte
	lz4Split int // LZ4 frame size for blobs; 0 means one frame per blob
}

// auxBytes is encoded as an ARRAY_TYPE_AUXILIARY_BUFFER of uint8.
type auxBytes []byte

func (w *kv3Writer) id(s string) uint32 {
	if w.ids == nil {
		w.ids = map[string]int{}
	}
	if i, ok := w.ids[s]; ok {
		return uint32(i)
	}
	w.ids[s] = len(w.strings)
	w.strings = append(w.strings, s)
	return uint32(len(w.strings) - 1)
}

func (w *kv3Writer) value(v any) {
	switch x := v.(type) {
	case nil:
		w.types = append(w.types, kvNull)
	case bool:
		w.types = append(w.types, map[bool]byte{true: kvTrue, false: kvFalse}[x])
	case int:
		w.types = append(w.types, kvInt32)
		w.b4.u32(uint32(int32(x)))
	case float64:
		w.types = append(w.types, kvDouble)
		w.b8.u64(math.Float64bits(x))
	case string:
		w.types = append(w.types, kvString)
		w.b4.u32(w.id(x))
	case []byte:
		w.types = append(w.types, kvBlob)
		w.blobs = append(w.blobs, x)
	case []float32:
		w.types = append(w.types, kvArrayTyped)
		w.b4.u32(uint32(len(x)))
		w.types = append(w.types, kvFloat)
		for _, f := range x {
			w.b4.f32(f)
		}
	case auxBytes:
		w.types = append(w.types, kvArrayAuxBuffer, kvUint8)
		w.b1.u8(uint8(len(x)))
		w.auxB1.Write(x)
	case []any:
		w.types = append(w.types, kvArray)
		w.b4.u32(uint32(len(x)))
		for _, e := range x {
			w.value(e)
		}
	case map[string]any:
		w.types = append(w.types, kvObject)
		w.objLens.u32(uint32(len(x)))
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		// Types and the b4 lane are separate streams: the member's type byte
		// lands in types and its name id in b4, ahead of the value's own data.
		for _, k := range keys {
			w.b4.u32(w.id(k))
			w.value(x[k])
		}
	}
}

func padTo(b *le, n int) {
	for b.Len()%n != 0 {
		b.u8(0)
	}
}

// encode returns the KV3 block for root compressed with method.
func encodeKV3(root any, method uint32) []byte {
	w := &kv3Writer{}
	w.value(root)
	return w.finish(method)
}

func (w *kv3Writer) finish(method uint32) []byte {
	var buf1 le
	for _, s := range w.strings {
		buf1.cstr(s)
	}
	buf1.Write(w.auxB1.Bytes())
	bytes1 := buf1.Len()
	padTo(&buf1, 4)
	buf1.u32(uint32(len(w.strings)))

	var buf2 le
	buf2.Write(w.objLens.Bytes())
	buf2.Write(w.b1.Bytes())
	if w.b4.Len() > 0 {
		padTo(&buf2, 4)
		buf2.Write(w.b4.Bytes())
	}
	if w.b8.Len() > 0 {
		padTo(&buf2, 8)
		buf2.Write(w.b8.Bytes())
	}
	buf2.Write(w.types)
	blobTotal := 0
	var blobData le
	if len(w.blobs) == 0 {
		buf2.u32(kv3Trailer)
	} else {
		for _, b := range w.blobs {
			buf2.u32(uint32(len(b)))
			blobTotal += len(b)
		}
		buf2.u32(kv3Trailer)
		for _, b := range w.blobs {
			if method == kv3LZ4 {
				for _, frame := range w.lz4Frames(b) {
					buf2.u16(uint16(len(frame)))
					blobData.Write(frame)
				}
			} else {
				blobData.Write(b)
			}
		}
	}
	c1, c2, cb := compress(buf1.Bytes(), method), compress(buf2.Bytes(), method), blobData.Bytes()
	if method == kv3Zstd && len(w.blobs) > 0 {
		cb = compress(blobData.Bytes(), method)
	}
	var out le
	out.u32(kv3Signature | 5)
	out.Write(make([]byte, kv3HeaderGUIDLen))
	out.u32(method)
	out.u16(0)
	out.u16(map[bool]uint16{true: 16384, false: 0}[method == kv3LZ4])
	for _, v := range []int{bytes1, 1, 0, len(w.types)} {
		out.u32(uint32(v))
	}
	out.u16(0)
	out.u16(0)
	for _, v := range []int{buf1.Len() + buf2.Len(), len(c1) + len(c2) + len(cb), len(w.blobs), blobTotal, 0, 0,
		buf1.Len(), len(c1), buf2.Len(), len(c2), w.b1.Len(), 0, w.b4.Len() / 4, w.b8.Len() / 8, 0, w.objLens.Len() / 4, 0, 0} {
		out.u32(uint32(v))
	}
	out.Write(c1)
	out.Write(c2)
	if len(w.blobs) > 0 {
		out.Write(cb)
		out.u32(kv3Trailer)
	}
	return out.Bytes()
}

func compress(b []byte, method uint32) []byte {
	switch method {
	case kv3LZ4:
		return lz4Literals(b)
	case kv3Zstd:
		enc, _ := zstd.NewWriter(nil)
		defer enc.Close()
		return enc.EncodeAll(b, nil)
	}
	return b
}

// lz4Literals is a valid LZ4 block made of one literal-only sequence.
func lz4Literals(b []byte) []byte {
	var out le
	n := len(b)
	if n < 15 {
		out.u8(uint8(n << 4))
	} else {
		out.u8(0xf0)
		for rest := n - 15; ; rest -= 255 {
			if rest < 255 {
				out.u8(uint8(rest))
				break
			}
			out.u8(255)
		}
	}
	out.Write(b)
	return out.Bytes()
}

func (w *kv3Writer) lz4Frames(b []byte) [][]byte {
	if w.lz4Split == 0 || len(b) <= w.lz4Split {
		return [][]byte{lz4Literals(b)}
	}
	var frames [][]byte
	for len(b) > 0 {
		n := min(w.lz4Split, len(b))
		frames = append(frames, lz4Literals(b[:n]))
		b = b[n:]
	}
	return frames
}

// buildResource wraps blocks into a compiled resource file (header version 12).
func buildResource(blocks map[string][]byte) []byte {
	kinds := make([]string, 0, len(blocks))
	for k := range blocks {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	var head, body le
	tableSize := 12 * len(kinds)
	dataStart := 16 + tableSize
	for i, k := range kinds {
		head.WriteString(k)
		fieldPos := 16 + 12*i + 4
		head.u32(uint32(dataStart + body.Len() - fieldPos))
		head.u32(uint32(len(blocks[k])))
		body.Write(blocks[k])
	}
	var out le
	out.u32(uint32(dataStart + body.Len()))
	out.u16(resourceHeaderVersion)
	out.u16(1)
	out.u32(8)
	out.u32(uint32(len(kinds)))
	out.Write(head.Bytes())
	out.Write(body.Bytes())
	return out.Bytes()
}
