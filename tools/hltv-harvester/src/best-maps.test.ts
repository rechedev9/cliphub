import { strict as assert } from "node:assert";
import { test } from "node:test";
import { mapKey, parseTeamMapStats, pickBestPerMap, teamSlug } from "./best-maps.ts";
import { parseArgs } from "./hltv.ts";

test("mapKey turns HLTV map names into demo map keys", () => {
  assert.equal(mapKey("Mirage - 63.6%"), "de_mirage");
  assert.equal(mapKey("Dust2"), "de_dust2");
  assert.equal(mapKey(""), "");
});

test("parseTeamMapStats reads wins, draws and losses per map", () => {
  const got = parseTeamMapStats([
    { name: "Mirage - 63.6%", rows: [["Wins / draws / losses", "7 / 0 / 4"], ["Win rate", "63.6%"]] },
    { name: "Nuke - 0%", rows: [["Total rounds", "0"]] },
    "junk",
  ]);
  assert.deepEqual(got, [{ map: "de_mirage", wins: 7, draws: 0, losses: 4 }]);
});

test("pickBestPerMap needs enough maps and breaks ties by maps then rank", () => {
  const picks = pickBestPerMap([
    { id: 1, name: "A", rank: 1, records: [{ map: "de_mirage", wins: 6, draws: 0, losses: 4 }, { map: "de_nuke", wins: 3, draws: 0, losses: 0 }] },
    { id: 2, name: "B", rank: 2, records: [{ map: "de_mirage", wins: 12, draws: 0, losses: 8 }] },
    { id: 3, name: "C", rank: 3, records: [{ map: "de_mirage", wins: 9, draws: 0, losses: 1 }] },
  ], 10);
  assert.deepEqual(picks, [{ map: "de_mirage", team_id: 3, team: "C", rank: 3, maps: 10, win_rate: 0.9 }]);
  const tie = pickBestPerMap([
    { id: 1, name: "A", rank: 1, records: [{ map: "de_mirage", wins: 6, draws: 0, losses: 4 }] },
    { id: 2, name: "B", rank: 2, records: [{ map: "de_mirage", wins: 12, draws: 0, losses: 8 }] },
  ], 10);
  assert.equal(tie[0].team, "B");
});

test("teamSlug and the best-maps flags", () => {
  assert.equal(teamSlug("Natus Vincere"), "natus-vincere");
  const opts = parseArgs(["--best-maps", "--days", "90", "--min-maps", "8", "--per-map", "3"], "out");
  assert.equal(opts.bestMaps, true);
  assert.equal(opts.days, 90);
  assert.equal(opts.minMaps, 8);
  assert.equal(opts.perMap, 3);
});
