package cloudbridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/rules"
)

func TestWorkerDeliversAShortEndToEnd(t *testing.T) {
	h := newHarness(t)
	videos := map[string][]byte{
		"seg-003": fakeVideo("first", 40),
		"seg-009": fakeVideo("second", 16),
	}
	h.renderOnCapture(videos)
	id := h.queueJob(nil)
	h.start()

	eventually(t, "the portal to mark the job done", func() bool { return h.portal.jobStatus(id) == "done" })
	h.settled()

	// The local pipeline got the user's target, rules and generate body.
	local := h.local.only()
	h.local.mu.Lock()
	admission, demo := h.local.admissions[0], h.local.demos[0]
	h.local.mu.Unlock()
	if admission.TargetSteamID != testTarget || admission.CloudRequestID != id {
		t.Fatalf("admission = %+v, want the spec's target and the cloud job id", admission)
	}
	wantRules := rules.Default()
	wantRules.Weapons, wantRules.MinKillsInWindow, wantRules.WindowSeconds, wantRules.PostRollSeconds = []string{"awp"}, 2, 10, 4
	if fmt.Sprint(admission.Rules) != fmt.Sprint(wantRules) {
		t.Fatalf("admitted rules = %+v, want %+v", admission.Rules, wantRules)
	}
	if !bytes.Equal(demo, h.portal.demo) {
		t.Fatal("the admitted demo is not the demo the portal served")
	}
	if string(local.generateBody) != testGenerate {
		t.Fatalf("generate body = %s, want the spec's generate unchanged", local.generateBody)
	}
	if local.generateCalls != 1 {
		t.Fatalf("generate calls = %d, want 1", local.generateCalls)
	}

	h.portal.with(func() {
		job := h.portal.jobs[id]
		if len(job.fails) != 0 {
			t.Fatalf("fail reports = %+v, want none", job.fails)
		}
		if len(job.phases) != 1 || job.phases[0].LocalJobID != local.id {
			t.Fatalf("phase reports = %+v, want one naming local job %s", job.phases, local.id)
		}
		if len(job.artifacts) != len(videos) {
			t.Fatalf("artifacts = %d, want %d", len(job.artifacts), len(videos))
		}
		for _, artifact := range job.artifacts {
			name := strings.TrimSuffix(artifact.init.Name, ".mp4")
			want := videos[name]
			var got []byte
			for _, number := range artifact.received() {
				got = append(got, artifact.parts[number]...)
			}
			if !artifact.complete || !bytes.Equal(got, want) {
				t.Fatalf("artifact %s: complete=%v, %d bytes, want the %d rendered bytes", artifact.init.Name, artifact.complete, len(got), len(want))
			}
			if artifact.init.Kind != artifactKindVideo || artifact.init.Variant != testVariant || artifact.init.SHA256 != sha256Hex(want) {
				t.Fatalf("artifact init = %+v", artifact.init)
			}
		}

		// The operator panel follows the job through these stages.
		stages := []string{}
		for _, beat := range h.portal.heartbeats {
			for _, reported := range beat.Jobs {
				if reported.ID != id {
					continue
				}
				if reported.Phase == phaseUploading {
					if reported.Stage != nil {
						t.Fatalf("uploading heartbeat carries stage %q, want null", *reported.Stage)
					}
					continue
				}
				if beat.State != stateBusy {
					t.Fatalf("worker state = %q while a job runs, want busy", beat.State)
				}
				if reported.Stage != nil && !slices.Contains(stages, *reported.Stage) {
					stages = append(stages, *reported.Stage)
				}
			}
		}
		for _, want := range []string{stageCapturing, stageRendering} {
			if !slices.Contains(stages, want) {
				t.Fatalf("heartbeat stages = %v, want %q among them", stages, want)
			}
		}
	})

	// Nothing of the job is left on the worker.
	if !local.deleted {
		t.Fatal("the local job was not deleted after delivery")
	}
	if left := h.incomingDemos(); len(left) != 0 {
		t.Fatalf("incoming demos left behind: %v", left)
	}
}

