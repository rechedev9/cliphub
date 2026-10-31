package library

import (
	"math"
	"strings"
	"testing"
)

func TestCleanAnnotation(t *testing.T) {
	line := Stroke{Color: "accent", Width: 3, Points: [][2]float64{{-100, 200}, {50, 60}}}
	cases := []struct {
		name    string
		in      Annotation
		wantErr string
	}{
		{name: "text only", in: Annotation{Round: 3, Tick: 1200, Text: "  rotan pronto  "}},
		{name: "drawing only", in: Annotation{Round: 1, Tick: 0, Strokes: []Stroke{line}}},
		{name: "empty", in: Annotation{Round: 1, Tick: 10, Text: "   "}, wantErr: "text or a drawing"},
		{name: "round zero", in: Annotation{Round: 0, Text: "x"}, wantErr: "round"},
		{name: "negative tick", in: Annotation{Round: 1, Tick: -1, Text: "x"}, wantErr: "tick"},
		{name: "long text", in: Annotation{Round: 1, Text: strings.Repeat("a", MaxAnnotationTextRunes+1)}, wantErr: "at most"},
		{name: "unknown colour", in: Annotation{Round: 1, Strokes: []Stroke{{Color: "red", Width: 2, Points: line.Points}}}, wantErr: "color"},
		{name: "width", in: Annotation{Round: 1, Strokes: []Stroke{{Color: "ct", Width: 0, Points: line.Points}}}, wantErr: "width"},
		{name: "one point", in: Annotation{Round: 1, Strokes: []Stroke{{Color: "t", Width: 2, Points: [][2]float64{{1, 1}}}}}, wantErr: "points"},
		{name: "infinite point", in: Annotation{Round: 1, Strokes: []Stroke{{Color: "t", Width: 2, Points: [][2]float64{{1, 1}, {math.Inf(1), 0}}}}}, wantErr: "finite"},
		{name: "off the map", in: Annotation{Round: 1, Strokes: []Stroke{{Color: "t", Width: 2, Points: [][2]float64{{1, 1}, {1e6, 0}}}}}, wantErr: "finite"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CleanAnnotation(tc.in)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("CleanAnnotation() error = %v, want it to mention %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("CleanAnnotation() = %v", err)
			}
			if got.Strokes == nil || got.Text != strings.TrimSpace(tc.in.Text) {
				t.Fatalf("CleanAnnotation() = %+v, want trimmed text and a non-nil drawing", got)
			}
		})
	}
}

func TestCleanAnnotationCapsTotalPoints(t *testing.T) {
	points := make([][2]float64, MaxAnnotationStrokePoint)
	strokes := make([]Stroke, 0, 11)
	for range 11 {
		strokes = append(strokes, Stroke{Color: "mark", Width: 2, Points: points})
	}
	if _, err := CleanAnnotation(Annotation{Round: 1, Strokes: strokes}); err == nil {
		t.Fatalf("CleanAnnotation(22000 points) = nil, want the total cap")
	}
}
