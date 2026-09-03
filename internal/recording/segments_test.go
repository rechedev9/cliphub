package recording

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestSegmentIDsReturnsUniqueNonEmptyPlanIDs(t *testing.T) {
	got := SegmentIDs(RecordingResult{
		Plan: RecordingPlan{
			Segments: []RecordingSegment{
				{ID: "seg-001"},
				{ID: ""},
				{ID: "seg-002"},
				{ID: "seg-001"},
			},
		},
	})
	want := []string{"seg-001", "seg-002"}
	if !slices.Equal(got, want) {
		t.Fatalf("SegmentIDs = %#v, want %#v", got, want)
	}
}

func TestSegmentIDsAllowsEmptyResult(t *testing.T) {
	got := SegmentIDs(RecordingResult{})
	if len(got) != 0 {
		t.Fatalf("SegmentIDs = %#v, want empty", got)
	}
}

// threeSegmentResult mirrors what mergeRecordingResults accumulates across
// reels: three capture segments in tick order, one clip each, one artifact not
// tied to a segment, and an editorial order that differs from capture order.
func threeSegmentResult() RecordingResult {
	return RecordingResult{
		Plan: RecordingPlan{
			Segments: []RecordingSegment{
				{ID: "seg-001", TickStart: 64, TickEnd: 128},
				{ID: "seg-002", TickStart: 256, TickEnd: 320},
				{ID: "seg-003", TickStart: 512, TickEnd: 576},
			},
			EditorialSegmentIDs: []string{"seg-003", "seg-001", "seg-002"},
		},
		Artifacts: []RecordingArtifact{
			{SegmentID: "seg-001", Role: "segment", Type: "video", Path: "seg-001.mp4"},
			{SegmentID: "seg-002", Role: "segment", Type: "video", Path: "seg-002.mp4"},
			{SegmentID: "seg-003", Role: "segment", Type: "video", Path: "seg-003.mp4"},
			{Role: "script", Type: "text", Path: "recording.js"},
		},
	}
}

