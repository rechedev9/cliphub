package cloudbridge

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

// requeue takes a job away from the worker on the portal side, the way an
// expired lease does, and makes it claimable again as the next attempt.
func (p *fakePortal) requeue(id string) {
	old := p.jobs[id]
	delete(p.jobs, id)
	p.queue = append(p.queue, old.job)
}

func (h *harness) localJobsOf(cloudID string) []fakeLocalJob {
	h.local.mu.Lock()
	defer h.local.mu.Unlock()
	jobs := []fakeLocalJob{}
	for _, job := range h.local.jobs {
		if job.cloudID == cloudID {
			jobs = append(jobs, *job)
		}
	}
	return jobs
}

// The portal requeued a job whose upload this worker still holds, and the
// claim hands it back before any heartbeat said the old lease was lost.
func TestAJobHandedBackWhileItsOldAttemptUploadsIsDeliveredByTheNewAttempt(t *testing.T) {
	h := newHarness(t)
	h.renderOnCapture(map[string][]byte{"seg-003": fakeVideo("handed-back", 80)})
	// Like production: the uploader sleeps long between passes, so it cannot
	// notice the lost lease by itself before the claim comes back.
	h.timing.UploadRetryMin, h.timing.UploadRetryMax = 3*time.Second, 3*time.Second
	h.timing.Heartbeat = 40 * time.Millisecond
	h.portal.outages["part"] = 1 << 30
	id := h.queueJob(nil)
	h.start()
	eventually(t, "the first attempt to reach uploading and fail a part", func() bool {
		stalled := false
		h.portal.with(func() {
			job := h.portal.jobs[id]
			stalled = job != nil && job.status == phaseUploading && h.portal.calls["part"] >= 1
		})
		return stalled
	})
	first := h.local.only().id

	h.portal.with(func() {
		h.portal.requeue(id)
		h.portal.outages["part"] = 0
		// The claim is the first call the portal answers after the outage.
		h.portal.outages["heartbeat"] = 2
	})

	eventually(t, "the second attempt to deliver", func() bool { return h.portal.jobStatus(id) == "done" })
	h.settled()
	h.portal.with(func() {
		job := h.portal.jobs[id]
		if job.job.Attempt != 2 || len(job.fails) != 0 || job.lateFails != 0 {
			t.Fatalf("portal job: attempt %d, fails %+v plus %d late, want attempt 2 delivered and nothing reported", job.job.Attempt, job.fails, job.lateFails)
		}
		if len(job.phases) != 1 {
			t.Fatalf("phase reports of attempt 2 = %d, want 1", len(job.phases))
		}
	})
	locals := h.localJobsOf(id)
	if len(locals) != 2 {
		t.Fatalf("local jobs = %d, want one per attempt", len(locals))
	}
	for _, local := range locals {
		if !local.deleted {
			t.Fatalf("local job %s (first attempt: %v) was left behind", local.id, local.id == first)
		}
	}
	if left := h.incomingDemos(); len(left) != 0 {
		t.Fatalf("incoming demos left behind: %v", left)
	}
}

// The same hand-back, for an upload that was still waiting behind another.
func TestAJobHandedBackWhileItsOldAttemptWaitsToUploadIsCapturedAgain(t *testing.T) {
	h := newHarness(t)
	h.renderOnCapture(map[string][]byte{"seg-003": fakeVideo("waiting", 40)})
	// The head upload never gets a part through, so the second job waits.
	h.portal.outages["part"] = 1 << 30
	head := h.queueJob(nil)
	waiting := h.queueJob(nil)
	h.start()
	eventually(t, "both jobs to reach uploading", func() bool {
		return h.portal.jobStatus(head) == phaseUploading && h.portal.jobStatus(waiting) == phaseUploading
	})
	firstLocal := h.localJobsOf(waiting)[0].id

	h.portal.with(func() {
		h.portal.requeue(waiting)
		// No heartbeat tells the worker; the claim is how it finds out.
		h.portal.outages["heartbeat"] = 1 << 30
	})

	eventually(t, "the second attempt to be captured again", func() bool { return len(h.localJobsOf(waiting)) == 2 })
	eventually(t, "the first attempt's local job to be deleted", func() bool {
		for _, local := range h.localJobsOf(waiting) {
			if local.id == firstLocal {
				return local.deleted
			}
		}
		return false
	})
	h.portal.with(func() { h.portal.outages["heartbeat"], h.portal.outages["part"] = 0, 0 })
	eventually(t, "both jobs to be delivered", func() bool {
		return h.portal.jobStatus(head) == "done" && h.portal.jobStatus(waiting) == "done"
	})
	h.settled()
	h.portal.with(func() {
		job := h.portal.jobs[waiting]
		if job.job.Attempt != 2 || len(job.fails) != 0 || job.lateFails != 0 {
			t.Fatalf("portal job: attempt %d, fails %+v plus %d late, want attempt 2 delivered and nothing reported", job.job.Attempt, job.fails, job.lateFails)
		}
	})
}

