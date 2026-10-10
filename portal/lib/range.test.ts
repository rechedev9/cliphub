import { test } from "node:test";
import assert from "node:assert/strict";

import { parseRange } from "./range.ts";

test("no Range header means the whole file", () => {
  assert.equal(parseRange(null, 1000), null);
});

test("an open range runs to the last byte", () => {
  assert.deepEqual(parseRange("bytes=0-", 1000), { start: 0, end: 999 });
  assert.deepEqual(parseRange("bytes=400-", 1000), { start: 400, end: 999 });
});

test("a closed range is inclusive and clipped to the file", () => {
  assert.deepEqual(parseRange("bytes=0-0", 1000), { start: 0, end: 0 });
  assert.deepEqual(parseRange("bytes=100-199", 1000), { start: 100, end: 199 });
  assert.deepEqual(parseRange("bytes=900-5000", 1000), { start: 900, end: 999 });
});

test("a range that starts at or past the end cannot be served", () => {
  assert.equal(parseRange("bytes=1000-", 1000), "invalid");
  assert.equal(parseRange("bytes=0-", 0), "invalid");
});

test("reversed, suffix, multiple and malformed ranges are invalid", () => {
  assert.equal(parseRange("bytes=200-100", 1000), "invalid");
  assert.equal(parseRange("bytes=-500", 1000), "invalid");
  assert.equal(parseRange("bytes=0-10,20-30", 1000), "invalid");
  assert.equal(parseRange("items=0-10", 1000), "invalid");
  assert.equal(parseRange("bytes=abc-", 1000), "invalid");
});
