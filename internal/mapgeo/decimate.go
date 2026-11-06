package mapgeo

import (
	"math"
	"slices"
)

// quadric is the symmetric 4x4 plane error matrix of Garland and Heckbert,
// stored as its upper triangle: a2 ab ac ad b2 bc bd c2 cd d2.
type quadric [10]float64

func planeQuadric(t Triangle) (quadric, bool) {
	ux, uy, uz := float64(t[1][0]-t[0][0]), float64(t[1][1]-t[0][1]), float64(t[1][2]-t[0][2])
	vx, vy, vz := float64(t[2][0]-t[0][0]), float64(t[2][1]-t[0][1]), float64(t[2][2]-t[0][2])
	nx, ny, nz := uy*vz-uz*vy, uz*vx-ux*vz, ux*vy-uy*vx
	length := math.Sqrt(nx*nx + ny*ny + nz*nz)
	if length == 0 {
		return quadric{}, false
	}
	// Weighting by area (length/2) lets large walls dominate small trim.
	w := length / 2
	a, b, c := nx/length, ny/length, nz/length
	d := -(a*float64(t[0][0]) + b*float64(t[0][1]) + c*float64(t[0][2]))
	return quadric{w * a * a, w * a * b, w * a * c, w * a * d, w * b * b, w * b * c, w * b * d, w * c * c, w * c * d, w * d * d}, true
}

type cellKey [3]int32

type cell struct {
	key   cellKey
	q     quadric
	sum   [3]float64
	count float64
	// vertex is the cell's index in the output plus one; zero means unused.
	vertex uint32
}

// clusterMesh simplifies a triangle soup by vertex clustering on a grid of
// size step (Lindstrom, "Out-of-core simplification of large polygonal
// models"): each cell gets the point that best fits the planes of the
// triangles touching it, so walls stay flat and corners stay sharp, and a
// triangle survives only if its corners land in three different cells.
func clusterMesh(mapName string, tris []Triangle, step float64) Mesh {
	index := map[cellKey]int32{}
	var cells []cell
	corners := make([][3]int32, len(tris))
	for i, t := range tris {
		q, ok := planeQuadric(t)
		for c, v := range t {
			k := cellKey{int32(math.Floor(float64(v[0]) / step)), int32(math.Floor(float64(v[1]) / step)), int32(math.Floor(float64(v[2]) / step))}
			id, seen := index[k]
			if !seen {
				id = int32(len(cells))
				index[k] = id
				cells = append(cells, cell{key: k})
			}
			corners[i][c] = id
			cl := &cells[id]
			for axis := range 3 {
				cl.sum[axis] += float64(v[axis])
			}
			cl.count++
			// A plane counts once per cell, however many of its corners land there.
			if ok && (c == 0 || id != corners[i][0]) && (c < 2 || id != corners[i][1]) {
				for j := range q {
					cl.q[j] += q[j]
				}
			}
		}
	}
	b := newMeshBuilder(1)
	for _, k := range corners {
		if k[0] == k[1] || k[1] == k[2] || k[0] == k[2] {
			continue
		}
		var tri [3]uint32
		for c, id := range k {
			cl := &cells[id]
			if cl.vertex == 0 {
				cl.vertex = b.vertex(cl.representative(cl.key, step)) + 1
			}
			tri[c] = cl.vertex - 1
		}
		b.addIndexed(tri)
	}
	return Mesh{Map: mapName, Source: SourcePhysics, Bounds: b.bounds, Vertices: b.vertices, Triangles: b.triangles, Areas: []MeshArea{}}
}

// representative minimises the cell's quadric around the mean of its vertices
// with a truncated pseudo-inverse, so where the planes pin only a line (a
// wall meeting a floor) or a plane, the point lands on it nearest the mean.
// It is clamped into the cell so geometry cannot drift across the map.
func (c *cell) representative(k cellKey, step float64) Vec3 {
	mean := [3]float64{c.sum[0] / c.count, c.sum[1] / c.count, c.sum[2] / c.count}
	q := c.q
	a := [3][3]float64{{q[0], q[1], q[2]}, {q[1], q[4], q[5]}, {q[2], q[5], q[7]}}
	// Residual of the normal equations a x = -b at the mean.
	var r [3]float64
	for i := range 3 {
		r[i] = -[3]float64{q[3], q[6], q[8]}[i] - (a[i][0]*mean[0] + a[i][1]*mean[1] + a[i][2]*mean[2])
	}
	values, vectors := symmetricEigen(a)
	largest := math.Max(math.Abs(values[0]), math.Max(math.Abs(values[1]), math.Abs(values[2])))
	p := mean
	for e := range 3 {
		if largest == 0 || math.Abs(values[e]) < 1e-3*largest {
			continue
		}
		v := vectors[e]
		scale := (v[0]*r[0] + v[1]*r[1] + v[2]*r[2]) / values[e]
		for axis := range 3 {
			p[axis] += scale * v[axis]
		}
	}
	var out Vec3
	for axis := range 3 {
		lo := float64(k[axis]) * step
		out[axis] = float32(math.Min(math.Max(p[axis], lo), lo+step))
	}
	return out
}

// symmetricEigen diagonalises a symmetric 3x3 matrix with Jacobi rotations and
// returns its eigenvalues and unit eigenvectors.
func symmetricEigen(a [3][3]float64) ([3]float64, [3][3]float64) {
	v := [3][3]float64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}} // columns are eigenvectors
	for sweep := 0; sweep < 32; sweep++ {
		off := math.Abs(a[0][1]) + math.Abs(a[0][2]) + math.Abs(a[1][2])
		if off < 1e-12*(math.Abs(a[0][0])+math.Abs(a[1][1])+math.Abs(a[2][2])+1e-300) {
			break
		}
		for p := 0; p < 2; p++ {
			for q := p + 1; q < 3; q++ {
				if a[p][q] == 0 {
					continue
				}
				theta := (a[q][q] - a[p][p]) / (2 * a[p][q])
				t := math.Copysign(1, theta) / (math.Abs(theta) + math.Sqrt(theta*theta+1))
				cos := 1 / math.Sqrt(t*t+1)
				sin := t * cos
				for k := range 3 {
					akp, akq := a[k][p], a[k][q]
					a[k][p], a[k][q] = cos*akp-sin*akq, sin*akp+cos*akq
				}
				for k := range 3 {
					apk, aqk := a[p][k], a[q][k]
					a[p][k], a[q][k] = cos*apk-sin*aqk, sin*apk+cos*aqk
				}
				for k := range 3 {
					vkp, vkq := v[k][p], v[k][q]
					v[k][p], v[k][q] = cos*vkp-sin*vkq, sin*vkp+cos*vkq
				}
			}
		}
	}
	var vectors [3][3]float64
	for e := range 3 {
		vectors[e] = [3]float64{v[0][e], v[1][e], v[2][e]}
	}
	return [3]float64{a[0][0], a[1][1], a[2][2]}, vectors
}
func (b *meshBuilder) addIndexed(t [3]uint32) {
	if t[0] == t[1] || t[1] == t[2] || t[0] == t[2] {
		return
	}
	key := t
	slices.Sort(key[:])
	if b.seen[key] {
		return
	}
	b.seen[key] = true
	b.triangles = append(b.triangles, t[0], t[1], t[2])
}
