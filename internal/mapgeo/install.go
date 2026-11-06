// Package mapgeo reads CS2 map geometry from the user's own game install, so
// Studio never ships Valve assets: a VPK reader, a nav mesh parser and the
// triangle mesh the 3D replay viewer draws.
package mapgeo

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// meshFormat versions the cached mesh; bump it when Mesh or its build changes.
const meshFormat = "v2"

var (
	// ErrCS2NotFound reports that no CS2 install is configured or on disk.
	ErrCS2NotFound = errors.New("cs2 install not found")
	// ErrMapNotFound reports that the install has no package for the map.
	ErrMapNotFound = errors.New("map not found in the cs2 install")
	// ErrNoNavMesh reports that the map package ships no navigation mesh.
	ErrNoNavMesh = errors.New("map has no navigation mesh")
)

// Source locates one map inside a CS2 install. CS2Exe is the path of
// game\bin\win64\cs2.exe, as capturetools resolves it.
type Source struct {
	CS2Exe string
	Map    string
}

func (s Source) csgoDir() string {
	return CSGODir(s.CS2Exe)
}

// CSGODir is game\csgo of the install that holds cs2Exe (game\bin\win64\cs2.exe).
func CSGODir(cs2Exe string) string {
	return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(cs2Exe))), "csgo")
}

func (s Source) vpkPath() string {
	return filepath.Join(s.csgoDir(), "maps", s.Map+".vpk")
}

// locate returns the map package's file info, or ErrCS2NotFound /
// ErrMapNotFound.
func (s Source) locate() (os.FileInfo, error) {
	if s.CS2Exe == "" {
		return nil, ErrCS2NotFound
	}
	if info, err := os.Stat(s.csgoDir()); err != nil || !info.IsDir() {
		return nil, ErrCS2NotFound
	}
	info, err := os.Stat(s.vpkPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrMapNotFound
	}
	if err != nil {
		return nil, err
	}
	return info, nil
}

// CacheKey names the cached mesh of one source kind for this map and game
// build. The VPK size and modification time change with every map update even
// when steam.inf does not.
func (s Source) CacheKey(kind string) (string, error) {
	info, err := s.locate()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("mapgeo/%s/%s/%s-%s-%d-%d.json",
		meshFormat, kind, s.Map, s.build(), info.Size(), info.ModTime().Unix()), nil
}

func (s Source) build() string {
	return ClientVersion(s.csgoDir())
}

// ClientVersion is the ClientVersion from csgoDir's steam.inf, or "unknown".
func ClientVersion(csgoDir string) string {
	// #nosec G304 -- fixed file inside the detected CS2 install.
	data, err := os.ReadFile(filepath.Join(csgoDir, "steam.inf"))
	if err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && key == "ClientVersion" && isDigits(value) {
			return value
		}
	}
	return "unknown"
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (s Source) open() (*VPK, error) {
	if _, err := s.locate(); err != nil {
		return nil, err
	}
	return OpenVPK(s.vpkPath())
}

// ExtractNav reads the map's nav mesh from the install and builds its mesh.
func (s Source) ExtractNav() (Mesh, error) {
	pack, err := s.open()
	if err != nil {
		return Mesh{}, err
	}
	defer pack.Close()
	mesh, err := s.navMesh(pack)
	if err != nil {
		return Mesh{}, err
	}
	mesh.Build = s.build()
	return mesh, nil
}

func (s Source) navMesh(pack *VPK) (Mesh, error) {
	data, err := pack.ReadFile("maps/" + s.Map + ".nav")
	if errors.Is(err, ErrNotInVPK) {
		return Mesh{}, ErrNoNavMesh
	}
	if err != nil {
		return Mesh{}, err
	}
	nav, err := ParseNav(data)
	if err != nil {
		return Mesh{}, fmt.Errorf("parse %s nav mesh: %w", s.Map, err)
	}
	mesh, err := MeshFromNav(s.Map, nav)
	if err != nil {
		return Mesh{}, fmt.Errorf("%s: %w", s.Map, err)
	}
	return mesh, nil
}

// Extract returns the mesh of kind (SourcePhysics or SourceNav). A physics
// read that fails for any reason but a missing install or map falls back to
// the nav mesh, and Mesh.Fallback says why.
func (s Source) Extract(kind string) (Mesh, error) {
	if kind == SourceNav {
		return s.ExtractNav()
	}
	mesh, err := s.ExtractPhysics()
	if err == nil || errors.Is(err, ErrCS2NotFound) || errors.Is(err, ErrMapNotFound) {
		return mesh, err
	}
	nav, navErr := s.ExtractNav()
	if navErr != nil {
		return Mesh{}, errors.Join(err, navErr)
	}
	nav.Fallback = err.Error()
	return nav, nil
}

// physicsResources are where a map package keeps its world collision.
var physicsResources = []string{"world_physics.vmdl_c", "world_physics.vphys_c"}

// MaxPhysicsJSONBytes is the size the decimated physics mesh must fit in.
const MaxPhysicsJSONBytes = 1_500_000

// physicsCellSizes are the clustering grids tried, finest first, until the
// mesh fits MaxPhysicsJSONBytes.
var physicsCellSizes = []float64{4, 8, 12, 16, 24, 32, 48, 64, 96}

// ExtractPhysics reads the map's static world collision and decimates it to
// fit MaxPhysicsJSONBytes. Collision outside the nav mesh's reach is dropped.
func (s Source) ExtractPhysics() (Mesh, error) {
	pack, err := s.open()
	if err != nil {
		return Mesh{}, err
	}
	defer pack.Close()
	var data []byte
	for _, name := range physicsResources {
		if data, err = pack.ReadFile("maps/" + s.Map + "/" + name); !errors.Is(err, ErrNotInVPK) {
			break
		}
	}
	if err != nil {
		return Mesh{}, fmt.Errorf("%s world physics: %w", s.Map, err)
	}
	tris, err := physicsTriangles(data)
	if err != nil {
		return Mesh{}, fmt.Errorf("%s world physics: %w", s.Map, err)
	}
	var area *Bounds
	if nav, err := s.navMesh(pack); err == nil {
		area = &nav.Bounds
	}
	tris = playableTriangles(tris, area)
	if len(tris) == 0 {
		return Mesh{}, fmt.Errorf("%s world physics has no geometry near the play area", s.Map)
	}
	mesh, err := fitPhysicsMesh(s.Map, tris, MaxPhysicsJSONBytes)
	if err != nil {
		return Mesh{}, err
	}
	mesh.Build = s.build()
	return mesh, nil
}

// fitPhysicsMesh returns the finest clustering whose JSON fits budget, or the
// coarsest one. Size shrinks as cells grow, so a binary search over
// physicsCellSizes needs about four clusterings instead of nine.
func fitPhysicsMesh(mapName string, tris []Triangle, budget int) (Mesh, error) {
	lo, hi := 0, len(physicsCellSizes)-1
	var best Mesh
	found := false
	for lo <= hi {
		mid := (lo + hi) / 2
		mesh := clusterMesh(mapName, tris, physicsCellSizes[mid])
		body, err := json.Marshal(mesh)
		if err != nil {
			return Mesh{}, err
		}
		if len(body) <= budget {
			best, found, hi = mesh, true, mid-1
		} else {
			lo = mid + 1
		}
	}
	mesh := best
	if !found {
		mesh = clusterMesh(mapName, tris, physicsCellSizes[len(physicsCellSizes)-1])
	}
	if len(mesh.Triangles) == 0 {
		return Mesh{}, fmt.Errorf("%s world physics collapsed to nothing", mapName)
	}
	return mesh, nil
}
