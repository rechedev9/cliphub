package customhud

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	st "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/sendtables"
	"github.com/rechedev9/cliphub/internal/mediafont"
)

type propertyEntity struct {
	st.Entity
	values map[string]st.PropertyValue
}

func TestPickerCatalogMatchesRenderer(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "web", "public", "hud", "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	var picker []Theme
	if err := json.Unmarshal(body, &picker); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(picker, Themes()) {
		t.Fatal("regenerate HUD picker catalog and previews with go run ./cmd/zv-hud-designs")
	}
}

func (e propertyEntity) PropertyValue(name string) (st.PropertyValue, bool) {
	v, ok := e.values[name]
	return v, ok
}

func TestSource2ClockAndMagazineFromRecordedProperties(t *testing.T) {
	// These values reproduce the Donk Mirage demo. The pinned parser's legacy
	// helpers return an unknown timer and 10 bullets, while CS2 shows 11 bullets
	// in the magazine. This is an ammo encoding difference, not a round number.
	rules := propertyEntity{values: map[string]st.PropertyValue{
		"m_pGameRules.m_fRoundStartTime": {Any: float32(257.75)},
		"m_pGameRules.m_iRoundTime":      {Any: int32(115)},
	}}
	if got := sourceClock(rules, nil, 17776, 64, "live"); got != 95 {
		t.Fatalf("round clock=%d", got)
	}
	if got := sourceClock(rules, nil, 16368, 64, "freeze"); got != 2 {
		t.Fatalf("freeze clock=%d", got)
	}
	bomb := propertyEntity{values: map[string]st.PropertyValue{"m_flC4Blow": {Any: float32(420.5)}}}
	for _, phase := range []string{"planted", "defusing"} {
		if got := sourceClock(rules, bomb, 25600, 64, phase); got != 21 {
			t.Fatalf("bomb clock=%d", got)
		}
	}
	for _, phase := range []string{"ended", "unknown"} {
		if sourceClock(rules, bomb, 25600, 64, phase) != -1 {
			t.Fatal("invented a clock after round end")
		}
	}
	if sourceClock(nil, nil, 1, 64, "live") != -1 || sourceClock(rules, nil, 0, 64, "live") != -1 {
		t.Fatal("missing clock source did not remain unknown")
	}
	for _, count := range []uint32{0, 1, 11, 30} {
		weapon := propertyEntity{values: map[string]st.PropertyValue{"m_iClip1": {Any: count}}}
		if got := sourceMagazine(weapon); got != int(count) {
			t.Fatalf("magazine=%d want %d", got, count)
		}
	}
	if sourceMagazine(nil) != -1 || sourceMagazine(propertyEntity{}) != -1 {
		t.Fatal("invented missing ammunition")
	}
}

func testTimeline() Timeline {
	s := Example()
	s.Tick = 100
	next := Example()
	next.Tick = 164
	next.Players[1].Health = 83
	dead := Example()
	dead.Tick = 228
	dead.Players[1].Health = 0
	dead.Players[1].Alive = false
	return Timeline{Version: TelemetryVersion, DemoSHA256: strings.Repeat("a", 64), TargetSteamID: ExampleTarget, TickRate: 64, EndTick: 400, Snapshots: []Snapshot{s, next, dead}}
}

func TestHUDUsesSourceFramesAcrossTrimsAndSponsorSplits(t *testing.T) {
	r, err := NewRenderer("arena")
	if err != nil {
		t.Fatal(err)
	}
	timeline := testTimeline()
	healthNode := func(s Snapshot) string {
		for _, n := range r.Scene(s, ExampleTarget) {
			if n.ID == "focus/health" {
				return assNode(n)
			}
		}
		t.Fatal("missing health")
		return ""
	}
	for _, tc := range []struct {
		offset     int64
		start, end string
	}{{30, "0:00:00.50", "0:00:01.50"}, {90, "0:00:00.00", "0:00:00.50"}} {
		ass, err := r.ASS(timeline, Window{StartTick: 100, SourceOffsetFrames: tc.offset, Frames: 150})
		if err != nil {
			t.Fatal(err)
		}
		expected := fmt.Sprintf("Dialogue: 10,%s,%s,HUD,,0,0,0,,%s", tc.start, tc.end, healthNode(timeline.Snapshots[1]))
		if !strings.Contains(ass, expected) {
			t.Fatalf("trim/split shifted source health state: missing %s", expected)
		}
	}
	for _, w := range []Window{{StartTick: 0, Frames: 60}, {StartTick: 400, Frames: 120}, {StartTick: 100, Frames: 0}, {StartTick: 100, Frames: 10, SourceOffsetFrames: -1}} {
		if _, err := r.ASS(timeline, w); err == nil {
			t.Fatalf("accepted uncovered window %+v", w)
		}
	}
	timeline.Snapshots[1].Players = timeline.Snapshots[1].Players[:1]
	if _, err := r.ASS(timeline, Window{StartTick: 100, Frames: 150}); err == nil {
		t.Fatal("accepted lost observed player")
	}
}

