package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

func TestOldRenderGalleryKeepsServingItsRevisionAfterCurrentSwap(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := job.Job{ID: uuid.New(), Status: job.StatusDone, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	const segmentID = "seg-001"
	variant := editor.PresetViral60Clean
	oldRevision := uuid.New()
	newRevision := uuid.New()

	oldBase := fmt.Sprintf(
		"/api/jobs/%s/renders/%s/revisions/%s",
		j.ID,
		variant,
		oldRevision,
	)
	oldGallery := fmt.Sprintf(
		`<!doctype html><video src="%s/videos/%s"></video><img src="%s/covers/%s"><a href="%s/captions/%s">Caption</a>`,
		oldBase,
		segmentID,
		oldBase,
		segmentID,
		oldBase,
		segmentID,
	)
	oldGalleryRef, err := renderplan.NewRenderVariantRevisionArtifactRef(
		j.ID,
		variant,
		oldRevision,
		renderplan.RenderVariantArtifactGallery,
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(oldGalleryRef.Key, bytes.NewBufferString(oldGallery)); err != nil {
		t.Fatal(err)
	}

	for _, artifact := range []struct {
		kind renderplan.RenderVariantArtifactKind
		body string
	}{
		{kind: renderplan.RenderVariantArtifactVideo, body: "old-video"},
		{kind: renderplan.RenderVariantArtifactCover, body: "old-cover"},
		{kind: renderplan.RenderVariantArtifactCaption, body: "old-caption"},
	} {
		oldRef, refErr := renderplan.NewRenderVariantRevisionArtifactRef(
			j.ID,
			variant,
			oldRevision,
			artifact.kind,
			segmentID,
		)
		if refErr != nil {
			t.Fatal(refErr)
		}
		if err := store.Put(oldRef.Key, bytes.NewBufferString(artifact.body)); err != nil {
			t.Fatal(err)
		}
		newRef, refErr := renderplan.NewRenderVariantRevisionArtifactRef(
			j.ID,
			variant,
			newRevision,
			artifact.kind,
			segmentID,
		)
		if refErr != nil {
			t.Fatal(refErr)
		}
		if err := store.Put(newRef.Key, bytes.NewBufferString("new-"+artifact.body[4:])); err != nil {
			t.Fatal(err)
		}
	}

	loadout, err := renderplan.LoadoutForVariant(variant)
	if err != nil {
		t.Fatal(err)
	}
	current, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:      j.ID,
		Loadout:    loadout,
		Status:     renderplan.RenderVariantStatusReady,
		RevisionID: newRevision,
	})
	if err != nil {
		t.Fatal(err)
	}
	stateKey, err := renderplan.RenderVariantStateKey(j.ID, variant)
	if err != nil {
		t.Fatal(err)
	}
	stateJSON, err := json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(stateKey, bytes.NewReader(stateJSON)); err != nil {
		t.Fatal(err)
	}

	router := Routes(NewHandlers(repo, store, &fakeQueue{}))
	galleryRequest := httptest.NewRequest(http.MethodGet, oldBase+"/gallery", nil)
	galleryResponse := httptest.NewRecorder()
	router.ServeHTTP(galleryResponse, galleryRequest)
	if galleryResponse.Code != http.StatusOK {
		t.Fatalf("old gallery status = %d, want 200; body=%s", galleryResponse.Code, galleryResponse.Body.String())
	}

	urlPattern := regexp.MustCompile(`(?:src|href)="([^"]+)"`)
	matches := urlPattern.FindAllStringSubmatch(galleryResponse.Body.String(), -1)
	if len(matches) != 3 {
		t.Fatalf("old gallery artifact URLs = %d, want 3: %s", len(matches), galleryResponse.Body.String())
	}
	for i, match := range matches {
		artifactRequest := httptest.NewRequest(http.MethodGet, match[1], nil)
		artifactResponse := httptest.NewRecorder()
		router.ServeHTTP(artifactResponse, artifactRequest)
		if artifactResponse.Code != http.StatusOK {
			t.Fatalf("old gallery artifact %d (%s) status = %d, want 200; body=%s", i, match[1], artifactResponse.Code, artifactResponse.Body.String())
		}
		if got := artifactResponse.Body.String(); got != []string{"old-video", "old-cover", "old-caption"}[i] {
			t.Fatalf("old gallery artifact %d (%s) = %q", i, match[1], got)
		}
	}

	currentRequest := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/api/jobs/%s/renders/%s/videos/%s", j.ID, variant, segmentID),
		nil,
	)
	currentResponse := httptest.NewRecorder()
	router.ServeHTTP(currentResponse, currentRequest)
	if currentResponse.Code != http.StatusOK || currentResponse.Body.String() != "new-video" {
		t.Fatalf("current video response = %d %q, want 200 new-video", currentResponse.Code, currentResponse.Body.String())
	}
}

func TestRenderRevisionRoutesRejectUnsafeIdentifiers(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := job.Job{ID: uuid.New(), Status: job.StatusDone, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	router := Routes(NewHandlers(repo, store, &fakeQueue{}))

	for _, path := range []string{
		fmt.Sprintf("/api/jobs/%s/renders/%s/revisions/not-a-uuid/gallery", j.ID, editor.PresetViral60Clean),
		fmt.Sprintf("/api/jobs/%s/renders/%s/revisions/%s/videos/..", j.ID, editor.PresetViral60Clean, uuid.New()),
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400; body=%s", path, response.Code, response.Body.String())
		}
	}
}
