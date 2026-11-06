package mapgeo

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestVPKReadsEntriesFromSplitArchives(t *testing.T) {
	dir := t.TempDir()
	files := []vpkFile{
		{path: "panorama/images/overheadmaps/de_test_radar_psd.vtex_c", data: []byte("radar-in-archive-0"), split: true, archive: 0},
		{path: "panorama/images/icons/equipment/ak47.vsvg_c", data: []byte("icon-in-archive-2"), split: true, archive: 2, preload: 5},
		{path: "panorama/images/icons/equipment/awp.vsvg_c", data: []byte("second-in-archive-2"), split: true, archive: 2},
		{path: "scripts/inline.txt", data: []byte("inline")},
	}
	dirFile, archives := buildSplitVPK(files)
	writeFile(t, filepath.Join(dir, "pak01_dir.vpk"), dirFile)
	for index, body := range archives {
		writeFile(t, filepath.Join(dir, fmt.Sprintf("pak01_%03d.vpk", index)), body)
	}
	pack, err := OpenVPK(filepath.Join(dir, "pak01_dir.vpk"))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 { // the second pass reads through the archives kept open
		for _, f := range files {
			got, err := pack.ReadFile(f.path)
			if err != nil || string(got) != string(f.data) {
				t.Errorf("ReadFile(%q) = %q, %v; want %q", f.path, got, err, f.data)
			}
		}
	}
	if err := pack.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestVPKSplitArchiveErrors(t *testing.T) {
	dir := t.TempDir()
	files := []vpkFile{{path: "a/missing.txt", data: []byte("x"), split: true, archive: 7}}
	dirFile, _ := buildSplitVPK(files)

	// The archive file is absent.
	writeFile(t, filepath.Join(dir, "pak01_dir.vpk"), dirFile)
	pack, err := OpenVPK(filepath.Join(dir, "pak01_dir.vpk"))
	if err != nil {
		t.Fatal(err)
	}
	defer pack.Close()
	if _, err := pack.ReadFile("a/missing.txt"); err == nil || !strings.Contains(err.Error(), "split archive") {
		t.Errorf("ReadFile with no archive file = %v, want a split archive error", err)
	}

	// A package not named <prefix>_dir.vpk has no way to name its archives.
	writeFile(t, filepath.Join(dir, "single.vpk"), dirFile)
	single, err := OpenVPK(filepath.Join(dir, "single.vpk"))
	if err != nil {
		t.Fatal(err)
	}
	defer single.Close()
	if _, err := single.ReadFile("a/missing.txt"); err == nil || !strings.Contains(err.Error(), "_dir.vpk") {
		t.Errorf("ReadFile from a non-directory package = %v, want a _dir.vpk error", err)
	}
}
