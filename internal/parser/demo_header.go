package parser

import (
	"fmt"
	"sync"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"
)

// DemoHeaderWatch keeps the CS2 demo file header seen on a live parser.
type DemoHeaderWatch struct {
	mu     sync.Mutex
	header *msg.CDemoFileHeader
}

// TrackDemoHeader registers a handler for the CS2 demo file header, which names
// the map and the CS2 build that recorded the demo.
func TrackDemoHeader(p demoinfocs.Parser) *DemoHeaderWatch {
	w := &DemoHeaderWatch{}
	p.RegisterNetMessageHandler(func(h *msg.CDemoFileHeader) {
		w.mu.Lock()
		defer w.mu.Unlock()
		w.header = h
	})
	return w
}

// Summary renders the header as log fields. It leaves out the server and
// client names, which can name a person.
func (w *DemoHeaderWatch) Summary() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.header == nil {
		return "header=missing"
	}
	return fmt.Sprintf("map=%s patch_version=%d build=%d", w.header.GetMapName(), w.header.GetPatchVersion(), w.header.GetBuildNum())
}

// DemoHeader is the part of the CS2 demo file header the library keeps or
// reads to detect where a demo came from.
type DemoHeader struct {
	ServerName   string
	BuildNum     int
	PatchVersion int
}

// Header returns the header seen so far, and false before the parser read it.
func (w *DemoHeaderWatch) Header() (DemoHeader, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.header == nil {
		return DemoHeader{}, false
	}
	return DemoHeader{
		ServerName:   w.header.GetServerName(),
		BuildNum:     int(w.header.GetBuildNum()),
		PatchVersion: int(w.header.GetPatchVersion()),
	}, true
}
