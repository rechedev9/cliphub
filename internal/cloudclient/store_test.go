package cloudclient

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestStoreSurvivesARestartAndKeepsTheTokenPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cloud", "client.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore of a missing file error = %v", err)
	}
	if store.Token() != "" || store.User() != nil || len(store.Submissions()) != 0 {
		t.Fatal("a new store is not empty")
	}
	if err := store.Link(testDeviceToken, StoredUser{Name: "Luis", Email: "luis@example.com"}); err != nil {
		t.Fatalf("Link error = %v", err)
	}
	older := Submission{CloudJobID: "job-old", LocalJobID: "local-1", Title: "Old", Kind: "short", CreatedAt: time.Unix(1_760_000_000, 0).UTC(), DemoUploaded: true}
	newer := Submission{CloudJobID: "job-new", LocalJobID: "local-2", Title: "New", Kind: "short", CreatedAt: time.Unix(1_760_000_600, 0).UTC()}
	for _, submission := range []Submission{older, newer} {
		if err := store.Add(submission); err != nil {
			t.Fatalf("Add error = %v", err)
		}
	}
	if found, err := store.Update("job-new", func(entry *Submission) {
		entry.Videos = append(entry.Videos, StoredVideo{ArtifactID: "a-1", Name: "seg-001.mp4", Ready: true})
	}); err != nil || !found {
		t.Fatalf("Update = %v, %v", found, err)
	}
	if found, err := store.Update("job-missing", func(*Submission) { t.Fatal("mutated a missing submission") }); err != nil || found {
		t.Fatalf("Update of a missing submission = %v, %v, want not found", found, err)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Fatalf("client.json mode = %o, want 600: it holds the device token", mode)
		}
	}

	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	if reopened.Token() != testDeviceToken || reopened.User().Email != "luis@example.com" {
		t.Fatalf("reopened store lost the link: token set=%v user=%+v", reopened.Token() != "", reopened.User())
	}
	submissions := reopened.Submissions()
	if len(submissions) != 2 || submissions[0].CloudJobID != "job-new" || submissions[1].CloudJobID != "job-old" {
		t.Fatalf("submissions = %+v, want both, newest first", submissions)
	}
	if len(submissions[0].Videos) != 1 || !submissions[0].Videos[0].Ready || !submissions[1].DemoUploaded {
		t.Fatalf("submissions lost detail: %+v", submissions)
	}

	// Unlinking forgets the account but keeps the jobs and their videos.
	if err := reopened.Unlink(); err != nil {
		t.Fatalf("Unlink error = %v", err)
	}
	if err := reopened.Remove("job-old"); err != nil {
		t.Fatalf("Remove error = %v", err)
	}
	final, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if final.Token() != "" || final.User() != nil || len(final.Submissions()) != 1 {
		t.Fatalf("after unlink and remove: token set=%v user=%+v submissions=%d", final.Token() != "", final.User(), len(final.Submissions()))
	}
}

func TestOpenStoreRefusesAFileFromANewerStudio(t *testing.T) {
	path := filepath.Join(t.TempDir(), "client.json")
	const future = `{"schema_version":"cliphub.cloud-client.v9","device_token":"x"}`
	if err := os.WriteFile(path, []byte(future), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenStore(path); err == nil {
		t.Fatal("OpenStore of a newer schema error = nil, want it refused instead of silently dropping the link")
	}
	if kept, err := os.ReadFile(path); err != nil || string(kept) != future {
		t.Fatalf("the newer file was touched (err %v)", err)
	}
}

func TestOpenStoreStartsOverFromADamagedFileAndKeepsItAside(t *testing.T) {
	for name, content := range map[string]string{
		"empty":     "",
		"cut short": `{"schema_version":"cliphub.cloud-client.v1","device_token":"chd_ab`,
		"zeroed":    "\x00\x00\x00\x00\x00\x00\x00\x00",
		"not ours":  `["a list"]`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cloud", "client.json")
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			store, err := OpenStore(path)
			if err != nil {
				t.Fatalf("OpenStore error = %v, want an empty store: a damaged file must not keep the cloud off for good", err)
			}
			if store.Token() != "" || store.User() != nil || len(store.Submissions()) != 0 {
				t.Fatal("the store opened from a damaged file is not empty")
			}
			if kept, err := os.ReadFile(path + ".damaged"); err != nil || string(kept) != content {
				t.Fatalf("the damaged file was not kept aside untouched (err %v)", err)
			}
			// The PC can be linked again, and that survives the next start.
			if err := store.Link(testDeviceToken, StoredUser{Name: "Luis", Email: "luis@example.com"}); err != nil {
				t.Fatalf("Link after the recovery error = %v", err)
			}
			reopened, err := OpenStore(path)
			if err != nil || reopened.Token() != testDeviceToken {
				t.Fatalf("reopened store: token set=%v err=%v, want the new link", reopened != nil && reopened.Token() != "", err)
			}
		})
	}
}

func TestAbandonedJobsSurviveARestartUntilTheyAreSettled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cloud", "client.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	since := time.Unix(1_760_000_000, 0).UTC()
	for _, id := range []string{"job-a", "job-b", "job-a"} {
		if err := store.Abandon(id, since); err != nil {
			t.Fatalf("Abandon error = %v", err)
		}
	}
	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	abandoned := reopened.Abandoned()
	if len(abandoned) != 2 || abandoned[0] != (AbandonedJob{CloudJobID: "job-a", Since: since}) || !reopened.IsAbandoned("job-b") {
		t.Fatalf("abandoned after a restart = %+v, want job-a and job-b once each", abandoned)
	}
	if err := reopened.Settle("job-a"); err != nil {
		t.Fatalf("Settle error = %v", err)
	}
	if reopened.IsAbandoned("job-a") || !reopened.IsAbandoned("job-b") {
		t.Fatalf("abandoned after settling job-a = %+v", reopened.Abandoned())
	}
}
