import { test } from "node:test";
import assert from "node:assert/strict";

import { isDemoHeader } from "./demo-validate.ts";

test("accepts a CS2 (Source 2) demo header", () => {
  assert.equal(isDemoHeader(Buffer.from("PBDEMS2\x00rest-of-demo")), true);
});

test("accepts a legacy GOTV (Source 1) demo header", () => {
  assert.equal(isDemoHeader(Buffer.from("HL2DEMO\x00rest-of-demo")), true);
});

test("rejects an unrelated file", () => {
  assert.equal(isDemoHeader(Buffer.from("just some bytes")), false);
});

test("rejects a short non-demo body", () => {
  assert.equal(isDemoHeader(Buffer.from("PB2")), false);
});

test("rejects a compressed .dem.bz2 (BZh magic)", () => {
  assert.equal(isDemoHeader(Buffer.from("BZh91AY&SY")), false);
});
