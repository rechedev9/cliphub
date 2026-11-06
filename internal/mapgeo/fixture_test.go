package mapgeo

import (
	"bytes"
	"encoding/binary"
	"math"
	"sort"
	"strings"
)

// le appends little-endian values; tests build every fixture by hand so no
// Valve file ever enters the repo.
type le struct{ bytes.Buffer }

func (b *le) u8(v uint8)   { b.WriteByte(v) }
func (b *le) u16(v uint16) { _ = binary.Write(&b.Buffer, binary.LittleEndian, v) }
func (b *le) u32(v uint32) { _ = binary.Write(&b.Buffer, binary.LittleEndian, v) }
func (b *le) u64(v uint64) { _ = binary.Write(&b.Buffer, binary.LittleEndian, v) }
func (b *le) f32(v float32) {
	b.u32(math.Float32bits(v))
}
func (b *le) vec(v Vec3) {
	for _, c := range v {
		b.f32(c)
	}
}
func (b *le) cstr(s string) {
	b.WriteString(s)
	b.WriteByte(0)
}

type vpkFile struct {
	path    string
	data    []byte
	preload int
	// split stores the data in numbered archive instead of the directory file.
	split   bool
	archive uint16
}

// buildVPK writes a version 2 single-file VPK holding files.
func buildVPK(files []vpkFile) []byte {
	dir, _ := buildSplitVPK(files)
	return dir
}

// buildSplitVPK writes a version 2 directory file and the contents of each
// numbered archive its split entries point into.
func buildSplitVPK(files []vpkFile) ([]byte, map[uint16][]byte) {
	type leaf struct {
		name string
		file vpkFile
	}
	tree := map[string]map[string][]leaf{}
	for _, f := range files {
		dir, base := " ", f.path
		if i := strings.LastIndex(f.path, "/"); i >= 0 {
			dir, base = f.path[:i], f.path[i+1:]
		}
		name, ext, found := strings.Cut(base, ".")
		if !found {
			ext = " "
		}
		if tree[ext] == nil {
			tree[ext] = map[string][]leaf{}
		}
		tree[ext][dir] = append(tree[ext][dir], leaf{name, f})
	}
	var t, data le
	archives := map[uint16][]byte{}
	for _, ext := range sortedKeys(tree) {
		t.cstr(ext)
		for _, dir := range sortedKeys(tree[ext]) {
			t.cstr(dir)
			for _, l := range tree[ext][dir] {
				t.cstr(l.name)
				t.u32(0)
				t.u16(uint16(l.file.preload))
				body := l.file.data[l.file.preload:]
				if l.file.split {
					t.u16(l.file.archive)
					t.u32(uint32(len(archives[l.file.archive])))
					archives[l.file.archive] = append(archives[l.file.archive], body...)
				} else {
					t.u16(vpkInline)
					t.u32(uint32(data.Len()))
					data.Write(body)
				}
				t.u32(uint32(len(body)))
				t.u16(vpkTerminator)
				t.Write(l.file.data[:l.file.preload])
			}
			t.cstr("")
		}
		t.cstr("")
	}
	t.cstr("")
	var out le
	out.u32(vpkSignature)
	out.u32(2)
	out.u32(uint32(t.Len()))
	out.u32(uint32(data.Len()))
	out.u32(0)
	out.u32(0)
	out.u32(0)
	out.Write(t.Bytes())
	out.Write(data.Bytes())
	return out.Bytes(), archives
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// kv3Empty is an uncompressed version 5 KV3 block holding a null root, laid
// out like the blocks CS2 writes into its nav files.
func (b *le) kv3Empty() {
	for b.Len()%8 != 0 {
		b.u8(0)
	}
	b.u32(kv3Signature | 5)
	b.Write(make([]byte, kv3HeaderGUIDLen))
	b.u32(kv3Uncompressed)
	b.u16(0)
	b.u16(0)
	for _, v := range []uint32{0, 1, 0, 1} { // bytes1, bytes4, bytes8, types
		b.u32(v)
	}
	b.u16(0)
	b.u16(0)
	for _, v := range []uint32{9, 9, 0, 0, 0, 0, 4, 0, 5, 0, 0, 0, 0, 0, 1, 0, 0, 0} {
		b.u32(v)
	}
	b.u32(0)                                   // buffer 1: string count
	b.Write([]byte{1, 0x00, 0xdd, 0xee, 0xff}) // buffer 2: null type, trailer
}

type navAreaFixture struct {
	id, polygon uint32
	hull        uint8
	connections int
	hiding      int
	ladders     int
}

// buildNav writes a nav file of the given version (31 to 36) with shared
// corners, the polygons that index them, and areas that use the polygons.
func buildNav(version uint32, corners []Vec3, polygons [][]uint32, movable []bool, areas []navAreaFixture) []byte {
	var b le
	b.u32(navMagic)
	b.u32(version)
	b.u32(0)
	b.u8(1)
	b.u16(1)
	b.u16(10)
	b.cstr("BombsiteA")
	b.u8(1)
	if version >= 36 {
		b.kv3Empty()
	}
	b.u32(uint32(len(corners)))
	for _, c := range corners {
		b.vec(c)
	}
	b.u32(uint32(len(polygons)))
	for i, p := range polygons {
		b.u8(uint8(len(p)))
		for _, idx := range p {
			b.u32(idx)
		}
		if version >= 35 {
			id := uint32(navStaticWorld)
			if movable[i] {
				id = 0
			}
			b.u32(id)
		}
	}
	if version >= 32 {
		b.u32(1)
		b.u32(1)
		b.vec(Vec3{0, 0, -1})
	}
	if version >= 35 {
		b.u32(1)
		b.cstr("1:2")
		b.Write(make([]byte, navMatrix3x4Bytes))
	}
	if version >= 36 {
		b.kv3Empty()
	}
	b.u32(uint32(len(areas)))
	for _, a := range areas {
		b.u32(a.id)
		b.u64(0)
		b.u8(a.hull)
		b.u32(a.polygon)
		b.f32(0)
		edges := 0
		if int(a.polygon) < len(polygons) {
			edges = len(polygons[a.polygon])
		}
		for range edges {
			b.u32(uint32(a.connections))
			b.Write(make([]byte, a.connections*navConnectionBytes))
		}
		b.u8(uint8(a.hiding))
		b.Write(make([]byte, a.hiding*navHidingSpotBytes))
		b.u32(0)
		b.u32(uint32(a.ladders))
		b.Write(make([]byte, a.ladders*4))
		b.u32(0)
	}
	b.u32(0) // ladders, never read
	return b.Bytes()
}

// sampleNav is two floor polygons sharing an edge, the same quad again for a
// bigger hull, and a movable platform. The quad runs clockwise from above.
func sampleNav(version uint32) []byte {
	corners := []Vec3{{0, 0, 10}, {0, 100, 10}, {100, 100, 10}, {100, 0, 10}, {200, 50, 20.004}, {5, 5, 5}}
	polygons := [][]uint32{{0, 1, 2, 3}, {3, 2, 4}, {5, 0, 1}}
	movable := []bool{false, false, true}
	areas := []navAreaFixture{
		{id: 7, polygon: 0, connections: 1, hiding: 2},
		{id: 9, polygon: 1, ladders: 1},
		{id: 11, polygon: 0, hull: 1},
		{id: 13, polygon: 2},
	}
	return buildNav(version, corners, polygons, movable, areas)
}
