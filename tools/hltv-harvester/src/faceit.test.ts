import { strict as assert } from "node:assert";
import { test } from "node:test";
import {
  demoFileName, faceitSidecarFor, isDemoResource, isSidecarOf, matchScript, parseFaceitMatch, parseReply, parseSignedUrl, replyKind, roomIds, sidecarName, signScript,
} from "./faceit.ts";
import { parseArgs } from "./hltv.ts";

const ROOM = "1-5a968f7b-a1e1-420a-95c3-f0715701759d";
const DEMO = `https://demos-europe-central.backblaze.faceit-cdn.net/cs2/${ROOM}-1-1.dem.zst`;

function matchText(payload: Record<string, unknown>): string {
  return JSON.stringify({ code: "OPERATION-OK", payload: {
    id: ROOM, status: "FINISHED", startedAt: "2026-11-25T02:58:16Z", finishedAt: "2026-11-25T03:25:56Z",
    maps: [{ class_name: "de_anubis" }],
    demoURLs: [DEMO, DEMO, "https://evil.example/x.dem.zst", "javascript:alert(1)"],
    results: [{ factions: { faction1: { score: 13 }, faction2: { score: 4 } } }],
    teams: {
      faction1: { name: "team_dezl3bio", roster: [{ nickname: "dezl3bio", gameId: "76561198179270719" }, { nickname: "bot", gameId: "BOT" }] },
      faction2: { name: "team_qymax", roster: [{ nickname: "qymax", gameId: "76561199095486270" }] },
    },
    ...payload,
  } });
}

test("roomIds reads ids from ids, room links and JSON text, once each and in order", () => {
  const text = `https://www.faceit.com/en/cs2/room/${ROOM}\n{"match_id":"1-6D4B438F-1238-445b-bc9e-5d46d6ee44dd"},${ROOM}`;
  assert.deepEqual(roomIds(text), [ROOM, "1-6d4b438f-1238-445b-bc9e-5d46d6ee44dd"]);
  assert.deepEqual(roomIds("C:\\no\\ids.txt"), []);
});

test("parseFaceitMatch keeps the room's FACEIT CDN demos, map, score and Steam ids", () => {
  const match = parseFaceitMatch(matchText({}), ROOM);
  assert.ok(match);
  assert.deepEqual(match.demoUrls, [DEMO]);
  assert.equal(match.map, "de_anubis");
  assert.deepEqual(match.teams.map((t) => [t.name, t.score]), [["team_dezl3bio", 13], ["team_qymax", 4]]);
  assert.deepEqual(match.teams[0].players, [{ nickname: "dezl3bio", steamid64: "76561198179270719" }, { nickname: "bot", steamid64: "" }]);
});

test("parseFaceitMatch refuses another room, an unfinished match and a page that is not JSON", () => {
  assert.equal(parseFaceitMatch(matchText({ id: "1-00000000-0000-0000-0000-000000000000" }), ROOM), null);
  assert.equal(parseFaceitMatch(matchText({ status: "ONGOING" }), ROOM), null);
  assert.equal(parseFaceitMatch("<html>Just a moment...</html>", ROOM), null);
});

test("demo resources keep FACEIT's file name so Studio reads the room and the map number", () => {
  assert.equal(demoFileName(DEMO), `${ROOM}-1-1.dem.zst`);
  assert.equal(isDemoResource(DEMO), true);
  assert.equal(isDemoResource(`http://demos.faceit-cdn.net/cs2/${ROOM}-1-1.dem.zst`), false);
  assert.equal(isDemoResource(`https://demos.faceit-cdn.net/cs2/..%2F${ROOM}.dem.zst`), false);
  assert.equal(isDemoResource("https://faceit-cdn.net.evil.example/cs2/a.dem.zst"), false);
});

