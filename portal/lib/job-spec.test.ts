import { test } from "node:test";
import assert from "node:assert/strict";

import { parseJobSpec } from "./job-spec.ts";

function shortSpec(): Record<string, unknown> {
  return {
    version: 1,
    targetSteamId: "76561198000000001",
    rules: { pre_roll: 3 },
    capture: {
      tickrate: 64,
      windows: [
        { id: "seg-003", tickStart: 6400, tickEnd: 7040 },
        { id: "seg-007", tickStart: 12800, tickEnd: 14080 },
      ],
    },
    generate: {
      preset: "viral-60-clean",
      music: null,
      segment_ids: ["seg-003", "seg-007"],
      edit: { captions: true },
    },
    client: { studioVersion: "5.4.4" },
  };
}

function withCapture(capture: Record<string, unknown>): Record<string, unknown> {
  return { ...shortSpec(), capture };
}

function expectRejected(input: unknown, pattern: RegExp): void {
  const parsed = parseJobSpec(input, "short");
  assert.equal(parsed.ok, false);
  if (!parsed.ok) assert.match(parsed.error, pattern);
}

test("a valid short spec is accepted and its capture time includes 2 s per window", () => {
  const parsed = parseJobSpec(shortSpec(), "short");
  assert.equal(parsed.ok, true);
  if (!parsed.ok) return;
  // 640 and 1280 ticks at 64 tick are 10 s and 20 s, plus 2 s for each of the 2 windows.
  assert.equal(parsed.captureSeconds, 34);
  assert.deepEqual(parsed.spec.capture.windows.map((w) => w.id), ["seg-003", "seg-007"]);
  assert.deepEqual(parsed.spec.generate, shortSpec().generate);
  assert.deepEqual(parsed.spec.rules, { pre_roll: 3 });
});

test("a spec that is not version 1 is rejected", () => {
  expectRejected({ ...shortSpec(), version: 2 }, /version/);
  expectRejected("nope", /object/);
});

test("the target must be a 17 digit SteamID64", () => {
  expectRejected({ ...shortSpec(), targetSteamId: "STEAM_0:1:123" }, /targetSteamId/);
  expectRejected({ ...shortSpec(), targetSteamId: "7656119800000000" }, /targetSteamId/);
});

test("tickrate must be an integer from 16 to 256", () => {
  const windows = [{ id: "a", tickStart: 0, tickEnd: 10 }];
  expectRejected(withCapture({ tickrate: 15, windows }), /tickrate/);
  expectRejected(withCapture({ tickrate: 257, windows }), /tickrate/);
  expectRejected(withCapture({ tickrate: 64.5, windows }), /tickrate/);
});

test("a spec needs between 1 and 60 windows", () => {
  expectRejected(withCapture({ tickrate: 64, windows: [] }), /1 to 60/);
  const many = Array.from({ length: 61 }, (_, i) => ({
    id: `seg-${i}`,
    tickStart: i * 100,
    tickEnd: i * 100 + 64,
  }));
  expectRejected(withCapture({ tickrate: 64, windows: many }), /1 to 60/);
});

test("window ids must be unique and match the id pattern", () => {
  const twice = [
    { id: "seg-1", tickStart: 0, tickEnd: 64 },
    { id: "seg-1", tickStart: 100, tickEnd: 164 },
  ];
  expectRejected(withCapture({ tickrate: 64, windows: twice }), /repeated/);
  const badId = [{ id: "../etc", tickStart: 0, tickEnd: 64 }];
  expectRejected(withCapture({ tickrate: 64, windows: badId }), /window id/);
});

test("ticks must satisfy 0 <= tickStart < tickEnd", () => {
  for (const [tickStart, tickEnd] of [[-1, 64], [64, 64], [100, 64], [0.5, 64]]) {
    const windows = [{ id: "seg-1", tickStart, tickEnd }];
    expectRejected(withCapture({ tickrate: 64, windows }), /ticks/);
  }
});

test("a window longer than 180 s is rejected, exactly 180 s is accepted", () => {
  const spec = (ticks: number) => ({
    ...shortSpec(),
    capture: { tickrate: 64, windows: [{ id: "seg-1", tickStart: 0, tickEnd: ticks }] },
    generate: { preset: "p", segment_ids: ["seg-1"] },
  });
  assert.equal(parseJobSpec(spec(180 * 64), "short").ok, true);
  expectRejected(spec(180 * 64 + 1), /longer than 180/);
});

test("windows adding up to more than 900 s are rejected", () => {
  const windows = Array.from({ length: 6 }, (_, i) => ({
    id: `seg-${i}`,
    tickStart: i * 20000,
    tickEnd: i * 20000 + 151 * 64,
  }));
  const spec = {
    ...shortSpec(),
    capture: { tickrate: 64, windows },
    generate: { preset: "p", segment_ids: windows.map((w) => w.id) },
  };
  expectRejected(spec, /900/);
});

test("segment_ids must equal the window ids in order", () => {
  const reordered = { ...shortSpec(), generate: { preset: "p", segment_ids: ["seg-007", "seg-003"] } };
  expectRejected(reordered, /segment_ids/);
  const partial = { ...shortSpec(), generate: { preset: "p", segment_ids: ["seg-003"] } };
  expectRejected(partial, /segment_ids/);
  const missing = { ...shortSpec(), generate: { preset: "p" } };
  expectRejected(missing, /segment_ids/);
});

test("the preset must be a non-empty string of at most 64 characters", () => {
  const ids = ["seg-003", "seg-007"];
  expectRejected({ ...shortSpec(), generate: { preset: "", segment_ids: ids } }, /preset/);
  expectRejected(
    { ...shortSpec(), generate: { preset: "x".repeat(65), segment_ids: ids } },
    /preset/,
  );
});

test("music and edit must be objects or null", () => {
  const ids = ["seg-003", "seg-007"];
  expectRejected(
    { ...shortSpec(), generate: { preset: "p", segment_ids: ids, music: "track.mp3" } },
    /music/,
  );
  expectRejected(
    { ...shortSpec(), generate: { preset: "p", segment_ids: ids, edit: [1] } },
    /edit/,
  );
});

test("a spec over 64 KB once serialized is rejected", () => {
  const spec = { ...shortSpec(), rules: { padding: "x".repeat(65536) } };
  expectRejected(spec, /65536/);
});

test("a full demo spec needs its options and no segment ids", () => {
  const spec = {
    ...shortSpec(),
    generate: { preset: "full", segment_ids: [], edit: null },
    fullDemo: { options: { hud: true } },
  };
  assert.equal(parseJobSpec(spec, "full_demo").ok, true);
  assert.equal(parseJobSpec({ ...spec, fullDemo: undefined }, "full_demo").ok, false);
  assert.equal(parseJobSpec(shortSpec(), "full_demo").ok, false);
});
