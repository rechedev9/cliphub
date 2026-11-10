import { strict as assert } from "node:assert";
import { test } from "node:test";
import { FailureBreaker, isTransient, withRetry } from "./retry.ts";

function harness(attempts: number) {
  const waits: number[] = [];
  const retried: number[] = [];
  return {
    waits,
    retried,
    opts: { attempts, baseMs: 100, sleep: async (ms: number) => { waits.push(ms); }, onRetry: (n: number) => { retried.push(n); }, isTransient: () => true },
  };
}

test("withRetry returns the first success without waiting", async () => {
  const h = harness(3);
  assert.equal(await withRetry(async () => "ok", h.opts), "ok");
  assert.deepEqual(h.waits, []);
});

test("withRetry backs off exponentially and succeeds after transient failures", async () => {
  const h = harness(4);
  let calls = 0;
  const got = await withRetry(async () => {
    calls++;
    if (calls < 3) throw new Error("ERR_CONNECTION_CLOSED");
    return calls;
  }, h.opts);
  assert.equal(got, 3);
  assert.deepEqual(h.waits, [100, 200]);
  assert.deepEqual(h.retried, [1, 2]);
});

test("withRetry gives up after the last attempt and throws the last error", async () => {
  const h = harness(3);
  let calls = 0;
  await assert.rejects(withRetry(async () => { calls++; throw new Error(`fail ${calls}`); }, h.opts), /fail 3/);
  assert.equal(calls, 3);
  assert.deepEqual(h.waits, [100, 200]);
});

test("withRetry rethrows a non-transient error at once", async () => {
  const h = harness(4);
  let calls = 0;
  await assert.rejects(
    withRetry(async () => { calls++; throw new Error("ERR_NAME_NOT_RESOLVED"); }, { ...h.opts, isTransient }),
    /ERR_NAME_NOT_RESOLVED/,
  );
  assert.equal(calls, 1);
  assert.deepEqual(h.waits, []);
});

test("isTransient accepts cut connections only", () => {
  for (const code of ["ERR_CONNECTION_CLOSED", "ERR_CONNECTION_RESET", "ERR_TIMED_OUT", "ERR_ABORTED", "ERR_NETWORK_CHANGED", "ERR_HTTP2_PROTOCOL_ERROR"]) {
    assert.equal(isTransient(new Error(`${code} (-100) loading 'https://www.hltv.org/'`)), true, code);
  }
  for (const msg of ["ERR_NAME_NOT_RESOLVED", "ERR_INVALID_URL", "Object has been destroyed"]) {
    assert.equal(isTransient(new Error(msg)), false, msg);
  }
});

test("FailureBreaker trips after the limit in a row and a success resets it", () => {
  const b = new FailureBreaker(3);
  assert.deepEqual([b.failure(), b.failure()], [false, false]);
  b.success();
  assert.deepEqual([b.failure(), b.failure(), b.failure()], [false, false, true]);
});
