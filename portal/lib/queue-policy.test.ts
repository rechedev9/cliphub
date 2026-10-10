import { test } from "node:test";
import assert from "node:assert/strict";

import {
  calibration,
  estimateSeconds,
  maxRuntimeSeconds,
  pickNext,
  queueStateOf,
  selectCandidates,
  simulateQueue,
  usage24h,
  usageByUser,
  type QueueJob,
  type UsageJob,
} from "./queue-policy.ts";

const NOW = 1_760_000_000_000;
const MINUTE = 60_000;
const HOUR = 60 * MINUTE;
const NO_USAGE = new Map<string, number>();
const BOTH_KINDS = ["short", "full_demo"] as const;

function job(overrides: Partial<QueueJob> & { id: string }): QueueJob {
  return {
    userId: `user-${overrides.id}`,
    kind: "short",
    enqueuedAt: NOW,
    estimatedSeconds: 480,
    priorityBoost: 0,
    ...overrides,
  };
}

function pick(candidates: QueueJob[], usage = NO_USAGE): string | undefined {
  return pickNext({ candidates, usageByUser: usage, now: NOW })?.id;
}

function finished(overrides: Partial<UsageJob>): UsageJob {
  return {
    userId: "u1",
    status: "done",
    finishedAt: NOW - HOUR,
    machineSeconds: 600,
    estimatedSeconds: 480,
    failureCode: null,
    canceledBy: null,
    ...overrides,
  };
}

test("the estimate follows the formula and scales with calibration", () => {
  // 240 + 1.3 * 100 + 1.5 * 100
  assert.equal(estimateSeconds({ kind: "short", captureSeconds: 100, calibration: 1 }), 520);
  // 420 + 1.3 * 1800 + 1.6 * 1800
  assert.equal(estimateSeconds({ kind: "full_demo", captureSeconds: 1800, calibration: 1 }), 5640);
  assert.equal(estimateSeconds({ kind: "short", captureSeconds: 100, calibration: 1.5 }), 780);
});

test("the max runtime is three estimates, between 20 minutes and 4 hours", () => {
  assert.equal(maxRuntimeSeconds(100), 1200);
  assert.equal(maxRuntimeSeconds(1000), 3000);
  assert.equal(maxRuntimeSeconds(10000), 14400);
});

test("calibration stays 1 until there are 5 samples", () => {
  const samples = Array.from({ length: 4 }, () => ({ machineSeconds: 900, uncalibratedSeconds: 300 }));
  assert.equal(calibration(samples), 1);
});

test("calibration is the median ratio, clamped to 0.5 and 3", () => {
  const ratios = (values: number[]) =>
    values.map((ratio) => ({ machineSeconds: ratio * 100, uncalibratedSeconds: 100 }));
  assert.equal(calibration(ratios([1, 1.2, 1.4, 1.6, 9])), 1.4);
  assert.equal(calibration(ratios([1, 1.2, 1.4, 1.6, 1.8, 2])), 1.5);
  assert.equal(calibration(ratios([5, 5, 5, 5, 5])), 3);
  assert.equal(calibration(ratios([0.1, 0.1, 0.1, 0.1, 0.1])), 0.5);
});

test("calibration only looks at the 30 newest samples", () => {
  const recent = Array.from({ length: 30 }, () => ({ machineSeconds: 200, uncalibratedSeconds: 100 }));
  const old = Array.from({ length: 40 }, () => ({ machineSeconds: 50, uncalibratedSeconds: 100 }));
  assert.equal(calibration([...recent, ...old]), 2);
});

test("usage counts done jobs, charged failures and the user's own cancels of the last 24 h", () => {
  const jobs = [
    finished({ machineSeconds: 600 }),
    finished({ status: "failed", failureCode: "render_failed", machineSeconds: 300 }),
    finished({ status: "failed", failureCode: "worker_lost", machineSeconds: 100 }),
    finished({ status: "canceled", canceledBy: "user", machineSeconds: 50 }),
    finished({ status: "canceled", canceledBy: "admin", machineSeconds: 5000 }),
    finished({ finishedAt: NOW - 25 * HOUR, machineSeconds: 5000 }),
  ];
  assert.equal(usage24h(jobs, NOW), 1050);
});

test("a job in progress counts its machine time, or its estimate before any is reported", () => {
  const jobs = [
    finished({ status: "running", finishedAt: null, machineSeconds: 0, estimatedSeconds: 480 }),
    finished({ status: "uploading", finishedAt: null, machineSeconds: 700, estimatedSeconds: 480 }),
    finished({ status: "queued", finishedAt: null, machineSeconds: 0, estimatedSeconds: 900 }),
  ];
  assert.equal(usage24h(jobs, NOW), 1180);
});

test("a retried job that runs again counts its estimate on top of what the failed attempt used", () => {
  const retried = finished({ status: "running", finishedAt: null, machineSeconds: 5, estimatedSeconds: 2700 });
  assert.equal(usage24h([retried], NOW), 2705);
  const uploading = finished({ status: "uploading", finishedAt: null, machineSeconds: 0, estimatedSeconds: 2700 });
  assert.equal(usage24h([uploading], NOW), 0);
});

