package recapplan

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTransitionsPreserveLegacyHashAndCapture(t *testing.T) {
	d := fixtureDocument(t)
	legacy, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(legacy), `"transitions"`) {
		t.Fatal("new defaults changed historical approval wire")
	}
	var decoded Document
	if err := decodeStrict(legacy, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatal(err)
	}
	options := d.Options
	transitions := DefaultTransitions()
	transitions.Enabled = true
	options.Transitions = &transitions
	changed, err := Plan(fixtureFacts(), options, d.Voice, d.Assets, d.Input.FactsRef)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d.Timeline, changed.Timeline) || !reflect.DeepEqual(d.Rounds, changed.Rounds) {
		t.Fatal("transitions changed the editorial clock or source coverage")
	}
	oldCapture, _ := d.CaptureHash()
	newCapture, _ := changed.CaptureHash()
	if oldCapture != newCapture || !CaptureCovers(d, changed) {
		t.Fatal("editing effects invalidated reusable gameplay")
	}
	if d.PlanHash == changed.PlanHash {
		t.Fatal("effects were omitted from approval hash")
	}
	snapshot := Snapshot{Document: changed, Approval: Approval{PlanHash: d.PlanHash, AllowSafeTailTrim: true, Timestamp: time.Now()}}
	if err := snapshot.Validate(); err == nil {
		t.Fatal("old approval accepted new effects")
	}
}

func TestTransitionOptionsStrictWire(t *testing.T) {
	o := fixtureOptions()
	value := DefaultTransitions()
	o.Transitions = &value
	b, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Options
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(o, decoded) {
		t.Fatal("transition decisions did not roundtrip")
	}
	for _, bad := range []string{
		strings.Replace(string(b), `"whip":true,`, "", 1),
		strings.Replace(string(b), `"duration_frames":8`, `"duration_frames":8.5`, 1),
		strings.Replace(string(b), `"direction":"alternate"`, `"direction":"ffmpeg-expression"`, 1),
		strings.Replace(string(b), `"enabled":false,"duration_frames"`, `"enabled":false,"enabled":true,"duration_frames"`, 1),
		strings.Replace(string(b), `"blur_pixels":24`, `"blur_pixels":null`, 1),
		strings.Replace(string(b), `"game_tail_lowpass_hz":0`, `"game_tail_lowpass_hz":100`, 1),
	} {
		if err := json.Unmarshal([]byte(bad), &decoded); err == nil {
			t.Fatal("partial, duplicate or invalid transition object accepted")
		}
	}
	for _, invalid := range []float64{math.NaN(), math.Inf(1), -.1, 2} {
		value.CommsTailSeconds = invalid
		if err := value.Validate(); err == nil {
			t.Fatalf("accepted invalid tail %g", invalid)
		}
	}
}

func TestTransitionBoundariesRespectAdsSplitsAndShortRounds(t *testing.T) {
	o := DefaultTransitions()
	o.Enabled = true
	d := Document{Options: Options{Transitions: &o}, Timeline: []TimelineItem{
		{Role: "round", SourceRef: "a", StartFrame: 0, EndFrame: 120},
		{Role: "round", SourceRef: "b", StartFrame: 120, EndFrame: 240},
		{Role: "sponsor", StartFrame: 240, EndFrame: 300},
		{Role: "round", SourceRef: "b", StartFrame: 300, EndFrame: 360},
		{Role: "round", SourceRef: "b", StartFrame: 360, EndFrame: 420},
		{Role: "round", SourceRef: "c", StartFrame: 420, EndFrame: 423},
		{Role: "round", SourceRef: "d", StartFrame: 423, EndFrame: 483},
	}}
	got := d.TransitionBoundaries()
	want := []TransitionBoundary{{0, 1, 120, 4, 4}, {4, 5, 420, 4, 1}, {5, 6, 423, 1, 4}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("boundaries=%+v want=%+v", got, want)
	}
	if !d.HasTransitionSFX() {
		t.Fatal("SFX should prevent silent-program approval")
	}
	o.Enabled = false
	if len(d.TransitionBoundaries()) != 0 || d.HasTransitionSFX() {
		t.Fatal("disabled effects were scheduled")
	}
}
