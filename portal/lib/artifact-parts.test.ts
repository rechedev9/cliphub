import { test } from "node:test";
import assert from "node:assert/strict";

import {
  ARTIFACT_VARIANT_PATTERN,
  addReceivedPart,
  expectedPartLength,
  isArtifactName,
  missingParts,
  parseReceivedParts,
  partCount,
} from "./artifact-parts.ts";

const PART = 32 * 1024 * 1024;

test("the part count rounds up and a file that fits has one part", () => {
  assert.equal(partCount(48211332, PART), 2);
  assert.equal(partCount(PART, PART), 1);
  assert.equal(partCount(PART + 1, PART), 2);
  assert.equal(partCount(1, PART), 1);
});

test("every part is a full part except the last, which carries the remainder", () => {
  const sizeBytes = 48211332;
  assert.equal(expectedPartLength({ sizeBytes, partSize: PART, part: 1 }), PART);
  assert.equal(expectedPartLength({ sizeBytes, partSize: PART, part: 2 }), sizeBytes - PART);
  assert.equal(expectedPartLength({ sizeBytes: 2 * PART, partSize: PART, part: 2 }), PART);
});

test("a part number outside the file has no length", () => {
  const sizeBytes = 48211332;
  assert.equal(expectedPartLength({ sizeBytes, partSize: PART, part: 0 }), null);
  assert.equal(expectedPartLength({ sizeBytes, partSize: PART, part: 3 }), null);
  assert.equal(expectedPartLength({ sizeBytes, partSize: PART, part: 1.5 }), null);
});

test("missing parts are the ones not yet received, in order", () => {
  assert.deepEqual(missingParts(4, [3, 1]), [2, 4]);
  assert.deepEqual(missingParts(2, [1, 2]), []);
});

test("the received parts column survives bad data and re-sent parts", () => {
  assert.deepEqual(parseReceivedParts(null), []);
  assert.deepEqual(parseReceivedParts("not json"), []);
  assert.deepEqual(parseReceivedParts('{"a":1}'), []);
  assert.deepEqual(parseReceivedParts('[3,1,"2",1,-4]'), [1, 3]);
  assert.deepEqual(addReceivedPart("[1,3]", 2), [1, 2, 3]);
  assert.deepEqual(addReceivedPart("[1,3]", 3), [1, 3]);
});

test("artifact names cannot carry a path and must match their kind", () => {
  assert.equal(isArtifactName("short-01.mp4", "video"), true);
  assert.equal(isArtifactName("cover_01.jpg", "cover"), true);
  assert.equal(isArtifactName("short-01.jpg", "video"), false);
  assert.equal(isArtifactName("../short-01.mp4", "video"), false);
  assert.equal(isArtifactName("dir/short.mp4", "video"), false);
  assert.equal(isArtifactName(".hidden.mp4", "video"), false);
  assert.equal(isArtifactName("short.exe", "video"), false);
  assert.equal(isArtifactName(`${"a".repeat(121)}.mp4`, "video"), false);
});

test("variants are short lowercase slugs", () => {
  assert.equal(ARTIFACT_VARIANT_PATTERN.test("viral-60-clean"), true);
  assert.equal(ARTIFACT_VARIANT_PATTERN.test("Viral"), false);
  assert.equal(ARTIFACT_VARIANT_PATTERN.test("a/b"), false);
  assert.equal(ARTIFACT_VARIANT_PATTERN.test(""), false);
});
