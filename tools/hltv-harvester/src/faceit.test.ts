import { strict as assert } from "node:assert";
import { test } from "node:test";
import { demoFileName, faceitSidecarFor, isDemoResource, parseFaceitMatch, parseSigned, roomIds, signScript } from "./faceit.ts";
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
  assert.equal(parseFaceitMatch(undefined, ROOM), null);
});

test("demo resources keep FACEIT's file name so Studio reads the room and the map number", () => {
  assert.equal(demoFileName(DEMO), `${ROOM}-1-1.dem.zst`);
  assert.equal(isDemoResource(DEMO), true);
  assert.equal(isDemoResource(`http://demos.faceit-cdn.net/cs2/${ROOM}-1-1.dem.zst`), false);
  assert.equal(isDemoResource(`https://demos.faceit-cdn.net/cs2/..%2F${ROOM}.dem.zst`), false);
  assert.equal(isDemoResource("https://faceit-cdn.net.evil.example/cs2/a.dem.zst"), false);
});

test("parseSigned takes only an https FACEIT or Backblaze link and tells sign-in from failure", () => {
  const ok = { status: 200, body: JSON.stringify({ payload: { download_url: "https://demos-europe-central-faceit-cdn.s3.eu-central-003.backblazeb2.com/cs2/x.dem.zst?X-Amz-Signature=1" } }) };
  assert.equal(parseSigned(ok).kind, "signed");
  assert.deepEqual(parseSigned({ status: 401, body: "" }), { kind: "needs_user" });
  assert.deepEqual(parseSigned({ status: 403, body: "<title>Just a moment...</title>" }), { kind: "needs_user" });
  assert.deepEqual(parseSigned({ status: 404, body: "" }), { kind: "failed", status: 404 });
  assert.deepEqual(parseSigned({ status: 200, body: JSON.stringify({ payload: { download_url: "https://evil.example/x" } }) }), { kind: "failed", status: 200 });
  assert.deepEqual(parseSigned({ status: 200, body: "<html>" }), { kind: "failed", status: 200 });
  assert.deepEqual(parseSigned(null), { kind: "failed", status: 0 });
});

test("signScript sends the resource as JSON from the FACEIT origin only", () => {
  const script = signScript(DEMO);
  assert.ok(script.startsWith("location.origin !== 'https://www.faceit.com'"));
  assert.ok(script.includes(JSON.stringify(JSON.stringify({ resource_url: DEMO }))));
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
  assert.equal(parseArgs(["--faceit", "--max", "2"], "out").faceit, "");
});
