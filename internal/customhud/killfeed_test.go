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
	if len(first) != 1 || first[0].slot != 0 || !first[0].first || !first[0].last {
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
	if raised.slot != 0 || raised.shift != 1 || raised.first || !raised.last {
		t.Fatalf("second notice then %+v, want a slide from row 1 to row 0", raised)
	}
}

func TestKillfeedSpansKeepOnlyTheNewestRows(t *testing.T) {
	list := notices(1, 1.1, 1.2, 1.3, 1.4, 1.5)
	oldest := killfeedStack{notices: list, duration: 30}.spans(0)
	if len(oldest) != 1 || oldest[0].end != 1.5 || oldest[0].last {
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
