package library

import (
	"math"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func validLineup() Lineup {
	return Lineup{Title: "  Humo a  ventana ", Map: "DE_Mirage", Side: "t", Grenade: "Smoke", Origin: [3]float64{1, 2, 3},
		Pitch: 300, Yaw: 270, Tags: []string{"ct", "CT"}}
}

func TestCleanLineupFillsDefaultsAndBringsAnglesIntoSetang(t *testing.T) {
	l, err := CleanLineup(validLineup())
	if err != nil {
		t.Fatal(err)
	}
	if l.Title != "Humo a ventana" || l.Map != "de_mirage" || l.Side != "T" || l.Grenade != "smoke" || l.Status != StatusActive ||
		l.Movement != MovementStanding || l.Click != ClickLeft || len(l.Tags) != 1 {
		t.Fatalf("clean lineup = %+v", l)
	}
	if math.Abs(l.Pitch+60) > 1e-9 || math.Abs(l.Yaw+90) > 1e-9 {
		t.Fatalf("angles = %v %v, want -60 and -90", l.Pitch, l.Yaw)
	}
}

func TestCleanLineupRejectsOutOfVocabularyFields(t *testing.T) {
	bad := [3]float64{math.NaN(), 0, 0}
	cases := map[string]func(l *Lineup){
		"empty title":  func(l *Lineup) { l.Title = " " },
		"bad map":      func(l *Lineup) { l.Map = "../x" },
		"bad side":     func(l *Lineup) { l.Side = "spec" },
		"bad grenade":  func(l *Lineup) { l.Grenade = "c4" },
		"bad status":   func(l *Lineup) { l.Status = "lost" },
		"bad movement": func(l *Lineup) { l.Movement = "flying" },
		"bad click":    func(l *Lineup) { l.Click = "middle" },
		"bad origin":   func(l *Lineup) { l.Origin = bad },
		"bad landing":  func(l *Lineup) { l.Landing = &bad },
		"bad source":   func(l *Lineup) { l.Source = &tacticalplan.Evidence{MatchID: uuid.Nil, Round: 1} },
	}
	for name, mutate := range cases {
		l := validLineup()
		mutate(&l)
		if _, err := CleanLineup(l); err == nil {
			t.Errorf("%s: CleanLineup = nil error", name)
		}
	}
}
