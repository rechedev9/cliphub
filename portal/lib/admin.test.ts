import { test } from "node:test";
import assert from "node:assert/strict";

import { parseAdminAccounts } from "./admin.ts";

test("parses a single provider:id pair", () => {
  assert.deepEqual(parseAdminAccounts("google:12345"), [
    { provider: "google", providerAccountId: "12345" },
  ]);
});

test("parses multiple comma-separated pairs and trims whitespace", () => {
  assert.deepEqual(parseAdminAccounts(" google:123 , discord:456 "), [
    { provider: "google", providerAccountId: "123" },
    { provider: "discord", providerAccountId: "456" },
  ]);
});

test("returns an empty list for undefined/empty input", () => {
  assert.deepEqual(parseAdminAccounts(undefined), []);
  assert.deepEqual(parseAdminAccounts(""), []);
});

test("skips malformed entries missing a provider or id", () => {
  assert.deepEqual(parseAdminAccounts("google:,:456,justtext,google:789"), [
    { provider: "google", providerAccountId: "789" },
  ]);
});
