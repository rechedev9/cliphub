package httpapi

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"unicode"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/rules"
)

const cloudDemoBody = "PBDEMS2\x00rest-of-demo"

// A cloud demo's name comes from a stranger through the portal, so it is
// stored under the same rules as the name of a manual upload.
func TestAdmitCloudDemoStoresASafeDisplayName(t *testing.T) {
	cases := []struct {
		name     string
		fileName string
		want     string
	}{
		{
			name:     "the handle the cloud worker builds is kept",
			fileName: "cloud-5fa7627c-Luis-match.dem",
			want:     "cloud-5fa7627c-Luis-match.dem",
		},
		{
			// U+202E (right-to-left override) and U+200B (zero width space).
			name:     "control and invisible format characters are dropped",
			fileName: "cloud-5fa7627c-mallory-ma\xe2\x80\xaetch\xe2\x80\x8b\x00\n.dem",
			want:     "cloud-5fa7627c-mallory-match.dem",
		},
		{
			name:     "a path keeps only its last element",
			fileName: `cloud-5fa7627c-mallory-..\..\Windows/System32/evil.dem`,
			want:     "evil.dem",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{})
			created, err := h.AdmitCloudDemo(context.Background(), CloudDemoAdmission{
				Demo:           strings.NewReader(cloudDemoBody),
				FileName:       tc.fileName,
				CloudRequestID: "5fa7627c-a94f-4855-9bbe-6af9997e6f3e",
				TargetSteamID:  "76561198000000001",
				Rules:          rules.Default(),
			})
			if err != nil {
				t.Fatalf("AdmitCloudDemo error = %v", err)
			}
			if created.DemoFileName != tc.want {
				t.Fatalf("DemoFileName = %q, want %q", created.DemoFileName, tc.want)
			}
			for _, r := range created.DemoFileName {
				if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '/' || r == '\\' {
					t.Fatalf("DemoFileName %q still carries %q", created.DemoFileName, r)
				}
			}
		})
	}
}

// cancelAwareRepo refuses to create a job for a request that already ended,
// as the SQLite repository does.
type cancelAwareRepo struct {
	*fakeRepo
}

func (r cancelAwareRepo) Create(ctx context.Context, j *job.Job) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.fakeRepo.Create(ctx, j)
}

// A cancel that lands while the demo is being stored leaves no job row, so
// the stored demo must go too: nothing could ever find it again.
func TestAdmitCloudDemoRemovesTheStoredDemoWhenTheJobIsNotCreated(t *testing.T) {
	store := newFakeStorage()
	repo := newFakeRepo()
	h := NewHandlers(cancelAwareRepo{fakeRepo: repo}, store, &fakeQueue{})
	ctx, cancel := context.WithCancel(context.Background())
	// The portal took the job back while the demo was still being copied.
	demo := &cancelOnEOF{reader: bytes.NewReader([]byte(cloudDemoBody)), cancel: cancel}

	_, err := h.AdmitCloudDemo(ctx, CloudDemoAdmission{
		Demo:           demo,
		FileName:       "cloud-5fa7627c-Luis-match.dem",
		CloudRequestID: "5fa7627c-a94f-4855-9bbe-6af9997e6f3e",
		TargetSteamID:  "76561198000000001",
		Rules:          rules.Default(),
	})
	if err == nil || !strings.Contains(err.Error(), "create job") {
		t.Fatalf("AdmitCloudDemo error = %v, want the job creation to fail", err)
	}
	if len(repo.jobs) != 0 {
		t.Fatalf("jobs created = %d, want none", len(repo.jobs))
	}
	if len(store.puts) != 0 {
		t.Fatalf("stored demos left without a job = %d, want none (deleted: %v)", len(store.puts), store.deleted)
	}
	if len(store.deleted) != 1 || !strings.HasPrefix(store.deleted[0], "demos/") {
		t.Fatalf("deleted keys = %v, want the one stored demo", store.deleted)
	}
}

// cancelOnEOF cancels a context once its reader has been read to the end.
type cancelOnEOF struct {
	reader *bytes.Reader
	cancel context.CancelFunc
}

func (r *cancelOnEOF) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if err != nil {
		r.cancel()
	}
	return n, err
}
