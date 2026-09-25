package recording

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// validFullDemoEvidenceLines is the console transport of one complete capture
// of fullDemoCaptureFixture: snapshot, application, restoration and the
// certified end of its only round, each prefixed with the CS2 timestamp.
func validFullDemoEvidenceLines(t *testing.T, p RecordingPlan, token string) []string {
	t.Helper()
	applied := map[string]any{
		"voice_modenable": 0, "snd_voipvolume": 0, "tv_listen_voice_indices": 0, "tv_listen_voice_indices_h": 0,
		"spec_show_xray": 0, "spec_autodirector": 0, "cl_show_observer_crosshair": 2,
		"cl_drawhud": 1, "cl_draw_only_deathnotices": 1, "crosshair": 1, "cl_demo_predict": 0, "cl_trueview_show_status": 0,
		"cl_spec_show_bindings": 0, "cl_drawhud_specvote": 0, "cl_teamid_overhead_mode": 0, "cl_drawhud_force_teamid_overhead": -1, "hud_showtargetid": 0,
		"cl_drawhud_force_radar": 1, "cl_drawhud_force_deathnotices": 1,
		"cl_hud_radar_background_alpha": .35, "cl_hud_radar_map_additive": 0, "cl_hud_radar_scale": .85, "cl_hud_color": 0, "safezonex": .97, "safezoney": .95,
	}
	var before, after []CvarValue
	for name, value := range applied {
		v, _ := json.Marshal(value)
		before = append(before, CvarValue{Name: name, Value: json.RawMessage(`1`)})
		after = append(after, CvarValue{Name: name, Value: v})
	}
	marker := func(event map[string]any) string {
		b, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		return "09/06 17:58:41 ZV_FULL_DEMO:" + token + ":" + string(b)
	}
	return []string{
		marker(map[string]any{"kind": "settings_before", "values": before}),
		marker(map[string]any{"kind": "settings_applied", "values": after}),
		marker(map[string]any{"kind": "settings_restored", "success": true}),
		marker(map[string]any{"kind": "certified_end", "round_id": p.Segments[0].ID, "end_tick": p.Segments[0].TickEnd}),
	}
}

func TestFullDemoConsoleTransportRejectsIncompleteOrForeignEvidence(t *testing.T) {
	p := fullDemoCaptureFixture(t)
	valid := validFullDemoEvidenceLines(t, p, "test")
	join := func(lines []string) string { return strings.Join(lines, "\n") + "\n" }
	evidence, err := ReadFullDemoCaptureEvidence(strings.NewReader(join(valid)), "test", p)
	if err != nil {
		t.Fatalf("untouched evidence stream rejected: %v", err)
	}
	if !evidence.Restored || evidence.CertifiedEnds[p.Segments[0].ID] != p.Segments[0].TickEnd {
		t.Fatalf("untouched evidence stream parsed as %+v", evidence)
	}
	const restored = 2
	for _, tc := range []struct {
		name  string
		forge func(lines []string) string
		want  string
	}{
		{"restoration under a foreign token", func(l []string) string {
			l[restored] = strings.Replace(l[restored], "ZV_FULL_DEMO:test:", "ZV_FULL_DEMO:foreign:", 1)
			return join(l)
		}, "Missing verified Full Demo settings restoration"},
		{"restoration echoed from player chat", func(l []string) string {
			l[restored] = strings.Replace(l[restored], "ZV_FULL_DEMO:", "player said ZV_FULL_DEMO:", 1)
			return join(l)
		}, "Missing verified Full Demo settings restoration"},
		{"no events under the token", func(l []string) string {
			return strings.ReplaceAll(join(l), "ZV_FULL_DEMO:test:", "ZV_FULL_DEMO:other:")
		}, "Missing verified Full Demo settings restoration"},
		{"truncated final marker", func(l []string) string {
			return join(l[:3]) + `09/06 17:58:41 ZV_FULL_DEMO:test:{"kind":"certified_end","end_tick":`
		}, "incomplete or malformed Full Demo runtime marker"},
		{"incomplete marker before the next event", func(l []string) string {
			l[0] = l[0][:len(l[0])/2]
			return join(l)
		}, "incomplete Full Demo runtime marker before next event"},
		{"trailing content after a marker", func(l []string) string {
			l[restored] += " trailing text"
			return join(l)
		}, "incomplete Full Demo runtime marker before next event"},
		{"wrapped marker over 1 MiB", func(l []string) string {
			return join(l[:3]) + "ZV_FULL_DEMO:test:{" + strings.Repeat(strings.Repeat(" ", 1<<16)+"\n", 17)
		}, "Full Demo runtime marker exceeds 1 MiB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ReadFullDemoCaptureEvidence(strings.NewReader(tc.forge(slices.Clone(valid))), "test", p)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v, want %q", err, tc.want)
			}
		})
	}
	t.Run("single console line over 1 MiB", func(t *testing.T) {
		text := join(valid[:3]) + "ZV_FULL_DEMO:test:{" + strings.Repeat(" ", 1<<20)
		if _, err := ReadFullDemoCaptureEvidence(strings.NewReader(text), "test", p); !errors.Is(err, bufio.ErrTooLong) {
			t.Fatalf("err=%v, want %v", err, bufio.ErrTooLong)
		}
	})
	_ = fmt.Sprint
}