func TestTenDistinctHUDsFitLongNamesAndKeepUnknownDataUnknown(t *testing.T) {
	if len(Themes()) != 10 {
		t.Fatal("catalog must contain ten designs")
	}
	seen := map[string]bool{}
	scenes := map[string]bool{}
	for _, theme := range Themes() {
		if seen[theme.ID] {
			t.Fatal("duplicate theme")
		}
		seen[theme.ID] = true
		r, err := NewRenderer(theme.ID)
		if err != nil {
			t.Fatal(err)
		}
		state := Example()
		state.Players[1].Name = strings.Repeat("W", 100) + `{\pos(0,0)}<script>`
		state.Players[1].Known = false
		nodes := r.Scene(state, ExampleTarget)
		ids := map[string]bool{}
		for _, n := range nodes {
			if ids[n.ID] {
				t.Fatalf("%s duplicate node %s", theme.ID, n.ID)
			}
			ids[n.ID] = true
			if n.X < 0 || n.X > Width || n.Y < 0 || n.Y > Height {
				t.Fatalf("offscreen text %+v", n)
			}
			if n.ID == "focus/name" && (!strings.HasSuffix(n.Text, "…") || len(n.Text) >= 100) {
				t.Fatal("long name did not fit")
			}
			if (n.ID == "focus/health" || n.ID == "focus/ammo" || n.ID == "focus/armor") && n.Text != "—" {
				t.Fatal("unknown became factual statistics")
			}
		}
		b, _ := json.Marshal(nodes)
		if scenes[string(b)] {
			t.Fatal("identical designs")
		}
		scenes[string(b)] = true
		if strings.Contains(r.SVG(state, ExampleTarget), "<script>") {
			t.Fatal("unescaped SVG text")
		}
	}
	if strings.Contains(assText(`name{\pos(0,0)}\N`), "\\") || strings.Contains(assText("{name}"), "{") {
		t.Fatal("name can inject ASS tags")
	}
}

func TestTelemetryRejectsAmbiguousOrInvalidDocuments(t *testing.T) {
	good := testTimeline()
	body, _ := json.Marshal(good)
	if _, err := Decode(strings.NewReader(string(body))); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{string(body) + "{}", strings.Replace(string(body), `"tick_rate":64`, `"tick_rate":0`, 1), strings.Replace(string(body), `"version":`, `"surprise":true,"version":`, 1)} {
		if _, err := Decode(strings.NewReader(bad)); err == nil {
			t.Fatal("invalid telemetry accepted")
		}
	}
	good.Snapshots[1].Tick = good.Snapshots[0].Tick
	if good.Validate() == nil {
		t.Fatal("duplicate tick accepted")
	}
}

func TestInactiveIdentityCannotHideCurrentRosterMember(t *testing.T) {
	state := Example()
	// An older identity sorts before the five current CTs. It must not evict
	// the fifth player or occupy a living player's slot.
	state.Players = append(state.Players, Player{SteamID: "1", Name: "disconnected", Side: "CT", Inactive: true})
	for _, theme := range Themes() {
		r, _ := NewRenderer(theme.ID)
		cards, liveSlots, deadSlots := 0, 0, 0
		for _, n := range r.Scene(state, ExampleTarget) {
			if n.ID == "player/1/name" {
				t.Fatal("inactive player received a roster card")
			}
			if strings.HasPrefix(n.ID, "player/") && strings.HasSuffix(n.ID, "/name") {
				cards++
			}
			if strings.HasPrefix(n.ID, "player/") && strings.HasSuffix(n.ID, "/bar") {
				liveSlots++
			}
			if strings.HasPrefix(n.ID, "player/") && strings.HasSuffix(n.ID, "/skull") {
				deadSlots++
			}
		}
		if cards != 10 {
			t.Fatalf("%s roster has %d cards", theme.ID, cards)
		}
		if liveSlots != 5 || deadSlots != 5 {
			t.Fatalf("%s live/dead roster slots=%d/%d", theme.ID, liveSlots, deadSlots)
		}
	}
}

func TestASSFilterRendersPathsWithSpacesAndPunctuation(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg unavailable")
	}
	fontDir, err := mediafont.MaterializeHUD()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "HUD's [preview], samples")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer("arena")
	if err != nil {
		t.Fatal(err)
	}
	ass, err := r.ASS(testTimeline(), Window{StartTick: 100, Frames: 60})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "frame's [1],hud.ass")
	if err := os.WriteFile(path, []byte(ass), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=black@0:s=1920x1080:r=60,format=rgba", "-vf", ASSFilter(path, fontDir, true), "-frames:v", "1", "-f", "null", "-")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ASS filter: %v\n%s", err, output)
	}
}
