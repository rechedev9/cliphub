package cloudbridge

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

// claimedDemo puts one job in the fake portal's running state, as a first
// claim would, and returns that attempt and its demo reference.
func claimedDemo(t *testing.T, portal *fakePortal) (jobRef, demoRef) {
	t.Helper()
	job := jobRef{ID: uuid.NewString(), Attempt: 1}
	var ref demoRef
	portal.with(func() {
		portal.jobs[job.ID] = &portalJob{job: &workerJob{ID: job.ID, Attempt: job.Attempt}, status: phaseRunning}
		ref = demoRef{SHA256: sha256Hex(portal.demo), SizeBytes: int64(len(portal.demo)), FileName: "match.dem"}
	})
	return job, ref
}

func TestDownloadDemoResumesAPartialFile(t *testing.T) {
	portal := newFakePortal(t)
	id, ref := claimedDemo(t, portal)
	dest := filepath.Join(t.TempDir(), "incoming", id.ID+".dem")
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		t.Fatal(err)
	}
	// A previous attempt got the first 100 bytes before the connection died.
	if err := os.WriteFile(dest, portal.demo[:100], 0o600); err != nil {
		t.Fatal(err)
	}
	if err := newClient(portal.server.URL, testToken).downloadDemo(context.Background(), id, dest, ref); err != nil {
		t.Fatalf("downloadDemo error = %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, portal.demo) {
		t.Fatalf("downloaded %d bytes that differ from the portal's %d", len(got), len(portal.demo))
	}
	portal.with(func() {
		if len(portal.demoRanges) != 1 || portal.demoRanges[0] != "bytes=100-" {
			t.Fatalf("Range headers = %q, want one resume from byte 100", portal.demoRanges)
		}
	})

	// A complete file needs no request at all.
	if err := newClient(portal.server.URL, testToken).downloadDemo(context.Background(), id, dest, ref); err != nil {
		t.Fatalf("downloadDemo of a complete file error = %v", err)
	}
	portal.with(func() {
		if len(portal.demoRanges) != 1 {
			t.Fatalf("requests for an already complete demo = %d, want none", len(portal.demoRanges)-1)
		}
	})
}

func TestDownloadDemoDiscardsBytesThatDoNotMatchTheClaim(t *testing.T) {
	portal := newFakePortal(t)
	id, ref := claimedDemo(t, portal)
	dest := filepath.Join(t.TempDir(), id.ID+".dem")
	// The partial file has the right length so far, but the wrong content.
	if err := os.WriteFile(dest, bytes.Repeat([]byte("x"), 100), 0o600); err != nil {
		t.Fatal(err)
	}

	err := newClient(portal.server.URL, testToken).downloadDemo(context.Background(), id, dest, ref)
	if !errors.Is(err, errDemoCorrupt) {
		t.Fatalf("downloadDemo error = %v, want errDemoCorrupt", err)
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Fatalf("the corrupt file is still there (stat error %v), so a retry would resume it", statErr)
	}
	// The retry starts from zero and succeeds.
	if err := newClient(portal.server.URL, testToken).downloadDemo(context.Background(), id, dest, ref); err != nil {
		t.Fatalf("downloadDemo retry error = %v", err)
	}
}