// The old attempt is not uploading any more: it failed, and its failure
// report is being repeated against a portal that does not answer it.
func TestAJobHandedBackWhileItsOldAttemptStillReportsAFailureStartsAgain(t *testing.T) {
	h := newHarness(t)
	// Not an mp4, so the portal refuses the video and the upload fails.
	h.renderOnCapture(map[string][]byte{"seg-003": bytes.Repeat([]byte("x"), 40)})
	h.portal.outages["fail"] = 1 << 30
	id := h.queueJob(nil)
	h.start()
	eventually(t, "the first attempt to keep reporting its failed upload", func() bool {
		calls := 0
		h.portal.with(func() { calls = h.portal.calls["fail"] })
		return calls >= 3
	})
	first := h.local.only().id

	h.portal.with(func() { h.portal.requeue(id) })

	eventually(t, "the second attempt to be captured", func() bool { return len(h.localJobsOf(id)) == 2 })
	eventually(t, "the second attempt to reach uploading", func() bool { return h.portal.jobStatus(id) == phaseUploading })
	for _, local := range h.localJobsOf(id) {
		if local.id == first && !local.deleted {
			t.Fatal("the first attempt's local job was left behind")
		}
	}
	h.portal.with(func() {
		if job := h.portal.jobs[id]; job.job.Attempt != 2 || len(job.fails) != 0 || job.lateFails != 0 {
			t.Fatalf("portal job: attempt %d, fails %+v plus %d late, want attempt 2 with nothing reported by attempt 1", job.job.Attempt, job.fails, job.lateFails)
		}
	})
}

func TestTrackerNeverLetsAnOldAttemptTouchANewerOne(t *testing.T) {
	const id = "0b0f5c0e-0000-4000-8000-000000000001"
	old, current := jobRef{ID: id, Attempt: 1}, jobRef{ID: id, Attempt: 2}
	tr := newTracker()
	oldCtx := tr.begin(context.Background(), old, jobProgress{Phase: phaseUploading})
	// A heartbeat about attempt 1 is on its way while attempt 2 takes over.
	sent, _ := tr.request()
	if len(sent.Jobs) != 1 || sent.Jobs[0].Attempt != 1 {
		t.Fatalf("heartbeat jobs = %+v, want the job with attempt 1", sent.Jobs)
	}
	tr.end(old)
	currentCtx := tr.begin(context.Background(), current, jobProgress{Phase: phaseRunning, Stage: stageDownloading})

	tr.apply(sent, heartbeatResponse{Jobs: []heartbeatJobStatus{{ID: id, Lease: leaseLost}}})
	tr.signal(old, errLeaseLost)
	tr.update(old, func(progress *jobProgress) { progress.Percent = 99 })
	tr.end(old)

	if oldCtx.Err() == nil {
		t.Fatal("the old attempt's context is still alive after it ended")
	}
	if cause := context.Cause(currentCtx); cause != nil {
		t.Fatalf("the new attempt was stopped by the old one: %v", cause)
	}
	now, capturing := tr.request()
	if len(now.Jobs) != 1 || now.Jobs[0].Attempt != 2 || now.Jobs[0].Percent != 0 || !capturing {
		t.Fatalf("heartbeat jobs = %+v, want attempt 2 untouched and still running", now.Jobs)
	}

	// The answer to a heartbeat about attempt 2 does reach it.
	tr.apply(now, heartbeatResponse{Jobs: []heartbeatJobStatus{{ID: id, Lease: leaseLost}}})
	if cause := context.Cause(currentCtx); !errors.Is(cause, errLeaseLost) {
		t.Fatalf("cause after a lost lease for attempt 2 = %v, want errLeaseLost", cause)
	}
}
