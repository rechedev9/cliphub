package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestListIndexAt(t *testing.T) {
	tests := []struct {
		name                      string
		y, cursor, total, visible int
		want                      int
	}{
		{"first row", listTopRow, 0, 5, 10, 0},
		{"third row", listTopRow + 2, 0, 5, 10, 2},
		{"above list", listTopRow - 1, 0, 5, 10, -1},
		{"below visible rows", listTopRow + 10, 0, 5, 10, -1},
		{"empty row past last item", listTopRow + 4, 0, 3, 10, -1},
		{"scrolled list maps through scrollStart", listTopRow, 10, 20, 4, 8},
		{"empty list", listTopRow, 0, 0, 10, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := listIndexAt(tt.y, tt.cursor, tt.total, tt.visible)
			if got != tt.want {
				t.Errorf("listIndexAt(%d,%d,%d,%d) got %d, want %d", tt.y, tt.cursor, tt.total, tt.visible, got, tt.want)
			}
		})
	}
}

func TestTabAtX(t *testing.T) {
	// Derive the click zones from the rendered header, not from the widths
	// tabAtX computes, so a header layout change that tabAtX misses fails here.
	cl := tuiclient.New(tuiclient.Config{BaseURL: "http://127.0.0.1:8080"})
	for _, sc := range []screen{screenDemos, screenStreams} {
		m := model{cl: cl, screen: sc, width: 120}
		header := ansi.Strip(m.viewHeader())
		span := func(label string) (int, int) {
			t.Helper()
			idx := strings.Index(header, label)
			if idx < 0 {
				t.Fatalf("header %q does not contain %q", header, label)
			}
			start := ansi.StringWidth(header[:idx])
			return start, start + ansi.StringWidth(label)
		}
		zones := []struct {
			label string
			want  int
		}{
			{label: "ClipHub", want: -1},
			{label: "Demos → Reel", want: 0},
			{label: "Stream Clips", want: 1},
			{label: cl.BaseURL(), want: -1},
		}
		for _, zone := range zones {
			start, end := span(zone.label)
			for x := start; x < end; x++ {
				if got := tabAtX(x); got != zone.want {
					t.Errorf("screen %v: tabAtX(%d) over %q = %d, want %d (header %q)", sc, x, zone.label, got, zone.want, header)
				}
			}
		}
	}
}
