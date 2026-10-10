import { test } from "node:test";
import assert from "node:assert/strict";

import {
  captureSeconds,
  estimateRangeText,
  formatBytes,
  formatDuration,
  formatRelative,
  normalizeUserCode,
  percentOf,
  safeCallbackPath,
} from "./format.ts";

const MIN = 60_000;

test("durations drop seconds from one minute up and split hours", () => {
  assert.equal(formatDuration(45), "45 s");
  assert.equal(formatDuration(480), "8 min");
  assert.equal(formatDuration(3900), "1 h 5 min");
  assert.equal(formatDuration(7200), "2 h");
  assert.equal(formatDuration(-5), "0 s");
});

test("a duration just under an hour does not read as 60 min", () => {
  assert.equal(formatDuration(3599), "1 h");
});

test("byte sizes use binary units with a Spanish decimal comma", () => {
  assert.equal(formatBytes(512), "512 B");
  assert.equal(formatBytes(48_211_332), "46,0 MB");
  assert.equal(formatBytes(412_000_000_000), "384 GB");
});

test("relative time picks the largest unit and never reports the future", () => {
  const now = 1_760_000_000_000;
  assert.equal(formatRelative(now - 2000, now), "ahora");
  assert.equal(formatRelative(now + 60_000, now), "ahora");
  assert.equal(formatRelative(now - 40_000, now), "hace 40 s");
  assert.equal(formatRelative(now - 26 * 60 * MIN, now), "hace 1 d");
});

test("the start estimate is a range from the estimate to 35 percent later", () => {
  const now = 1_760_000_000_000;
  assert.equal(estimateRangeText(now + 20 * MIN, now), "empieza en unos 20 a 30 min");
  assert.equal(estimateRangeText(now + 4 * MIN, now), "empieza en unos 4 a 6 min");
});

test("an estimate that is due or already past does not show a negative range", () => {
  const now = 1_760_000_000_000;
  assert.equal(estimateRangeText(now + 20_000, now), "empieza en menos de un minuto");
  assert.equal(estimateRangeText(now - 5 * MIN, now), "empieza en menos de un minuto");
});

test("long waits are given in hours", () => {
  const now = 1_760_000_000_000;
  assert.equal(estimateRangeText(now + 120 * MIN, now), "empieza en 2 h a 2 h 45 min");
});

test("percentOf clamps and survives a zero total", () => {
  assert.equal(percentOf(412, 1000), 41);
  assert.equal(percentOf(5, 0), 0);
  assert.equal(percentOf(20, 10), 100);
});

test("capture seconds follow the contract formula: window time plus 2 s per window", () => {
  const spec = {
    targetSteamId: "76561198000000001",
    tickrate: 64,
    preset: null,
    planSchema: null,
    windows: [
      { id: "a", tickStart: 0, tickEnd: 640 },
      { id: "b", tickStart: 1000, tickEnd: 1320 },
    ],
  };
  assert.equal(captureSeconds(spec), 19);
  assert.equal(captureSeconds({ ...spec, tickrate: 0 }), 0);
});

test("link codes are accepted in any case, with or without the dash", () => {
  assert.equal(normalizeUserCode("k7qm2xhd"), "K7QM-2XHD");
  assert.equal(normalizeUserCode(" K7QM-2XHD "), "K7QM-2XHD");
});

test("link codes with a wrong length or a letter outside the alphabet are rejected", () => {
  assert.equal(normalizeUserCode("K7QM-2XH"), null);
  assert.equal(normalizeUserCode("K7QM-2XH0"), null);
  assert.equal(normalizeUserCode("K7QM-2XHI"), null);
});

test("the post-login destination must be a path on this site", () => {
  assert.equal(safeCallbackPath("/link?code=K7QM-2XHD"), "/link?code=K7QM-2XHD");
  assert.equal(safeCallbackPath(undefined), null);
  assert.equal(safeCallbackPath("https://evil.example/link"), null);
  assert.equal(safeCallbackPath("//evil.example"), null);
  assert.equal(safeCallbackPath("/\\evil.example"), null);
});
