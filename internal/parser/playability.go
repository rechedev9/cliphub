package parser

// PlayabilityClass is the HLAE-free probe verdict. Recorder classes
// (demo_incompatible, capture_incompatible) are separate and require CS2.
type PlayabilityClass string

const (
	PlayabilityPlayable        PlayabilityClass = "playable"
	PlayabilityUnplayableStart PlayabilityClass = "unplayable_start"
	PlayabilityCorrupt         PlayabilityClass = "corrupt"
	PlayabilityUnknown         PlayabilityClass = "unknown"
)

// ClassifyPlayability decides whether CS2's mandatory playdemo rewind to
// demo tick 0 is safe, given the first full-snapshot tick and the demo tickrate.
//
//	no snapshot          → corrupt
//	tickrate missing     → unknown
//	0 <= tick <= rate    → playable
//	tick > max(64, rate) → unplayable_start
//	gap when rate < 64   → unknown
func ClassifyPlayability(tickrate, firstFullPacketTick int, sawPacket bool) PlayabilityClass {
	if !sawPacket {
		return PlayabilityCorrupt
	}
	if tickrate <= 0 {
		return PlayabilityUnknown
	}
	if firstFullPacketTick >= 0 && firstFullPacketTick <= tickrate {
		return PlayabilityPlayable
	}
	if firstFullPacketTick > max(64, tickrate) {
		return PlayabilityUnplayableStart
	}
	return PlayabilityUnknown
}

// PlayabilityReason is the operator-facing explanation for class.
func PlayabilityReason(class PlayabilityClass, tickrate, firstTick int) string {
	switch class {
	case PlayabilityPlayable:
		return "first full-snapshot tick is inside the opening second; playdemo rewind to tick 0 should survive"
	case PlayabilityUnplayableStart:
		return "first full-snapshot tick is past the opening second; CS2 playdemo rewinds to demo tick 0 and crashes on this class of SourceTV mid-start / second-half demo"
	case PlayabilityCorrupt:
		return "no PacketEntities snapshot before parse stopped; re-extract the demo from the original archive"
	default:
		return "first full-snapshot tick is inconclusive for this tickrate; do not capture unless a vanilla playdemo smoke is approved"
	}
}
