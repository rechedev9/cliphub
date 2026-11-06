package tacticalplan

// FramedMatch is a scouted match whose position blob is read when a builder
// reaches it, so one blob is in memory at a time; nil means no positions.
type FramedMatch struct {
	ScoutMatch
	Blob func() ([]byte, error)
}

// frameSource decodes the rounds of one match's blob. A match without a
// usable blob answers every round with no frames.
type frameSource struct {
	blob   []byte
	header Positions
	doc    Document
}

// openFrames reads a match's blob. A blob that cannot be read or decoded
// leaves the match without positions instead of failing the whole view.
func openFrames(m FramedMatch) frameSource {
	spatial := SpatialMatch{MatchID: m.MatchID, Document: m.Document, Blob: m.Blob}
	blob, header, ok, err := spatial.positions()
	if err != nil || !ok {
		return frameSource{doc: m.Document}
	}
	return frameSource{blob: blob, header: header, doc: m.Document}
}

// round decodes one round's frames; a broken round reads as no frames.
func (s frameSource) round(number int) []Frame {
	if s.blob == nil {
		return nil
	}
	frames, err := roundFrames(s.blob, s.header, s.doc, number)
	if err != nil {
		return nil
	}
	return frames
}

// teamFrame is one live frame of a round seen as the callouts the team's alive
// players stood in.
type teamFrame struct {
	tick     int
	seconds  float64
	callouts map[uint8]string
	// pos is where each named player stood, for spots drawn at real positions.
	pos map[uint8][3]float64
}

// teamTrack turns a round's frames into team frames, from freeze end to the
// round end. Each sample is named once here, since naming is the costly part.
func teamTrack(doc Document, r Round, team map[uint8]bool, frames []Frame) []teamFrame {
	end := roundEndTick(r)
	out := make([]teamFrame, 0, len(frames))
	for _, f := range frames {
		if end > 0 && f.Tick > end {
			break
		}
		secs, ok := secondsSince(r, f.Tick, doc.Demo.Tickrate)
		if !ok {
			continue
		}
		tf := teamFrame{tick: f.Tick, seconds: secs, callouts: map[uint8]string{}, pos: map[uint8][3]float64{}}
		for _, s := range f.Samples {
			if !team[s.Slot] || !s.Flags.Has(FlagAlive) {
				continue
			}
			at := [3]float64{s.X, s.Y, s.Z}
			if callout := NearestCallout(doc.Geometry, at); callout != "" {
				tf.callouts[s.Slot] = callout
				tf.pos[s.Slot] = at
			}
		}
		out = append(out, tf)
	}
	return out
}

// counts is how many of the frame's players stood in each callout.
func (f teamFrame) counts() map[string]int {
	out := map[string]int{}
	for _, callout := range f.callouts {
		out[callout]++
	}
	return out
}
