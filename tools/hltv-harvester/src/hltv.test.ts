import { strict as assert } from "node:assert";
import { test } from "node:test";
import { isCandidate, needsVisit, parseArgs, parseMatch, parseRanking, parseResults, parseSidecar, parseState, rankedTeamIn, selectCandidates, sidecarFor, windowMs } from "./hltv.ts";

test("parseRanking keeps teams with an id, rank and name", () => {
  const got = parseRanking([
    { pos: "#1", name: "FURIA", href: "/team/8297/furia" },
    { pos: "#2", name: "", href: "/team/9565/vitality" },
    { pos: "x", name: "Spirit", href: "/team/7020/spirit" },
    "junk",
  ]);
  assert.deepEqual(got, [{ id: 8297, name: "FURIA", rank: 1 }]);
});

test("parseResults drops rows without a match path or time", () => {
  const got = parseResults([
    { href: "/matches/2400009/mouz-vs-betboom", team1: "MOUZ", team2: "BetBoom", event: "IEM", unix: "1794044700000" },
    { href: "/news/1/x", team1: "a", team2: "b", event: "", unix: "1" },
    { href: "/matches/2/x", team1: "a", team2: "b", event: "", unix: "" },
  ]);
  assert.equal(got.length, 1);
  assert.equal(got[0].matchId, 2400009);
});

test("candidate rows and ranked teams match by normalized name and by id", () => {
  const row = { matchId: 1, path: "/matches/1/x", team1: "Natus Vincere", team2: "B8", event: "", unixMs: 1 };
  assert.equal(isCandidate(row, new Set(["natusvincere"])), true);
  assert.equal(isCandidate(row, new Set(["vitality"])), false);
  const match = parseMatch({ teams: [{ href: "/team/4608/natus-vincere", name: "Natus Vincere" }], event: "", unix: "5", demos: ["/download/demo/1", "/download/demo/1", "javascript:x"], maps: ["Mirage", "TBA"] });
  assert.ok(match);
  assert.deepEqual(match.demoPaths, ["/download/demo/1"]);
  assert.deepEqual(match.maps, ["Mirage"]);
  assert.equal(rankedTeamIn(match, [{ id: 4608, name: "NAVI", rank: 6 }])?.rank, 6);
  assert.equal(rankedTeamIn(match, [{ id: 1, name: "Natus Vincere", rank: 6 }]), null);
});

test("only matches never seen or still without a demo are visited again", () => {
  const state = parseState({ matches: { "1": { status: "downloaded", at: "" }, "2": { status: "no_demo", at: "" }, "3": { status: "bogus" } } });
  assert.equal(needsVisit(state, 1), false);
  assert.equal(needsVisit(state, 2), true);
  assert.equal(needsVisit(state, 3), true);
});

test("parseArgs ignores invalid numbers", () => {
  assert.deepEqual(parseArgs(["--days", "7", "--top", "-3", "--out", "D:\\x"], "C:\\d"), { days: 7, top: 50, outDir: "D:\\x", maxMatches: 1000, refetch: "", bestMaps: false, minMaps: 10, perMap: 5, vetoBackfill: "", hours: 0, rankedOnly: false });
});

test("--hours shortens the window and --ranked-only restores the top filter", () => {
  const o = parseArgs(["--hours", "6", "--ranked-only"], "out");
  assert.equal(o.rankedOnly, true);
  assert.equal(windowMs(o), 6 * 3600 * 1000);
  assert.equal(windowMs(parseArgs(["--days", "2"], "out")), 2 * 24 * 3600 * 1000);
  assert.equal(parseArgs(["--hours", "0"], "out").hours, 0);
});

const row = (matchId: number, team1: string, team2: string) => ({ matchId, path: `/matches/${matchId}/x`, team1, team2, event: "", unixMs: 1 });

