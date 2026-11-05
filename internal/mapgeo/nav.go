package mapgeo

import (
	"errors"
	"fmt"
)

const (
	navMagic      = 0xfeedface
	navMinVersion = 30
	navMaxVersion = 36
	// navStaticWorld is the movable mesh id of an area fixed in the world.
	navStaticWorld = 0xffffffff
	// Byte sizes of records the parser steps over.
	navConnectionBytes  = 8
	navHidingSpotBytes  = 17
	navMatrix3x4Bytes   = 48
	navVec3Bytes        = 12
	maxNavPolygonCorner = 64
)

// Vec3 is a world-space point in Hammer units, Z up.
type Vec3 [3]float32

// NavArea is one walkable polygon of the navigation mesh. Movable is true for
// areas on a moving platform, whose corners are in the platform's local space.
type NavArea struct {
	ID      uint32
	Hull    uint8
	Movable bool
	Corners []Vec3
}

// Nav is the part of a CS2 .nav file a floor mesh needs.
type Nav struct {
	Version uint32
	Areas   []NavArea
}

type navPolygon struct {
	corners []Vec3
	movable bool
}

// ParseNav reads the areas of a CS2 navigation mesh (versions 30 to 36).
// Format reference: ValveResourceFormat's NavMeshFile; reading stops after the
// areas, so ladders, generation parameters and bot analysis are never parsed.
func ParseNav(data []byte) (*Nav, error) {
	r := &byteReader{data: data}
	if r.u32() != navMagic {
		return nil, errors.New("not a nav file")
	}
	nav := &Nav{Version: r.u32()}
	if nav.Version < navMinVersion || nav.Version > navMaxVersion {
		return nil, fmt.Errorf("unsupported nav version %d", nav.Version)
	}
	r.u32() // sub version
	r.u8()  // analyzed
	skipNavPlaces(r)
	if nav.Version >= 36 {
		if err := skipKV3(r); err != nil {
			return nil, fmt.Errorf("movable mesh settings: %w", err)
		}
	}
	var polygons []navPolygon
	if nav.Version >= 31 {
		var err error
		if polygons, err = readNavPolygons(r, nav.Version); err != nil {
			return nil, err
		}
	}
	if nav.Version >= 35 {
		skipNavMovableMeshes(r)
	}
	if nav.Version >= 36 {
		if err := skipKV3(r); err != nil {
			return nil, fmt.Errorf("game data: %w", err)
		}
	}
	areas, err := readNavAreas(r, nav.Version, polygons)
	if err != nil {
		return nil, err
	}
	nav.Areas = areas
	return nav, nil
}

// skipNavPlaces steps over the place directory. Areas of these versions carry
// no place index, so the names cannot be tied to polygons.
func skipNavPlaces(r *byteReader) {
	count := int(r.u16())
	for i := 0; i < count && r.err == nil; i++ {
		r.take(int(r.u16()))
	}
	r.u8() // has unnamed areas
}

func readNavPolygons(r *byteReader, version uint32) ([]navPolygon, error) {
	cornerCount := int(r.u32())
	if cornerCount > r.remaining()/navVec3Bytes {
		return nil, fmt.Errorf("nav corner count %d exceeds the file", cornerCount)
	}
	corners := make([]Vec3, cornerCount)
	for i := range corners {
		corners[i] = readVec3(r)
	}
	polygonCount := int(r.u32())
	if polygonCount > r.remaining() {
		return nil, fmt.Errorf("nav polygon count %d exceeds the file", polygonCount)
	}
	polygons := make([]navPolygon, polygonCount)
	for i := range polygons {
		n := int(r.u8())
		poly := navPolygon{corners: make([]Vec3, n)}
		for j := range poly.corners {
			index := r.u32()
			if r.err == nil && int(index) >= len(corners) {
				return nil, fmt.Errorf("nav polygon %d uses corner %d of %d", i, index, len(corners))
			}
			if r.err == nil {
				poly.corners[j] = corners[index]
			}
		}
		if version >= 35 {
			poly.movable = r.u32() != navStaticWorld
		}
		polygons[i] = poly
	}
	if version >= 32 {
		skipNavCornerGravity(r)
	}
	if r.err != nil {
		return nil, fmt.Errorf("nav polygons: %w", r.err)
	}
	return polygons, nil
}

func skipNavCornerGravity(r *byteReader) {
	if r.u32()&1 == 0 {
		return
	}
	r.take(int(r.u32()) * navVec3Bytes)
}

func skipNavMovableMeshes(r *byteReader) {
	count := int(r.u32())
	for i := 0; i < count && r.err == nil; i++ {
		r.cstring()
		r.take(navMatrix3x4Bytes)
	}
}

func readNavAreas(r *byteReader, version uint32, polygons []navPolygon) ([]NavArea, error) {
	count := int(r.u32())
	if r.err != nil {
		return nil, fmt.Errorf("nav areas: %w", r.err)
	}
	if count > r.remaining() {
		return nil, fmt.Errorf("nav area count %d exceeds the file", count)
	}
	areas := make([]NavArea, 0, count)
	for i := 0; i < count; i++ {
		area, err := readNavArea(r, version, polygons)
		if err != nil {
			return nil, fmt.Errorf("nav area %d: %w", i, err)
		}
		areas = append(areas, area)
	}
	return areas, nil
}

func readNavArea(r *byteReader, version uint32, polygons []navPolygon) (NavArea, error) {
	area := NavArea{ID: r.u32()}
	r.u64() // attribute flags
	area.Hull = r.u8()
	if version >= 31 {
		index := r.u32()
		if r.err == nil && int(index) >= len(polygons) {
			return area, fmt.Errorf("polygon %d of %d", index, len(polygons))
		}
		if r.err == nil {
			area.Corners = polygons[index].corners
			area.Movable = polygons[index].movable
		}
	} else {
		n := int(r.u32())
		if n > maxNavPolygonCorner {
			return area, fmt.Errorf("%d corners", n)
		}
		area.Corners = make([]Vec3, n)
		for i := range area.Corners {
			area.Corners[i] = readVec3(r)
		}
	}
	r.f32() // unused, always zero
	for range area.Corners {
		r.take(int(r.u32()) * navConnectionBytes)
	}
	r.take(int(r.u8()) * navHidingSpotBytes)
	if encounters := r.u32(); encounters != 0 && r.err == nil {
		return area, fmt.Errorf("%d encounter paths are not supported", encounters)
	}
	r.take(int(r.u32()) * 4) // ladders above
	r.take(int(r.u32()) * 4) // ladders below
	return area, r.err
}

func readVec3(r *byteReader) Vec3 {
	return Vec3{r.f32(), r.f32(), r.f32()}
}
