package library

import (
	"testing"
	"time"
)

func TestMatchDatePrefersTheImporterThenTheDemo(t *testing.T) {
	imported := time.Date(2026, time.November, 13, 21, 31, 0, 0, time.UTC)
	told := time.Date(2026, time.October, 1, 18, 0, 0, 0, time.UTC)
	shown := time.Date(2026, time.October, 17, 16, 34, 18, 0, time.UTC)
	late := imported.Add(time.Minute)
	cases := []struct {
		name  string
		told  *time.Time
		shown *time.Time
		want  time.Time
	}{
		{"the importer's date wins", &told, &shown, told},
		{"the demo's date comes next", nil, &shown, shown},
		{"a demo date after the import is not the match's", nil, &late, imported},
		{"a zero demo date is no date", nil, &time.Time{}, imported},
		{"nothing known keeps the import time", nil, nil, imported},
	}
	for _, tc := range cases {
		d := Demo{PlayedAt: tc.told, ImportedAt: imported}
		if got := MatchDate(d, tc.shown); !got.Equal(tc.want) {
			t.Errorf("%s: MatchDate = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestDatedByImportOnlyForTheFallbackDate(t *testing.T) {
	imported := time.Date(2026, time.November, 13, 21, 31, 0, 0, time.UTC)
	told := imported
	if !DatedByImport(Match{PlayedAt: imported}, Demo{ImportedAt: imported}) {
		t.Fatal("a match on its import time is dated by its import")
	}
	if DatedByImport(Match{PlayedAt: imported.Add(-time.Hour)}, Demo{ImportedAt: imported}) {
		t.Fatal("a match with another date is not dated by its import")
	}
	if DatedByImport(Match{PlayedAt: imported}, Demo{ImportedAt: imported, PlayedAt: &told}) {
		t.Fatal("a date the importer gave is never the fallback")
	}
}

func TestFaceitMatchIDReadsTheRoomIDFromTheFileName(t *testing.T) {
	cases := map[string]string{
		"1-d424a72f-c3f4-4e81-b4e5-1d0d8ea1d7a8-1-1.dem":     "1-d424a72f-c3f4-4e81-b4e5-1d0d8ea1d7a8",
		"1-EB3246B4-CE6D-493A-B506-A790643A765C-3-2.dem.zst": "1-eb3246b4-ce6d-493a-b506-a790643a765c",
		"match730_003712345678901234567_1234567890_123.dem":  "",
		"vitality-vs-mouz-m1-mirage.dem":                     "",
	}
	for name, want := range cases {
		got, ok := FaceitMatchID(name)
		if got != want || ok != (want != "") {
			t.Errorf("FaceitMatchID(%q) = %q, %v; want %q", name, got, ok, want)
		}
	}
}