test("selectCandidates keeps every unseen match by default and counts what it leaves", () => {
  const state = parseState({ matches: { "1": { status: "downloaded", at: "" }, "2": { status: "no_demo", at: "" } } });
  const rows = [row(1, "FURIA", "x"), row(2, "a", "b"), row(3, "c", "d"), row(3, "c", "d"), row(4, "FURIA", "e")];
  const got = selectCandidates(rows, { state, rankedNames: new Set(["furia"]), rankedOnly: false, max: 10 });
  assert.deepEqual(got.todo.map((r) => r.matchId), [2, 3, 4]);
  assert.deepEqual([got.seen, got.unranked], [1, 0]);
});

test("selectCandidates with rankedOnly drops other teams, and with everything seen it reports why none are left", () => {
  const state = parseState({ matches: { "1": { status: "downloaded", at: "" } } });
  const rows = [row(1, "FURIA", "x"), row(2, "a", "b"), row(3, "NAVI", "d")];
  const got = selectCandidates(rows, { state, rankedNames: new Set(["furia", "navi"]), rankedOnly: true, max: 10 });
  assert.deepEqual(got.todo.map((r) => r.matchId), [3]);
  assert.deepEqual([got.seen, got.unranked], [1, 1]);
  const none = selectCandidates([row(1, "FURIA", "x")], { state, rankedNames: new Set(), rankedOnly: false, max: 10 });
  assert.deepEqual([none.todo.length, none.seen], [0, 1]);
  assert.equal(selectCandidates(rows, { state: { matches: {} }, rankedNames: new Set(), rankedOnly: false, max: 2 }).todo.length, 2);
});

test("--refetch names a folder of sidecars to download again", () => {
  assert.equal(parseArgs(["--refetch", "C:\sidecars", "--max", "3"], "out").refetch, "C:\sidecars");
  assert.equal(parseArgs([], "out").refetch, "");
  assert.equal(parseArgs(["--veto-backfill", "sidecars"], "out").vetoBackfill, "sidecars");
});

test("parseMatch reads the series format and the ordered veto from the veto boxes", () => {
  const match = parseMatch({
    teams: [], event: "IEM", unix: "5", demos: [], maps: ["Ancient", "Dust2", "Nuke"],
    vetoBoxes: [
      ["Best of 3 (LAN)\n\n* Upper bracket final"],
      ["1. FURIA removed Mirage", "2. Natus Vincere removed Inferno", "3. FURIA picked Ancient", "4. Natus Vincere picked Dust2",
        "5. FURIA removed Train", "6. Natus Vincere removed Anubis", "7. Nuke was left over"],
    ],
  });
  assert.ok(match);
  assert.equal(match.format, "bo3");
  assert.equal(match.veto.length, 7);
  assert.deepEqual(match.veto[1], { team: "Natus Vincere", action: "ban", map: "Inferno" });
  assert.deepEqual(match.veto[2], { team: "FURIA", action: "pick", map: "Ancient" });
  assert.deepEqual(match.veto[6], { team: "", action: "decider", map: "Nuke" });
  const sidecar = sidecarFor(1, "/download/demo/1", match, []);
  assert.equal(parseSidecar(sidecar)?.veto?.length, 7);
  const bare = parseMatch({ teams: [], event: "", unix: "5", demos: [], maps: [] });
  assert.deepEqual([bare?.format, bare?.veto], ["", []]);
  assert.equal(sidecarFor(1, "/download/demo/1", { ...match, format: "", veto: [] }, []).veto, undefined);
});

test("parseSidecar keeps only an HLTV demo download of a real match", () => {
  const good = { source: "hltv", match_id: 2398992, demo_path: "/download/demo/123", event: "IEM", played_at: "2026-11-01T00:00:00Z", teams: [], maps: ["Mirage"] };
  assert.equal(parseSidecar(good)?.match_id, 2398992);
  assert.equal(parseSidecar({ ...good, demo_path: "https://evil.example/x" }), null);
  assert.equal(parseSidecar({ ...good, match_id: 0 }), null);
  assert.equal(parseSidecar({ ...good, source: "faceit" }), null);
  assert.equal(parseSidecar("junk"), null);
});
