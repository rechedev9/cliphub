package tacticalplan

// DropRepeatedUtility removes each grenade event that repeats an earlier one
// of the same kind, thrower and tick, keeping the first. Some CS2 demos report
// one detonation twice; a player cannot detonate two grenades of one kind on
// the same tick, so the repeat is never a second grenade.
func DropRepeatedUtility(events []Event) []Event {
	type key struct {
		kind EventKind
		slot uint8
		tick int
	}
	seen := map[key]bool{}
	out := events[:0:0]
	for _, ev := range events {
		if isGrenadeEvent(ev.Kind) && ev.ActorSlot != nil {
			k := key{kind: ev.Kind, slot: *ev.ActorSlot, tick: ev.Tick}
			if seen[k] {
				continue
			}
			seen[k] = true
		}
		out = append(out, ev)
	}
	return out
}

// DropRepeatedUtility applies the package function to every round, so a
// document scanned before the scan dropped repeats counts each grenade once.
func (d *Document) DropRepeatedUtility() {
	for i := range d.Rounds {
		d.Rounds[i].Events = DropRepeatedUtility(d.Rounds[i].Events)
	}
}

func isGrenadeEvent(kind EventKind) bool {
	switch kind {
	case EventSmoke, EventFlash, EventHE, EventMolotov, EventDecoy:
		return true
	default:
		return false
	}
}
