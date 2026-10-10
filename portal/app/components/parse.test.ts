import { test } from "node:test";
import assert from "node:assert/strict";

import { splitPauseReason } from "./labels.ts";
import {
  parseAdminJobDetail,
  parseAdminJobs,
  parseAdminWorkers,
  parseErrorCode,
  parseLinkInfo,
  parseOverview,
  parseStudioJobs,
  parseStudioMe,
} from "./parse.ts";

// The StudioJob example of contract section H, verbatim apart from the elided ids.
const CONTRACT_STUDIO_JOB = {
  id: "j1",
  kind: "short",
  title: "t",
  status: "queued",
  stage: null,
  progressPercent: null,
  createdAt: 0,
  enqueuedAt: 0,
  startedAt: null,
  finishedAt: null,
  estimatedSeconds: 480,
  attempt: 0,
  cancelRequested: false,
  queue: { position: 3, estimatedStartAt: 1760001500000, estimatedDoneAt: 1760001980000, state: "online" },
  failure: null,
  artifacts: [
    {
      id: "a1",
      kind: "video",
      variant: "viral-60-clean",
      name: "short-01.mp4",
      sizeBytes: 48211332,
      sha256: "x",
      expiresAt: 1760600000000,
    },
  ],
};

const CONTRACT_ME = {
  user: { id: "u", name: "Luis", email: "l@example.com", image: null },
  access: "allowed",
  limits: { maxActive: 3, dailySeconds: 5400, maxDemoBytes: 734003200 },
  usage: { active: 1, secondsLast24h: 1200, secondsCommitted: 480 },
  kinds: ["short"],
  queue: { state: "online", queued: 4, waitSeconds: { short: 1500 } },
};

const ADMIN_JOB = {
  ...CONTRACT_STUDIO_JOB,
  user: { id: "u", name: null, email: "l@example.com" },
  progressDetail: null,
  workerId: null,
  localJobId: null,
  leaseExpiresAt: null,
  priorityBoost: 0,
  machineSeconds: 0,
  maxAttempts: 2,
  waitedSeconds: 60,
  failureDetail: null,
  canceledBy: null,
  demo: { fileName: "match.dem", sizeBytes: 213000000, sha256: "x", present: true },
  legacyStatus: false,
};

test("the contract examples of the user API parse", () => {
  const jobs = parseStudioJobs({ jobs: [CONTRACT_STUDIO_JOB] });
  assert.equal(jobs.jobs[0]?.queue?.position, 3);
  assert.equal(jobs.jobs[0]?.artifacts[0]?.name, "short-01.mp4");
  assert.equal(parseStudioMe(CONTRACT_ME).limits.dailySeconds, 5400);
});

test("a bad field is reported with its path instead of reaching the UI", () => {
  const broken = { jobs: [{ ...CONTRACT_STUDIO_JOB, attempt: "0" }] };
  assert.throws(() => parseStudioJobs(broken), /jobs\.jobs\[0\]\.attempt/);
});

test("an unknown status is rejected rather than rendered as a blank pill", () => {
  const broken = { jobs: [{ ...CONTRACT_STUDIO_JOB, status: "exploded" }] };
  assert.throws(() => parseStudioJobs(broken), /status/);
});

test("a body that is not the expected object is rejected", () => {
  assert.throws(() => parseOverview(null));
  assert.throws(() => parseOverview([]));
  assert.throws(() => parseAdminJobs({ jobs: "none", nextBefore: null }), /jobs\.jobs/);
});

test("legacy manual rows parse with their old status and no cloud fields", () => {
  const row = {
    ...ADMIN_JOB,
    kind: "manual",
    status: "pending",
    title: null,
    queue: null,
    estimatedSeconds: null,
    enqueuedAt: null,
    waitedSeconds: null,
    legacyStatus: true,
  };
  const legacy = parseAdminJobs({ jobs: [row], nextBefore: null }).jobs[0];
  assert.equal(legacy?.kind, "manual");
  assert.equal(legacy?.status, "pending");
  assert.equal(legacy?.estimatedSeconds, null);
});

test("the empty title and demo values the portal sends for a legacy row read as missing", () => {
  const row = {
    ...ADMIN_JOB,
    kind: "manual",
    status: "pending",
    title: "",
    demo: { fileName: "vieja.dem", sizeBytes: 0, sha256: "", present: true },
    legacyStatus: true,
  };
  const legacy = parseAdminJobs({ jobs: [row], nextBefore: null }).jobs[0];
  assert.equal(legacy?.title, null);
  assert.deepEqual(legacy?.demo, { fileName: "vieja.dem", sizeBytes: null, sha256: null, present: true });
  assert.equal(parseStudioJobs({ jobs: [{ ...CONTRACT_STUDIO_JOB, title: "  " }] }).jobs[0]?.title, null);
});

const NEW_WORKER = {
  id: "w",
  name: "nuevo",
  online: false,
  lastSeenAt: null,
  state: null,
  blocked: null,
  paused: false,
  pausedBy: null,
  pauseReason: null,
  revoked: false,
  createdAt: 1,
  health: null,
  currentJobId: null,
};

