package httpapi

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/rechedev9/cliphub/internal/renderplan"
)

// Every JSON field the render POST accepts must reach the merged
// renderplan.EditRequest; a field present in the wire type but missing from
// merge silently drops a user choice on rerender (overlay_theme did exactly
// that). The test walks the struct so a new field cannot be added to one side
// only.
func TestRenderEditRequestMergeCoversEveryField(t *testing.T) {
	const full = `{
		"format":"landscape-16x9","killEffect":"clean","transition":"cut",
		"intro":true,"outro":true,"hook_text":true,"kill_counter":true,
		"match_recap":true,"voice_comms":true,"voice_volume":0.5,"native_hud":true,
		"cover_strategy":"generated-gameplay","cover_first_frame":true,
		"intro_text":"hi","outro_text":"bye",
		"keydrop_family":"keydrop","keydrop_style":"jcorko","keydrop_code":"ZACK",
		"keydrop_position_y":0.7,"keydrop_start_seconds":1.5,"keydrop_end_seconds":9,
		"demo_source":"faceit","overlay_theme":"neon-violet"
	}`
	var patch renderEditRequest
	if err := json.Unmarshal([]byte(full), &patch); err != nil {
		t.Fatal(err)
	}
	pv := reflect.ValueOf(patch)
	for i := range pv.NumField() {
		if pv.Field(i).IsNil() {
			t.Fatalf("fixture leaves %s unset; extend the JSON above", pv.Type().Field(i).Name)
		}
	}

	merged := patch.merge(renderplan.DefaultEditRequest())
	var want renderplan.EditRequest
	if err := json.Unmarshal([]byte(full), &want); err != nil {
		t.Fatal(err)
	}
	want = renderplan.NormalizeEditRequest(want)
	if !reflect.DeepEqual(merged, want) {
		t.Fatalf("merge dropped or altered fields:\n got %+v\nwant %+v", merged, want)
	}

	// An absent field keeps the base value: presence, not zero-value, drives merge.
	base := renderplan.DefaultEditRequest()
	base.OverlayTheme = renderplan.OverlayThemeFaceitOrange
	base.DemoSource = renderplan.DemoSourceFACEIT
	var empty renderEditRequest
	kept := empty.merge(base)
	if kept.OverlayTheme != renderplan.OverlayThemeFaceitOrange || kept.DemoSource != renderplan.DemoSourceFACEIT {
		t.Fatalf("empty patch changed base: %+v", kept)
	}
}
