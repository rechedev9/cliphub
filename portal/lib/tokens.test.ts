import { test } from "node:test";
import assert from "node:assert/strict";

import {
  bearerToken,
  formatUserCode,
  hashToken,
  isDeviceToken,
  isWorkerToken,
  newDeviceToken,
  newPollToken,
  newUserCode,
  newWorkerToken,
  normalizeUserCode,
} from "./tokens.ts";

test("a worker token is 64 lowercase hex characters, as ZV_BRIDGE_TOKEN requires", () => {
  const token = newWorkerToken();
  assert.match(token, /^[0-9a-f]{64}$/);
  assert.equal(isWorkerToken(token), true);
  assert.notEqual(newWorkerToken(), token);
});

test("a device token is chd_ plus 64 hex and is not mistaken for a worker token", () => {
  const token = newDeviceToken();
  assert.match(token, /^chd_[0-9a-f]{64}$/);
  assert.equal(isDeviceToken(token), true);
  assert.equal(isWorkerToken(token), false);
  assert.equal(isDeviceToken(newWorkerToken()), false);
  assert.match(newPollToken(), /^[0-9a-f]{64}$/);
});

test("tokens are stored as their lowercase hex sha256", () => {
  assert.equal(
    hashToken("abc"),
    "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
  );
});

test("a user code has 8 characters from the unambiguous alphabet and shows as XXXX-XXXX", () => {
  for (let i = 0; i < 200; i += 1) {
    assert.match(newUserCode(), /^[ABCDEFGHJKLMNPQRSTUVWXYZ23456789]{8}$/);
  }
  assert.equal(formatUserCode("K7QM2XHD"), "K7QM-2XHD");
});

test("a typed code is normalized: case, dash and spaces do not matter", () => {
  assert.equal(normalizeUserCode("k7qm-2xhd"), "K7QM2XHD");
  assert.equal(normalizeUserCode(" K7QM 2XHD "), "K7QM2XHD");
  assert.equal(normalizeUserCode(formatUserCode("K7QM2XHD")), "K7QM2XHD");
});

test("a code with the wrong length or a character outside the alphabet is rejected", () => {
  assert.equal(normalizeUserCode("K7QM-2XH"), null);
  assert.equal(normalizeUserCode("K7QM-2XH0"), null);
  assert.equal(normalizeUserCode("K7QM-2XHDX"), null);
  assert.equal(normalizeUserCode(""), null);
});

test("bearerToken reads the Authorization header and ignores other schemes", () => {
  const withHeader = (value: string) =>
    new Request("http://portal.test/", { headers: { authorization: value } });
  assert.equal(bearerToken(withHeader("Bearer abc")), "abc");
  assert.equal(bearerToken(withHeader("Basic abc")), null);
  assert.equal(bearerToken(new Request("http://portal.test/")), null);
});