func TestEditorialSegmentIDs(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*RecordingResult)
		want   []string
	}{
		{
			name:   "editorial order wins over capture order",
			mutate: func(*RecordingResult) {},
			want:   []string{"seg-003", "seg-001", "seg-002"},
		},
		{
			name:   "capture order when the plan carries no editorial order",
			mutate: func(r *RecordingResult) { r.Plan.EditorialSegmentIDs = nil },
			want:   []string{"seg-001", "seg-002", "seg-003"},
		},
		{
			name:   "capture order when the editorial order is incomplete",
			mutate: func(r *RecordingResult) { r.Plan.EditorialSegmentIDs = []string{"seg-003"} },
			want:   []string{"seg-001", "seg-002", "seg-003"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := threeSegmentResult()
			tc.mutate(&result)
			if got := EditorialSegmentIDs(result); !slices.Equal(got, tc.want) {
				t.Fatalf("EditorialSegmentIDs = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFilterResultSegments(t *testing.T) {
	cases := []struct {
		name          string
		mutate        func(*RecordingResult)
		ids           []string
		wantCapture   []string
		wantEditorial []string
		wantArtifacts []string
		wantErr       []string
		// wantNotRecorded expects the error to wrap ErrSegmentNotRecorded.
		wantNotRecorded bool
	}{
		{
			name:          "empty ids returns the result unchanged",
			ids:           nil,
			wantCapture:   []string{"seg-001", "seg-002", "seg-003"},
			wantEditorial: []string{"seg-003", "seg-001", "seg-002"},
			wantArtifacts: []string{"seg-001.mp4", "seg-002.mp4", "seg-003.mp4", "recording.js"},
		},
		{
			name:          "single segment",
			ids:           []string{"seg-002"},
			wantCapture:   []string{"seg-002"},
			wantEditorial: []string{"seg-002"},
			wantArtifacts: []string{"seg-002.mp4", "recording.js"},
		},
		{
			name:          "subset keeps capture order and compiles in requested order",
			ids:           []string{"seg-003", "seg-001"},
			wantCapture:   []string{"seg-001", "seg-003"},
			wantEditorial: []string{"seg-003", "seg-001"},
			wantArtifacts: []string{"seg-001.mp4", "seg-003.mp4", "recording.js"},
		},
		{
			name:          "requested order applies even when the source had no editorial order",
			mutate:        func(r *RecordingResult) { r.Plan.EditorialSegmentIDs = nil },
			ids:           []string{"seg-002", "seg-001"},
			wantCapture:   []string{"seg-001", "seg-002"},
			wantEditorial: []string{"seg-002", "seg-001"},
			wantArtifacts: []string{"seg-001.mp4", "seg-002.mp4", "recording.js"},
		},
		{
			name:            "missing ids are named",
			ids:             []string{"seg-001", "seg-404", "seg-405"},
			wantErr:         []string{"seg-404", "seg-405"},
			wantNotRecorded: true,
		},
		{
			name:    "duplicate ids are rejected",
			ids:     []string{"seg-001", "seg-002", "seg-001"},
			wantErr: []string{"repeats", "seg-001"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := threeSegmentResult()
			if tc.mutate != nil {
				tc.mutate(&source)
			}
			before := threeSegmentResult()
			if tc.mutate != nil {
				tc.mutate(&before)
			}
			got, err := FilterResultSegments(source, tc.ids)
			if len(tc.wantErr) > 0 {
				if err == nil {
					t.Fatalf("FilterResultSegments(%v) error = nil, want error mentioning %v", tc.ids, tc.wantErr)
				}
				for _, fragment := range tc.wantErr {
					if !strings.Contains(err.Error(), fragment) {
						t.Fatalf("error = %q, want it to mention %q", err.Error(), fragment)
					}
				}
				if errors.Is(err, ErrSegmentNotRecorded) != tc.wantNotRecorded {
					t.Fatalf("errors.Is(err, ErrSegmentNotRecorded) = %v, want %v for %q", !tc.wantNotRecorded, tc.wantNotRecorded, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("FilterResultSegments(%v) error = %v", tc.ids, err)
			}
			if captureIDs := SegmentIDs(got); !slices.Equal(captureIDs, tc.wantCapture) {
				t.Fatalf("capture segments = %v, want %v", captureIDs, tc.wantCapture)
			}
			if !slices.Equal(got.Plan.EditorialSegmentIDs, tc.wantEditorial) {
				t.Fatalf("editorial_segment_ids = %v, want %v", got.Plan.EditorialSegmentIDs, tc.wantEditorial)
			}
			if editorialIDs := EditorialSegmentIDs(got); !slices.Equal(editorialIDs, tc.wantEditorial) {
				t.Fatalf("EditorialSegmentIDs = %v, want %v", editorialIDs, tc.wantEditorial)
			}
			artifactPaths := make([]string, 0, len(got.Artifacts))
			for _, artifact := range got.Artifacts {
				artifactPaths = append(artifactPaths, artifact.Path)
			}
			if !slices.Equal(artifactPaths, tc.wantArtifacts) {
				t.Fatalf("artifacts = %v, want %v", artifactPaths, tc.wantArtifacts)
			}
			if !reflect.DeepEqual(source, before) {
				t.Fatalf("FilterResultSegments mutated its input: %#v", source)
			}
		})
	}
}

// A narrowed result must still satisfy the plan invariants a later Validate
// enforces: capture segments in tick order and an editorial order that names
// each of them exactly once.
func TestFilterResultSegmentsKeepsPlanValid(t *testing.T) {
	plan := testPlan()
	plan.Segments = append(plan.Segments, RecordingSegment{ID: "seg-003", TickStart: 35000, TickEnd: 35500})
	plan.EditorialSegmentIDs = []string{"seg-002", "seg-003", "seg-001"}
	if err := plan.Validate(); err != nil {
		t.Fatalf("fixture plan Validate error = %v", err)
	}
	got, err := FilterResultSegments(RecordingResult{Plan: plan}, []string{"seg-003", "seg-001"})
	if err != nil {
		t.Fatalf("FilterResultSegments error = %v", err)
	}
	if err := got.Plan.Validate(); err != nil {
		t.Fatalf("narrowed plan Validate error = %v", err)
	}
	if got := EditorialSegmentIDs(got); !slices.Equal(got, []string{"seg-003", "seg-001"}) {
		t.Fatalf("EditorialSegmentIDs = %v, want the requested order", got)
	}
}
