package main

import "testing"

func TestIsHookErrorWindowTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  bool
	}{
		{"afxhooksource2 dialog", "Error - AfxHookSource2", true},
		{"afxhooksource dialog", "Error - AfxHookSource", true},
		{"afxhookgold dialog", "Error - AfxHookGold", true},
		{"game window", "Counter-Strike 2", false},
		{"empty", "", false},
		{"na placeholder", "N/A", false},
		{"errors plural prefix", "Errors - Afx", false},
		{"lowercase is case sensitive", "error - afxhooksource2", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isHookErrorWindowTitle(tt.title); got != tt.want {
				t.Errorf("isHookErrorWindowTitle(%q) = %v, want %v", tt.title, got, tt.want)
			}
		})
	}
}

func TestJudgedWindowTitlePrefersTheHookCrashDialog(t *testing.T) {
	tests := []struct {
		name   string
		titles []string
		want   string
	}{
		{"no windows", nil, ""},
		{"game window only", []string{"Counter-Strike 2"}, ""},
		{"dialog after the game window", []string{"Counter-Strike 2", "Error - AfxHookSource2"}, "Error - AfxHookSource2"},
		{"launcher error dialog", []string{"Launcher Error"}, "Launcher Error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := judgedWindowTitle(tt.titles); got != tt.want {
				t.Errorf("judgedWindowTitle(%q) = %q, want %q", tt.titles, got, tt.want)
			}
		})
	}
}
