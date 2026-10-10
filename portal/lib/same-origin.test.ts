import { test } from "node:test";
import assert from "node:assert/strict";

import { isSameOrigin, portalOrigin } from "./same-origin.ts";

const ENV = { AUTH_URL: "https://cliphub.gravityroom.app" };

function post(origin?: string): Request {
  return new Request("http://127.0.0.1:3000/api/link/approve", {
    method: "POST",
    headers: origin === undefined ? {} : { origin },
  });
}

test("a request from the portal's own origin passes", () => {
  assert.equal(isSameOrigin(post("https://cliphub.gravityroom.app"), ENV), true);
});

test("a request from another site, or with no Origin header, is refused", () => {
  assert.equal(isSameOrigin(post("https://evil.example"), ENV), false);
  assert.equal(isSameOrigin(post("https://cliphub.gravityroom.app.evil.example"), ENV), false);
  assert.equal(isSameOrigin(post("http://cliphub.gravityroom.app"), ENV), false);
  assert.equal(isSameOrigin(post("null"), ENV), false);
  assert.equal(isSameOrigin(post(), ENV), false);
});

test("AUTH_URL with a path still compares by origin only", () => {
  const env = { AUTH_URL: "https://cliphub.gravityroom.app/api/auth" };
  assert.equal(isSameOrigin(post("https://cliphub.gravityroom.app"), env), true);
});

test("without AUTH_URL the request's own origin is the reference", () => {
  assert.equal(isSameOrigin(post("http://127.0.0.1:3000"), {}), true);
  assert.equal(isSameOrigin(post("http://localhost:3000"), {}), false);
  assert.equal(portalOrigin(post(), {}), "http://127.0.0.1:3000");
});
