package customhud

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

func notices(at ...float64) []KillNotice {
	out := make([]KillNotice, 0, len(at))
	for i, t := range at {
		out = append(out, KillNotice{AtSeconds: t, Attacker: "target", AttackerSide: "CT", Victim: "victim-" + strconv.Itoa(i), VictimSide: "T", Weapon: "USP-S"})
	}
	return out
}

func TestKillfeedSpansStackNewestBelowAndSlideUp(t *testing.T) {
	list := notices(1, 2)
	first := killfeedStack{notices: list, duration: 30}.spans(0)
	if len(first) != 1 || first[0].slot != 0 || first[0].start != 1 || first[0].end != 1+killfeedLifetime {
		t.Fatalf("first notice spans = %+v, want one slot-0 span", first)
	}
	second := killfeedStack{notices: list, duration: 30}.spans(1)
	if len(second) != 2 {
		t.Fatalf("second notice spans = %+v, want two", second)
	}
	below, raised := second[0], second[1]
	if below.slot != 1 || below.end != 1+killfeedLifetime {
		t.Fatalf("second notice starts in %+v, want row 1 until the first expires", below)
	}
	if raised.slot != 0 || raised.shift != 1 || raised.end != 2+killfeedLifetime {
		t.Fatalf("second notice then %+v, want a slide from row 1 to row 0", raised)
	}
}

func TestKillfeedSpansKeepOnlyTheNewestRows(t *testing.T) {
	list := notices(1, 1.1, 1.2, 1.3, 1.4, 1.5)
	oldest := killfeedStack{notices: list, duration: 30}.spans(0)
	if len(oldest) != 1 || oldest[0].end != 1.5 {
		t.Fatalf("oldest notice spans = %+v, want hidden once a sixth row arrives", oldest)
	}
	if newest := (killfeedStack{notices: list, duration: 30}).spans(5); newest[0].slot != killfeedMaxRows-1 {
		t.Fatalf("newest notice slot = %d, want the bottom row", newest[0].slot)
	}
}

func TestKillfeedSpansClipToTheVideo(t *testing.T) {
	spans := killfeedStack{notices: notices(4), duration: 5}.spans(0)
	if len(spans) != 1 || spans[0].end != 5 {
		t.Fatalf("spans = %+v, want the notice to end with the video", spans)
	}
	if spans := (killfeedStack{notices: notices(6), duration: 5}).spans(0); spans != nil {
		t.Fatalf("spans after the video = %+v", spans)
	}
}

func TestKillfeedRowIsCenteredAndKeepsUnknownLabels(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	rows := []KillNotice{
		{Attacker: "target", Victim: "x", Weapon: "USP-S", Headshot: true},
		{Attacker: "target", Victim: "a much longer victim name", Weapon: "flare"},
	}
	for _, notice := range rows {
		parts := killfeedRow(notice, killfeedRowBox{centerX: 540, top: 300, scale: 1})
		minX, maxX := math.Inf(1), math.Inf(-1)
		axis := 0
		for _, field := range strings.Fields(parts[0].path) {
			v, err := strconv.ParseFloat(field, 64)
			if err != nil {
				axis = 0
				continue
			}
			if axis%2 == 0 {
				minX, maxX = min(minX, v), max(maxX, v)
			}
			axis++
		}
		if center := (minX + maxX) / 2; math.Abs(center-540) > 1 {
			t.Fatalf("%q row spans %.1f..%.1f, centered at %.1f", notice.Victim, minX, maxX, center)
		}
	}
	labelled := false
	for _, part := range killfeedRow(rows[1], killfeedRowBox{centerX: 540, top: 300, scale: 1}) {
		labelled = labelled || part.text == "flare"
	}
	if !labelled {
		t.Fatal("unknown weapon lost its source label")
	}
}

func TestKillfeedASSRejectsInvalidLayout(t *testing.T) {
	if _, err := KillfeedASS(notices(1), KillfeedLayout{Width: 1080, Height: 1920, DurationSeconds: 0}); err == nil {
		t.Fatal("accepted a zero-length video")
	}
}

func TestKillfeedLabelsMeasureWhatLibassDraws(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	fit := func(value string) killfeedLabel {
		return killfeedFit{value: value, size: 36, budget: 330, weight: 700}.label()
	}
	// The HUD fonts lack these glyphs; libass draws them with a wider system font.
	if emoji := fit(strings.Repeat("🔥", 13)); emoji.width > 330 || !strings.HasSuffix(emoji.text, "…") {
		t.Fatalf("emoji name = %q, %d px; want it cut to the budget", emoji.text, emoji.width)
	}
	if cjk := fit("我是一个很长的名字"); cjk.width < 9*36 {
		t.Fatalf("CJK name reserves %d px, less than a full font size per character", cjk.width)
	}
	if braces := fit(`{\pos(0,0)}evil`); strings.ContainsAny(braces.text, `{}\`) {
		t.Fatalf("label %q is measured before escaping", braces.text)
	}
	if long := fit(strings.Repeat("W", 23)); long.width <= 300 || long.width > 330 {
		t.Fatalf("long name = %q, %d px; want it to fill the 330 px budget at the size libass draws", long.text, long.width)
	}
}

// libassAlpha is libass's \fade interpolation; 255 is transparent.
func libassAlpha(fade [7]int, now int) float64 {
	a1, a2, a3, t1, t2, t3, t4 := float64(fade[0]), float64(fade[1]), float64(fade[2]), fade[3], fade[4], fade[5], fade[6]
	switch {
	case now < t1:
		return a1
	case now < t2:
		cf := float64(now-t1) / float64(t2-t1)
		return a1*(1-cf) + a2*cf
	case now < t3:
		return a2
	case now < t4:
		cf := float64(now-t3) / float64(t4-t3)
		return a2*(1-cf) + a3*cf
	}
	return a3
}

func TestKillfeedRowsFadeAndSlideWithoutJumps(t *testing.T) {
	const step = 66
	// A spray: the rows expire 50 ms apart, shorter than a slide or a fade.
	stack := killfeedStack{notices: notices(1, 1.05, 1.1), duration: 30}
	events := stack.events(2, step)
	if len(events) != 3 {
		t.Fatalf("events = %+v, want three rows for the newest notice", events)
	}
	ms := func(e killfeedEvent) int { return int(math.Round((e.end - e.start) * 1000)) }
	y := func(e killfeedEvent, now int) float64 {
		return float64(e.slot*step) + float64(e.offset)*max(0, 1-float64(now)/killfeedSlideMS)
	}
	if got := libassAlpha(events[0].fade, 0); got != 255 {
		t.Fatalf("first frame alpha = %v, want a fade in from transparent", got)
	}
	for i := 1; i < len(events); i++ {
		prev, next := events[i-1], events[i]
		if a, b := libassAlpha(prev.fade, ms(prev)), libassAlpha(next.fade, 0); math.Abs(a-b) > 3 {
			t.Fatalf("alpha jumps from %.0f to %.0f at %.2fs", a, b, next.start)
		}
		if a, b := y(prev, ms(prev)), y(next, 0); math.Abs(a-b) > 1 {
			t.Fatalf("row jumps from y %.1f to %.1f at %.2fs", a, b, next.start)
		}
	}
	last := events[len(events)-1]
	if got := libassAlpha(last.fade, ms(last)); got != 255 {
		t.Fatalf("last frame alpha = %v, want the fade out to finish", got)
	}
}