func TestClientTellsARejectedTokenAndALostLeaseApart(t *testing.T) {
	portal := newFakePortal(t)
	id, ref := claimedDemo(t, portal)
	api := newClient(portal.server.URL, testToken)
	dest := filepath.Join(t.TempDir(), id.ID+".dem")

	portal.with(func() { portal.jobs[id.ID].leaseLost = true })
	if _, err := api.fail(context.Background(), id, failRequest{Code: codeInternal}); !errors.Is(err, errLeaseLost) {
		t.Fatalf("fail on a lost lease error = %v, want errLeaseLost", err)
	}
	if err := api.downloadDemo(context.Background(), id, dest, ref); !errors.Is(err, errLeaseLost) {
		t.Fatalf("downloadDemo on a lost lease error = %v, want errLeaseLost", err)
	}

	portal.with(func() { portal.unauthorized = true })
	if _, _, err := api.claim(context.Background(), claimRequest{Kinds: []string{kindShort}}); !errors.Is(err, errUnauthorized) {
		t.Fatalf("claim with a rejected token error = %v, want errUnauthorized", err)
	}
	if _, err := api.heartbeat(context.Background(), heartbeatRequest{State: stateIdle}); !errors.Is(err, errUnauthorized) {
		t.Fatalf("heartbeat with a rejected token error = %v, want errUnauthorized", err)
	}
	if retryablePortalError(errUnauthorized) || retryablePortalError(errLeaseLost) {
		t.Fatal("a rejected token or a lost lease must not be retried")
	}
}

// The attempt number of the claim is a fencing token: every call about a job
// names it, and the portal refuses the calls of an attempt it took back.
func TestEveryJobCallCarriesItsAttemptAndAnOlderAttemptLosesTheLease(t *testing.T) {
	portal := newFakePortal(t)
	first, ref := claimedDemo(t, portal)
	api := newClient(portal.server.URL, testToken)
	ctx := context.Background()
	video := fakeVideo("fence", 20)
	init := artifactInit{Name: "seg-003.mp4", Kind: artifactKindVideo, Variant: testVariant, SizeBytes: int64(len(video)), SHA256: sha256Hex(video)}
	dest := filepath.Join(t.TempDir(), first.ID+".dem")

	// The whole life of attempt 1, one call per route.
	if err := api.downloadDemo(ctx, first, dest, ref); err != nil {
		t.Fatalf("downloadDemo error = %v", err)
	}
	if err := api.enterUploading(ctx, first, 12, uuid.NewString()); err != nil {
		t.Fatalf("enterUploading error = %v", err)
	}
	upload, err := api.initArtifact(ctx, first, init)
	if err != nil {
		t.Fatalf("initArtifact error = %v", err)
	}
	for number, part := range [][]byte{video[:16], video[16:]} {
		ref := partRef{Job: first, ArtifactID: upload.ArtifactID, Number: number + 1}
		if _, err := api.putPart(ctx, ref, bytes.NewReader(part), int64(len(part))); err != nil {
			t.Fatalf("putPart %d error = %v", number+1, err)
		}
	}
	if err := api.completeArtifact(ctx, first, upload.ArtifactID); err != nil {
		t.Fatalf("completeArtifact error = %v", err)
	}
	portal.with(func() {
		if portal.unfenced != 0 || portal.stale != 0 {
			t.Fatalf("calls without an attempt = %d, with a stale one = %d, want none", portal.unfenced, portal.stale)
		}
		// The portal took the job back and handed it out again.
		portal.jobs[first.ID].job.Attempt = 2
	})

	if err := api.completeJob(ctx, first); !errors.Is(err, errLeaseLost) {
		t.Fatalf("completeJob from attempt 1 error = %v, want errLeaseLost", err)
	}
	if _, err := api.fail(ctx, first, failRequest{Code: codeUploadFailed}); !errors.Is(err, errLeaseLost) {
		t.Fatalf("fail from attempt 1 error = %v, want errLeaseLost", err)
	}
	if _, err := api.initArtifact(ctx, first, init); !errors.Is(err, errLeaseLost) {
		t.Fatalf("initArtifact from attempt 1 error = %v, want errLeaseLost", err)
	}
	portal.with(func() {
		job := portal.jobs[first.ID]
		if job.status != phaseUploading || len(job.fails) != 0 {
			t.Fatalf("attempt 2 after the calls of attempt 1: status %q, fails %d, want it untouched", job.status, len(job.fails))
		}
	})
	if err := api.completeJob(ctx, jobRef{ID: first.ID, Attempt: 2}); err != nil {
		t.Fatalf("completeJob from attempt 2 error = %v", err)
	}
}
