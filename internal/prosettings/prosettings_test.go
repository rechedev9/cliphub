package prosettings

import "testing"

func TestCatalogCapturesEveryPlayerWithinTheDeliveryFrame(t *testing.T) {
	players := Catalog()
	if len(players) == 0 {
		t.Fatal("empty pro settings catalog")
	}
	for _, p := range players {
		width, height := p.CaptureSize()
		if height != 1080 || width%2 != 0 || width < 1080 || width > 1920 {
			t.Errorf("%s captures at %dx%d", p.ID, width, height)
		}
	}
}

func TestCaptureWidthKeepsTheAspectRatio(t *testing.T) {
	for _, tc := range []struct{ width, height, want int }{
		{1280, 960, 1440}, {1024, 768, 1440}, {1280, 1024, 1350}, {1680, 1050, 1728}, {1920, 1080, 1920},
	} {
		if got := CaptureWidth(tc.width, tc.height); got != tc.want {
			t.Errorf("CaptureWidth(%d, %d) = %d, want %d", tc.width, tc.height, got, tc.want)
		}
	}
}

func TestDetectMatchesWholeNameTokens(t *testing.T) {
	for name, want := range map[string]string{
		"ZywOo": "zywoo", "Vitality | ZywOo": "zywoo", "huNter-": "hunter", "m0NESY": "m0nesy",
		"donkey": "", "random player": "", "im_noob": "", "apex legend": "",
	} {
		got, ok := Detect(name)
		if want == "" {
			if ok {
				t.Errorf("Detect(%q) = %s, want no match", name, got.ID)
			}
			continue
		}
		if !ok || got.ID != want {
			t.Errorf("Detect(%q) = %q, %v; want %q", name, got.ID, ok, want)
		}
	}
}

func TestLoadRejectsSettingsCS2WouldClamp(t *testing.T) {
	raw := `[{"id":"x","name":"X","team":"T","width":1280,"height":960,"scaling":"stretched","viewmodel":{"fov":90,"offset_x":0,"offset_y":0,"offset_z":0},"source":"s","updated":"2026-10-01"}]`
	if _, err := load([]byte(raw)); err == nil {
		t.Fatal("viewmodel_fov 90 was accepted")
	}
}
