import { test } from "node:test";
import assert from "node:assert/strict";

import {
  checkCreateLimits,
  effectiveLimits,
  loadAccessDefault,
  loadUserLimits,
  resolveAccess,
  type CreateLimitInput,
} from "./user-limits.ts";

const LIMITS = { maxActive: 3, dailySeconds: 5400 };

function check(overrides: Partial<CreateLimitInput>) {
  return checkCreateLimits({
    limits: LIMITS,
    kind: "short",
    active: 0,
    activeFullDemo: 0,
    usedSeconds: 0,
    committedSeconds: 0,
    estimatedSeconds: 480,
    ...overrides,
  });
}

test("limits default to 3 active jobs and 90 minutes a day", () => {
  assert.deepEqual(loadUserLimits({}), LIMITS);
});

test("limits read env overrides and ignore values that are not positive integers", () => {
  assert.deepEqual(
    loadUserLimits({ MAX_ACTIVE_REQUESTS_PER_USER: "5", MAX_DAILY_SECONDS_PER_USER: "600" }),
    { maxActive: 5, dailySeconds: 600 },
  );
  assert.deepEqual(
    loadUserLimits({ MAX_ACTIVE_REQUESTS_PER_USER: "abc", MAX_DAILY_SECONDS_PER_USER: "-1" }),
    LIMITS,
  );
});

test("a per-user override replaces only the limit it sets", () => {
  assert.deepEqual(effectiveLimits(LIMITS, { maxActive: 10, dailySeconds: null }), {
    maxActive: 10,
    dailySeconds: 5400,
  });
  assert.deepEqual(effectiveLimits(LIMITS, null), LIMITS);
});

test("a job within every limit is allowed", () => {
  assert.equal(check({ active: 2, usedSeconds: 1000, committedSeconds: 1000 }), null);
});

test("the active cap rejects the job that would exceed it", () => {
  assert.equal(check({ active: 3 }), "limit_active");
});

test("only one full demo can be active per user", () => {
  assert.equal(check({ kind: "full_demo", active: 1, activeFullDemo: 1 }), "limit_full_demo");
  assert.equal(check({ kind: "short", active: 1, activeFullDemo: 1 }), null);
});

test("used time, queued estimates and the new job must fit the daily budget", () => {
  assert.equal(check({ usedSeconds: 3000, committedSeconds: 1920 }), null);
  assert.equal(check({ usedSeconds: 3000, committedSeconds: 1921 }), "limit_daily");
  assert.equal(check({ estimatedSeconds: 5401 }), "limit_daily");
});

test("access falls back to the default, and to pending when the default is not valid", () => {
  assert.equal(loadAccessDefault({ CLOUD_ACCESS_DEFAULT: "allowed" }), "allowed");
  assert.equal(loadAccessDefault({ CLOUD_ACCESS_DEFAULT: "everyone" }), "pending");
  assert.equal(loadAccessDefault({}), "pending");
  assert.equal(resolveAccess({ stored: null, isAdmin: false, accessDefault: "pending" }), "pending");
  assert.equal(resolveAccess({ stored: "blocked", isAdmin: false, accessDefault: "allowed" }), "blocked");
});

test("an admin is always allowed, even when stored as blocked", () => {
  assert.equal(resolveAccess({ stored: "blocked", isAdmin: true, accessDefault: "pending" }), "allowed");
});
