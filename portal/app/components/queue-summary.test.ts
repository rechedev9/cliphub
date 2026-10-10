import { test } from "node:test";
import assert from "node:assert/strict";

import type { AdminJob, AdminOverview, AdminWorker } from "./api-types.ts";
import {
  causeHeadline,
  forgotItsJob,
  leaseOutlook,
  queueSummary,
  recentFailures,
  workerStatus,
} from "./queue-summary.ts";

const NOW = 1_760_000_000_000;

function worker(patch: Partial<AdminWorker>): AdminWorker {
  return {
    id: "w1",
    name: "capture-01",
    online: true,
    lastSeenAt: NOW - 4000,
    state: "idle",
    blocked: null,
    paused: false,
    pausedBy: null,
    pauseReason: null,
    revoked: false,
    createdAt: 1,
    health: null,
    currentJobId: null,
    ...patch,
  };
}

function job(patch: Partial<AdminJob>): AdminJob {
  return {
    id: "j1",
    kind: "short",
    title: "R3 4k",
    status: "queued",
    stage: null,
    progressPercent: null,
    createdAt: NOW - 600_000,
    enqueuedAt: NOW - 600_000,
    startedAt: null,
    finishedAt: null,
    estimatedSeconds: 480,
    attempt: 0,
    cancelRequested: false,
    queue: null,
    failure: null,
    artifacts: [],
    user: { id: "u1", name: "Luis", email: null },
    progressDetail: null,
    workerId: null,
    localJobId: null,
    leaseExpiresAt: null,
    priorityBoost: 0,
    machineSeconds: 0,
    maxAttempts: 2,
    waitedSeconds: 600,
    failureDetail: null,
    canceledBy: null,
    demo: { fileName: "match.dem", sizeBytes: 1, sha256: "x", present: true },
    legacyStatus: false,
    ...patch,
  };
}

function overview(patch: Partial<AdminOverview>): AdminOverview {
  return {
    now: NOW,
    workers: [worker({})],
    running: [],
    uploading: [],
    queue: [],
    stats: {
      queued: 0,
      done24h: 0,
      failed24h: 0,
      canceled24h: 0,
      medianWaitSeconds24h: null,
      machineSeconds24h: 0,
      storage: { freeBytes: 1, totalBytes: 2, demoBytes: 0, artifactBytes: 0 },
    },
    pendingUsers: 0,
    ...patch,
  };
}

function cells(view: AdminOverview, failures: AdminJob[] = []) {
  return Object.fromEntries(queueSummary(view, failures).map((cell) => [cell.key, cell]));
}

test("a busy worker is named after what it is doing, and only capturing reads as recording", () => {
  const busy = worker({ state: "busy" });
  assert.equal(workerStatus(busy, job({ stage: "capturing" })).label, "Grabando");
  assert.equal(workerStatus(busy, job({ stage: "rendering" })).label, "Montando");
  assert.equal(workerStatus(busy, null).label, "Ocupado");
});

test("the most severe worker condition wins", () => {
  const paused = worker({ online: false, paused: true, state: "busy" });
  assert.deepEqual(workerStatus(paused, null), { tone: "bad", label: "Sin conexión" });
  assert.equal(workerStatus(worker({ paused: true, state: "busy" }), null).label, "En pausa");
  assert.equal(workerStatus(worker({ blocked: { code: "disk_full", detail: "" } }), null).tone, "warn");
});

test("an idle cloud with nothing queued reads as calm", () => {
  const summary = cells(overview({}));
  assert.equal(summary.worker?.value, "Libre");
  assert.equal(summary.now?.value, "Nada en curso");
  assert.equal(summary.waiting?.value, "Nadie espera");
  assert.equal(summary.failures?.value, "Ninguno");
});

test("a running job shows its stage, percent, title and owner", () => {
  const running = job({ id: "j9", status: "running", stage: "capturing", progressPercent: 62, workerId: "w1" });
  const view = overview({ workers: [worker({ state: "busy", currentJobId: "j9" })], running: [running] });
  const summary = cells(view);
  assert.equal(summary.worker?.value, "Grabando");
  assert.equal(summary.now?.value, "Grabando 62 %");
  assert.equal(summary.now?.detail, "R3 4k · Luis");
  assert.equal(summary.now?.href, "/admin/jobs/j9");
});