test("usageByUser keeps each user's total apart", () => {
  const usage = usageByUser(
    [finished({ userId: "a" }), finished({ userId: "a" }), finished({ userId: "b", machineSeconds: 10 })],
    NOW,
  );
  assert.equal(usage.get("a"), 1200);
  assert.equal(usage.get("b"), 10);
});

test("a fresh Short beats a fresh Full Demo", () => {
  const fullDemo = job({ id: "full", kind: "full_demo", estimatedSeconds: 5600, enqueuedAt: NOW - 60_000 });
  const short = job({ id: "short", estimatedSeconds: 480, enqueuedAt: NOW - 30_000 });
  assert.equal(pick([fullDemo, short]), "short");
});

test("a Full Demo waiting 2.5 h beats a fresh Short", () => {
  const fullDemo = job({ id: "full", kind: "full_demo", estimatedSeconds: 5600, enqueuedAt: NOW - 2.5 * HOUR });
  const short = job({ id: "short", estimatedSeconds: 360, enqueuedAt: NOW - 30_000 });
  assert.equal(pick([short, fullDemo]), "full");
});

test("overdue jobs go strictly oldest first, whatever their score", () => {
  const heavy = new Map([["user-old", 100_000]]);
  const oldest = job({ id: "old", kind: "full_demo", estimatedSeconds: 5600, enqueuedAt: NOW - 5 * HOUR });
  const overdue = job({ id: "overdue", enqueuedAt: NOW - 3 * HOUR });
  const fresh = job({ id: "fresh", enqueuedAt: NOW - MINUTE });
  assert.equal(pick([fresh, overdue, oldest], heavy), "old");
  assert.equal(pick([fresh, overdue], heavy), "overdue");
});

test("a job is not overdue one second before the 3 hour mark", () => {
  const heavy = new Map([["user-almost", 100_000]]);
  const almost = job({ id: "almost", enqueuedAt: NOW - 3 * HOUR + 1000 });
  const fresh = job({ id: "fresh", enqueuedAt: NOW - 30 * MINUTE });
  assert.equal(pick([almost, fresh], heavy), "fresh");
});

test("boosted jobs go first, highest boost first", () => {
  const overdue = job({ id: "overdue", enqueuedAt: NOW - 6 * HOUR });
  const boosted = job({ id: "boosted", priorityBoost: 1 });
  const front = job({ id: "front", priorityBoost: 2 });
  assert.equal(pick([overdue, boosted, front]), "front");
  assert.equal(pick([overdue, boosted]), "boosted");
});

test("at equal wait a heavy user loses to a light user", () => {
  const usage = new Map([["heavy", 3600]]);
  const heavy = job({ id: "a", userId: "heavy", enqueuedAt: NOW - 10 * MINUTE });
  const light = job({ id: "b", userId: "light", enqueuedAt: NOW - 10 * MINUTE });
  assert.equal(pick([heavy, light], usage), "b");
  assert.equal(pick([heavy, light]), "a");
});

test("ties go to the oldest job, then to the lowest id", () => {
  const first = job({ id: "b", enqueuedAt: NOW - 1 });
  const second = job({ id: "a", enqueuedAt: NOW });
  assert.equal(pickNext({ candidates: [second, first], usageByUser: NO_USAGE, now: NOW - 1 })?.id, "b");
  const sameTime = [job({ id: "z" }), job({ id: "m" })];
  assert.equal(pick(sameTime), "m");
});

test("an empty queue picks nothing", () => {
  assert.equal(pickNext({ candidates: [], usageByUser: NO_USAGE, now: NOW }), null);
});

test("only a user's oldest job is a candidate", () => {
  const queued = [
    job({ id: "u1-new", userId: "u1", enqueuedAt: NOW - MINUTE }),
    job({ id: "u1-old", userId: "u1", enqueuedAt: NOW - 10 * MINUTE }),
    job({ id: "u2", userId: "u2", enqueuedAt: NOW - 5 * MINUTE }),
  ];
  const candidates = selectCandidates({ queued, kinds: BOTH_KINDS, blockedUserIds: new Set() });
  assert.deepEqual(candidates.map((c) => c.id).sort(), ["u1-old", "u2"]);
});

test("a boosted job is a candidate even when it is not its user's oldest", () => {
  const queued = [
    job({ id: "old", userId: "u1", enqueuedAt: NOW - 10 * MINUTE }),
    job({ id: "boosted", userId: "u1", enqueuedAt: NOW - MINUTE, priorityBoost: 1 }),
  ];
  const candidates = selectCandidates({ queued, kinds: BOTH_KINDS, blockedUserIds: new Set() });
  assert.deepEqual(candidates.map((c) => c.id).sort(), ["boosted", "old"]);
});