func TestWorkerReportsEachFailureExactlyOnce(t *testing.T) {
	const hookCrash = `C:\Studio\zv-recorder.exe failed: exit status 6: HLAE hook crashed with a native error dialog ("Error - AfxHookSource2")`
	failCapture := func(reason, code string) func(*harness) {
		return func(h *harness) {
			h.local.onAdmit = func(job *fakeLocalJob) {
				job.afterGenerate = []func(*fakeLocalJob) bool{then(func(job *fakeLocalJob) {
					job.view = LocalJob{Status: localJobFailed, FailureReason: reason, FailureCode: code}
				})}
			}
		}
	}
	cases := []struct {
		name       string
		job        func(*workerJob)
		prepare    func(*harness)
		wantCode   string
		wantDetail string
		// wantLocalJobs is how many local jobs the attempt may create.
		wantLocalJobs int
	}{
		{
			name: "a window the worker's own plan places elsewhere",
			prepare: func(h *harness) {
				h.local.onAdmit = func(job *fakeLocalJob) { job.plan.Segments[1].TickEnd = 52100 }
			},
			wantCode:      codeSpecMismatch,
			wantDetail:    `window "seg-003" covers ticks 51234 to 52010 in the spec and 51234 to 52100 in the worker's plan`,
			wantLocalJobs: 1,
		},
		{
			name: "generate rejected by the local pipeline",
			prepare: func(h *harness) {
				h.local.generate = func(*fakeLocalJob) GenerateResult {
					return GenerateResult{Status: http.StatusBadRequest, Code: "invalid_request", Message: `unknown preset "nope"`}
				}
			},
			wantCode:      codeInvalidSpec,
			wantDetail:    `unknown preset "nope"`,
			wantLocalJobs: 1,
		},
		{
			name: "a spec that would record more than its windows",
			job: func(job *workerJob) {
				job.Spec = json.RawMessage(strings.Replace(string(testSpec()), `"segment_ids":["seg-003"]`, `"segment_ids":[]`, 1))
			},
			wantCode:   codeInvalidSpec,
			wantDetail: "generate.segment_ids must list exactly the capture windows, in order",
		},
		{
			name:          "HLAE broken by a CS2 update",
			prepare:       failCapture(hookCrash, "capture_incompatible"),
			wantCode:      codeCaptureIncompatible,
			wantDetail:    hookCrash,
			wantLocalJobs: 1,
		},
		{
			name:          "a capture flake",
			prepare:       failCapture("observer target 5 drifted from 3", "capture_flake"),
			wantCode:      codeCaptureFlake,
			wantDetail:    "observer target 5 drifted from 3",
			wantLocalJobs: 1,
		},
		{
			name:          "any other capture failure",
			prepare:       failCapture("cs2.exe exited with status 1", ""),
			wantCode:      codeJobFailed,
			wantDetail:    "cs2.exe exited with status 1",
			wantLocalJobs: 1,
		},
		{
			name: "a demo CS2 can no longer play, found while parsing",
			prepare: func(h *harness) {
				h.local.onAdmit = func(job *fakeLocalJob) {
					job.view = LocalJob{Status: localJobFailed, FailureReason: "parsing demo: demo_incompatible: unsupported build", FailureCode: "demo_incompatible"}
				}
			},
			wantCode:      codeDemoIncompatible,
			wantDetail:    "parsing demo: demo_incompatible: unsupported build",
			wantLocalJobs: 1,
		},
		{
			name: "a failed render after a good capture",
			prepare: func(h *harness) {
				h.local.onAdmit = func(job *fakeLocalJob) {
					job.afterGenerate = []func(*fakeLocalJob) bool{then(func(job *fakeLocalJob) {
						job.view = LocalJob{Status: "recorded"}
						job.variant = &LocalVariant{Status: localVariantFailed, Error: "ffmpeg exited with status 1"}
					})}
				}
			},
			wantCode:      codeRenderFailed,
			wantDetail:    "ffmpeg exited with status 1",
			wantLocalJobs: 1,
		},
		{
			name: "a capture that outlives the job's time limit",
			job:  func(job *workerJob) { job.MaxRuntimeSeconds = 1 },
			// The default script leaves the job recording forever.
			wantCode:      codeTimeout,
			wantDetail:    "the attempt exceeded its limit of 1s",
			wantLocalJobs: 1,
		},
		{
			name:       "a demo whose bytes do not match the claim",
			job:        func(job *workerJob) { job.Demo.SHA256 = strings.Repeat("0", 64) },
			wantCode:   codeDemoDownloadFailed,
			wantDetail: "downloaded demo does not match",
		},
		{
			name: "a file the local pipeline refuses as a demo",
			prepare: func(h *harness) {
				h.local.admitErr = fmt.Errorf("%w: not a CS2 demo", ErrDemoRejected)
			},
			wantCode:   codeDemoIncompatible,
			wantDetail: "not a CS2 demo",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			if tc.prepare != nil {
				tc.prepare(h)
			}
			id := h.queueJob(tc.job)
			h.start()

			eventually(t, "the failure report", func() bool { return h.portal.jobStatus(id) == "failed" })
			h.settled()
			// Give a second, wrong report every chance to arrive.
			time.Sleep(40 * time.Millisecond)

			h.portal.with(func() {
				job := h.portal.jobs[id]
				if len(job.fails) != 1 || job.lateFails != 0 {
					t.Fatalf("fail reports = %+v plus %d late, want exactly one", job.fails, job.lateFails)
				}
				report := job.fails[0]
				if report.Code != tc.wantCode {
					t.Fatalf("code = %q, want %q (detail %q)", report.Code, tc.wantCode, report.Detail)
				}
				if !strings.Contains(report.Detail, tc.wantDetail) {
					t.Fatalf("detail = %q, want it to carry %q", report.Detail, tc.wantDetail)
				}
				if report.Message != failureMessages[tc.wantCode] || strings.Contains(report.Message, tc.wantDetail) {
					t.Fatalf("message = %q, want the fixed sentence for %s", report.Message, tc.wantCode)
				}
				if len(job.phases) != 0 || len(job.artifacts) != 0 {
					t.Fatalf("a failed job reached uploading: phases=%d artifacts=%d", len(job.phases), len(job.artifacts))
				}
			})
			if got := h.local.count(); got != tc.wantLocalJobs {
				t.Fatalf("local jobs created = %d, want %d", got, tc.wantLocalJobs)
			}
			if tc.wantLocalJobs == 1 {
				local := h.local.only()
				if !local.deleted {
					t.Fatal("the failed attempt left its local job behind")
				}
				if tc.wantCode == codeTimeout && local.cancelCalls == 0 {
					t.Fatal("a timed out capture was never canceled")
				}
			}
			if left := h.incomingDemos(); len(left) != 0 {
				t.Fatalf("incoming demos left behind: %v", left)
			}
		})
	}
}

