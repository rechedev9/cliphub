import { test } from "node:test";
import assert from "node:assert/strict";

import { loadRetentionConfig, retentionCutoffs } from "./retention.ts";

test("loadRetentionConfig falls back to sane defaults", () => {
  assert.deepEqual(loadRetentionConfig({}), {
    retentionDays: 30,
    abandonedUploadHours: 24,
    sweepIntervalMs: 60 * 60 * 1000,
  });
});

test("loadRetentionConfig reads env overrides", () => {
  assert.deepEqual(
    loadRetentionConfig({
      RETENTION_DAYS: "14",
      ABANDONED_UPLOAD_HOURS: "6",
      RETENTION_SWEEP_INTERVAL_MS: "1000",
    }),
    { retentionDays: 14, abandonedUploadHours: 6, sweepIntervalMs: 1000 },
  );
});

test("retentionCutoffs subtracts the configured windows from now", () => {
  const now = new Date("2026-09-07T12:00:00.000Z");
  const cutoffs = retentionCutoffs(now, {
    retentionDays: 30,
    abandonedUploadHours: 24,
    sweepIntervalMs: 1000,
  });
  assert.equal(cutoffs.terminalBefore.toISOString(), "2026-08-08T12:00:00.000Z");
  assert.equal(
    cutoffs.awaitingDemoBefore.toISOString(),
    "2026-09-06T12:00:00.000Z",
  );
});

test("a shorter retention window moves the cutoff closer to now", () => {
  const now = new Date("2026-09-07T12:00:00.000Z");
  const cutoffs = retentionCutoffs(now, {
    retentionDays: 1,
    abandonedUploadHours: 1,
    sweepIntervalMs: 1000,
  });
  assert.equal(cutoffs.terminalBefore.toISOString(), "2026-09-06T12:00:00.000Z");
  assert.equal(
    cutoffs.awaitingDemoBefore.toISOString(),
    "2026-09-07T11:00:00.000Z",
  );
});
