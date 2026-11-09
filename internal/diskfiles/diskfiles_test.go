package diskfiles

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanDemosSkipsDataDirAndSystemFolders(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "AppData", "ClipHub", "data")
	writeFile(t, filepath.Join(root, "Downloads", "match.DEM"), "big demo")
	writeFile(t, filepath.Join(root, "Steam", "replays", "a.dem"), "x")
	writeFile(t, filepath.Join(root, "Downloads", "notes.txt"), "no")
	writeFile(t, filepath.Join(dataDir, "demos", "copy.dem"), "studio copy")
	writeFile(t, filepath.Join(root, "node_modules", "pkg", "fixture.dem"), "tooling")
	writeFile(t, filepath.Join(root, "Windows", "x.dem"), "system")

	files := ScanDemos([]string{root, filepath.Join(root, "missing")}, []string{dataDir})
	if len(files) != 2 {
		t.Fatalf("got %d demos, want 2: %+v", len(files), files)
	}
	if filepath.Base(files[0].Path) != "match.DEM" || files[0].Size != 8 || files[0].Kind != KindDemo {
		t.Fatalf("largest demo first, got %+v", files[0])
	}
}

func TestClassifyStudioFile(t *testing.T) {
	cases := []struct {
		rel  string
		kind Kind
		job  string
		ok   bool
	}{
		{"demos/j1.dem", KindStudioDemo, "j1", true},
		{"library/l1/demo.dem", KindStudioDemo, "", true},
		{"jobs/j1/renders/short-pov/videos/reel.mp4", KindShort, "j1", true},
		{"jobs/j1/renders/short-pov/revisions/r2/videos/reel.mp4", KindShort, "j1", true},
		{"jobs/j1/renders/gameplay-pov-60/revisions/r1/videos/demo.mp4", KindFullDemo, "j1", true},
		{"jobs/j1/composition/final.mp4", KindShort, "j1", true},
		{"jobs/j1/recording/segments/seg-001.mp4", "", "", false},
		{"jobs/j1/recording/revisions/a1/segments/round-001.mp4", "", "", false},
		{"stream-jobs/s1/source.mp4", "", "", false},
		{"stream-jobs/s1/summary.mp4", KindStream, "s1", true},
		{"stream-jobs/s1/renders/v/revisions/r/videos/clip.mp4", KindStream, "s1", true},
		{"editor-jobs/e1/renders/revisions/r1/final.mp4", KindEditor, "e1", true},
		{"editor-assets/a1/media.mp4", "", "", false},
		{"cloud/results/c1/video.mp4", KindCloud, "", true},
		{"jobs/j1/renders/short-pov/covers/reel.jpg", "", "", false},
	}
	for _, tc := range cases {
		kind, job, ok := classifyStudioFile(tc.rel)
		if kind != tc.kind || job != tc.job || ok != tc.ok {
			t.Errorf("%s: got (%q, %q, %v), want (%q, %q, %v)", tc.rel, kind, job, ok, tc.kind, tc.job, tc.ok)
		}
	}
}

func TestSiblingsOfRenderVideo(t *testing.T) {
	video := filepath.Join("r", "videos", "reel.mp4")
	got := Siblings(video)
	want := []string{filepath.Join("r", "covers", "reel.jpg"), filepath.Join("r", "captions", "reel.caption.txt")}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v", got, want)
	}
	if Siblings(filepath.Join("s", "summary.mp4")) != nil {
		t.Fatal("a file outside videos/ has no siblings")
	}
}

func TestScanStudioListsAHardLinkedDemoOnce(t *testing.T) {
	dataDir := t.TempDir()
	library := filepath.Join(dataDir, "library", "l1", "demo.dem")
	writeFile(t, library, "the same demo")
	if err := os.MkdirAll(filepath.Join(dataDir, "demos"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(library, filepath.Join(dataDir, "demos", "j1.dem")); err != nil {
		t.Skipf("hard links unsupported here: %v", err)
	}
	writeFile(t, filepath.Join(dataDir, "library", "l2", "demo.dem"), "other demo")

	files, err := ScanStudio(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want the linked demo once plus the other: %+v", len(files), files)
	}
}