func TestWorkerCancelsTheCaptureWhenThePortalAsks(t *testing.T) {
	h := newHarness(t)
	id := h.queueJob(nil)
	h.start()
	eventually(t, "the capture to start", func() bool {
		return h.local.count() == 1 && h.local.only().view.Status == localJobRecording
	})

	h.portal.with(func() { h.portal.jobs[id].cancelRequested = true })

	eventually(t, "the cancel to be acknowledged", func() bool { return h.portal.jobStatus(id) == "failed" })
	h.settled()
	time.Sleep(40 * time.Millisecond)
	local := h.local.only()
	if local.cancelCalls == 0 || !local.deleted {
		t.Fatalf("local job after cancel: cancel calls=%d deleted=%v, want it stopped and deleted", local.cancelCalls, local.deleted)
	}
	h.portal.with(func() {
		job := h.portal.jobs[id]
		if len(job.fails) != 1 || job.fails[0].Code != codeCanceled || job.lateFails != 0 {
			t.Fatalf("fail reports = %+v plus %d late, want exactly one canceled", job.fails, job.lateFails)
		}
	})
}

func TestWorkerAbandonsAJobWhoseLeaseIsLost(t *testing.T) {
	h := newHarness(t)
	id := h.queueJob(nil)
	h.start()
	eventually(t, "the capture to start", func() bool {
		return h.local.count() == 1 && h.local.only().view.Status == localJobRecording
	})

	h.portal.with(func() { h.portal.jobs[id].leaseLost = true })

	h.settled()
	eventually(t, "the local job to be deleted", func() bool { return h.local.only().deleted })
	time.Sleep(40 * time.Millisecond)
	if h.local.only().cancelCalls == 0 {
		t.Fatal("the capture of a job that is no longer ours kept running")
	}
	h.portal.with(func() {
		job := h.portal.jobs[id]
		if len(job.fails) != 0 || job.lateFails != 0 {
			t.Fatalf("fail reports = %+v plus %d late, want none: the job is not ours to fail", job.fails, job.lateFails)
		}
	})
	if left := h.incomingDemos(); len(left) != 0 {
		t.Fatalf("incoming demos left behind: %v", left)
	}
}

