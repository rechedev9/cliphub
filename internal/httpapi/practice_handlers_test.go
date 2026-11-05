package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/practice"
)

type fakePracticeGame struct {
	dir      string
	dirErr   error
	running  bool
	launched []string
}

func (g *fakePracticeGame) CfgDir() (string, error) { return g.dir, g.dirErr }
func (g *fakePracticeGame) Running() (bool, error)  { return g.running, nil }
func (g *fakePracticeGame) Launch(m string) error {
	g.launched = append(g.launched, m)
	return nil
}

const practiceBody = `{"map":"de_ancient","spots":[
 {"label":"Cueva\"; quit","pos":[-500,1200.5,64],"ang":[-5,90],"grenade":"molotov"},
 {"label":"Medio","pos":[0,0,0],"ang":[0,180]}]}`

func postPractice(t *testing.T, game practice.Game, body string) *httptest.ResponseRecorder {
	t.Helper()
	var opts []Option
	if game != nil {
		opts = append(opts, WithPractice(game))
	}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, opts...)
	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/practice/launch", strings.NewReader(body)))
	return rw
}

func TestLaunchPracticeWritesCfgAndLaunches(t *testing.T) {
	game := &fakePracticeGame{dir: t.TempDir()}
	rw := postPractice(t, game, practiceBody)
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rw.Code, rw.Body)
	}
	var got practiceLaunchResponse
	if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Launched || got.AlreadyRunning || len(game.launched) != 1 || game.launched[0] != "de_ancient" {
		t.Fatalf("response %+v, launches %v", got, game.launched)
	}
	if got.CfgPath != filepath.Join(game.dir, practice.CfgName) || got.ExecCommand != "exec cliphub_practice" {
		t.Fatalf("cfg_path %q exec %q", got.CfgPath, got.ExecCommand)
	}
	if len(got.Spots) != 2 || got.Spots[0].Label != "Cueva quit" || got.Spots[0].Alias != "cliphub_spot_1" ||
		got.Spots[0].Key != "KP_END" || got.Spots[0].Grenade != "molotov" || got.Spots[1].Pos != [3]float64{} {
		t.Fatalf("spots = %+v", got.Spots)
	}
	cfg, err := os.ReadFile(got.CfgPath)
	if err != nil || !strings.Contains(string(cfg), "give weapon_molotov; echo ClipHub 1/2: Cueva quit\"") {
		t.Fatalf("cfg not written with the sanitized label: %v\n%s", err, cfg)
	}
}

func TestLaunchPracticeDoesNotRelaunchRunningCS2(t *testing.T) {
	game := &fakePracticeGame{dir: t.TempDir(), running: true}
	rw := postPractice(t, game, practiceBody)
	var got practiceLaunchResponse
	if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil || rw.Code != http.StatusOK {
		t.Fatalf("status %d err %v", rw.Code, err)
	}
	if got.Launched || !got.AlreadyRunning || len(game.launched) != 0 {
		t.Fatalf("response %+v, launches %v", got, game.launched)
	}
}

func TestLaunchPracticeErrors(t *testing.T) {
	cases := []struct {
		name   string
		game   practice.Game
		body   string
		status int
		code   string
	}{
		{"bad map", &fakePracticeGame{}, `{"map":"de_x;quit","spots":[{"label":"a","pos":[0,0,0],"ang":[0,0]}]}`, 400, codeInvalidPractice},
		{"unknown field", &fakePracticeGame{}, `{"map":"de_nuke","spots":[],"cmd":"quit"}`, 400, codeInvalidPractice},
		{"string number", &fakePracticeGame{}, `{"map":"de_nuke","spots":[{"pos":["1",0,0],"ang":[0,0]}]}`, 400, codeInvalidPractice},
		{"no game", nil, practiceBody, 503, codePracticeUnavailable},
		{"no CS2", &fakePracticeGame{dirErr: practice.ErrCS2NotFound}, practiceBody, 503, codeCS2NotFound},
		{"cfg dir failure", &fakePracticeGame{dirErr: errors.New("disk")}, practiceBody, 500, "internal_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rw := postPractice(t, tc.game, tc.body)
			var body struct {
				Code string `json:"code"`
			}
			_ = json.Unmarshal(rw.Body.Bytes(), &body)
			if rw.Code != tc.status || body.Code != tc.code {
				t.Fatalf("status %d code %q, want %d %q (%s)", rw.Code, body.Code, tc.status, tc.code, rw.Body)
			}
		})
	}
}
