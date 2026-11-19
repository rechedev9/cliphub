package httpapi

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/parser"
)

const cardPlayer = "76561198000000001"

func playerCardJob(t *testing.T, repo *fakeRepo, files *fakeStorage, multi bool) job.Job {
	t.Helper()
	j := job.Job{ID: uuid.New(), TargetSteamID: cardPlayer, Status: job.StatusParsed}
	if multi {
		j.MultiPOV = &job.MultiPOV{SteamIDs: []string{cardPlayer, "76561198000000002"}}
	}
	repo.jobs[j.ID] = j
	roster, err := json.Marshal(parser.RosterResult{
		Players: []parser.PlayerStat{{SteamID64: cardPlayer, Name: "donk", Team: "CT", Kills: 24, Deaths: 10, Rounds: 20, ADR: 112.4, KAST: 80, HSPct: 54}},
		Match:   parser.MatchInfo{Map: "de_mirage", ScoreCT: 13, ScoreT: 7, ClanNameCT: "Spirit"},
	})
	if err != nil {
		t.Fatal(err)
	}
	files.puts[artifacts.RosterKey(j.ID)] = roster
	return j
}

type cardInfo struct {
	Templates []struct{ ID string } `json:"templates"`
	Available bool                  `json:"available"`
	Reason    string                `json:"reason"`
	Player    struct {
		SteamID64 string `json:"steamid64"`
		Name      string `json:"name"`
		Image     string `json:"image"`
	} `json:"player"`
}

func cardRequest(t *testing.T, router http.Handler, method, path string, body []byte, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rw := httptest.NewRecorder()
	router.ServeHTTP(rw, req)
	return rw
}

