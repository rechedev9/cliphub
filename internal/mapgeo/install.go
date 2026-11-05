// Package mapgeo reads CS2 map geometry from the user's own game install, so
// Studio never ships Valve assets: a VPK reader, a nav mesh parser and the
// triangle mesh the 3D replay viewer draws.
package mapgeo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// meshFormat versions the cached mesh; bump it when Mesh or its build changes.
const meshFormat = "v1"

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
	return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(s.CS2Exe))), "csgo")
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

// CacheKey names the cached mesh for this map and game build. The VPK size
// and modification time change with every map update even when steam.inf
// does not.
func (s Source) CacheKey() (string, error) {
	info, err := s.locate()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("mapgeo/%s/%s-%s-%d-%d.json",
		meshFormat, s.Map, s.build(), info.Size(), info.ModTime().Unix()), nil
}

// build is the ClientVersion from steam.inf, or "unknown".
func (s Source) build() string {
	// #nosec G304 -- fixed file inside the detected CS2 install.
	data, err := os.ReadFile(filepath.Join(s.csgoDir(), "steam.inf"))
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

// Extract reads the map's nav mesh from the install and builds its mesh.
func (s Source) Extract() (Mesh, error) {
	if _, err := s.locate(); err != nil {
		return Mesh{}, err
	}
	pack, err := OpenVPK(s.vpkPath())
	if err != nil {
		return Mesh{}, err
	}
	defer pack.Close()
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
	mesh.Build = s.build()
	return mesh, nil
}
