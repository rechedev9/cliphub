import { strict as assert } from "node:assert";
import { mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { needsVisit } from "./hltv.ts";
import { analysesOf, hasLocalDemo, loadState, recordMatch, splitRegistered, storePaths } from "./store.ts";

test("one state per store keeps every match by its HLTV id across runs", () => {
  const paths = storePaths(mkdtempSync(join(tmpdir(), "hltv-store-")));
  const state = loadState(paths);
  recordMatch({ paths, state, matchId: 7, rec: { status: "downloaded", file: "x.rar" } });
  recordMatch({ paths, state, matchId: 8, rec: { status: "registered" } });
  const again = loadState(paths);
  assert.deepEqual(Object.keys(again.matches).sort(), ["7", "8"]);
  assert.equal(needsVisit(again, 8), false);
});

test("a match is local while its archive waits in downloads or its demos are kept", () => {
  const paths = storePaths(mkdtempSync(join(tmpdir(), "hltv-store-")));
  assert.equal(hasLocalDemo(paths, 5), false);
  mkdirSync(paths.demos, { recursive: true });
  writeFileSync(join(paths.demos, "5__a-vs-b-m1-nuke.dem"), "");
  writeFileSync(join(paths.demos, "55__a-vs-b-m1-nuke.dem"), "");
  assert.equal(hasLocalDemo(paths, 5), true);
  mkdirSync(paths.downloads, { recursive: true });
  writeFileSync(join(paths.downloads, "6.rar"), "");
  assert.equal(hasLocalDemo(paths, 6), true);
  mkdirSync(paths.analyses, { recursive: true });
  writeFileSync(join(paths.analyses, "5__a-vs-b-m1-nuke.zvpro.zst"), "");
  writeFileSync(join(paths.analyses, "55__a-vs-b-m1-nuke.zvpro.zst"), "");
  assert.deepEqual(analysesOf(paths, 5), ["5__a-vs-b-m1-nuke.zvpro.zst"]);
});

test("splitRegistered drops what the VPS holds and keeps everything when the API failed", () => {
  const rows = [{ matchId: 1 }, { matchId: 2 }];
  assert.deepEqual(splitRegistered(rows, { ok: true, registered: new Set([2]) }), { todo: [{ matchId: 1 }], registered: [{ matchId: 2 }] });
  assert.deepEqual(splitRegistered(rows, { ok: false, error: "down" }), { todo: rows, registered: [] });
});