func readCardInfo(t *testing.T, router http.Handler, j job.Job) cardInfo {
	t.Helper()
	rw := cardRequest(t, router, http.MethodGet, "/api/jobs/"+j.ID.String()+"/player-card", nil, "")
	if rw.Code != http.StatusOK {
		t.Fatalf("player card info: %d %s", rw.Code, rw.Body)
	}
	var info cardInfo
	if err := json.Unmarshal(rw.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	return info
}

func imageForm(t *testing.T, data []byte) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("image", "player.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(data)
	_ = form.Close()
	return body.Bytes(), form.FormDataContentType()
}

func TestPlayerCardImageIsRememberedForThePlayer(t *testing.T) {
	repo, files := newFakeRepo(), newFakeStorage()
	router := Routes(NewHandlers(repo, files, &fakeQueue{}))
	first, second := playerCardJob(t, repo, files, false), playerCardJob(t, repo, files, false)

	info := readCardInfo(t, router, first)
	if !info.Available || len(info.Templates) != 3 || info.Player.Name != "donk" || info.Player.SteamID64 != cardPlayer || info.Player.Image != "" {
		t.Fatalf("info before an upload: %+v", info)
	}
	imagePath := "/api/jobs/" + first.ID.String() + "/player-card/image"
	body, contentType := imageForm(t, []byte("not an image"))
	if rw := cardRequest(t, router, http.MethodPost, imagePath, body, contentType); rw.Code != http.StatusBadRequest {
		t.Fatalf("non-image upload = %d %s", rw.Code, rw.Body)
	}
	var portrait bytes.Buffer
	if err := png.Encode(&portrait, image.NewNRGBA(image.Rect(0, 0, 40, 40))); err != nil {
		t.Fatal(err)
	}
	body, contentType = imageForm(t, portrait.Bytes())
	req := httptest.NewRequest(http.MethodPost, imagePath, bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	if !isMultipartUpload(req) {
		t.Fatal("the player image bypasses the upload limiter")
	}
	if rw := cardRequest(t, router, http.MethodPost, imagePath, body, contentType); rw.Code != http.StatusOK {
		t.Fatalf("upload = %d %s", rw.Code, rw.Body)
	}
	if got := readCardInfo(t, router, second).Player.Image; got != "upload" {
		t.Fatalf("another match of the same player shows image %q, want the upload", got)
	}

	for _, template := range []string{"carta", "ficha", "mvp"} {
		rw := cardRequest(t, router, http.MethodGet, "/api/jobs/"+second.ID.String()+"/player-card/preview?template="+template, nil, "")
		if rw.Code != http.StatusOK || rw.Header().Get("Content-Type") != "image/svg+xml" || rw.Header().Get("Content-Security-Policy") == "" {
			t.Fatalf("%s preview = %d %s", template, rw.Code, rw.Header())
		}
		decoder := xml.NewDecoder(strings.NewReader(rw.Body.String()))
		for {
			if _, err := decoder.Token(); errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				t.Fatalf("%s preview is not an image the browser can parse: %v", template, err)
			}
		}
		for _, want := range []string{"donk", "data:image/png;base64,"} {
			if !strings.Contains(rw.Body.String(), want) {
				t.Errorf("%s preview does not show %q", template, want)
			}
		}
	}
	if rw := cardRequest(t, router, http.MethodGet, "/api/jobs/"+second.ID.String()+"/player-card/preview?template=fifa", nil, ""); rw.Code != http.StatusBadRequest {
		t.Fatalf("unknown template preview = %d", rw.Code)
	}

	if rw := cardRequest(t, router, http.MethodDelete, imagePath, nil, ""); rw.Code != http.StatusNoContent {
		t.Fatalf("remove = %d %s", rw.Code, rw.Body)
	}
	if got := readCardInfo(t, router, second).Player.Image; got != "" {
		t.Fatalf("image after removal = %q, want the initials", got)
	}
}

func TestPlayerCardsAreOffForAMultiPlayerShort(t *testing.T) {
	repo, files := newFakeRepo(), newFakeStorage()
	router := Routes(NewHandlers(repo, files, &fakeQueue{}))
	j := playerCardJob(t, repo, files, true)
	info := readCardInfo(t, router, j)
	if info.Available || info.Reason == "" || len(info.Templates) != 3 {
		t.Fatalf("multi-player info: %+v", info)
	}
	body, contentType := imageForm(t, []byte("x"))
	if rw := cardRequest(t, router, http.MethodPost, "/api/jobs/"+j.ID.String()+"/player-card/image", body, contentType); rw.Code != http.StatusConflict {
		t.Fatalf("multi-player upload = %d, want 409", rw.Code)
	}
}

// Opening the card panel asks for the card info and the three previews at once; the player is gathered
// once for all of them, and a new image shows on the next read.
func TestPlayerCardPanelGathersThePlayerOnce(t *testing.T) {
	repo, files := newFakeRepo(), newFakeStorage()
	router := Routes(NewHandlers(repo, files, &fakeQueue{}))
	j := playerCardJob(t, repo, files, false)
	base := "/api/jobs/" + j.ID.String() + "/player-card"
	var wg sync.WaitGroup
	for _, path := range []string{base, base + "/preview?template=carta", base + "/preview?template=ficha", base + "/preview?template=mvp"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rw := cardRequest(t, router, http.MethodGet, path, nil, ""); rw.Code != http.StatusOK {
				t.Errorf("%s = %d %s", path, rw.Code, rw.Body)
			}
		}()
	}
	wg.Wait()
	if got := files.openCount(artifacts.RosterKey(j.ID)); got != 1 {
		t.Fatalf("the panel read the roster %d times, want once", got)
	}
	var portrait bytes.Buffer
	if err := png.Encode(&portrait, image.NewNRGBA(image.Rect(0, 0, 40, 40))); err != nil {
		t.Fatal(err)
	}
	body, contentType := imageForm(t, portrait.Bytes())
	if rw := cardRequest(t, router, http.MethodPost, base+"/image", body, contentType); rw.Code != http.StatusOK {
		t.Fatalf("upload = %d %s", rw.Code, rw.Body)
	}
	if got := readCardInfo(t, router, j).Player.Image; got != "upload" {
		t.Fatalf("image after an upload = %q, want the upload", got)
	}
}
