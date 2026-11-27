import { strict as assert } from "node:assert";
import { test } from "node:test";
import { communityURL, DEFAULT_COMMUNITY_URL, registeredMatches } from "./registry.ts";
import { fakeCommunity } from "./testing/fake-community.ts";

test("registeredMatches asks in batches and keeps only the ids it asked for", async () => {
  const api = await fakeCommunity(new Set([3, 700]));
  const ids = Array.from({ length: 701 }, (_, i) => i + 1);
  const reply = await registeredMatches({ base: api.url, ids: [...ids, 3] });
  assert.deepEqual(reply.ok && [...reply.registered].sort((a, b) => a - b), [3, 700]);
  assert.equal(api.calls.length, 2);
});

test("registeredMatches reports a failing, unreachable or turned off API instead of throwing", async () => {
  const broken = await fakeCommunity(new Set(), 500);
  assert.deepEqual(await registeredMatches({ base: broken.url, ids: [1] }), { ok: false, error: "community API answered 500" });
  const down = await fakeCommunity(new Set());
  await new Promise<void>((resolve) => down.server.close(() => resolve()));
  assert.equal((await registeredMatches({ base: down.url, ids: [1], timeoutMs: 2000 })).ok, false);
  assert.equal((await registeredMatches({ base: "", ids: [1] })).ok, false);
});

test("communityURL takes the flag, then the environment, then the public server, and off turns it off", () => {
  assert.equal(communityURL("http://a/", { CLIPHUB_COMMUNITY_URL: "http://b" }), "http://a");
  assert.equal(communityURL("", { CLIPHUB_COMMUNITY_URL: "http://b" }), "http://b");
  assert.equal(communityURL("", {}), DEFAULT_COMMUNITY_URL);
  assert.equal(communityURL("off", {}), "");
});
