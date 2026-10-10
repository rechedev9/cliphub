package cloudbridge

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func (p *fakePortal) countStatus(status string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	count := 0
	for _, job := range p.jobs {
		if job.status == status {
			count++
		}
	}
	return count
}

func (p *fakePortal) claimCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.claims
}

func TestWorkerStopsClaimingWhileThreeFinishedJobsWaitToUpload(t *testing.T) {
	h := newHarness(t)
	h.renderOnCapture(map[string][]byte{"seg-003": fakeVideo("backlog", 40)})
	// No part gets through, so every captured job joins the upload queue.
	h.portal.outages["part"] = 1 << 30
	ids := []string{}
	for range maxUploadBacklog + 2 {
		ids = append(ids, h.queueJob(nil))
	}
	h.start()

	eventually(t, "three jobs to wait for their upload", func() bool { return h.portal.countStatus(phaseUploading) == maxUploadBacklog })
	claims := h.portal.claimCount()
	time.Sleep(60 * time.Millisecond)
	if got := h.local.count(); got != maxUploadBacklog {
		t.Fatalf("local jobs captured = %d, want %d: the worker must not capture more while uploads pile up", got, maxUploadBacklog)
	}
	if got := h.portal.claimCount(); got != claims {
		t.Fatalf("claims while the upload backlog is full = %d, want none", got-claims)
	}

	// Once uploads move again the rest of the queue is taken.
	h.portal.with(func() { h.portal.outages["part"] = 0 })
	eventually(t, "every job to be delivered", func() bool { return h.portal.countStatus("done") == len(ids) })
	h.settled()
}

func TestWorkerWaitsOnAClaimRefusalItDoesNotKnow(t *testing.T) {
	h := newHarness(t)
	h.portal.claimRefusal = "upload_backlog"
	id := h.queueJob(nil)
	h.start()

	eventually(t, "the worker to keep asking", func() bool { return h.portal.claimCount() >= 3 })
	if h.local.count() != 0 || len(h.tracked()) != 0 {
		t.Fatal("a refused claim started work")
	}
	h.portal.with(func() { h.portal.claimRefusal = "" })
	eventually(t, "the job to be claimed once the portal allows it", func() bool { return h.portal.jobStatus(id) == phaseRunning })
}

func TestWorkerStopsClaimingWhileThePortalRefusesItsHeartbeats(t *testing.T) {
	h := newHarness(t)
	h.start()
	heartbeats := func() int {
		calls := 0
		h.portal.with(func() { calls = h.portal.calls["heartbeat"] })
		return calls
	}
	eventually(t, "the first heartbeats", func() bool { return heartbeats() >= 2 })

	// From now on no lease would be renewed, so a claimed job would be lost.
	h.portal.with(func() { h.portal.heartbeatStatus = http.StatusBadRequest })
	refusedFrom := heartbeats()
	eventually(t, "the worker to see a refused heartbeat", func() bool { return heartbeats() >= refusedFrom+2 })
	id := h.queueJob(nil)
	claims := h.portal.claimCount()
	time.Sleep(60 * time.Millisecond)
	if status := h.portal.jobStatus(id); status != "" {
		t.Fatalf("job status = %q, want it left in the queue while heartbeats are refused", status)
	}
	if got := h.portal.claimCount(); got != claims {
		t.Fatalf("claims while heartbeats are refused = %d, want none", got-claims)
	}

	h.portal.with(func() { h.portal.heartbeatStatus = 0 })
	eventually(t, "the job to be claimed after a heartbeat got through", func() bool { return h.portal.jobStatus(id) == phaseRunning })
}

func TestAHeartbeatOutageDoesNotStopClaims(t *testing.T) {
	h := newHarness(t)
	h.portal.outages["heartbeat"] = 1 << 30
	id := h.queueJob(nil)
	h.start()
	eventually(t, "the job to be claimed", func() bool { return h.portal.jobStatus(id) == phaseRunning })
}

