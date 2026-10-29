package parser

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	dp "github.com/markus-wa/godispatch"
)

// povParser dispatches one POV detection during ParseToEnd, the way the
// library does while it reads the userinfo string table.
type povParser struct {
	demoinfocs.Parser
	event    events.POVRecordingPlayerDetected
	handlers []func(events.POVRecordingPlayerDetected)
	parseErr error
}

func (p *povParser) RegisterEventHandler(handler any) dp.HandlerIdentifier {
	if h, ok := handler.(func(events.POVRecordingPlayerDetected)); ok {
		p.handlers = append(p.handlers, h)
	}
	return nil
}

func (p *povParser) ParseToEnd() error {
	for _, h := range p.handlers {
		h(p.event)
	}
	return p.parseErr
}

// The library dispatches POVRecordingPlayerDetected when the header's client
// name matches a user in the demo. Only a real player makes it a POV demo.
func TestParseToEndRefusesClientRecordedDemos(t *testing.T) {
	cases := []struct {
		name     string
		recorder common.PlayerInfo
		parseErr error
		want     error
	}{
		{name: "player client", recorder: common.PlayerInfo{Name: "player", XUID: 76561198000000001}, want: errPOVDemo},
		{name: "player client, truncated", recorder: common.PlayerInfo{Name: "player"}, parseErr: demoinfocs.ErrUnexpectedEndOfDemo, want: errPOVDemo},
		{name: "GOTV relay", recorder: common.PlayerInfo{Name: "GOTV", IsHltv: true}},
		{name: "bot", recorder: common.PlayerInfo{Name: "BOT Ivan", IsFakePlayer: true}},
		{name: "cancelled", recorder: common.PlayerInfo{Name: "player"}, parseErr: demoinfocs.ErrCancelled, want: demoinfocs.ErrCancelled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &povParser{
				event:    events.POVRecordingPlayerDetected{PlayerSlot: 3, PlayerInfo: tc.recorder},
				parseErr: tc.parseErr,
			}

			err := parseToEnd(p)
			if !errors.Is(err, tc.want) {
				t.Fatalf("parseToEnd error = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestParseToEndRecoversCorruptDemoPanic reproduces a real incident: a
// header-valid but otherwise garbage payload (a file
// that isn't truly a CS2 demo despite passing the magic-byte check) made
// demoinfocs.Parser.ParseToEnd panic with invalid protobuf wire-format data,
// which — unrecovered — crashed the whole orchestrator process instead of
// just failing this one job.
func TestParseToEndRecoversCorruptDemoPanic(t *testing.T) {
	garbage := bytes.Repeat([]byte{0x01}, 4096)
	demo := append([]byte("PBDEMS2\x00"), garbage...)

	p := demoinfocs.NewParser(bytes.NewReader(demo))
	defer p.Close()

	err := parseToEnd(p)
	if err == nil {
		t.Fatal("parseToEnd error = nil, want a recovered error for a corrupt demo")
	}
	if !strings.HasPrefix(err.Error(), "demo_incompatible:") {
		t.Fatalf("parseToEnd error = %q, want it to start with %q", err.Error(), "demo_incompatible:")
	}
}

// TestParseToEndTagsParserErrorsAsDemoIncompatible covers the errors
// demoinfocs returns instead of panicking. A CS:GO demo is the common real
// case: v5 refuses it with ErrInvalidFileType, and before this tag the scan
// failed with no failure code, so the user only saw "could not scan".
func TestParseToEndTagsParserErrorsAsDemoIncompatible(t *testing.T) {
	p := demoinfocs.NewParser(bytes.NewReader([]byte("HL2DEMO\x00legacy-csgo-demo")))
	defer p.Close()

	err := parseToEnd(p)
	if err == nil {
		t.Fatal("parseToEnd error = nil, want an error for a CS:GO demo")
	}
	if !strings.HasPrefix(err.Error(), "demo_incompatible:") {
		t.Fatalf("parseToEnd error = %q, want it to start with %q", err.Error(), "demo_incompatible:")
	}
	if !errors.Is(err, demoinfocs.ErrInvalidFileType) {
		t.Fatalf("parseToEnd error = %v, want it to wrap ErrInvalidFileType", err)
	}
}
