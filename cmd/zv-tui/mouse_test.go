package main

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
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
	// Zones track titleStyle("ClipHub") + gap; compute the same base the
	// production helper uses so a brand rename does not hard-code widths.
	base := lipgloss.Width(titleStyle.Render("ClipHub")) + 2
	demosW := lipgloss.Width(tabInactive.Render("Demos → Reel"))
	streamsStart := base + demosW + 1
	tests := []struct {
		name string
		x    int
		want int
	}{
		{"title area", max(0, base-2), -1},
		{"demos tab start", base, 0},
		{"demos tab end", base + demosW - 1, 0},
		{"streams tab", streamsStart, 1},
		{"past tabs", 60, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tabAtX(tt.x); got != tt.want {
				t.Errorf("tabAtX(%d) got %d, want %d (base=%d demosW=%d)", tt.x, got, tt.want, base, demosW)
			}
		})
	}
}