test("the progress of a job whose worker went silent is not shown as live", () => {
  const running = job({ id: "j9", status: "running", stage: "rendering", progressPercent: 40, workerId: "w1" });
  const silent = worker({ online: false, state: "busy", currentJobId: "j9", lastSeenAt: NOW - 70_000 });
  const now = cells(overview({ workers: [silent], running: [running] })).now;
  assert.equal(now?.value, "Parado");
  assert.equal(now?.tone, "bad");
  assert.equal(now?.detail, "R3 4k · Luis · se quedó en montando 40 %, sin señal del worker");
  // A revoked worker is not listed at all, and its job is just as stuck.
  assert.equal(cells(overview({ workers: [], running: [running] })).now?.value, "Parado");
});

test("waiting jobs count people, and warn when no worker can take them", () => {
  const queue = [
    job({ id: "a", waitedSeconds: 120 }),
    job({ id: "b", waitedSeconds: 900, user: { id: "u2", name: null, email: "b@example.com" } }),
    job({ id: "c", waitedSeconds: 30 }),
  ];
  const fine = cells(overview({ queue })).waiting;
  assert.equal(fine?.value, "3 trabajos");
  assert.equal(fine?.detail, "de 2 personas · el más antiguo espera 15 min");
  assert.equal(fine?.tone, "plain");

  const paused = overview({ queue, workers: [worker({ paused: true, pausedBy: "auto", pauseReason: "disk_full: C:" })] });
  const stuck = cells(paused);
  assert.equal(stuck.waiting?.tone, "warn");
  assert.match(stuck.waiting?.detail ?? "", /ahora nadie los puede grabar$/);
  assert.equal(stuck.worker?.value, "En pausa");
  assert.equal(stuck.worker?.detail, "Disco lleno");
});

test("an offline worker says since when, and no worker at all points to Workers", () => {
  const offline = cells(overview({ workers: [worker({ online: false, lastSeenAt: NOW - 180_000 })] })).worker;
  assert.equal(offline?.value, "Sin conexión");
  assert.equal(offline?.detail, "última señal hace 3 min");
  const none = cells(overview({ workers: [] })).worker;
  assert.equal(none?.tone, "bad");
  assert.equal(none?.href, "/admin/workers");
});

test("failures link to the history and name the latest cause", () => {
  const failed = [
    job({ id: "old", status: "failed", finishedAt: NOW - 3 * 3600_000, failure: { code: "timeout", message: "" } }),
    job({ id: "new", status: "failed", finishedAt: NOW - 120_000, failure: { code: "render_failed", message: "" } }),
    job({ id: "stale", status: "failed", finishedAt: NOW - 30 * 3600_000, failure: { code: "internal", message: "" } }),
  ];
  assert.deepEqual(
    recentFailures(failed, NOW).map((entry) => entry.id),
    ["new", "old"],
  );
  const view = overview({ stats: { ...overview({}).stats, failed24h: 2 } });
  const cell = cells(view, failed).failures;
  assert.equal(cell?.value, "2");
  assert.equal(cell?.detail, "último: Fallo de montaje · hace 2 min");
  assert.equal(cell?.href, "/admin/history?status=failed");
});

test("the cause headline is the first non-empty line, cut to one row", () => {
  assert.equal(causeHeadline("\nffmpeg exit 1: boom\nConversion failed!"), "ffmpeg exit 1: boom");
  assert.equal(causeHeadline(null), null);
  assert.equal(causeHeadline("   "), null);
  assert.equal(causeHeadline("x".repeat(300))?.length, 160);
});

test("a silent worker's job says when it returns to the queue", () => {
  const running = job({ status: "running", leaseExpiresAt: NOW + 45_000 });
  assert.equal(leaseOutlook(running, NOW), "Si no da señal en 45 s, el trabajo vuelve a la cola.");
  assert.match(leaseOutlook(job({ status: "running", leaseExpiresAt: NOW - 1 }), NOW), /ha caducado/);
  assert.match(leaseOutlook(job({ status: "uploading", leaseExpiresAt: NOW - 1 }), NOW), /media hora más/);
});

test("an idle report right after a claim is not a lost job, a lasting one is", () => {
  const idle = worker({ state: "idle", currentJobId: "j1" });
  const fresh = job({ status: "running", stage: "downloading", startedAt: NOW - 5_000 });
  const old = job({ status: "running", stage: "capturing", startedAt: NOW - 120_000 });
  assert.equal(workerStatus(idle, fresh).label, "Preparando");
  assert.equal(forgotItsJob(idle, fresh, NOW), false);
  assert.equal(forgotItsJob(idle, old, NOW), true);
  assert.equal(forgotItsJob(worker({ state: "busy" }), old, NOW), false);
  assert.equal(forgotItsJob(idle, null, NOW), false);
});
