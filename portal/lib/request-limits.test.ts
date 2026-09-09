import { test } from "node:test";
import assert from "node:assert/strict";

import { checkRequestLimits, loadRequestLimits } from "./request-limits.ts";

test("loadRequestLimits falls back to sane defaults", () => {
  assert.deepEqual(loadRequestLimits({}), { maxActive: 3, maxDaily: 5 });
});

test("loadRequestLimits reads env overrides", () => {
  assert.deepEqual(
    loadRequestLimits({ MAX_ACTIVE_REQUESTS_PER_USER: "10", MAX_DAILY_REQUESTS_PER_USER: "20" }),
    { maxActive: 10, maxDaily: 20 },
  );
});

test("allows a request under both caps", () => {
  assert.equal(
    checkRequestLimits({ active: 1, daily: 1 }, { maxActive: 3, maxDaily: 5 }),
    null,
  );
});

test("flags the active cap when reached", () => {
  assert.equal(
    checkRequestLimits({ active: 3, daily: 1 }, { maxActive: 3, maxDaily: 5 }),
    "active",
  );
});

test("flags the daily cap when reached, even under the active cap", () => {
  assert.equal(
    checkRequestLimits({ active: 0, daily: 5 }, { maxActive: 3, maxDaily: 5 }),
    "daily",
  );
});

test("active cap is checked before the daily cap", () => {
  assert.equal(
    checkRequestLimits({ active: 3, daily: 5 }, { maxActive: 3, maxDaily: 5 }),
    "active",
  );
});
