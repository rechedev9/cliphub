package parser

import (
	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"

	"github.com/rechedev9/cliphub/internal/radarmap"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// Player places are sampled every half second at 64 tick into the grid the
// tactical analysis names zones with, so a landing gets CS2's own place name.
const (
	placeSampleTicks = 32
	placeCellSize    = 64
)

func (w *utilityWatch) samplePlaces(p demoinfocs.Parser) {
	tick := p.GameState().IngameTick()
	if w.placesSampled && tick-w.lastPlaceTick < placeSampleTicks {
		return
	}
	w.placesSampled = true
	w.lastPlaceTick = tick
	for _, pl := range p.GameState().Participants().Playing() {
		if pl == nil || !pl.IsAlive() {
			continue
		}
		place := safeLastPlaceName(pl)
		if place == "" {
			continue
		}
		pos := playerPosition(pl)
		w.places.Add(pos[0], pos[1], pos[2], place)
	}
}

// placeGeometry is the place grid sampled so far. The zero calibration keeps
// one level, so a cell shared by two floors is told apart by height alone.
func (w *utilityWatch) placeGeometry() tacticalplan.MapGeometry {
	if w.placeGeo == nil {
		geo := w.places.Build("", radarmap.Calibration{}, 1)
		w.placeGeo = &geo
	}
	return *w.placeGeo
}
