package lineups

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/rechedev9/cliphub/internal/killplan"
)

func TestCatalogMatchesSmokeByLandingAndThrow(t *testing.T) {
	dir := t.TempDir()
	writeCatalog(t, dir, `{
  "map": "de_ancient",
  "smokes": [
    {
      "id": "ancient_t_spawn_to_ct",
      "destination": "CT",
      "from_area": "T Spawn",
      "side": "T",
      "landing": [100, 200, 0],
      "landing_radius": 96,
      "throw": [10, 20, 0],
      "throw_radius": 64
    }
  ]
}`)

	catalog, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir error = %v", err)
	}
	match, ok := catalog.MatchSmoke("de_ancient", killplan.UtilityThrow{
		Type:       "smokegrenade",
		ThrowPos:   [3]float64{20, 25, 0},
		LandingPos: [3]float64{120, 210, 0},
	})
	if !ok {
		t.Fatal("MatchSmoke ok = false, want true")
	}
	if match.ID != "ancient_t_spawn_to_ct" || match.Destination != "CT" || match.FromArea != "T Spawn" || match.Side != "T" {
		t.Fatalf("match = %#v", match)
	}
	if match.Confidence <= 0 || match.DistanceUnits <= 0 {
		t.Fatalf("match confidence/distance = %#v", match)
	}
}

func TestCatalogDoesNotInventDestinationOutsideRadius(t *testing.T) {
	dir := t.TempDir()
	writeCatalog(t, dir, `{
  "map": "de_ancient",
  "smokes": [
    {
      "id": "ancient_a_main",
      "destination": "A Main",
      "landing": [0, 0, 0],
      "landing_radius": 32
    }
  ]
}`)

	catalog, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir error = %v", err)
	}
	if _, ok := catalog.MatchSmoke("de_ancient", killplan.UtilityThrow{
		Type:       "smokegrenade",
		LandingPos: [3]float64{200, 0, 0},
	}); ok {
		t.Fatal("MatchSmoke ok = true, want false")
	}
}

func TestCatalogDoesNotMatchNonSmokeUtility(t *testing.T) {
	dir := t.TempDir()
	writeCatalog(t, dir, `{
  "map": "de_inferno",
  "smokes": [
    {
      "id": "inferno_t_ramp",
      "destination": "T ramp",
      "landing": [100, 200, 0],
      "landing_radius": 96
    }
  ]
}`)

	catalog, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir error = %v", err)
	}
	if _, ok := catalog.MatchSmoke("de_inferno", killplan.UtilityThrow{
		Type:       "flashbang",
		LandingPos: [3]float64{100, 200, 0},
	}); ok {
		t.Fatal("MatchSmoke ok = true for flashbang, want false")
	}
}

func writeCatalog(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "de_ancient.smokes.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMatchLandingIgnoresTheThrowAndLoadFSReadsTheRoot(t *testing.T) {
	fsys := fstest.MapFS{
		"de_mirage.smokes.json": {Data: []byte(`{"map":"de_mirage","smokes":[{"id":"mirage-ct","destination":"CT","landing":[0,0,0],"landing_radius":50,"throw":[900,900,0],"throw_radius":10}]}`)},
		"notes.txt":             {Data: []byte("not a catalog")},
	}
	catalog, err := LoadFS(fsys)
	if err != nil {
		t.Fatalf("LoadFS error = %v", err)
	}
	if catalog.MapSize("DE_MIRAGE") != 1 || catalog.MapSize("de_nuke") != 0 {
		t.Fatalf("MapSize = %d / %d", catalog.MapSize("de_mirage"), catalog.MapSize("de_nuke"))
	}
	match, ok := catalog.MatchLanding("de_mirage", [3]float64{25, 0, 0})
	if !ok || match.ID != "mirage-ct" || match.Confidence != 0.5 {
		t.Fatalf("MatchLanding = %+v, %v; want the CT smoke at half confidence", match, ok)
	}
	if _, ok := catalog.MatchSmoke("de_mirage", killplan.UtilityThrow{Type: "smokegrenade", LandingPos: [3]float64{25, 0, 0}}); ok {
		t.Fatal("MatchSmoke ok = true with a throw far from the lineup, want false")
	}
}
