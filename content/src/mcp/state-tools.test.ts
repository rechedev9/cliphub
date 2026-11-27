import assert from "node:assert/strict";
import { cpSync, mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { DatabaseSync } from "node:sqlite";
import { afterEach, test } from "node:test";
import { at, LEGACY_MAPS, LEGACY_METRICS, openHarness, type Harness } from "../testing/harness.ts";

let harness: Harness | null = null;

afterEach(async () => {
  await harness?.close();
  harness = null;
});

async function imported(): Promise<Harness> {
  harness = await openHarness({ legacyMapsDir: LEGACY_MAPS, legacyMetricsDir: LEGACY_METRICS });
  const result = await harness.call("import_legacy_state", { write: true });
  assert.equal(result.isError, false, result.text);
  return harness;
}

async function stageOf(h: Harness, id: string): Promise<unknown> {
  return at((await h.call("get_map", { id })).body, "stage.kind");
}

test("a dry run reports what it would import and leaves the state empty", async () => {
  harness = await openHarness({ legacyMapsDir: LEGACY_MAPS, legacyMetricsDir: LEGACY_METRICS });
  const dry = await harness.call("import_legacy_state");
  assert.equal(at(dry.body, "dryRun"), true);
  assert.equal(at(dry.body, "maps.created"), 6);
  assert.equal(at(dry.body, "uploads.created"), 5);
  assert.equal(at(dry.body, "metricSnapshots.created"), 7);
  assert.deepEqual(harness.store.counts(), { maps: 0, shorts: 0, uploads: 0, metricSnapshots: 0 });
});

test("the import derives each map's stage from its fields and reports bad records", async () => {
  const h = await imported();
  const list = await h.call("list_maps");
  const stages = Object.fromEntries(
    [0, 1, 2, 3, 4, 5].map((i) => [at(list.body, `maps.${i}.id`), at(list.body, `maps.${i}.stage.kind`)]),
  );
  assert.deepEqual(stages, {
    "2400204-mirage": "cleaned",
    "2400573-anubis": "cleaned",
    "2400574-nuke": "skipped",
    "2400581-mirage": "ready",
    "clip-ropz-impossible-jump": "done",
    "faceit-1-00000000-0000-4000-8000-000000000006-ancient": "cleaned",
  });
  const report = await h.call("import_legacy_state");
  assert.equal(at(report.body, "maps.created"), 0);
  assert.equal(at(report.body, "maps.updated"), 6);
  assert.equal(at(report.body, "metricSnapshots.existing"), 7);
  assert.equal(at(report.body, "issues.0.file"), "broken-kills/map.json");
  assert.equal(at(report.body, "issues.1"), undefined);
  const faceit = await h.call("list_maps", { source: "faceit" });
  assert.equal(at(faceit.body, "maps.1"), undefined);
  assert.equal(at(faceit.body, "maps.0.player"), "m3wsu");
});

test("legacy fields without a column survive in the raw JSON and never become columns", async () => {
  const h = await imported();
  const faceit = await h.call("get_map", { id: "faceit-1-00000000-0000-4000-8000-000000000006-ancient" });
  assert.equal(at(faceit.body, "record.legacy.competition"), "Europe 5v5 Queue");
  assert.equal(at(faceit.body, "record.legacy.pov.voice"), "team comms (voice_status available)");
  assert.equal(at(faceit.body, "record.shorts.0.legacy.crossposted_queued"), true);
  assert.equal(at(faceit.body, "record.source"), "faceit");
  const db = new DatabaseSync(h.config.statePath, { readOnly: true });
  const columns = ["maps", "shorts", "uploads", "metric_snapshots"].flatMap((table) =>
    db.prepare(`SELECT name FROM pragma_table_info('${table}')`).all().map((row) => String(row.name)),
  );
  db.close();
  assert.deepEqual(columns.filter((name) => /tiktok|instagram|queue|crosspost/i.test(name)), []);
});

test("record_map_event walks a Short to done and refuses steps out of order", async () => {
  const h = await imported();
  const id = "2400581-mirage";
  const event = (e: Record<string, unknown>) => h.call("record_map_event", { mapId: id, event: e });

  assert.equal((await event({ type: "map_cleaned", at: "2026-11-28T10:00:00+01:00" })).isError, true);
  assert.equal((await event({ type: "short_thumbnail_checked", position: 1 })).isError, true);
  assert.equal((await event({ type: "pov_published", youtubeId: "x", publishedAt: "2026-11-28T10:00:00+01:00" })).isError, true);
  assert.equal((await event({ type: "short_published", position: 2, youtubeId: "x", publishedAt: "2026-11-28T10:00:00+01:00" })).isError, true);

  const published = await event({ type: "short_published", position: 1, youtubeId: "abc123", publishedAt: "2026-11-28T10:00:00+01:00" });
  assert.equal(at(published.body, "shorts.0.status.kind"), "published");
  assert.deepEqual(at(published.body, "shorts.0.status.needs"), ["thumbnail_check"]);
  assert.equal((await event({ type: "short_related_linked", position: 1 })).isError, true);
  assert.equal((await event({ type: "short_published", position: 1, youtubeId: "other", publishedAt: "2026-11-28T11:00:00+01:00" })).isError, true);
  assert.equal((await event({ type: "short_youtube_skipped", position: 1, reason: "cadence" })).isError, true);
  assert.equal((await event({ type: "map_cleaned", at: "2026-11-28T10:00:00+01:00" })).isError, true);

  await event({ type: "short_thumbnail_checked", position: 1 });
  assert.equal(await stageOf(h, id), "done");
  assert.equal((await event({ type: "map_skipped", at: "2026-11-28T10:05:00+01:00", reason: "late" })).isError, true);
  assert.equal((await event({ type: "map_cleaned", at: "2026-11-28T10:10:00+01:00" })).isError, false);
  assert.equal(await stageOf(h, id), "cleaned");
  const after = await event({ type: "short_related_linked", position: 1 });
  assert.equal(after.isError, true);
  assert.match(after.text, /cleaned/);
});

test("a skipped map takes only an unskip", async () => {
  const h = await imported();
  const id = "2400574-nuke";
  const event = (e: Record<string, unknown>) => h.call("record_map_event", { mapId: id, event: e });
  assert.equal((await event({ type: "pov_skipped", reason: "cadence" })).isError, true);
  assert.equal((await event({ type: "map_unskipped" })).isError, false);
  assert.equal(await stageOf(h, id), "open");
  assert.equal((await event({ type: "map_unskipped" })).isError, true);
});

test("an unknown map or an invalid event is an error, not a crash", async () => {
  const h = await imported();
  const missing = await h.call("record_map_event", { mapId: "nope", event: { type: "map_unskipped" } });
  assert.equal(missing.isError, true);
  const invalid = await h.call("record_map_event", { mapId: "2400581-mirage", event: { type: "short_published", position: 1 } });
  assert.equal(invalid.isError, true);
  assert.equal((await h.call("get_map", { id: "nope" })).isError, true);
});

test("the import refuses a state database inside a legacy folder and runs without folders configured", async () => {
  harness = await openHarness();
  assert.equal((await harness.call("import_legacy_state")).isError, true);
  await harness.close();
  harness = null;
  const copy = mkdtempSync(path.join(tmpdir(), "cliphub-content-maps-"));
  try {
    cpSync(LEGACY_MAPS, copy, { recursive: true });
    harness = await openHarness({ legacyMapsDir: copy, statePath: path.join(copy, "state.db") });
    const refused = await harness.call("import_legacy_state");
    assert.equal(refused.isError, true);
    assert.match(refused.text, /inside the legacy folder/);
  } finally {
    await harness?.close();
    harness = null;
    rmSync(copy, { recursive: true, force: true });
  }
});