func TestWorkerRetriesTheFailReportUntilThePortalHasIt(t *testing.T) {
	h := newHarness(t)
	h.local.onAdmit = func(job *fakeLocalJob) { job.plan.Demo.Tickrate = 128 }
	h.portal.outages["fail"] = 3
	id := h.queueJob(nil)
	h.start()

	eventually(t, "the failure report to land", func() bool { return h.portal.jobStatus(id) == "failed" })
	h.settled()
	h.portal.with(func() {
		job := h.portal.jobs[id]
		if len(job.fails) != 1 || job.fails[0].Code != codeSpecMismatch {
			t.Fatalf("fail reports = %+v, want one spec_mismatch", job.fails)
		}
		if h.portal.calls["fail"] != 4 {
			t.Fatalf("fail calls = %d, want 3 refused and 1 accepted", h.portal.calls["fail"])
		}
	})
}

func TestWorkerReportsACaptureInterruptedByARestart(t *testing.T) {
	h := newHarness(t)
	id := h.queueJob(nil)
	h.start()
	eventually(t, "the capture to start", func() bool {
		return h.local.count() == 1 && h.local.only().view.Status == localJobRecording
	})
	interrupted := h.local.only().id

	// The process dies mid-capture; the orchestrator's startup sweep fails
	// the local job before anything else runs.
	h.stop()
	h.local.mu.Lock()
	h.local.jobs[interrupted].view = LocalJob{Status: localJobFailed, FailureReason: "interrupted: orchestrator restarted during recording"}
	h.local.mu.Unlock()
	if tracked := h.persisted(); len(tracked) != 1 || tracked[0].Phase != phaseRunning || tracked[0].LocalJobID != interrupted {
		t.Fatalf("state after the crash = %+v, want the running job with its local id", tracked)
	}
	h.portal.with(func() {
		if got := len(h.portal.jobs[id].fails); got != 0 {
			t.Fatalf("fail reports before the restart = %d, want 0", got)
		}
	})

	h.start()

	eventually(t, "the interrupted report", func() bool { return h.portal.jobStatus(id) == "failed" })
	h.settled()
	time.Sleep(40 * time.Millisecond)
	h.portal.with(func() {
		job := h.portal.jobs[id]
		if len(job.fails) != 1 || job.fails[0].Code != codeInterrupted || job.lateFails != 0 {
			t.Fatalf("fail reports = %+v plus %d late, want exactly one interrupted", job.fails, job.lateFails)
		}
	})
	h.local.mu.Lock()
	defer h.local.mu.Unlock()
	if len(h.local.jobs) != 1 || !h.local.jobs[interrupted].deleted {
		t.Fatalf("local jobs after the restart = %d (interrupted deleted=%v), want the one job deleted and nothing run again", len(h.local.jobs), h.local.jobs[interrupted].deleted)
	}
	if h.local.jobs[interrupted].generateCalls != 1 {
		t.Fatalf("generate calls = %d, want 1: the capture must not run twice", h.local.jobs[interrupted].generateCalls)
	}
}

func TestWorkerResumesAnUploadAfterARestartWithoutCapturingAgain(t *testing.T) {
	h := newHarness(t)
	video := fakeVideo("resume", 40) // three parts of 16, 16 and 8 bytes
	h.renderOnCapture(map[string][]byte{"seg-003": video})
	// The portal goes away while the second part is on its way.
	h.portal.outages["part"] = 1000
	id := h.queueJob(nil)
	h.start()
	eventually(t, "the upload to start", func() bool {
		received := false
		h.portal.with(func() { received = h.portal.calls["part"] > 2 })
		return received
	})
	h.stop()
	if tracked := h.persisted(); len(tracked) != 1 || tracked[0].Phase != phaseUploading || !tracked[0].PhaseReported {
		t.Fatalf("state after the crash = %+v, want the job in uploading", tracked)
	}
	local := h.local.only()

	// One part made it before the restart.
	h.portal.with(func() {
		h.portal.outages["part"] = 0
		artifact := h.portal.jobs[id].artifacts[0]
		artifact.parts[1] = video[:16]
		artifact.puts[1] = 1
	})
	h.start()

	eventually(t, "the portal to mark the job done", func() bool { return h.portal.jobStatus(id) == "done" })
	h.settled()
	h.portal.with(func() {
		job := h.portal.jobs[id]
		artifact := job.artifacts[0]
		if artifact.puts[1] != 1 || artifact.puts[2] != 1 || artifact.puts[3] != 1 {
			t.Fatalf("part uploads = %v, want part 1 kept and parts 2 and 3 sent once", artifact.puts)
		}
		if len(job.phases) != 1 || len(job.fails) != 0 {
			t.Fatalf("phases=%d fails=%d, want the single phase report from before the restart and no failure", len(job.phases), len(job.fails))
		}
	})
	if h.local.count() != 1 || h.local.only().generateCalls != 1 {
		t.Fatalf("local jobs = %d, generate calls = %d, want the one capture from before the restart", h.local.count(), h.local.only().generateCalls)
	}
	if !h.local.only().deleted || local.id != h.local.only().id {
		t.Fatal("the delivered job was not cleaned up")
	}
}