func TestJanitorKeepsRunningWhileTheWorkerIsIdle(t *testing.T) {
	h := newHarness(t)
	h.timing.Janitor = 5 * time.Millisecond
	h.start()
	eventually(t, "the worker to be idle", func() bool { return h.portal.claimCount() >= 2 })

	// A leftover that appears long after the start: a local job no cloud job
	// owns and a demo nothing tracks.
	orphanCloud := uuid.NewString()
	orphan := &fakeLocalJob{id: uuid.NewString(), cloudID: orphanCloud, view: LocalJob{Status: localJobFailed}}
	h.local.mu.Lock()
	h.local.jobs[orphan.id] = orphan
	h.local.mu.Unlock()
	incoming := filepath.Join(h.dataDir, "cloudbridge", "incoming")
	if err := os.MkdirAll(incoming, 0o750); err != nil {
		t.Fatal(err)
	}
	strayDemo := filepath.Join(incoming, orphanCloud+".dem")
	if err := os.WriteFile(strayDemo, []byte("PBDEMS2"), 0o600); err != nil {
		t.Fatal(err)
	}

	eventually(t, "the leftover local job to be deleted without a restart", func() bool {
		h.local.mu.Lock()
		defer h.local.mu.Unlock()
		return orphan.deleted
	})
	eventually(t, "the stray demo to be removed without a restart", func() bool {
		_, err := os.Stat(strayDemo)
		return os.IsNotExist(err)
	})
}

// Canceling a render must not delete the local job, or let the next capture
// start, while the editor's process tree is still going down.
func TestWorkerWaitsForCanceledWorkToStopBeforeDeletingTheLocalJob(t *testing.T) {
	h := newHarness(t)
	h.local.onAdmit = func(job *fakeLocalJob) {
		job.afterGenerate = []func(*fakeLocalJob) bool{then(func(job *fakeLocalJob) {
			job.view = LocalJob{Status: "recorded", Progress: &LocalProgress{Stage: "encode", Percent: 10}}
			job.variant = &LocalVariant{Status: localVariantRendering}
			job.working = 5
		})}
	}
	id := h.queueJob(nil)
	h.start()
	eventually(t, "the render to start", func() bool { return h.sawStage(id, stageRendering) })

	h.portal.with(func() { h.portal.jobs[id].cancelRequested = true })

	eventually(t, "the cancel to be acknowledged", func() bool { return h.portal.jobStatus(id) == "failed" })
	h.settled()
	local := h.local.only()
	if !local.deleted || local.deletedWhileWorking {
		t.Fatalf("local job: deleted=%v while its work was still stopping=%v, want it deleted only afterwards", local.deleted, local.deletedWhileWorking)
	}
	if local.cancelCalls < 6 {
		t.Fatalf("cancel calls = %d, want the worker to ask until nothing is in flight", local.cancelCalls)
	}
}

func TestWorkerDoesNotClaimWhileCanceledWorkIsStillRunning(t *testing.T) {
	h := newHarness(t)
	h.timing.SettleTimeout = 30 * time.Millisecond
	h.local.onAdmit = func(job *fakeLocalJob) {
		// The first job's render ignores the cancel for as long as the test says.
		if len(h.local.jobs) == 0 {
			job.afterGenerate = []func(*fakeLocalJob) bool{then(func(job *fakeLocalJob) {
				job.view = LocalJob{Status: "recorded"}
				job.variant = &LocalVariant{Status: localVariantRendering}
				job.working = -1
			})}
		}
	}
	first := h.queueJob(nil)
	h.start()
	eventually(t, "the render to start", func() bool { return h.sawStage(first, stageRendering) })
	stuck := h.local.only().id
	h.portal.with(func() { h.portal.jobs[first].cancelRequested = true })
	eventually(t, "the cancel to be acknowledged", func() bool { return h.portal.jobStatus(first) == "failed" })

	second := h.queueJob(nil)
	eventually(t, "a heartbeat that says why the worker is not claiming", func() bool {
		blocked := false
		h.portal.with(func() {
			last := h.portal.heartbeats[len(h.portal.heartbeats)-1]
			blocked = last.State == stateBlocked && last.Blocked != nil && last.Blocked.Code == blockCS2Running
		})
		return blocked
	})
	time.Sleep(40 * time.Millisecond)
	if status := h.portal.jobStatus(second); status != "" {
		t.Fatalf("second job status = %q, want it unclaimed while the first job's work still runs", status)
	}
	h.local.mu.Lock()
	if h.local.jobs[stuck].deleted {
		t.Fatal("the local job was deleted while its work was still running")
	}
	// The process tree is finally gone.
	h.local.jobs[stuck].working = 0
	h.local.mu.Unlock()

	eventually(t, "the second job to be claimed", func() bool { return h.portal.jobStatus(second) == phaseRunning })
	h.local.mu.Lock()
	defer h.local.mu.Unlock()
	if !h.local.jobs[stuck].deleted {
		t.Fatal("the first local job was not deleted once its work stopped")
	}
}
