package httpapi

import (
	"fmt"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/mapgeo"
	"github.com/rechedev9/cliphub/internal/storage"
)

func TestPruneStaleScenesKeepsOnlyTheCurrentBuild(t *testing.T) {
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	current := fmt.Sprintf("mapgeo/scene/v%d/de_nuke-b2.chms", mapgeo.SceneFileVersion)
	old := fmt.Sprintf("mapgeo/scene/v%d/de_nuke-b1.chms", mapgeo.SceneFileVersion-1)
	keys := []string{current, old,
		fmt.Sprintf("mapgeo/scene/v%d/de_nuke-b1.chms", mapgeo.SceneFileVersion),
		fmt.Sprintf("mapgeo/scene/v%d/de_nuke_old-b1.chms", mapgeo.SceneFileVersion),
		fmt.Sprintf("mapgeo/scene/v%d/de_dust2-b1.chms", mapgeo.SceneFileVersion)}
	for _, key := range keys {
		if err := store.Put(key, strings.NewReader("x")); err != nil {
			t.Fatal(err)
		}
	}
	(&Handlers{storage: store}).pruneStaleScenes("de_nuke", current)
	want := map[string]bool{keys[0]: true, keys[1]: false, keys[2]: false, keys[3]: true, keys[4]: true}
	for key, kept := range want {
		if ok, _ := store.Exists(key); ok != kept {
			t.Errorf("%s exists = %v, want %v", key, ok, kept)
		}
	}
}
