import { strict as assert } from "node:assert";
import { test } from "node:test";
import { withRetry } from "./retry.ts";

function harness(attempts: number) {
  const waits: number[] = [];
  const retried: number[] = [];
  return {
    waits,
    retried,
    opts: { attempts, baseMs: 100, sleep: async (ms: number) => { waits.push(ms); }, onRetry: (n: number) => { retried.push(n); } },
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
