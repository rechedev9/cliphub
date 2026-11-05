package mapgeo

import (
	"errors"
	"math"
	"slices"
)

// SourceNav marks a mesh built from the map's navigation mesh: walkable floor
// only, no walls or ceilings.
const SourceNav = "nav"

// Bounds is the axis-aligned box around every vertex, in world units.
type Bounds struct {
	Min Vec3 `json:"min"`
	Max Vec3 `json:"max"`
}

// MeshArea is one nav area: Polygon lists its corners as indices into
// Mesh.Vertices, counter-clockwise seen from above.
type MeshArea struct {
	ID      uint32   `json:"id"`
	Polygon []uint32 `json:"polygon"`
}

// Mesh is the map geometry the replay viewer draws. Coordinates are CS2 world
// units, Z up, the same space as demo positions. Vertices is a flat x,y,z list
// and Triangles a flat list of vertex indices, ready for a Three.js
// BufferGeometry position attribute and index.
type Mesh struct {
	Map       string     `json:"map"`
	Source    string     `json:"source"`
	Build     string     `json:"build"`
	Bounds    Bounds     `json:"bounds"`
	Vertices  []float32  `json:"vertices"`
	Triangles []uint32   `json:"triangles"`
	Areas     []MeshArea `json:"areas"`
	// Fallback is why a physics request was answered with the nav mesh.
	Fallback string `json:"fallback,omitempty"`
}

// MeshFromNav turns the static areas of the smallest hull into an indexed
// triangle mesh. Larger hulls repeat the same floor for bigger agents and
// movable areas are in platform-local space, so both are left out.
func MeshFromNav(mapName string, nav *Nav) (Mesh, error) {
	hull, ok := smallestHull(nav.Areas)
	if !ok {
		return Mesh{}, errors.New("nav mesh has no static areas")
	}
	b := newMeshBuilder(navStep)
	mesh := Mesh{Map: mapName, Source: SourceNav, Areas: []MeshArea{}}
	for _, area := range nav.Areas {
		if area.Movable || area.Hull != hull || len(area.Corners) < 3 {
			continue
		}
		corners := slices.Clone(area.Corners)
		if signedAreaXY(corners) < 0 {
			slices.Reverse(corners)
		}
		polygon := make([]uint32, len(corners))
		for i, c := range corners {
			polygon[i] = b.vertex(c)
		}
		for i := 1; i+1 < len(polygon); i++ {
			b.triangles = append(b.triangles, polygon[0], polygon[i], polygon[i+1])
		}
		mesh.Areas = append(mesh.Areas, MeshArea{ID: area.ID, Polygon: polygon})
	}
	mesh.Vertices = b.vertices
	mesh.Triangles = b.triangles
	mesh.Bounds = b.bounds
	return mesh, nil
}

func smallestHull(areas []NavArea) (uint8, bool) {
	hull, found := uint8(math.MaxUint8), false
	for _, a := range areas {
		if !a.Movable && len(a.Corners) >= 3 && a.Hull <= hull {
			hull, found = a.Hull, true
		}
	}
	return hull, found
}

// signedAreaXY is twice the polygon's area projected on the floor plane,
// positive when the corners run counter-clockwise seen from +Z.
func signedAreaXY(corners []Vec3) float64 {
	var sum float64
	for i, a := range corners {
		b := corners[(i+1)%len(corners)]
		sum += float64(a[0])*float64(b[1]) - float64(b[0])*float64(a[1])
	}
	return sum
}

// navStep rounds nav corners to centi-units, far below what a player can
// stand on, so the JSON stays short and float noise cannot split a corner.
const navStep = 0.01

type meshBuilder struct {
	step      float64
	index     map[Vec3]uint32
	vertices  []float32
	triangles []uint32
	bounds    Bounds
	seen      map[[3]uint32]bool
}

func newMeshBuilder(step float64) *meshBuilder {
	return &meshBuilder{step: step, index: map[Vec3]uint32{}, seen: map[[3]uint32]bool{}}
}

// vertex snaps v to the builder's grid and welds it with any vertex already
// on the same grid point.
func (b *meshBuilder) vertex(v Vec3) uint32 {
	for axis := range v {
		v[axis] = float32(math.Round(float64(v[axis])/b.step) * b.step)
	}
	if i, ok := b.index[v]; ok {
		return i
	}
	i := uint32(len(b.vertices) / 3)
	b.index[v] = i
	if i == 0 {
		b.bounds = Bounds{Min: v, Max: v}
	}
	for axis := range v {
		b.bounds.Min[axis] = min(b.bounds.Min[axis], v[axis])
		b.bounds.Max[axis] = max(b.bounds.Max[axis], v[axis])
	}
	b.vertices = append(b.vertices, v[0], v[1], v[2])
	return i
}
