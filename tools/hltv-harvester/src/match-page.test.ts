import { strict as assert } from "node:assert";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { parseHTML } from "linkedom";
import { MATCH_SCRIPT, parseMatch, parseSidecar, withSeries } from "./hltv.ts";

const page = readFileSync(new URL("./testdata/match-veto.html", import.meta.url), "utf8");

function readPage(html: string): unknown {
  const { document } = parseHTML(html);
  const run: unknown = new Function("document", `return ${MATCH_SCRIPT}`);
  if (typeof run !== "function") throw new Error("page script did not compile");
  const out: unknown = run(document);
  return out;
}

test("MATCH_SCRIPT reads the teams, maps, demo and veto of a match page", () => {
  const match = parseMatch(readPage(page));
  assert.ok(match);
  assert.deepEqual(match.teams.map((t) => t.id), [8297, 12394]);
  assert.equal(match.event, "IEM Beijing 2026");
  assert.deepEqual(match.maps, ["Ancient", "Dust2", "Nuke"]);
  assert.deepEqual(match.demoPaths, ["/download/demo/113501"]);
  assert.equal(match.format, "bo3");
  assert.deepEqual(match.veto.map((v) => `${v.team}:${v.action}:${v.map}`), [
    "BETBOOM:ban:Inferno", "FURIA:ban:Mirage", "FURIA:pick:Ancient", "BETBOOM:pick:Dust2",
    "BETBOOM:ban:Train", "FURIA:ban:Anubis", ":decider:Nuke",
  ]);
});

test("a veto backfill fills a sidecar without a veto only", () => {
  const sidecar = parseSidecar({ source: "hltv", match_id: 2400007, demo_path: "/download/demo/113501", event: "IEM Beijing 2026", played_at: "", teams: [], maps: ["Ancient", "Dust2", "Nuke"] });
  assert.ok(sidecar);
  const filled = withSeries(sidecar, parseMatch(readPage(page)));
  assert.equal(filled?.format, "bo3");
  assert.equal(filled?.veto?.length, 7);
  assert.equal(filled && withSeries(filled, parseMatch(readPage(page))), null);
  assert.equal(withSeries(sidecar, parseMatch(readPage("<html><body></body></html>"))), null);
});