func TestUploaderSendsOnlyMissingPartsAfterAnOutage(t *testing.T) {
	h := newHarness(t)
	video := fakeVideo("outage", 72) // five parts
	h.renderOnCapture(map[string][]byte{"seg-003": video})
	id := h.queueJob(nil)
	// Parts 1 and 2 arrive, part 3 fails twice, then the portal is back.
	h.portal.partOutages[3] = 2
	h.start()

	eventually(t, "the portal to mark the job done", func() bool { return h.portal.jobStatus(id) == "done" })
	h.portal.with(func() {
		artifact := h.portal.jobs[id].artifacts[0]
		for number := 1; number <= 5; number++ {
			if artifact.puts[number] != 1 {
				t.Fatalf("part uploads = %v, want every part stored exactly once", artifact.puts)
			}
		}
		if len(h.portal.jobs[id].fails) != 0 {
			t.Fatalf("fail reports = %+v, want none", h.portal.jobs[id].fails)
		}
	})
}

func TestUploaderGivesUpWithUploadFailedWhenThePortalRejectsTheVideo(t *testing.T) {
	h := newHarness(t)
	// Not an mp4: the portal answers 422 not_a_video, which no retry fixes.
	h.renderOnCapture(map[string][]byte{"seg-003": bytes.Repeat([]byte("x"), 40)})
	id := h.queueJob(nil)
	h.start()

	eventually(t, "the failure report", func() bool { return h.portal.jobStatus(id) == "failed" })
	h.settled()
	h.portal.with(func() {
		job := h.portal.jobs[id]
		if len(job.fails) != 1 || job.fails[0].Code != codeUploadFailed || !strings.Contains(job.fails[0].Detail, "not_a_video") {
			t.Fatalf("fail reports = %+v, want one upload_failed naming not_a_video", job.fails)
		}
		if job.fails[0].MachineSeconds != 0 {
			t.Fatalf("machine seconds = %d, want 0: the capture time went with the phase report", job.fails[0].MachineSeconds)
		}
	})
	if !h.local.only().deleted {
		t.Fatal("the failed upload left its local job behind")
	}
}

func TestWorkerClaimsTheNextJobWhileThePreviousOneUploads(t *testing.T) {
	h := newHarness(t)
	h.renderOnCapture(map[string][]byte{"seg-003": fakeVideo("overlap", 40)})
	h.portal.outages["part"] = 1 << 30
	first := h.queueJob(nil)
	second := h.queueJob(nil)
	h.start()

	// The first job cannot finish uploading, yet the second one is captured.
	eventually(t, "the second job to reach uploading", func() bool {
		return h.portal.jobStatus(first) == phaseUploading && h.portal.jobStatus(second) == phaseUploading
	})
	h.portal.with(func() { h.portal.outages["part"] = 0 })
	eventually(t, "both jobs to be delivered", func() bool {
		return h.portal.jobStatus(first) == "done" && h.portal.jobStatus(second) == "done"
	})
	h.settled()
}

