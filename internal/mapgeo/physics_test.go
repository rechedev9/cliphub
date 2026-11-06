package mapgeo

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDecodeKV3RoundTripsEveryMethod(t *testing.T) {
	blobA := bytes.Repeat([]byte("vertex-data-"), 50)
	root := map[string]any{
		"name":   "world",
		"count":  7,
		"ratio":  0.25,
		"on":     true,
		"off":    false,
		"none":   nil,
		"floats": []float32{1.5, -2, 3.25},
		"aux":    auxBytes{4, 5, 6},
		"blobs":  []any{blobA, []byte("x")},
		"nested": []any{map[string]any{"tag": "playerclip"}, "plain", 3},
	}
	want := map[string]any{
		"name":   "world",
		"count":  int64(7),
		"ratio":  0.25,
		"on":     true,
		"off":    false,
		"none":   nil,
		"floats": []float64{1.5, -2, 3.25},
		"aux":    []float64{4, 5, 6},
		"blobs":  []any{blobA, []byte("x")},
		"nested": []any{map[string]any{"tag": "playerclip"}, "plain", int64(3)},
	}
	for _, method := range []uint32{kv3Uncompressed, kv3LZ4, kv3Zstd} {
		w := &kv3Writer{lz4Split: 128}
		w.value(root)
		got, err := decodeKV3(w.finish(method))
		if err != nil {
			t.Fatalf("method %d: %v", method, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("method %d decoded\n%#v\nwant\n%#v", method, got, want)
		}
	}
}

func TestDecodeKV3RejectsDamage(t *testing.T) {
	block := encodeKV3(map[string]any{"a": []any{1, 2, 3}, "b": []byte("blob")}, kv3Uncompressed)
	for n := 0; n < len(block); n++ {
		if _, err := decodeKV3(block[:n]); err == nil {
			t.Fatalf("decodeKV3 accepted a block cut at %d of %d bytes", n, len(block))
		}
	}
	if _, err := decodeKV3(append([]byte("KV3\x09"), block[4:]...)); err == nil {
		t.Error("decodeKV3 accepted an unknown version")
	}
}

func TestLZ4ChainsMatchesAcrossFrames(t *testing.T) {
	out := make([]byte, 17)
	n, err := lz4DecodeBlock(lz4Literals([]byte("abcdabcd")), out, 0)
	if err != nil || n != 8 {
		t.Fatalf("first frame: %d, %v", n, err)
	}
	// Second frame: no literals, an 8-byte match 8 back (into frame one), then "x".
	n, err = lz4DecodeBlock([]byte{0x04, 8, 0, 0x10, 'x'}, out, 8)
	if err != nil || n != 9 || string(out) != "abcdabcdabcdabcdx" {
		t.Fatalf("second frame: %d, %v, %q", n, err, out)
	}
	// An overlapping match repeats its own output.
	run := make([]byte, 10)
	if n, err := lz4DecodeBlock([]byte{0x15, 'z', 1, 0, 0x00}, run, 0); err != nil || string(run[:n]) != "zzzzzzzzzz" {
		t.Errorf("overlap: %q, %v", run[:n], err)
	}
	if _, err := lz4DecodeBlock([]byte{0x04, 9, 0}, make([]byte, 20), 8); err == nil {
		t.Error("lz4 accepted a match before the start of the output")
	}
}

func TestResourceBlocks(t *testing.T) {
	data := buildResource(map[string][]byte{"PHYS": []byte("phys-bytes"), "DATA": []byte("d")})
	blocks, err := ResourceBlocks(data)
	if err != nil {
		t.Fatal(err)
	}
	if string(blocks["PHYS"]) != "phys-bytes" || string(blocks["DATA"]) != "d" {
		t.Errorf("blocks = %q", blocks)
	}
	binary.LittleEndian.PutUint32(data[16+4:], 1<<20) // PHYS offset far past the end
	if _, err := ResourceBlocks(data); err == nil {
		t.Error("ResourceBlocks accepted a block outside the file")
	}
}

// tetraHull is a tetrahedron as a half-edge hull: four triangular faces.
func tetraHull() map[string]any {
	verts := []Vec3{{0, 0, 0}, {10, 0, 0}, {0, 10, 0}, {0, 0, 10}}
	faces := [][3]byte{{0, 2, 1}, {0, 1, 3}, {1, 2, 3}, {2, 0, 3}}
	var pos, edges le
	for _, v := range verts {
		pos.vec(v)
	}
	var faceStarts []byte
	for f, face := range faces {
		base := byte(3 * f)
		faceStarts = append(faceStarts, base)
		for i := range 3 {
			edges.Write([]byte{base + byte((i+1)%3), 0, face[i], byte(f)})
		}
	}
	return map[string]any{"m_VertexPositions": pos.Bytes(), "m_Edges": edges.Bytes(), "m_Faces": faceStarts}
}

func meshShape(attr int, tris ...Triangle) map[string]any {
	var verts, idx le
	for i, tri := range tris {
		for c, v := range tri {
			verts.vec(v)
			idx.u32(uint32(3*i + c))
		}
	}
	return map[string]any{"m_nCollisionAttributeIndex": attr, "m_Mesh": map[string]any{"m_Vertices": verts.Bytes(), "m_Triangles": idx.Bytes()}}
}

func attribute(group string, interactAs, exclude []any) map[string]any {
	return map[string]any{"m_CollisionGroupString": group, "m_InteractAsStrings": interactAs, "m_InteractExcludeStrings": exclude}
}

// samplePhysics is a world physics resource with a solid tetrahedron, a solid
// wall, a player clip and a bullet-only shape; only the first two block both.
func samplePhysics(method uint32) []byte {
	wall := []Triangle{{{0, 0, 0}, {100, 0, 0}, {100, 0, 100}}, {{0, 0, 0}, {100, 0, 100}, {0, 0, 100}}}
	root := map[string]any{
		"m_collisionAttributes": []any{
			attribute("Default", []any{}, []any{}),
			attribute("ConditionallySolid", []any{"npcclip", "playerclip"}, []any{}),
			attribute("Default", []any{}, []any{"player"}),
		},
		"m_parts": []any{map[string]any{"m_rnShape": map[string]any{
			"m_hulls": []any{map[string]any{"m_nCollisionAttributeIndex": 0, "m_Hull": tetraHull()}},
			"m_meshes": []any{
				meshShape(0, wall...),
				meshShape(1, Triangle{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}),
				meshShape(2, Triangle{{5, 5, 5}, {6, 5, 5}, {5, 6, 5}}),
			},
		}}},
	}
	return buildResource(map[string][]byte{"PHYS": encodeKV3(root, method), "CTRL": encodeKV3(map[string]any{}, kv3Uncompressed)})
}

func TestPhysicsTrianglesKeepsWhatBlocksPlayersAndBullets(t *testing.T) {
	for _, method := range []uint32{kv3Uncompressed, kv3Zstd} {
		tris, err := physicsTriangles(samplePhysics(method))
		if err != nil {
			t.Fatal(err)
		}
		if len(tris) != 4+2 {
			t.Fatalf("method %d: %d triangles, want 4 hull + 2 wall", method, len(tris))
		}
		if tris[0] != (Triangle{{0, 0, 0}, {0, 10, 0}, {10, 0, 0}}) {
			t.Errorf("first hull face = %v", tris[0])
		}
		for _, tri := range tris {
			for _, c := range tri {
				if c == (Vec3{1, 0, 0}) || c == (Vec3{5, 5, 5}) {
					t.Errorf("clip or bullet-only triangle kept: %v", tri)
				}
			}
		}
	}
}

func TestPlayableTrianglesDropsUndersidesAndFarGeometry(t *testing.T) {
	up := Triangle{{0, 0, 0}, {10, 0, 0}, {0, 10, 0}}
	down := Triangle{{0, 0, 0}, {0, 10, 0}, {10, 0, 0}}
	wall := Triangle{{0, 0, 0}, {10, 0, 0}, {10, 0, 10}}
	far := Triangle{{5000, 0, 0}, {5010, 0, 0}, {5000, 10, 0}}
	// A huge ground triangle reaching into the play area is kept.
	ground := Triangle{{-9000, -9000, 0}, {9000, -9000, 0}, {0, 9000, 0}}
	area := Bounds{Min: Vec3{-100, -100, -10}, Max: Vec3{100, 100, 50}}
	got := playableTriangles([]Triangle{up, down, wall, far, ground}, &area)
	if !reflect.DeepEqual(got, []Triangle{up, wall, ground}) {
		t.Errorf("kept %v", got)
	}
}

func TestClusterMeshKeepsWallsFlatAndCornersSharp(t *testing.T) {
	// A 256x256 floor and a wall along y=0, each as a fine grid of triangles.
	var tris []Triangle
	for i := float32(0); i < 256; i += 8 {
		for j := float32(0); j < 256; j += 8 {
			tris = append(tris,
				Triangle{{i, j, 0}, {i + 8, j, 0}, {i + 8, j + 8, 0}}, Triangle{{i, j, 0}, {i + 8, j + 8, 0}, {i, j + 8, 0}},
				Triangle{{i, 0, j}, {i + 8, 0, j + 8}, {i + 8, 0, j}}, Triangle{{i, 0, j}, {i, 0, j + 8}, {i + 8, 0, j + 8}})
		}
	}
	mesh := clusterMesh("de_test", tris, 32)
	if got, raw := len(mesh.Triangles)/3, len(tris); got >= raw/4 || got == 0 {
		t.Fatalf("%d triangles from %d, want a real reduction", got, raw)
	}
	for i := 0; i < len(mesh.Vertices); i += 3 {
		x, y, z := mesh.Vertices[i], mesh.Vertices[i+1], mesh.Vertices[i+2]
		if z != 0 && y != 0 {
			t.Fatalf("vertex %v,%v,%v left both the floor and the wall plane", x, y, z)
		}
	}
	// Open edges are pinned by no plane, so they may slide, but never past their cell.
	for axis := range 3 {
		if mesh.Bounds.Min[axis] < 0 || mesh.Bounds.Min[axis] > 32 || mesh.Bounds.Max[axis] < 224 || mesh.Bounds.Max[axis] > 256 {
			t.Errorf("bounds = %+v, a corner left its cell", mesh.Bounds)
		}
	}
}

func TestFitPhysicsMeshMeetsTheBudget(t *testing.T) {
	var tris []Triangle
	for i := float32(0); i < 512; i += 4 {
		for j := float32(0); j < 512; j += 4 {
			h := float32(math.Sin(float64(i)/40) * 30)
			tris = append(tris, Triangle{{i, j, h}, {i + 4, j, h}, {i, j + 4, h + 1}})
		}
	}
	fine, err := fitPhysicsMesh("de_test", tris, 1<<30)
	if err != nil {
		t.Fatal(err)
	}
	small, err := fitPhysicsMesh("de_test", tris, 20_000)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(small)
	if len(body) > 20_000 || len(small.Triangles) >= len(fine.Triangles) {
		t.Errorf("budget mesh is %d bytes with %d triangles (fine %d)", len(body), len(small.Triangles)/3, len(fine.Triangles)/3)
	}
}

func TestSourceExtractPhysicsAndFallback(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "game", "bin", "win64", "cs2.exe")
	maps := filepath.Join(root, "game", "csgo", "maps")
	writeFile(t, exe, []byte("exe"))
	writeFile(t, filepath.Join(maps, "de_good.vpk"), buildVPK([]vpkFile{
		{path: "maps/de_good.nav", data: sampleNav(36)},
		{path: "maps/de_good/world_physics.vmdl_c", data: samplePhysics(kv3Zstd)},
	}))
	writeFile(t, filepath.Join(maps, "de_broken.vpk"), buildVPK([]vpkFile{
		{path: "maps/de_broken.nav", data: sampleNav(36)},
		{path: "maps/de_broken/world_physics.vmdl_c", data: []byte("garbage")},
	}))

	mesh, err := Source{CS2Exe: exe, Map: "de_good"}.Extract(SourcePhysics)
	if err != nil {
		t.Fatal(err)
	}
	if mesh.Source != SourcePhysics || mesh.Fallback != "" || len(mesh.Triangles) == 0 || len(mesh.Areas) != 0 {
		t.Errorf("physics mesh = %+v", mesh)
	}
	mesh, err = Source{CS2Exe: exe, Map: "de_broken"}.Extract(SourcePhysics)
	if err != nil {
		t.Fatal(err)
	}
	if mesh.Source != SourceNav || !strings.Contains(mesh.Fallback, "world physics") {
		t.Errorf("fallback mesh source %q fallback %q", mesh.Source, mesh.Fallback)
	}
	if _, err := (Source{CS2Exe: exe, Map: "de_missing"}).Extract(SourcePhysics); err != ErrMapNotFound {
		t.Errorf("missing map error = %v", err)
	}
}
