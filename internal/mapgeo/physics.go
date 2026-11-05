package mapgeo

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
)

// SourcePhysics marks a mesh built from the map's static collision: walls,
// floors and props that stop players and bullets.
const SourcePhysics = "physics"

// Triangle is three world-space corners.
type Triangle [3]Vec3

// nonBlockingTags are interact-as tags of collision that a player or a bullet
// passes through, or that only exists for tools: clips, ladders, glass, sky.
var nonBlockingTags = []string{"clip", "passbullets", "sky", "ladder", "window", "trigger", "water"}

// physicsTriangles decodes a compiled world physics resource (world_physics
// .vmdl_c with an embedded PHYS block, or a .vphys_c) and returns the
// triangles of every static hull and mesh whose collision blocks players and
// bullets. Field names follow ValveResourceFormat's PhysAggregateData.
func physicsTriangles(resource []byte) ([]Triangle, error) {
	blocks, err := resourceBlocks(resource)
	if err != nil {
		return nil, err
	}
	block, ok := blocks["PHYS"]
	if !ok {
		block, ok = blocks["DATA"]
	}
	if !ok {
		return nil, errors.New("resource has no physics block")
	}
	root, err := decodeKV3(block)
	if err != nil {
		return nil, err
	}
	agg, ok := root.(map[string]any)
	if !ok {
		return nil, errors.New("physics root is not an object")
	}
	keep := blockingAttributes(list(agg["m_collisionAttributes"]))
	tris := make([]Triangle, 0, countTriangles(agg, keep))
	for _, part := range list(agg["m_parts"]) {
		shape := object(object(part)["m_rnShape"])
		for _, h := range list(shape["m_hulls"]) {
			desc := object(h)
			if !keep(desc) {
				continue
			}
			if tris, err = appendHull(tris, object(desc["m_Hull"])); err != nil {
				return nil, err
			}
		}
		for _, m := range list(shape["m_meshes"]) {
			desc := object(m)
			if !keep(desc) {
				continue
			}
			if tris, err = appendMesh(tris, object(desc["m_Mesh"])); err != nil {
				return nil, err
			}
		}
	}
	if len(tris) == 0 {
		return nil, errors.New("physics has no blocking geometry")
	}
	return tris, nil
}

// countTriangles sizes the soup up front: the big maps carry millions of
// triangles, and growing the slice by doubling would hold two copies.
func countTriangles(agg map[string]any, keep func(map[string]any) bool) int {
	n := 0
	for _, part := range list(agg["m_parts"]) {
		shape := object(object(part)["m_rnShape"])
		for _, h := range list(shape["m_hulls"]) {
			if desc := object(h); keep(desc) {
				edges, _ := object(desc["m_Hull"])["m_Edges"].([]byte)
				n += len(edges) / 4 / 2 // a closed hull has about one triangle per edge pair
			}
		}
		for _, m := range list(shape["m_meshes"]) {
			if desc := object(m); keep(desc) {
				indices, _ := object(desc["m_Mesh"])["m_Triangles"].([]byte)
				n += len(indices) / 12
			}
		}
	}
	return n
}

// Margins around the nav mesh inside which collision is kept: wide enough for
// the walls that frame the playable area, tight enough to drop the skybox
// scenery and the underside of the world.
const (
	playMarginXY    = 256
	playMarginBelow = 128
	playMarginAbove = 512
	// downFacingZ drops ceilings and undersides, which a viewer looking down
	// at the map never sees and which would hide the rooms below them.
	downFacingZ = -0.7
)

// playableTriangles keeps the triangles that face sideways or up and overlap
// the box around the nav mesh; a nil box keeps every position.
func playableTriangles(tris []Triangle, nav *Bounds) []Triangle {
	out := tris[:0]
	for _, t := range tris {
		if facingZ(t) < downFacingZ {
			continue
		}
		if nav != nil && !overlapsPlayArea(t, *nav) {
			continue
		}
		out = append(out, t)
	}
	return out
}

func facingZ(t Triangle) float64 {
	ux, uy, uz := float64(t[1][0]-t[0][0]), float64(t[1][1]-t[0][1]), float64(t[1][2]-t[0][2])
	vx, vy, vz := float64(t[2][0]-t[0][0]), float64(t[2][1]-t[0][1]), float64(t[2][2]-t[0][2])
	nx, ny, nz := uy*vz-uz*vy, uz*vx-ux*vz, ux*vy-uy*vx
	length := math.Sqrt(nx*nx + ny*ny + nz*nz)
	if length == 0 {
		return 0
	}
	return nz / length
}