func TestWorkerDoesNotClaimWhileItCannotCapture(t *testing.T) {
	cases := []struct {
		name     string
		prepare  func(*harness)
		wantCode string
	}{
		{name: "Steam is closed", prepare: func(h *harness) { h.machine.health.SteamRunning = false }, wantCode: blockSteamUnavailable},
		{name: "a cs2.exe is already running", prepare: func(h *harness) { h.machine.cs2 = true }, wantCode: blockCS2Running},
		{name: "the disk is nearly full", prepare: func(h *harness) { h.machine.health.DiskFreeBytes = 1 << 30 }, wantCode: blockDiskFull},
		{name: "capture tools are missing", prepare: func(h *harness) { h.machine.health.RecordEnabled = false }, wantCode: blockToolsMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			tc.prepare(h)
			id := h.queueJob(nil)
			h.start()

			eventually(t, "a blocked heartbeat", func() bool {
				blocked := false
				h.portal.with(func() {
					for _, beat := range h.portal.heartbeats {
						if beat.State == stateBlocked && beat.Blocked != nil && beat.Blocked.Code == tc.wantCode {
							blocked = true
						}
					}
				})
				return blocked
			})
			time.Sleep(40 * time.Millisecond)
			h.portal.with(func() {
				if h.portal.claims != 0 {
					t.Fatalf("claims while blocked = %d, want 0", h.portal.claims)
				}
			})

			// Once the machine is fit again the queue moves without a restart.
			h.machine.mu.Lock()
			h.machine.health = readyMachine().health
			h.machine.cs2 = false
			h.machine.mu.Unlock()
			eventually(t, "the job to be claimed", func() bool { return h.portal.jobStatus(id) == phaseRunning })
		})
	}
}

func TestWorkerStopsClaimingWhenItsTokenIsRejected(t *testing.T) {
	h := newHarness(t)
	h.portal.unauthorized = true
	h.queueJob(nil)
	h.start()

	eventually(t, "heartbeats to keep coming", func() bool {
		calls := 0
		h.portal.with(func() { calls = h.portal.calls["heartbeat"] })
		return calls >= 3
	})
	h.portal.with(func() {
		if h.portal.calls["claim"] > 1 {
			t.Fatalf("claim calls with a rejected token = %d, want at most the one that found out", h.portal.calls["claim"])
		}
	})
	if h.local.count() != 0 {
		t.Fatal("a worker with a rejected token started a job")
	}
}

func TestWorkerDoesNotClaimWhilePaused(t *testing.T) {
	h := newHarness(t)
	h.portal.paused = true
	id := h.queueJob(nil)
	h.start()

	eventually(t, "heartbeats to carry the pause", func() bool {
		calls := 0
		h.portal.with(func() { calls = h.portal.calls["heartbeat"] })
		return calls >= 4
	})
	h.portal.with(func() {
		if h.portal.claims > 1 {
			t.Fatalf("claims while paused = %d, want at most the one sent before the first heartbeat answered", h.portal.claims)
		}
		h.portal.paused = false
	})
	eventually(t, "the job to be claimed after the resume", func() bool { return h.portal.jobStatus(id) == phaseRunning })
}

func TestJanitorRemovesOrphansAndKeepsManualBridgeJobs(t *testing.T) {
	h := newHarness(t)
	legacyCloud, orphanCloud := uuid.NewString(), uuid.NewString()
	legacy := &fakeLocalJob{id: uuid.NewString(), cloudID: legacyCloud, view: LocalJob{Status: "scanned"}}
	orphan := &fakeLocalJob{id: uuid.NewString(), cloudID: orphanCloud, view: LocalJob{Status: "recorded"}}
	h.local.jobs[legacy.id], h.local.jobs[orphan.id] = legacy, orphan
	// The manual bridge that preceded the worker wrote entries like this one.
	oldState := `[{"cloud_request_id":"` + legacyCloud + `","local_job_id":"` + legacy.id + `","local_job_reported":true,"uploaded_artifacts":["viral-60-clean/seg-1"]}]`
	if err := os.MkdirAll(h.dataDir+"/cloudbridge/incoming", 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h.statePath(), []byte(oldState), 0o600); err != nil {
		t.Fatal(err)
	}
	strayDemo := h.dataDir + "/cloudbridge/incoming/" + orphanCloud + ".dem"
	if err := os.WriteFile(strayDemo, []byte("PBDEMS2"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.start()

	eventually(t, "the orphan to be deleted", func() bool {
		h.local.mu.Lock()
		defer h.local.mu.Unlock()
		return orphan.deleted
	})
	eventually(t, "the stray demo to be removed", func() bool {
		_, err := os.Stat(strayDemo)
		return os.IsNotExist(err)
	})
	h.local.mu.Lock()
	defer h.local.mu.Unlock()
	if legacy.deleted {
		t.Fatal("the janitor deleted a job of the manual bridge")
	}
	tracked := h.tracked()
	if len(tracked) != 1 || tracked[0].CloudRequestID != legacyCloud || !slices.Equal(tracked[0].UploadedArtifacts, []string{"viral-60-clean/seg-1"}) {
		t.Fatalf("state = %+v, want the manual bridge entry untouched", tracked)
	}
}