test("blocked users and kinds the worker does not take are left out", () => {
  const queued = [
    job({ id: "blocked", userId: "bad" }),
    job({ id: "full", userId: "u2", kind: "full_demo", enqueuedAt: NOW - HOUR }),
    job({ id: "short", userId: "u2" }),
  ];
  const candidates = selectCandidates({ queued, kinds: ["short"], blockedUserIds: new Set(["bad"]) });
  assert.deepEqual(candidates.map((c) => c.id), ["short"]);
});

test("the simulation numbers positions 1..n with no gaps and chains the start times", () => {
  const queued = [
    job({ id: "a", userId: "u1", enqueuedAt: NOW - 30 * MINUTE, estimatedSeconds: 600 }),
    job({ id: "b", userId: "u1", enqueuedAt: NOW - 20 * MINUTE, estimatedSeconds: 300 }),
    job({ id: "c", userId: "u2", enqueuedAt: NOW - 10 * MINUTE, estimatedSeconds: 400 }),
    job({ id: "d", userId: "u3", enqueuedAt: NOW - 5 * MINUTE, estimatedSeconds: 500 }),
  ];
  const placements = simulateQueue({ queued, running: [], usageByUser: NO_USAGE, now: NOW, queueState: "online" });
  const positions = [...placements.values()].map((p) => p.position).sort((x, y) => x - y);
  assert.deepEqual(positions, [1, 2, 3, 4]);

  const ordered = [...placements.entries()].sort((x, y) => x[1].position - y[1].position);
  let clock = NOW;
  for (const [id, placement] of ordered) {
    const estimated = queued.find((q) => q.id === id)?.estimatedSeconds ?? 0;
    assert.equal(placement.estimatedStartAt, clock);
    clock += estimated * 1000;
    assert.equal(placement.estimatedDoneAt, clock);
  }
});

test("a user's second job waits behind other users once the first one is placed", () => {
  const queued = [
    job({ id: "a1", userId: "a", enqueuedAt: NOW - 30 * MINUTE, estimatedSeconds: 1800 }),
    job({ id: "a2", userId: "a", enqueuedAt: NOW - 29 * MINUTE, estimatedSeconds: 1800 }),
    job({ id: "b1", userId: "b", enqueuedAt: NOW - 20 * MINUTE, estimatedSeconds: 1800 }),
  ];
  const placements = simulateQueue({ queued, running: [], usageByUser: NO_USAGE, now: NOW, queueState: "online" });
  assert.equal(placements.get("a1")?.position, 1);
  assert.equal(placements.get("b1")?.position, 2);
  assert.equal(placements.get("a2")?.position, 3);
});

test("the simulation starts after the running job, with at least a minute left", () => {
  const queued = [job({ id: "a" })];
  const midway = simulateQueue({
    queued,
    running: [{ estimatedSeconds: 600, claimedAt: NOW - 200_000 }],
    usageByUser: NO_USAGE,
    now: NOW,
    queueState: "online",
  });
  assert.equal(midway.get("a")?.estimatedStartAt, NOW + 400_000);

  const overrun = simulateQueue({
    queued,
    running: [{ estimatedSeconds: 600, claimedAt: NOW - 900_000 }],
    usageByUser: NO_USAGE,
    now: NOW,
    queueState: "online",
  });
  assert.equal(overrun.get("a")?.estimatedStartAt, NOW + 60_000);
});

test("the simulation gives positions but no times when the queue is not online", () => {
  const queued = [job({ id: "a", enqueuedAt: NOW - MINUTE }), job({ id: "b" })];
  for (const queueState of ["paused", "offline"] as const) {
    const placements = simulateQueue({ queued, running: [], usageByUser: NO_USAGE, now: NOW, queueState });
    assert.deepEqual(placements.get("a"), { position: 1, estimatedStartAt: null, estimatedDoneAt: null });
    assert.deepEqual(placements.get("b"), { position: 2, estimatedStartAt: null, estimatedDoneAt: null });
  }
});

test("the queue is offline with no worker, or when the last heartbeat is over a minute old", () => {
  const worker = { lastSeenAt: NOW - 61_000, revokedAt: null, paused: false, state: "idle" };
  assert.equal(queueStateOf([], NOW), "offline");
  assert.equal(queueStateOf([worker], NOW), "offline");
  assert.equal(queueStateOf([{ ...worker, lastSeenAt: null }], NOW), "offline");
  assert.equal(queueStateOf([{ ...worker, lastSeenAt: NOW - 60_000 }], NOW), "online");
});

test("a paused or blocked worker makes the queue paused, a revoked one does not count", () => {
  const worker = { lastSeenAt: NOW - 5000, revokedAt: null, paused: false, state: "busy" };
  assert.equal(queueStateOf([worker], NOW), "online");
  assert.equal(queueStateOf([{ ...worker, paused: true }], NOW), "paused");
  assert.equal(queueStateOf([{ ...worker, state: "blocked" }], NOW), "paused");
  assert.equal(queueStateOf([{ ...worker, revokedAt: NOW - 1000 }], NOW), "offline");
  assert.equal(queueStateOf([{ ...worker, paused: true }, worker], NOW), "online");
});