func overlapsPlayArea(t Triangle, nav Bounds) bool {
	lo := [3]float32{nav.Min[0] - playMarginXY, nav.Min[1] - playMarginXY, nav.Min[2] - playMarginBelow}
	hi := [3]float32{nav.Max[0] + playMarginXY, nav.Max[1] + playMarginXY, nav.Max[2] + playMarginAbove}
	for axis := range 3 {
		tmin := min(t[0][axis], t[1][axis], t[2][axis])
		tmax := max(t[0][axis], t[1][axis], t[2][axis])
		if tmax < lo[axis] || tmin > hi[axis] {
			return false
		}
	}
	return true
}

// blockingAttributes returns a filter over shape descriptors that keeps the
// collision attributes a player and a bullet both collide with.
func blockingAttributes(attrs []any) func(map[string]any) bool {
	blocking := make([]bool, len(attrs))
	for i, a := range attrs {
		attr := object(a)
		blocking[i] = !slicesContainsFold(stringList(attr["m_InteractExcludeStrings"]), "player")
		for _, tag := range stringList(attr["m_InteractAsStrings"]) {
			for _, nb := range nonBlockingTags {
				if strings.Contains(strings.ToLower(tag), nb) {
					blocking[i] = false
				}
			}
		}
	}
	return func(desc map[string]any) bool {
		i := int(toFloat(desc["m_nCollisionAttributeIndex"]))
		// Without an attribute table every shape is plain world collision.
		return len(blocking) == 0 || (i >= 0 && i < len(blocking) && blocking[i])
	}
}

// appendHull triangulates every face of a convex hull as a fan over its
// half-edge loop.
func appendHull(tris []Triangle, hull map[string]any) ([]Triangle, error) {
	positions := hull["m_VertexPositions"]
	if positions == nil {
		positions = hull["m_Vertices"] // hulls compiled before explicit vertex indices
	}
	verts, err := vec3s(positions)
	if err != nil {
		return nil, fmt.Errorf("hull vertices: %w", err)
	}
	edges, okEdges := hull["m_Edges"].([]byte)
	faces, okFaces := hull["m_Faces"].([]byte)
	if !okEdges || !okFaces || len(edges)%4 != 0 {
		return nil, errors.New("hull edges or faces are not packed")
	}
	edgeCount := len(edges) / 4
	for _, start := range faces {
		var loop []int
		for e, steps := int(start), 0; ; steps++ {
			if e >= edgeCount || steps > edgeCount {
				return nil, errors.New("hull face loop is broken")
			}
			origin := int(edges[4*e+2])
			if origin >= len(verts) {
				return nil, errors.New("hull edge origin out of range")
			}
			loop = append(loop, origin)
			if e = int(edges[4*e]); e == int(start) {
				break
			}
		}
		for i := 1; i+1 < len(loop); i++ {
			tris = append(tris, Triangle{verts[loop[0]], verts[loop[i]], verts[loop[i+1]]})
		}
	}
	return tris, nil
}

func appendMesh(tris []Triangle, mesh map[string]any) ([]Triangle, error) {
	verts, err := vec3s(mesh["m_Vertices"])
	if err != nil {
		return nil, fmt.Errorf("mesh vertices: %w", err)
	}
	indices, ok := mesh["m_Triangles"].([]byte)
	if !ok || len(indices)%12 != 0 {
		return nil, errors.New("mesh triangles are not packed")
	}
	for i := 0; i < len(indices); i += 12 {
		var t Triangle
		for c := range t {
			idx := binary.LittleEndian.Uint32(indices[i+4*c:])
			if int(idx) >= len(verts) {
				return nil, errors.New("mesh triangle index out of range")
			}
			t[c] = verts[idx]
		}
		tris = append(tris, t)
	}
	return tris, nil
}

func vec3s(v any) ([]Vec3, error) {
	blob, ok := v.([]byte)
	if !ok || len(blob)%12 != 0 {
		return nil, errors.New("not a packed float3 array")
	}
	out := make([]Vec3, len(blob)/12)
	for i := range out {
		for c := range 3 {
			f := math.Float32frombits(binary.LittleEndian.Uint32(blob[12*i+4*c:]))
			if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
				return nil, errors.New("non-finite vertex")
			}
			out[i][c] = f
		}
	}
	return out, nil
}

func object(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func list(v any) []any {
	l, _ := v.([]any)
	return l
}

func stringList(v any) []string {
	var out []string
	for _, s := range list(v) {
		if str, ok := s.(string); ok {
			out = append(out, str)
		}
	}
	return out
}

func slicesContainsFold(list []string, want string) bool {
	for _, s := range list {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}
