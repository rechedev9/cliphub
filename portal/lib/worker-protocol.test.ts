import { test } from "node:test";
import assert from "node:assert/strict";

import {
  parseArtifactInit,
  parseAttemptHeader,
  parseFail,
  parseHeartbeat,
  parseKinds,
  parsePhase,
} from "./worker-protocol.ts";

const SHA = "b".repeat(64);

function beat(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    state: "busy",
    blocked: null,
    health: {
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
    },
    jobs: [
      {
        id: "job-1",
        phase: "running",
        attempt: 1,
        stage: "capturing",
        percent: 62,
        detail: "REC 3/5",
        localJobId: "l-1",
      },
    ],
    ...overrides,
  };
}

test("the contract's heartbeat example is accepted as is", () => {
  const body = parseHeartbeat(beat());
  assert.equal(body?.state, "busy");
  assert.equal(body?.blocked, null);
  assert.equal(body?.health.cs2PatchVersion, "1.41.8.5");
  assert.deepEqual(body?.health.kinds, ["short"]);
  assert.equal(body?.health.planSchema, "kill-plan/3");
  assert.deepEqual(body?.jobs, [
    {
      id: "job-1",
      phase: "running",
      attempt: 1,
      stage: "capturing",
      percent: 62,
      detail: "REC 3/5",
      localJobId: "l-1",
    },
  ]);
});

test("a job entry without a usable attempt is kept, with no attempt, for the store to answer as lost", () => {
  const attempts = [undefined, 0, -1, 1.5, "1"].map(
    (attempt) => parseHeartbeat(beat({ jobs: [{ id: "job-1", phase: "running", attempt }] }))?.jobs[0]?.attempt,
  );
  assert.deepEqual(attempts, [null, null, null, null, null]);
  assert.equal(parseHeartbeat(beat({ health: {} }))?.health.planSchema, "");
});

test("a heartbeat is never refused for the number of jobs: the first 200 are read", () => {
  const many = Array.from({ length: 250 }, (_, i) => ({ id: `job-${i}`, phase: "uploading", attempt: 1 }));
  const body = parseHeartbeat(beat({ jobs: many }));
  assert.equal(body?.jobs.length, 200);
  assert.equal(body?.jobs[199]?.id, "job-199");
});

test("the attempt header is a positive decimal integer and nothing else", () => {
  const header = (value: string | null) =>
    parseAttemptHeader(
      new Request("http://portal.test/x", { headers: value === null ? {} : { "X-ClipHub-Attempt": value } }),
    );
  assert.equal(header("1"), 1);
  assert.equal(header("12"), 12);
  for (const bad of [null, "", "0", "-1", "1.0", "01", "2x", "1e3"]) assert.equal(header(bad), null);
});

test("a blocked worker reports a known code; an unknown one is refused", () => {
  const blocked = parseHeartbeat(beat({ state: "blocked", blocked: { code: "steam_unavailable", detail: "no steam.exe" }, jobs: [] }));
  assert.deepEqual(blocked?.blocked, { code: "steam_unavailable", detail: "no steam.exe" });
  assert.equal(parseHeartbeat(beat({ blocked: { code: "on_fire", detail: "" } })), null);
});

test("a heartbeat with an unknown state or a bad job is refused", () => {
  assert.equal(parseHeartbeat(beat({ state: "sleeping" })), null);
  assert.equal(parseHeartbeat(beat({ jobs: [{ id: "job-1", phase: "resting" }] })), null);
  assert.equal(parseHeartbeat(beat({ jobs: [{ phase: "running" }] })), null);
  assert.equal(parseHeartbeat("busy"), null);
});

test("missing health never blocks a heartbeat: unknown values become empty", () => {
  const body = parseHeartbeat(beat({ health: undefined }));
  assert.equal(body?.health.hlaeVersion, "");
  assert.equal(body?.health.steamRunning, false);
  assert.equal(body?.health.diskFreeBytes, 0);
});

test("progress is clamped and an uploading job carries no stage", () => {
  const body = parseHeartbeat(
    beat({
      jobs: [
        { id: "a", phase: "running", stage: "defragmenting", percent: 140 },
        { id: "b", phase: "uploading", stage: "rendering", percent: -3 },
      ],
    }),
  );
  assert.deepEqual(body?.jobs.map((job) => [job.stage, job.percent]), [
    [null, 100],
    [null, 0],
  ]);
});

test("claim kinds keep only the kinds the portal knows", () => {
  assert.deepEqual(parseKinds(["short", "stream_reel", "short", 7]), ["short"]);
  assert.deepEqual(parseKinds(undefined), []);
});

test("the phase call only moves a job to uploading", () => {
  assert.deepEqual(parsePhase({ phase: "uploading", machineSeconds: 512, localJobId: "l-1" }), {
    machineSeconds: 512,
    localJobId: "l-1",
  });
  assert.equal(parsePhase({ phase: "done" }), null);
});

test("an artifact needs a safe name for its kind, a slug variant, a size and a hash", () => {
  const valid = { name: "short-01.mp4", kind: "video", variant: "viral-60-clean", sizeBytes: 48211332, sha256: SHA };
  assert.deepEqual(parseArtifactInit(valid), valid);
  assert.equal(parseArtifactInit({ ...valid, name: "../../etc/passwd.mp4" }), null);
  assert.equal(parseArtifactInit({ ...valid, name: "cover.jpg" }), null);
  assert.notEqual(parseArtifactInit({ ...valid, name: "cover.jpg", kind: "cover" }), null);
  assert.equal(parseArtifactInit({ ...valid, variant: "Viral 60" }), null);
  assert.equal(parseArtifactInit({ ...valid, sizeBytes: 0 }), null);
  assert.equal(parseArtifactInit({ ...valid, sha256: "abc" }), null);
  assert.equal(parseArtifactInit({ ...valid, kind: "executable" }), null);
});

test("a failure report needs a known code; its texts are capped", () => {
  const body = parseFail({ code: "capture_flake", message: "m".repeat(900), detail: "d".repeat(9000), machineSeconds: 300 });
  assert.equal(body?.code, "capture_flake");
  assert.equal(body?.message.length, 500);
  assert.equal(body?.detail.length, 4000);
  assert.equal(body?.machineSeconds, 300);
  assert.equal(parseFail({ code: "bad_luck" }), null);
  assert.equal(parseFail({ code: "timeout", machineSeconds: -5 })?.machineSeconds, 0);
});
