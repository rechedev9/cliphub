package mapgeo

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// KV3 binary node types.
const (
	kvNull           = 1
	kvBool           = 2
	kvInt64          = 3
	kvUint64         = 4
	kvDouble         = 5
	kvString         = 6
	kvBlob           = 7
	kvArray          = 8
	kvObject         = 9
	kvArrayTyped     = 10
	kvInt32          = 11
	kvUint32         = 12
	kvTrue           = 13
	kvFalse          = 14
	kvInt64Zero      = 15
	kvInt64One       = 16
	kvDoubleZero     = 17
	kvDoubleOne      = 18
	kvFloat          = 19
	kvInt16          = 20
	kvUint16         = 21
	kvInt8           = 22
	kvUint8          = 23
	kvArrayByteLen   = 24
	kvArrayAuxBuffer = 25
	kvUint32Zero     = 26
	kvUint32One      = 27
	// maxKV3Depth bounds recursion on hostile input.
	maxKV3Depth = 64
)

var errKV3Lane = errors.New("kv3 value lane exhausted")

type kv3Decoder struct {
	version       int
	types         []byte
	objectLengths []byte
	strings       []string
	buf, aux      kv3Lanes
	blobs         [][]byte
	nextBlob      int
	err           error
}

func (d *kv3Decoder) root() (any, error) {
	t := d.readType()
	v := d.value(t, &d.buf, 0)
	return v, d.err
}

func (d *kv3Decoder) fail(err error) {
	if d.err == nil {
		d.err = err
	}
}

func (d *kv3Decoder) readType() byte {
	if len(d.types) == 0 {
		d.fail(errors.New("kv3 type stream exhausted"))
		return 0
	}
	t := d.types[0]
	d.types = d.types[1:]
	skip := 0
	mask := byte(0x7f)
	if t&0x80 != 0 {
		skip++ // flag byte; flags only matter for text output
	}
	if d.version >= 3 {
		mask = 0x3f
		if t&0x40 != 0 {
			skip++
		}
	}
	if skip > len(d.types) {
		d.fail(errors.New("kv3 type stream exhausted"))
		return 0
	}
	d.types = d.types[skip:]
	return t & mask
}

func laneTake(lane *[]byte, n int) []byte {
	if n < 0 || len(*lane) < n {
		return nil
	}
	out := (*lane)[:n]
	*lane = (*lane)[n:]
	return out
}

func laneI32(lane *[]byte) int {
	b := laneTake(lane, 4)
	if b == nil {
		return -1
	}
	return int(int32(binary.LittleEndian.Uint32(b)))
}

func (d *kv3Decoder) take(lane *[]byte, n int) []byte {
	b := laneTake(lane, n)
	if b == nil {
		d.fail(errKV3Lane)
	}
	return b
}

func (d *kv3Decoder) length(lane *[]byte) int {
	n := laneI32(lane)
	if n < 0 {
		d.fail(errKV3Lane)
		return 0
	}
	return n
}

func (d *kv3Decoder) str(id int) string {
	if id < 0 || id >= len(d.strings) {
		return ""
	}
	return d.strings[id]
}

// value reads one value; primitives come from lane, everything else from the main buffer.
func (d *kv3Decoder) value(t byte, lane *kv3Lanes, depth int) any {
	if d.err != nil {
		return nil
	}
	if depth > maxKV3Depth {
		d.fail(errors.New("kv3 nesting too deep"))
		return nil
	}
	switch t {
	case kvNull:
		return nil
	case kvTrue, kvFalse:
		return t == kvTrue
	case kvInt64Zero, kvInt64One:
		return int64(t - kvInt64Zero)
	case kvDoubleZero, kvDoubleOne:
		return float64(t - kvDoubleZero)
	case kvUint32Zero, kvUint32One:
		return uint64(t - kvUint32Zero)
	case kvString:
		return d.str(laneI32(&d.buf.b4))
	case kvBlob:
		if d.nextBlob >= len(d.blobs) {
			d.fail(errors.New("kv3 blob index out of range"))
			return nil
		}
		d.nextBlob++
		return d.blobs[d.nextBlob-1]
	case kvArray:
		n := d.length(&d.buf.b4)
		out := make([]any, 0, min(n, len(d.types)))
		for i := 0; i < n && d.err == nil; i++ {
			out = append(out, d.value(d.readType(), &d.buf, depth+1))
		}
		return out
	case kvArrayTyped, kvArrayByteLen, kvArrayAuxBuffer:
		return d.typedArray(t, depth)
	case kvObject:
		return d.object(depth)
	}
	if v, ok := d.number(t, lane); ok {
		return v
	}
	d.fail(fmt.Errorf("unknown kv3 type %d", t))
	return nil
}

