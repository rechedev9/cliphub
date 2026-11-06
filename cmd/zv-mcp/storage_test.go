package main

import (
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestFormatBytesMatchesTheStudioUI(t *testing.T) {
	cases := map[int64]string{0: "0 B", 512: "512 B", 1536: "1,5 KB", 1 << 30: "1 GB", 1073699840: "1 GB", 33500000000: "31,2 GB"}
	for n, want := range cases {
		if got := formatBytes(n); got != want {
			t.Errorf("formatBytes(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestDescribeStorageListSortsCapsAndHidesUnsafeNames(t *testing.T) {
	day := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	files := []tuiclient.StorageFile{
		{Path: `C:\demos\old.dem`, SizeBytes: 10, ModifiedAt: day.Add(-48 * time.Hour), Kind: "demo"},
		{Path: `C:\demos\evil` + "\u202e.dem", SizeBytes: 20, ModifiedAt: day, Kind: "demo"},
		{Path: `C:\demos\new.dem`, SizeBytes: 30, ModifiedAt: day.Add(-time.Hour), Kind: "demo"},
		{Path: `C:\demos\mid.dem`, SizeBytes: 40, ModifiedAt: day.Add(-24 * time.Hour), Kind: "demo"},
	}
	out := describeStorageList(files, 2)
	if out.Total != 4 || out.TotalBytes != 100 {
		t.Errorf("total = %d files, %d bytes", out.Total, out.TotalBytes)
	}
	if len(out.Files) != 2 || out.Files[0].Path != `C:\demos\new.dem` || out.Files[1].Path != `C:\demos\mid.dem` {
		t.Errorf("files = %+v", out.Files)
	}
}
