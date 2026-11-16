package workers

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/tasks"
)

func TestClaimLibraryRetryAfterUpgrade(t *testing.T) {
	failed := func(code, message string) library.Demo {
		return library.Demo{ID: uuid.New(), State: library.ParseFailed, ErrorCode: code, Error: message}
	}
	for _, tc := range []struct {
		name string
		demo library.Demo
		want bool
	}{
		{"incompatible", failed(obs.ClassDemoIncompatible, "parsing demo: demo_incompatible: nextByte"), true},
		{"studio bug", failed(tacticalClassInternal, "encode positions: slot 1 appears twice"), true},
		{"uncalibrated map", failed(tacticalClassMapUncalibrated, "map \"de_new\" has no usable radar calibration"), true},
		{"incompatible stored as unreadable by an older build", failed(tacticalClassDemoUnreadable, "scan tactical: parsing demo: demo_incompatible: nextByte: EOF"), true},
		{"truncated", failed(obs.ClassTruncatedDemo, "truncated_demo: cut before the first round"), false},
		{"unreadable file", failed(tacticalClassDemoUnreadable, "open demo: Access is denied."), false},
		{"pov", failed(obs.ClassPOVDemo, "pov_demo"), false},
		{"not failed", library.Demo{ID: uuid.New(), State: library.ParseReady, ErrorCode: obs.ClassDemoIncompatible}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := newFakeStorage()
			files.files[artifacts.LibraryScanFailedBuildKey(tc.demo.ID)] = []byte("an-older-build")
			if got := ClaimLibraryRetryAfterUpgrade(files, tc.demo); got != tc.want {
				t.Fatalf("first claim = %v, want %v", got, tc.want)
			}
			if ClaimLibraryRetryAfterUpgrade(files, tc.demo) {
				t.Fatal("second claim on the same build = true, want one retry per build")
			}
		})
	}
}

// An incompatible demo waits for another parser version; the other classes wait for another build.
func TestClaimLibraryRetryAfterUpgradeKeysEachClassOnWhatReadsIt(t *testing.T) {
	current := currentScanIdentity()
	newBuild := scanIdentity{parser: current.parser, build: current.build + "-older"}
	newParser := scanIdentity{parser: current.parser + "-older", build: current.build}
	for _, tc := range []struct {
		name     string
		code     string
		message  string
		recorded scanIdentity
		want     bool
	}{
		{"studio bug, same build", tacticalClassInternal, "encode positions: slot 1 appears twice", current, false},
		{"studio bug, new build", tacticalClassInternal, "encode positions: slot 1 appears twice", newBuild, true},
		{"uncalibrated map, new build", tacticalClassMapUncalibrated, "no calibration", newBuild, true},
		{"incompatible, new build with the same parser", obs.ClassDemoIncompatible, "demo_incompatible: nextByte", newBuild, false},
		{"incompatible, new parser", obs.ClassDemoIncompatible, "demo_incompatible: nextByte", newParser, true},
		{"older unreadable incompatible, same parser", tacticalClassDemoUnreadable, "parsing demo: demo_incompatible: nextByte", newBuild, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := library.Demo{ID: uuid.New(), State: library.ParseFailed, ErrorCode: tc.code, Error: tc.message}
			files := newFakeStorage()
			files.files[artifacts.LibraryScanFailedBuildKey(d.ID)] = []byte(tc.recorded.String())
			if got := ClaimLibraryRetryAfterUpgrade(files, d); got != tc.want {
				t.Fatalf("claim after %q = %v, want %v", tc.recorded, got, tc.want)
			}
		})
	}
}

// A failure the running build wrote is not retried at the next start of that build.
func TestLibraryWorkerRecordsTheBuildThatFailedADemo(t *testing.T) {
	repo := store.NewMemoryLibraryRepository()
	files := newFakeStorage()
	d := importLibraryDemo(t, repo, files, "incompatible.dem", incompatibleDemo())
	_ = NewLibraryWorker(repo, files).ProcessAnalyzeLibraryDemo(tasks.WithTaskAttempt(context.Background(), 0, 1), d.ID)

	got, _ := repo.GetDemo(context.Background(), d.ID)
	want := currentScanIdentity().String()
	if got.State != library.ParseFailed || string(files.files[artifacts.LibraryScanFailedBuildKey(d.ID)]) != want {
		t.Fatalf("demo = %+v, recorded %q; want failed by %q", got, files.files[artifacts.LibraryScanFailedBuildKey(d.ID)], want)
	}
	if ClaimLibraryRetryAfterUpgrade(files, got) {
		t.Fatal("a demo the running build failed was claimed for a retry")
	}
}