const HEALTH = {
  hostname: "capture-01",
  studioVersion: "5.4.4",
  cs2PatchVersion: "1.41.8.5",
  hlaeVersion: "2.192.6",
  steamRunning: true,
  recordEnabled: true,
  diskFreeBytes: 412000000000,
  diskTotalBytes: 1000000000000,
  kinds: ["short"],
  uptimeSeconds: 86400,
  planSchema: "kill-plan/3",
};

test("a worker that never sent a heartbeat parses with no health", () => {
  assert.equal(parseAdminWorkers({ workers: [NEW_WORKER] }).workers[0]?.health, null);
});

test("a worker's health carries its plan schema, and a health without it is rejected", () => {
  const parsed = parseAdminWorkers({ workers: [{ ...NEW_WORKER, health: HEALTH }] });
  assert.equal(parsed.workers[0]?.health?.planSchema, "kill-plan/3");
  const { planSchema: _dropped, ...older } = HEALTH;
  assert.throws(
    () => parseAdminWorkers({ workers: [{ ...NEW_WORKER, health: older }] }),
    /workers\[0\]\.health\.planSchema/,
  );
});

test("an admin job carries what the worker says it is doing; an empty detail reads as none", () => {
  const running = { ...ADMIN_JOB, status: "running", queue: null, progressDetail: "REC 2/5" };
  assert.equal(parseAdminJobs({ jobs: [running], nextBefore: null }).jobs[0]?.progressDetail, "REC 2/5");
  const silent = { ...running, progressDetail: "" };
  assert.equal(parseAdminJobs({ jobs: [silent], nextBefore: null }).jobs[0]?.progressDetail, null);
  assert.throws(
    () => parseAdminJobs({ jobs: [{ ...running, progressDetail: 5 }], nextBefore: null }),
    /progressDetail/,
  );
});

test("a link description must say whether it was asked for from this network", () => {
  const link = { deviceName: "DESKTOP-ABC", expiresAt: 5, sameNetwork: false };
  assert.deepEqual(parseLinkInfo(link), link);
  assert.throws(() => parseLinkInfo({ deviceName: "DESKTOP-ABC", expiresAt: 5 }), /link\.sameNetwork/);
});

test("the spec summary keeps the windows and the preset and ignores the opaque parts", () => {
  const spec = {
    version: 1,
    targetSteamId: "76561198000000001",
    rules: { anything: true },
    capture: {
      tickrate: 64,
      windows: [
        { id: "seg-003", tickStart: 51234, tickEnd: 52010 },
        { id: "seg-004", tickStart: 60100, tickEnd: 61500 },
      ],
    },
    generate: { preset: "viral-60-clean", music: {}, segment_ids: ["seg-003", "seg-004"], edit: {} },
    client: { studioVersion: "5.4.4", planSchema: "1" },
  };
  const detail = parseAdminJobDetail({ job: ADMIN_JOB, spec, events: [] });
  assert.equal(detail.spec?.preset, "viral-60-clean");
  assert.equal(detail.spec?.windows.length, 2);
  assert.equal(detail.spec?.tickrate, 64);
  assert.equal(detail.spec?.planSchema, "1");
  const { client: _client, ...anonymous } = spec;
  assert.equal(parseAdminJobDetail({ job: ADMIN_JOB, spec: anonymous, events: [] }).spec?.planSchema, null);
});

test("error bodies yield the code, falling back to the error text", () => {
  assert.equal(parseErrorCode({ error: "conflict", code: "invalid_state" }), "invalid_state");
  assert.equal(parseErrorCode({ error: "unauthorized" }), "unauthorized");
  assert.equal(parseErrorCode("<html>502</html>"), null);
});

test("an automatic pause reason is split into a readable cause and the raw detail", () => {
  assert.deepEqual(splitPauseReason("capture_incompatible: recorder exit 6: AfxHookSource2"), {
    label: "HLAE incompatible con CS2",
    detail: "recorder exit 6: AfxHookSource2",
    job: null,
  });
  assert.deepEqual(splitPauseReason("consecutive_failures"), {
    label: "3 fallos seguidos",
    detail: null,
    job: null,
  });
});

test("an automatic pause reason names the job that caused it", () => {
  const reason = 'capture_incompatible: job 0b7c "R3 4k: ace": recorder exit 6: AfxHookSource2';
  assert.deepEqual(splitPauseReason(reason), {
    label: "HLAE incompatible con CS2",
    detail: "recorder exit 6: AfxHookSource2",
    job: { id: "0b7c", title: "R3 4k: ace" },
  });
  assert.deepEqual(splitPauseReason('consecutive_failures: job 0b7c "": '), {
    label: "3 fallos seguidos",
    detail: null,
    job: { id: "0b7c", title: "" },
  });
});

test("an operator's free-text pause reason is shown as written, colon included", () => {
  assert.deepEqual(splitPauseReason("Mantenimiento: cambio de disco"), {
    label: "Mantenimiento: cambio de disco",
    detail: null,
    job: null,
  });
  assert.equal(splitPauseReason(null).label, "Sin motivo indicado");
});
