import { test } from "node:test";
import assert from "node:assert/strict";

import { isBridgeAuthorized } from "./bridge-auth.ts";

function requestWith(authorization?: string): Request {
  const headers = new Headers();
  if (authorization !== undefined) headers.set("authorization", authorization);
  return new Request("http://localhost/api/bridge/claim", { headers });
}

test("rejects when BRIDGE_SHARED_SECRET is unset", () => {
  const original = process.env.BRIDGE_SHARED_SECRET;
  delete process.env.BRIDGE_SHARED_SECRET;
  try {
    assert.equal(isBridgeAuthorized(requestWith("Bearer anything")), false);
  } finally {
    if (original !== undefined) process.env.BRIDGE_SHARED_SECRET = original;
  }
});

test("rejects a missing Authorization header", () => {
  process.env.BRIDGE_SHARED_SECRET = "secret-token";
  assert.equal(isBridgeAuthorized(requestWith()), false);
});

test("rejects a header without the Bearer prefix", () => {
  process.env.BRIDGE_SHARED_SECRET = "secret-token";
  assert.equal(isBridgeAuthorized(requestWith("secret-token")), false);
});

test("rejects a wrong token", () => {
  process.env.BRIDGE_SHARED_SECRET = "secret-token";
  assert.equal(isBridgeAuthorized(requestWith("Bearer wrong-token")), false);
});

test("accepts the correct token", () => {
  process.env.BRIDGE_SHARED_SECRET = "secret-token";
  assert.equal(isBridgeAuthorized(requestWith("Bearer secret-token")), true);
});
