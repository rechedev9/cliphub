package tacticalplan

import (
	"reflect"
	"testing"
)

// track builds team frames every half second; at(s) gives each slot's callout.
func track(until float64, at func(s float64) map[uint8]string) []teamFrame {
	var out []teamFrame
	for s := 0.0; s <= until; s += 0.5 {
		out = append(out, teamFrame{tick: int(s * 64), seconds: s, callouts: at(s)})
	}
	return out
}

func TestControlledZonesNeedTwoPlayersForFiveUnbrokenSeconds(t *testing.T) {
	got := controlledZones(track(60, func(s float64) map[uint8]string {
		switch {
		case s < 10:
			return map[uint8]string{2: "TSpawn", 3: "TSpawn", 4: "TSpawn"}
		case s < 14:
			// Four seconds: too short to be a hold.
			return map[uint8]string{2: "Palace", 3: "Palace"}
		case s < 16:
			return map[uint8]string{2: "Middle", 3: "Middle", 4: "Middle"}
		case s < 16.5:
			// One player steps out: the run breaks.
			return map[uint8]string{2: "Middle", 3: "Connector"}
		case s < 22:
			return map[uint8]string{2: "Middle", 3: "Middle"}
		case s < 38:
			// One player alone holds nothing.
			return map[uint8]string{2: "Underpass"}
		default:
			// Past the window, however long.
			return map[uint8]string{2: "BombsiteB", 3: "BombsiteB"}
		}
	}))
	want := []ZoneHold{{Zone: "Middle", FromSeconds: 16.5, Seconds: 5, Players: 3}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("zones = %+v, want %+v", got, want)
	}
	if !holdsMapZone(got) || holdsMapZone([]ZoneHold{{Zone: "BombsiteA"}}) {
		t.Fatal("a mid hold is map control and a site hold is not")
	}
}

func TestSitePresenceAndContact(t *testing.T) {
	tr := track(40, func(s float64) map[uint8]string {
		switch {
		case s < 10:
			return map[uint8]string{2: "BombsiteA"}
		case s < 20:
			return map[uint8]string{2: "BombsiteA", 3: "BombsiteA"}
		default:
			return map[uint8]string{2: "BombsiteB", 3: "BombsiteB"}
		}
	})
	presence := sitePresence(tr)
	if !reflect.DeepEqual(presence, map[Site]int{SiteA: 640, SiteB: 1280}) {
		t.Fatalf("presence = %v, want A at 10 s and B at 20 s", presence)
	}
	r := Round{TickFreezeEnd: 1, Bomb: &Bomb{PlantTick: 600, Site: SiteB}}
	if tick, site := siteContact(Document{}, r, map[uint8]bool{2: true}, presence); tick != 600 || site != SiteB {
		t.Fatalf("contact = %d at %s, want the earlier plant", tick, site)
	}
	slot := uint8(2)
	r.Bomb = nil
	r.Events = []Event{{Tick: 300, Kind: EventKill, TargetSlot: &slot, Place: "Palace", TargetPos: [3]float64{}}}
	doc := Document{Geometry: MapGeometry{Callouts: []Callout{{Name: "BombsiteA", Level: "default"}}}}
	if tick, site := siteContact(doc, r, map[uint8]bool{2: true}, presence); tick != 300 || site != SiteA {
		t.Fatalf("contact = %d at %s, want the team's death on A", tick, site)
	}
}

func TestSiteFromPlace(t *testing.T) {
	for place, want := range map[string]Site{
		"BombsiteA": SiteA, "Bombsite A": SiteA, "BombsiteB": SiteB, "Middle": SiteMid, "Mid": SiteMid,
		"Apartments": SiteNone, "": SiteNone,
	} {
		if got := SiteFromPlace(place); got != want {
			t.Errorf("SiteFromPlace(%q) = %s, want %s", place, got, want)
		}
	}
}
