package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/rechedev9/cliphub/internal/library"
)

func TestLibraryCollectionsLifecycle(t *testing.T) {
	env := newLibraryTestEnv(t)
	m := env.seedLibraryMatch("collections", "Spirit", "MOUZ")
	body := fmt.Sprintf(`{"name":" Clutches ","items":[{"match_id":%q,"round":1,"tick":64,"note":"1v2"}]}`, m.ID)
	rw := env.do(http.MethodPost, "/api/library/collections", []byte(body), "application/json")
	if rw.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", rw.Code, rw.Body)
	}
	var created library.Collection
	if err := json.Unmarshal(rw.Body.Bytes(), &created); err != nil || created.Name != "Clutches" || len(created.Items) != 1 {
		t.Fatalf("created = %+v, %v", created, err)
	}

	unknown := `{"name":"x","items":[{"match_id":"00000000-0000-0000-0000-000000000001","round":1,"tick":1}]}`
	if rw := env.do(http.MethodPost, "/api/library/collections", []byte(unknown), "application/json"); rw.Code == http.StatusCreated {
		t.Fatalf("create with an unknown match = %d, want an error", rw.Code)
	}
	badRound := fmt.Sprintf(`{"items":[{"match_id":%q,"round":99,"tick":1}]}`, m.ID)
	path := "/api/library/collections/" + created.ID.String()
	if rw := env.do(http.MethodPatch, path, []byte(badRound), "application/json"); rw.Code == http.StatusOK {
		t.Fatal("patch with a round out of the match was accepted")
	}
	if rw := env.do(http.MethodPatch, path, []byte(`{"name":"Clutches de Spirit"}`), "application/json"); rw.Code != http.StatusOK {
		t.Fatalf("rename = %d %s", rw.Code, rw.Body)
	}

	rw = env.do(http.MethodGet, path, nil, "")
	var detail collectionResponse
	if err := json.Unmarshal(rw.Body.Bytes(), &detail); err != nil || detail.Name != "Clutches de Spirit" || len(detail.MissingMatchIDs) != 0 {
		t.Fatalf("get = %d %s", rw.Code, rw.Body)
	}
	if rw := env.do(http.MethodGet, "/api/library/collections", nil, ""); rw.Code != http.StatusOK {
		t.Fatalf("list = %d", rw.Code)
	}
	if rw := env.do(http.MethodDelete, path, nil, ""); rw.Code != http.StatusNoContent {
		t.Fatalf("delete = %d", rw.Code)
	}
	if rw := env.do(http.MethodGet, path, nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("get after delete = %d", rw.Code)
	}
}
