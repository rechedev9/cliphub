package mapgeo

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestVPKReadsInlineAndPreloadedEntries(t *testing.T) {
	nav := []byte("nav-bytes")
	path := filepath.Join(t.TempDir(), "de_test.vpk")
	writeFile(t, path, buildVPK([]vpkFile{
		{path: "maps/de_test.nav", data: nav, preload: 3},
		{path: "readme", data: []byte("root file")},
		{path: "maps/de_test/world_physics.vmdl_c", data: []byte("phys")},
	}))
	pack, err := OpenVPK(path)
	if err != nil {
		t.Fatal(err)
	}
	defer pack.Close()
	for name, want := range map[string]string{"maps/de_test.nav": "nav-bytes", "readme": "root file", "maps/de_test/world_physics.vmdl_c": "phys"} {
		got, err := pack.ReadFile(name)
		if err != nil || string(got) != want {
			t.Errorf("ReadFile(%q) = %q, %v; want %q", name, got, err, want)
		}
	}
	if _, err := pack.ReadFile("maps/missing.nav"); !errors.Is(err, ErrNotInVPK) {
		t.Errorf("missing entry error = %v, want ErrNotInVPK", err)
	}
}

func TestVPKRejectsNonVPK(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.vpk")
	writeFile(t, path, []byte("definitely not a vpk header"))
	if _, err := OpenVPK(path); err == nil {
		t.Fatal("OpenVPK accepted a file without the VPK signature")
	}
}

func TestParseNavVersions(t *testing.T) {
	for _, version := range []uint32{31, 32, 35, 36} {
		nav, err := ParseNav(sampleNav(version))
		if err != nil {
			t.Fatalf("v%d: %v", version, err)
		}
		ids := make([]uint32, len(nav.Areas))
		for i, a := range nav.Areas {
			ids[i] = a.ID
		}
		if !slices.Equal(ids, []uint32{7, 9, 11, 13}) {
			t.Fatalf("v%d area ids = %v", version, ids)
		}
		if got := nav.Areas[1].Corners[2]; got != (Vec3{200, 50, 20.004}) {
			t.Errorf("v%d area 9 third corner = %v", version, got)
		}
		if movable := nav.Areas[3].Movable; movable != (version >= 35) {
			t.Errorf("v%d platform movable = %v", version, movable)
		}
	}
}

func TestParseNavFailsCleanlyOnTruncation(t *testing.T) {
	data := sampleNav(36)
	for n := 0; n < len(data)-4; n++ {
		if _, err := ParseNav(data[:n]); err == nil {
			t.Fatalf("ParseNav accepted the file cut at %d of %d bytes", n, len(data))
		}
	}
}

func TestParseNavRejectsOutOfRangeIndices(t *testing.T) {
	corners := []Vec3{{0, 0, 0}, {1, 0, 0}, {1, 1, 0}}
	badCorner := buildNav(36, corners, [][]uint32{{0, 1, 9}}, []bool{false}, []navAreaFixture{{id: 1}})
	if _, err := ParseNav(badCorner); err == nil || !strings.Contains(err.Error(), "corner 9") {
		t.Errorf("bad corner index error = %v", err)
	}
	badPolygon := buildNav(36, corners, [][]uint32{{0, 1, 2}}, []bool{false}, []navAreaFixture{{id: 1, polygon: 4}})
	if _, err := ParseNav(badPolygon); err == nil || !strings.Contains(err.Error(), "polygon 4 of 1") {
		t.Errorf("bad polygon index error = %v", err)
	}
}

func TestMeshFromNavKeepsStaticFloorOfSmallestHull(t *testing.T) {
	nav, err := ParseNav(sampleNav(36))
	if err != nil {
		t.Fatal(err)
	}
	mesh, err := MeshFromNav("de_test", nav)
	if err != nil {
		t.Fatal(err)
	}
	if len(mesh.Areas) != 2 || mesh.Areas[0].ID != 7 || mesh.Areas[1].ID != 9 {
		t.Fatalf("areas = %+v, want ids 7 and 9", mesh.Areas)
	}
	// Five shared corners: the quad's four plus the triangle's tip.
	if len(mesh.Vertices) != 5*3 {
		t.Fatalf("vertices = %v", mesh.Vertices)
	}
	if want := []uint32{0, 1, 2, 0, 2, 3, 4, 1, 0}; !slices.Equal(mesh.Triangles, want) {
		t.Fatalf("triangles = %v, want %v", mesh.Triangles, want)
	}
	for i := 0; i < len(mesh.Triangles); i += 3 {
		if z := normalZ(mesh, mesh.Triangles[i:i+3]); z <= 0 {
			t.Errorf("triangle %d faces down (normal z %v)", i/3, z)
		}
	}
	want := Bounds{Min: Vec3{0, 0, 10}, Max: Vec3{200, 100, 20}}
	if mesh.Bounds != want {
		t.Errorf("bounds = %+v, want %+v", mesh.Bounds, want)
	}
}

func normalZ(m Mesh, tri []uint32) float32 {
	p := func(i uint32) Vec3 { return Vec3{m.Vertices[3*i], m.Vertices[3*i+1], m.Vertices[3*i+2]} }
	a, b, c := p(tri[0]), p(tri[1]), p(tri[2])
	return (b[0]-a[0])*(c[1]-a[1]) - (b[1]-a[1])*(c[0]-a[0])
}

func TestSourceExtractAndErrors(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "game", "bin", "win64", "cs2.exe")
	csgo := filepath.Join(root, "game", "csgo")
	writeFile(t, exe, []byte("exe"))
	writeFile(t, filepath.Join(csgo, "steam.inf"), []byte("ClientVersion=2000942\r\nPatchVersion=1.41.9.6\r\n"))
	writeFile(t, filepath.Join(csgo, "maps", "de_test.vpk"), buildVPK([]vpkFile{{path: "maps/de_test.nav", data: sampleNav(36)}}))
	writeFile(t, filepath.Join(csgo, "maps", "de_nonav.vpk"), buildVPK([]vpkFile{{path: "maps/de_nonav.vmap_c", data: []byte("x")}}))

	mesh, err := Source{CS2Exe: exe, Map: "de_test"}.ExtractNav()
	if err != nil {
		t.Fatal(err)
	}
	if mesh.Map != "de_test" || mesh.Source != SourceNav || mesh.Build != "2000942" || len(mesh.Areas) != 2 {
		t.Errorf("mesh = %+v", mesh)
	}
	key, err := Source{CS2Exe: exe, Map: "de_test"}.CacheKey(SourceNav)
	if err != nil || !strings.HasPrefix(key, "mapgeo/"+meshFormat+"/nav/de_test-2000942-") {
		t.Errorf("CacheKey = %q, %v", key, err)
	}

	for _, tc := range []struct {
		src  Source
		want error
	}{
		{Source{Map: "de_test"}, ErrCS2NotFound},
		{Source{CS2Exe: filepath.Join(t.TempDir(), "game", "bin", "win64", "cs2.exe"), Map: "de_test"}, ErrCS2NotFound},
		{Source{CS2Exe: exe, Map: "de_missing"}, ErrMapNotFound},
		{Source{CS2Exe: exe, Map: "de_nonav"}, ErrNoNavMesh},
	} {
		if _, err := tc.src.ExtractNav(); !errors.Is(err, tc.want) {
			t.Errorf("Extract(%+v) error = %v, want %v", tc.src, err, tc.want)
		}
	}
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Clone(data), 0o600); err != nil {
		t.Fatal(err)
	}
}
