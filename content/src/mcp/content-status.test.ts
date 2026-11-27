import assert from "node:assert/strict";
import path from "node:path";
import { afterEach, test } from "node:test";
import { at, FAKE_STUDIO, openHarness, type Harness } from "../testing/harness.ts";

let harness: Harness | null = null;

afterEach(async () => {
  await harness?.close();
  harness = null;
});

test("content_status reads studio_status from the Studio MCP it spawns", async () => {
  harness = await openHarness({ studioMcp: { command: process.execPath, args: ["--no-warnings", FAKE_STUDIO] } });
  const status = await harness.call("content_status");
  assert.equal(status.isError, false, status.text);
  assert.equal(at(status.body, "config.found"), true);
  assert.equal(at(status.body, "state.path"), path.join(harness.dir, "state.db"));
  assert.equal(at(status.body, "state.schemaVersion"), 1);
  assert.equal(at(status.body, "studio.answers"), true);
  assert.equal(at(status.body, "studio.status.version"), "fake-studio");
});

test("content_status says when the Studio MCP is missing or does not answer", async () => {
  harness = await openHarness();
  const unset = await harness.call("content_status");
  assert.equal(at(unset.body, "studio.configured"), false);
  assert.equal(at(unset.body, "studio.answers"), false);
  await harness.close();
  harness = null;

  harness = await openHarness({ studioMcp: { command: path.join(path.dirname(FAKE_STUDIO), "missing-zv-mcp.exe") } });
  const broken = await harness.call("content_status");
  assert.equal(broken.isError, false);
  assert.equal(at(broken.body, "studio.configured"), true);
  assert.equal(at(broken.body, "studio.answers"), false);
  assert.equal(typeof at(broken.body, "studio.error"), "string");
});