func (d *kv3Decoder) object(depth int) map[string]any {
	var n int
	if d.version >= 5 {
		n = d.length(&d.objectLengths)
	} else {
		n = d.length(&d.buf.b4)
	}
	out := make(map[string]any, min(n, len(d.types)))
	for i := 0; i < n && d.err == nil; i++ {
		t := d.readType()
		name := d.str(laneI32(&d.buf.b4))
		out[name] = d.value(t, &d.buf, depth+1)
	}
	return out
}

func (d *kv3Decoder) typedArray(t byte, depth int) any {
	var n int
	if t == kvArrayTyped {
		n = d.length(&d.buf.b4)
	} else if b := d.take(&d.buf.b1, 1); b != nil {
		n = int(b[0])
	}
	sub := d.readType()
	lane := &d.buf
	if t == kvArrayAuxBuffer {
		lane = &d.aux
	}
	if packed, ok := d.packed(sub, lane, n); ok {
		return packed
	}
	out := make([]any, 0, min(n, len(d.types)+len(d.buf.b4)))
	for i := 0; i < n && d.err == nil; i++ {
		out = append(out, d.value(sub, lane, depth+1))
	}
	return out
}

// packed reads an array of one numeric type as []float64; float64 holds every
// value physics data stores (indices, counts, 32-bit floats) exactly.
func (d *kv3Decoder) packed(t byte, lane *kv3Lanes, n int) ([]float64, bool) {
	if _, ok := numberWidth(t); !ok {
		return nil, false
	}
	out := make([]float64, 0, min(n, 1<<20))
	for i := 0; i < n && d.err == nil; i++ {
		v, _ := d.number(t, lane)
		out = append(out, toFloat(v))
	}
	return out, true
}

func numberWidth(t byte) (int, bool) {
	switch t {
	case kvBool, kvInt8, kvUint8:
		return 1, true
	case kvInt16, kvUint16:
		return 2, true
	case kvInt32, kvUint32, kvFloat:
		return 4, true
	case kvInt64, kvUint64, kvDouble:
		return 8, true
	}
	return 0, false
}

func (d *kv3Decoder) number(t byte, lane *kv3Lanes) (any, bool) {
	width, ok := numberWidth(t)
	if !ok {
		return nil, false
	}
	src := &lane.b8
	switch width {
	case 1:
		src = &lane.b1
	case 2:
		src = &lane.b2
	case 4:
		src = &lane.b4
	}
	b := d.take(src, width)
	if b == nil {
		return int64(0), true
	}
	le := binary.LittleEndian
	switch t {
	case kvBool:
		return b[0] != 0, true
	case kvInt8:
		return int64(int8(b[0])), true
	case kvUint8:
		return uint64(b[0]), true
	case kvInt16:
		return int64(int16(le.Uint16(b))), true
	case kvUint16:
		return uint64(le.Uint16(b)), true
	case kvInt32:
		return int64(int32(le.Uint32(b))), true
	case kvUint32:
		return uint64(le.Uint32(b)), true
	case kvFloat:
		return float64(math.Float32frombits(le.Uint32(b))), true
	case kvInt64:
		return int64(le.Uint64(b)), true
	case kvUint64:
		return le.Uint64(b), true
	default:
		return math.Float64frombits(le.Uint64(b)), true
	}
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case int64:
		return float64(n)
	case uint64:
		return float64(n)
	case float64:
		return n
	case bool:
		if n {
			return 1
		}
	}
	return 0
}
