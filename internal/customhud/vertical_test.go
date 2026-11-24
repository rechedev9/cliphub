package customhud

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/overlaylang"
)

func verticalTimeline(states ...Snapshot) Timeline {
	d := Timeline{Version: TelemetryVersion, DemoSHA256: strings.Repeat("a", 64), TargetSteamID: ExampleTarget, Map: "de_mirage", TickRate: 64}
	for i, s := range states {
		s.Tick = 1000 + i*64
		d.Snapshots = append(d.Snapshots, s)
		d.EndTick = s.Tick + 63
	}
	return d
}

func TestVerticalTrackSplitsEachPlayByScoreboardChanges(t *testing.T) {
	first := Example()
	second := Example()
	second.TimeRemaining = 58
	same := second
	same.Players = append([]Player(nil), second.Players...)
	same.Players[0].Health = 40 // A health change alone keeps the scoreboard state.
	third := second
	third.Players = append([]Player(nil), second.Players...)
	third.Players[2].Alive, third.Players[2].Health = false, 0
	d := verticalTimeline(first, second, same, third)

	track, err := d.VerticalTrack([]VerticalWindow{{ID: "seg-1", TickStart: 1032, TickEnd: 1300}, {ID: "early", TickStart: 900, TickEnd: 1010}})
	if err != nil {
		t.Fatal(err)
	}
	spans := track.Segments[0].Spans
	if len(spans) != 3 {
		t.Fatalf("spans=%+v", spans)
	}
	if spans[0].TickStart != 1032 || spans[0].TickEnd != 1064 || spans[1].TickEnd != 1192 || spans[2].TickEnd != 1256 {
		t.Fatalf("span ticks=%+v", spans)
	}
	if spans[0].State.TimeRemaining != 59 || spans[1].State.TimeRemaining != 58 {
		t.Fatalf("clock=%d,%d", spans[0].State.TimeRemaining, spans[1].State.TimeRemaining)
	}
	if spans[1].State.CTAlive != 4 || spans[2].State.CTAlive != 3 || spans[2].State.TAlive != 1 || spans[2].State.CTSlots != 5 {
		t.Fatalf("alive=%+v", spans[2].State)
	}
	if spans[0].State.CTName != "CT" || spans[0].State.TName != "T" {
		t.Fatalf("generic clan names must fall back to the side: %q %q", spans[0].State.CTName, spans[0].State.TName)
	}
	if early := track.Segments[1].Spans; len(early) != 1 || early[0].TickStart != 1000 || early[0].TickEnd != 1010 {
		t.Fatalf("a play that starts before the telemetry must begin at its first snapshot: %+v", early)
	}
	if err := track.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestVerticalTrackLeavesOutAnUnknownAliveCount(t *testing.T) {
	s := Example()
	s.Players[6].Known = false
	track, err := verticalTimeline(s).VerticalTrack([]VerticalWindow{{ID: "seg-1", TickStart: 1000, TickEnd: 1064}})
	if err != nil {
		t.Fatal(err)
	}
	state := track.Segments[0].Spans[0].State
	if state.TAlive != -1 || state.CTAlive != 4 {
		t.Fatalf("alive ct=%d t=%d", state.CTAlive, state.TAlive)
	}
	nodes := verticalNodes(t, state)
	if _, ok := nodes["score/t-alive"]; ok {
		t.Fatal("an unknown count must not be drawn")
	}
	if _, ok := nodes["score/ct-pip-4"]; !ok {
		t.Fatal("the known side keeps its pips")
	}
}

func TestVerticalTrackRejectsInvalidSpans(t *testing.T) {
	good := VerticalTrack{Version: VerticalVersion, DemoSHA256: strings.Repeat("b", 64), TickRate: 64, Segments: []VerticalSegment{{ID: "seg-1", Spans: []VerticalSpan{{TickStart: 0, TickEnd: 10, State: VerticalExample()}}}}}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*VerticalSpan){
		"alive over slots": func(s *VerticalSpan) { s.State.CTAlive = 6 },
		"unknown phase":    func(s *VerticalSpan) { s.State.Phase = "overtime" },
		"empty span":       func(s *VerticalSpan) { s.TickEnd = s.TickStart },
	} {
		bad := good
		bad.Segments = []VerticalSegment{{ID: "seg-1", Spans: append([]VerticalSpan(nil), good.Segments[0].Spans...)}}
		mutate(&bad.Segments[0].Spans[0])
		if bad.Validate() == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	path := filepath.Join(t.TempDir(), "vertical.json")
	body, _ := json.Marshal(map[string]any{"version": VerticalVersion, "demo_sha256": strings.Repeat("b", 64), "tick_rate": 64, "segments": []any{}, "extra": 1})
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadVerticalTrack(path); err == nil {
		t.Fatal("unknown fields were accepted")
	}
}

func verticalNodes(t *testing.T, state VerticalState) map[string]Node {
	t.Helper()
	r, err := NewRenderer("arena")
	if err != nil {
		t.Fatal(err)
	}
	nodes := map[string]Node{}
	for _, n := range r.verticalScene(state, overlaylang.Spanish) {
		nodes[n.ID] = n
	}
	return nodes
}

func TestVerticalSceneStaysInsideItsBand(t *testing.T) {
	for _, phase := range []string{"freeze", "live", "planted", "defusing", "ended"} {
		state := VerticalExample()
		state.Phase, state.CTName = phase, "A VERY LONG TEAM NAME THAT DOES NOT FIT"
		for id, n := range verticalNodes(t, state) {
			top, bottom := n.Y, n.Y+n.H
			if n.Text != "" {
				top, bottom = n.Y-n.Size/2, n.Y+n.Size/2
			} else if n.H == 0 {
				continue // A plate outline carries no box; its text and pips are checked.
			}
			if top < verticalTop-4 || bottom > verticalAliveY+verticalAliveH+4 {
				t.Errorf("%s: %s spans y %d..%d outside the HUD band", phase, id, top, bottom)
			}
			if strings.ContainsAny(n.Text, "\u2014\u2013") {
				t.Errorf("%s: %s uses a dash: %q", phase, id, n.Text)
			}
		}
	}
	nodes := verticalNodes(t, VerticalState{Round: 2, Phase: "ended", CTAlive: 1, TAlive: 0, CTSlots: 5, TSlots: 5})
	if _, ok := nodes["score/time"]; ok {
		t.Fatal("an ended round must not show a clock")
	}
	if n := nodes["score/phase"]; n.Text != "FIN DE RONDA" {
		t.Fatalf("ended phase=%q", n.Text)
	}
	if n := verticalNodes(t, VerticalState{Round: 4, Phase: "planted", TimeRemaining: -1, CTAlive: 2, TAlive: 2, CTSlots: 5, TSlots: 5})["score/time"]; n.Text != "--:--" {
		t.Fatalf("unobserved clock=%q", n.Text)
	}
}

func TestVerticalASSWritesStaticPlatesOnceAndFadesAroundGaps(t *testing.T) {
	theme, err := VerticalTheme("")
	if err != nil {
		t.Fatal(err)
	}
	a := VerticalExample()
	b := a
	b.TimeRemaining--
	script, err := VerticalASS(VerticalScript{Theme: theme, Cues: []VerticalCue{
		{StartSeconds: 2, EndSeconds: 3, State: a},
		{StartSeconds: 3, EndSeconds: 4, State: b},
		{StartSeconds: 6, EndSeconds: 7, State: b},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, "PlayResX: 1080\nPlayResY: 1920") {
		t.Fatal("the script must be laid out on the 9:16 frame")
	}
	var clocks []string
	for _, line := range strings.Split(script, "\n") {
		if strings.HasPrefix(line, "Dialogue:") && (strings.Contains(line, "}1:14") || strings.Contains(line, "}1:13")) {
			clocks = append(clocks, line)
		}
	}
	if len(clocks) != 3 {
		t.Fatalf("clock lines=%d:\n%s", len(clocks), strings.Join(clocks, "\n"))
	}
	if !strings.Contains(clocks[0], "0:00:02.00,0:00:03.00") || !strings.Contains(clocks[0], `\fad(200,0)`) {
		t.Fatalf("the first clock must fade in at the run start: %s", clocks[0])
	}
	if !strings.Contains(clocks[1], "0:00:03.00,0:00:04.00") || !strings.Contains(clocks[1], `\fad(0,200)`) {
		t.Fatalf("the last clock of a run must fade out before the gap: %s", clocks[1])
	}
	run := 0
	for _, line := range strings.Split(script, "\n") {
		if strings.Contains(line, "0:00:02.00,0:00:04.00") {
			run++
		}
	}
	if run == 0 {
		t.Fatal("the plates must span both cues of the run as one line")
	}
	if _, err := VerticalASS(VerticalScript{Theme: theme, Cues: []VerticalCue{{StartSeconds: -1, EndSeconds: 1, State: a}}}); err == nil {
		t.Fatal("a cue before the video was accepted")
	}
}

func TestVerticalSVGDrawsTheBand(t *testing.T) {
	r, err := NewRenderer("prism")
	if err != nil {
		t.Fatal(err)
	}
	svg := r.VerticalSVG(VerticalExample(), overlaylang.Spanish)
	if !strings.Contains(svg, `viewBox="226 120 628 186"`) || !strings.Contains(svg, ">RONDA 17<") || !strings.Contains(svg, ">1:14<") {
		t.Fatalf("svg=%.400s", svg)
	}
	if _, err := VerticalTheme("missing"); err == nil {
		t.Fatal("an unknown design was accepted")
	}
}

func TestEnglishVerticalHUDNamesEveryPhaseInEnglish(t *testing.T) {
	theme, err := VerticalTheme("")
	if err != nil {
		t.Fatal(err)
	}
	var cues []VerticalCue
	for i, phase := range []string{"live", "freeze", "planted", "defusing", "ended"} {
		state := VerticalExample()
		state.Phase = phase
		cues = append(cues, VerticalCue{StartSeconds: float64(i), EndSeconds: float64(i + 1), State: state})
	}
	script, err := VerticalASS(VerticalScript{Theme: theme, Cues: cues, Language: overlaylang.English})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"}ROUND 17", "}FREEZE TIME", "}BOMB PLANTED", "}DEFUSING", "}ROUND END"} {
		if !strings.Contains(script, want) {
			t.Errorf("the English HUD lacks %q", want)
		}
	}
	for _, spanish := range []string{"RONDA", "COMPRAS", "BOMBA", "DESACTIVANDO"} {
		if strings.Contains(script, spanish) {
			t.Errorf("the English HUD still writes %q", spanish)
		}
	}
	r, err := NewRenderer("prism")
	if err != nil {
		t.Fatal(err)
	}
	if svg := r.VerticalSVG(VerticalExample(), overlaylang.English); !strings.Contains(svg, ">ROUND 17<") {
		t.Fatalf("the English preview does not name the round: %.400s", svg)
	}
}