test("parseSignedUrl takes only an https FACEIT or Backblaze link", () => {
  const signed = "https://demos-europe-central-faceit-cdn.s3.eu-central-003.backblazeb2.com/cs2/x.dem.zst?X-Amz-Signature=1";
  assert.equal(parseSignedUrl(JSON.stringify({ payload: { download_url: signed } })), signed);
  assert.equal(parseSignedUrl(JSON.stringify({ payload: { download_url: "https://evil.example/x" } })), "");
  assert.equal(parseSignedUrl(JSON.stringify({ payload: { download_url: "http://x.backblazeb2.com/a" } })), "");
  assert.equal(parseSignedUrl("<html>"), "");
});

test("replyKind asks the user on sign-in, a check or a page off FACEIT, and fails on service errors", () => {
  const reply = (status: number) => ({ status, body: "" });
  assert.equal(replyKind(reply(200), false), "ok");
  assert.equal(replyKind(reply(401), false), "needs_user");
  assert.equal(replyKind(reply(403), false), "needs_user");
  assert.equal(replyKind(reply(-1), false), "needs_user");
  assert.equal(replyKind(reply(404), true), "gone");
  assert.equal(replyKind(reply(410), false), "gone");
  assert.equal(replyKind(reply(0), false), "error");
  assert.equal(replyKind(reply(0), true), "needs_user");
  assert.equal(replyKind(reply(500), true), "error");
  assert.equal(replyKind(reply(429), false), "error");
  assert.deepEqual(parseReply(null), { status: 0, body: "" });
  assert.deepEqual(parseReply({ status: 403, body: 1 }), { status: 403, body: "" });
});

test("the page scripts run only on the FACEIT origin, send JSON and stop at the time limit", () => {
  const sign = signScript(DEMO, 30_000);
  assert.ok(sign.startsWith(`location.origin !== "https://www.faceit.com"`));
  assert.ok(sign.includes(JSON.stringify(JSON.stringify({ resource_url: DEMO }))));
  assert.ok(sign.includes("AbortSignal.timeout(30000)"));
  assert.ok(sign.includes("method: 'POST'"));
  const read = matchScript(ROOM, 1.4);
  assert.ok(read.includes(`"/api/match/v2/match/${ROOM}"`));
  assert.ok(read.includes("AbortSignal.timeout(1)"));
  assert.ok(!read.includes("POST"));
});

test("a sidecar is kept only when it is FACEIT's and names its own demo", () => {
  const name = `${ROOM}-1-1.dem.zst`;
  assert.equal(sidecarName(name), `${ROOM}-1-1.json`);
  const match = parseFaceitMatch(matchText({}), ROOM);
  assert.ok(match);
  assert.equal(isSidecarOf(JSON.stringify(faceitSidecarFor(match, DEMO)), name), true);
  assert.equal(isSidecarOf('{"source":"faceit","file":"other.dem.zst"}', name), false);
  assert.equal(isSidecarOf('{"source":"faceit","fi', name), false);
});

test("the sidecar names the demo file and the room it came from", () => {
  const match = parseFaceitMatch(matchText({}), ROOM);
  assert.ok(match);
  const sidecar = faceitSidecarFor(match, DEMO);
  assert.equal(sidecar.source, "faceit");
  assert.equal(sidecar.file, `${ROOM}-1-1.dem.zst`);
  assert.equal(sidecar.room_url, `https://www.faceit.com/en/cs2/room/${ROOM}`);
  assert.equal(sidecar.played_at, "2026-11-25T02:58:16Z");
});

test("--faceit takes room ids, links or a file", () => {
  assert.equal(parseArgs(["--faceit", ROOM, "--max", "2"], "out").faceit, ROOM);
  assert.equal(parseArgs(["--faceit", ROOM, "--max", "2"], "out").invalid, "");
  assert.equal(parseArgs(["--faceit", "--max", "2"], "out").invalid, "--faceit needs room ids, room links or a file");
  assert.equal(parseArgs(["--max", "2", "--faceit"], "out").invalid, "--faceit needs room ids, room links or a file");
});
