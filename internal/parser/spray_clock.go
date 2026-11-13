package parser

import (
	"math"
	"sync"
	"time"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	st "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/sendtables"
)

// cs2Release bounds the spray times worth reading: an unsigned spray carries zero.
var cs2Release = time.Date(2023, time.March, 22, 0, 0, 0, 0, time.UTC)

// SprayClock keeps the earliest time the Steam game coordinator signed a spray in a demo.
// The demo header has no date, so a signed spray is the only wall clock a CS2 demo carries.
type SprayClock struct {
	mu       sync.Mutex
	earliest time.Time
}

// TrackSprayClock registers a handler that reads the signing time of every spray in the demo.
func TrackSprayClock(p demoinfocs.Parser) *SprayClock {
	c := &SprayClock{}
	p.RegisterEventHandler(func(events.DataTablesParsed) {
		class := p.ServerClasses().FindByName("CPlayerSprayDecal")
		if class == nil {
			return
		}
		class.OnEntityCreated(func(spray st.Entity) {
			if v, ok := spray.PropertyValue("m_rtGcTime"); ok {
				c.observe(v.Any)
			}
		})
	})
	return c
}

// observe reads m_rtGcTime, an RTime32 that demoinfocs decodes as uint64.
func (c *SprayClock) observe(raw any) {
	var seconds uint64
	switch v := raw.(type) {
	case uint64:
		seconds = v
	case uint32:
		seconds = uint64(v)
	default:
		return
	}
	if seconds > math.MaxUint32 {
		return
	}
	at := time.Unix(int64(seconds), 0).UTC() // #nosec G115 -- bounded by MaxUint32 above.
	if at.Before(cs2Release) {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.earliest.IsZero() || at.Before(c.earliest) {
		c.earliest = at
	}
}

// Earliest returns the earliest signed spray, and false when the demo had none.
func (c *SprayClock) Earliest() (time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.earliest, !c.earliest.IsZero()
}
