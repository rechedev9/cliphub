import { mkdirSync } from "node:fs";
import path from "node:path";
import { DatabaseSync, type SQLInputValue } from "node:sqlite";
import type { JsonObject, MapRecord, MapSource, RelatedLink, ShortRecord } from "./model.ts";

export type UploadKind = "pov" | "short";

export type UploadReceipt = {
  youtubeId: string;
  mapId: string | null;
  receipt: string;
  kind: UploadKind;
  title: string | null;
  visibility: string | null;
  publishAt: string | null;
  uploadedAt: string | null;
  thumbnail: string | null;
  raw: JsonObject;
};

export type MetricKind = "channel" | "videos";

export type MetricSnapshot = {
  kind: MetricKind;
  source: string;
  fetchedAt: string;
  raw: unknown;
};

export type StateCounts = { maps: number; shorts: number; uploads: number; metricSnapshots: number };

export type StateStore = {
  readonly path: string;
  schemaVersion(): number;
  counts(): StateCounts;
  listMaps(): MapRecord[];
  getMap(id: string): MapRecord | null;
  hasMap(id: string): boolean;
  saveMap(map: MapRecord): void;
  hasUpload(youtubeId: string): boolean;
  saveUpload(upload: UploadReceipt): void;
  saveMetricSnapshot(snapshot: MetricSnapshot): boolean;
  transaction<T>(options: { commit: boolean }, work: () => T): T;
  close(): void;
};

type Migration = { version: number; name: string; sql: string };

const MIGRATIONS: Migration[] = [
  {
    version: 1,
    name: "maps, shorts, uploads and metric snapshots",
    sql: `
      CREATE TABLE maps (
        id TEXT PRIMARY KEY,
        source TEXT NOT NULL CHECK (source IN ('event', 'faceit', 'clip')),
        player TEXT NOT NULL,
        map_name TEXT,
        hltv_match_id INTEGER,
        faceit_match_id TEXT,
        match_url TEXT,
        match_time TEXT,
        match_date TEXT,
        event TEXT,
        event_stage TEXT,
        team TEXT,
        opponent TEXT,
        score TEXT,
        result TEXT,
        kills INTEGER,
        deaths INTEGER,
        adr REAL,
        rating REAL,
        hs_pct REAL,
        elo INTEGER,
        steamid TEXT,
        demo_file TEXT,
        job_id TEXT,
        pov_video TEXT,
        pov_thumbnail TEXT,
        pov_title TEXT,
        pov_tags TEXT,
        pov_youtube_id TEXT,
        pov_published_at TEXT,
        pov_skipped TEXT,
        skipped_at TEXT,
        skipped_reason TEXT,
        cleaned_at TEXT,
        legacy_json TEXT,
        updated_at TEXT NOT NULL,
        CHECK ((skipped_at IS NULL) = (skipped_reason IS NULL))
      ) STRICT;
      CREATE TABLE shorts (
        map_id TEXT NOT NULL REFERENCES maps(id) ON DELETE CASCADE,
        position INTEGER NOT NULL,
        round INTEGER,
        segments_json TEXT NOT NULL,
        play TEXT,
        video TEXT,
        thumbnail TEXT,
        title TEXT,
        tags TEXT,
        youtube_id TEXT,
        scheduled_for TEXT,
        published_at TEXT,
        related TEXT NOT NULL CHECK (related IN ('pending', 'linked', 'no-pov')),
        thumbnail_checked INTEGER NOT NULL CHECK (thumbnail_checked IN (0, 1)),
        youtube_skipped TEXT,
        legacy_json TEXT,
        PRIMARY KEY (map_id, position)
      ) STRICT;
      CREATE TABLE uploads (
        youtube_id TEXT PRIMARY KEY,
        map_id TEXT REFERENCES maps(id) ON DELETE SET NULL,
        receipt TEXT NOT NULL,
        kind TEXT NOT NULL CHECK (kind IN ('pov', 'short')),
        title TEXT,
        visibility TEXT,
        publish_at TEXT,
        uploaded_at TEXT,
        thumbnail TEXT,
        raw_json TEXT NOT NULL
      ) STRICT;
      CREATE TABLE metric_snapshots (
        id INTEGER PRIMARY KEY,
        kind TEXT NOT NULL CHECK (kind IN ('channel', 'videos')),
        source TEXT NOT NULL,
        fetched_at TEXT NOT NULL,
        raw_json TEXT NOT NULL,
        UNIQUE (kind, fetched_at)
      ) STRICT;
    `,
  },
];

type Row = Record<string, SQLInputValue>;

const MAP_COLUMNS = [
  "id",
  "source",
  "player",
  "map_name",
  "hltv_match_id",
  "faceit_match_id",
  "match_url",
  "match_time",
  "match_date",
  "event",
  "event_stage",
  "team",
  "opponent",
  "score",
  "result",
  "kills",
  "deaths",
  "adr",
  "rating",
  "hs_pct",
  "elo",
  "steamid",
  "demo_file",
  "job_id",
  "pov_video",
  "pov_thumbnail",
  "pov_title",
  "pov_tags",
  "pov_youtube_id",
  "pov_published_at",
  "pov_skipped",
  "skipped_at",
  "skipped_reason",
  "cleaned_at",
  "legacy_json",
  "updated_at",
] as const;

const SHORT_COLUMNS = [
  "map_id",
  "position",
  "round",
  "segments_json",
  "play",
  "video",
  "thumbnail",
  "title",
  "tags",
  "youtube_id",
  "scheduled_for",
  "published_at",
  "related",
  "thumbnail_checked",
  "youtube_skipped",
  "legacy_json",
] as const;

const UPLOAD_COLUMNS = ["youtube_id", "map_id", "receipt", "kind", "title", "visibility", "publish_at", "uploaded_at", "thumbnail", "raw_json"] as const;

type Params<C extends readonly string[]> = Record<C[number], SQLInputValue>;

function text(row: Row, key: string): string | null {
  const value = row[key];
  return typeof value === "string" ? value : null;
}

function num(row: Row, key: string): number | null {
  const value = row[key];
  if (typeof value === "number") return value;
  return typeof value === "bigint" ? Number(value) : null;
}

function json(value: string | null): JsonObject | null {
  if (value === null) return null;
  const parsed: unknown = JSON.parse(value);
  return typeof parsed === "object" && parsed !== null && !Array.isArray(parsed) ? { ...parsed } : null;
}

function legacyText(value: JsonObject | null): string | null {
  return value === null || Object.keys(value).length === 0 ? null : JSON.stringify(value);
}

const SOURCES: readonly MapSource[] = ["event", "faceit", "clip"];
const LINKS: readonly RelatedLink[] = ["pending", "linked", "no-pov"];

function source(row: Row): MapSource {
  const value = text(row, "source");
  const found = SOURCES.find((s) => s === value);
  if (found === undefined) throw new Error(`maps row ${text(row, "id")} has source ${value}`);
  return found;
}

function related(row: Row): RelatedLink {
  const value = text(row, "related");
  const found = LINKS.find((l) => l === value);
  if (found === undefined) throw new Error(`shorts row has related ${value}`);
  return found;
}

function shortFromRow(row: Row): ShortRecord {
  const segments: unknown = JSON.parse(text(row, "segments_json") ?? "[]");
  return {
    position: num(row, "position") ?? 0,
    round: num(row, "round"),
    segments: Array.isArray(segments) ? segments.map(String) : [],
    play: text(row, "play"),
    videoPath: text(row, "video"),
    thumbnailPath: text(row, "thumbnail"),
    title: text(row, "title"),
    tags: text(row, "tags"),
    youtubeId: text(row, "youtube_id"),
    scheduledFor: text(row, "scheduled_for"),
    publishedAt: text(row, "published_at"),
    related: related(row),
    thumbnailChecked: num(row, "thumbnail_checked") === 1,
    youtubeSkipped: text(row, "youtube_skipped"),
    legacy: json(text(row, "legacy_json")),
  };
}

function mapFromRow(row: Row, shorts: ShortRecord[]): MapRecord {
  const skippedAt = text(row, "skipped_at");
  const skippedReason = text(row, "skipped_reason");
  return {
    id: text(row, "id") ?? "",
    source: source(row),
    player: text(row, "player") ?? "",
    mapName: text(row, "map_name"),
    hltvMatchId: num(row, "hltv_match_id"),
    faceitMatchId: text(row, "faceit_match_id"),
    matchUrl: text(row, "match_url"),
    matchTime: text(row, "match_time"),
    matchDate: text(row, "match_date"),
    event: text(row, "event"),
    eventStage: text(row, "event_stage"),
    team: text(row, "team"),
    opponent: text(row, "opponent"),
    score: text(row, "score"),
    result: text(row, "result"),
    stats: {
      kills: num(row, "kills"),
      deaths: num(row, "deaths"),
      adr: num(row, "adr"),
      rating: num(row, "rating"),
      hsPct: num(row, "hs_pct"),
      elo: num(row, "elo"),
    },
    steamid: text(row, "steamid"),
    demoFile: text(row, "demo_file"),
    jobId: text(row, "job_id"),
    pov: {
      videoPath: text(row, "pov_video"),
      thumbnailPath: text(row, "pov_thumbnail"),
      title: text(row, "pov_title"),
      tags: text(row, "pov_tags"),
      youtubeId: text(row, "pov_youtube_id"),
      publishedAt: text(row, "pov_published_at"),
      skipped: text(row, "pov_skipped"),
    },
    shorts,
    skipped: skippedAt !== null && skippedReason !== null ? { at: skippedAt, reason: skippedReason } : null,
    cleanedAt: text(row, "cleaned_at"),
    legacy: json(text(row, "legacy_json")),
  };
}

function mapParams(map: MapRecord, updatedAt: string): Params<typeof MAP_COLUMNS> {
  return {
    id: map.id,
    source: map.source,
    player: map.player,
    map_name: map.mapName,
    hltv_match_id: map.hltvMatchId,
    faceit_match_id: map.faceitMatchId,
    match_url: map.matchUrl,
    match_time: map.matchTime,
    match_date: map.matchDate,
    event: map.event,
    event_stage: map.eventStage,
    team: map.team,
    opponent: map.opponent,
    score: map.score,
    result: map.result,
    kills: map.stats.kills,
    deaths: map.stats.deaths,
    adr: map.stats.adr,
    rating: map.stats.rating,
    hs_pct: map.stats.hsPct,
    elo: map.stats.elo,
    steamid: map.steamid,
    demo_file: map.demoFile,
    job_id: map.jobId,
    pov_video: map.pov.videoPath,
    pov_thumbnail: map.pov.thumbnailPath,
    pov_title: map.pov.title,
    pov_tags: map.pov.tags,
    pov_youtube_id: map.pov.youtubeId,
    pov_published_at: map.pov.publishedAt,
    pov_skipped: map.pov.skipped,
    skipped_at: map.skipped?.at ?? null,
    skipped_reason: map.skipped?.reason ?? null,
    cleaned_at: map.cleanedAt,
    legacy_json: legacyText(map.legacy),
    updated_at: updatedAt,
  };
}

function shortParams(mapId: string, short: ShortRecord): Params<typeof SHORT_COLUMNS> {
  return {
    map_id: mapId,
    position: short.position,
    round: short.round,
    segments_json: JSON.stringify(short.segments),
    play: short.play,
    video: short.videoPath,
    thumbnail: short.thumbnailPath,
    title: short.title,
    tags: short.tags,
    youtube_id: short.youtubeId,
    scheduled_for: short.scheduledFor,
    published_at: short.publishedAt,
    related: short.related,
    thumbnail_checked: short.thumbnailChecked ? 1 : 0,
    youtube_skipped: short.youtubeSkipped,
    legacy_json: legacyText(short.legacy),
  };
}

type Statement = { table: string; columns: readonly string[] };

function insertSql(target: Statement, conflict: string): string {
  const { table, columns } = target;
  const names = columns.join(", ");
  const values = columns.map((c) => `:${c}`).join(", ");
  return `INSERT INTO ${table} (${names}) VALUES (${values}) ${conflict}`;
}

function upsertSql(target: Statement, key: string[]): string {
  const updates = target.columns.filter((c) => !key.includes(c)).map((c) => `${c} = excluded.${c}`);
  return insertSql(target, `ON CONFLICT (${key.join(", ")}) DO UPDATE SET ${updates.join(", ")}`);
}

function migrate(db: DatabaseSync, now: () => string): void {
  db.exec("CREATE TABLE IF NOT EXISTS migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TEXT NOT NULL) STRICT");
  const applied = new Set(db.prepare("SELECT version FROM migrations").all().map((row) => Number(row.version)));
  for (const migration of MIGRATIONS) {
    if (applied.has(migration.version)) continue;
    db.exec("BEGIN IMMEDIATE");
    try {
      db.exec(migration.sql);
      db.prepare("INSERT INTO migrations (version, name, applied_at) VALUES (?, ?, ?)").run(migration.version, migration.name, now());
      db.exec("COMMIT");
    } catch (error) {
      db.exec("ROLLBACK");
      throw error;
    }
  }
}

export function openStateStore(options: { path: string; now?: () => string }): StateStore {
  const now = options.now ?? (() => new Date().toISOString());
  mkdirSync(path.dirname(options.path), { recursive: true });
  const db = new DatabaseSync(options.path);
  db.exec("PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL; PRAGMA busy_timeout = 5000");
  migrate(db, now);

  const upsertMap = db.prepare(upsertSql({ table: "maps", columns: MAP_COLUMNS }, ["id"]));
  const deleteShorts = db.prepare("DELETE FROM shorts WHERE map_id = ?");
  const insertShort = db.prepare(insertSql({ table: "shorts", columns: SHORT_COLUMNS }, ""));
  const selectShorts = db.prepare("SELECT * FROM shorts WHERE map_id = ? ORDER BY position");
  const upsertUpload = db.prepare(upsertSql({ table: "uploads", columns: UPLOAD_COLUMNS }, ["youtube_id"]));
  const insertSnapshot = db.prepare(insertSql({ table: "metric_snapshots", columns: ["kind", "source", "fetched_at", "raw_json"] }, "ON CONFLICT (kind, fetched_at) DO NOTHING"));

  const loadMap = (row: Row): MapRecord => mapFromRow(row, selectShorts.all(text(row, "id")).map(shortFromRow));
  const count = (table: string): number => Number(db.prepare(`SELECT COUNT(*) AS n FROM ${table}`).get()?.n ?? 0);

  return {
    path: options.path,
    schemaVersion: () => Number(db.prepare("SELECT MAX(version) AS v FROM migrations").get()?.v ?? 0),
    counts: () => ({ maps: count("maps"), shorts: count("shorts"), uploads: count("uploads"), metricSnapshots: count("metric_snapshots") }),
    listMaps: () => db.prepare("SELECT * FROM maps ORDER BY id").all().map(loadMap),
    getMap: (id) => {
      const row = db.prepare("SELECT * FROM maps WHERE id = ?").get(id);
      return row === undefined ? null : loadMap(row);
    },
    hasMap: (id) => db.prepare("SELECT 1 FROM maps WHERE id = ?").get(id) !== undefined,
    saveMap: (map) => {
      upsertMap.run(mapParams(map, now()));
      deleteShorts.run(map.id);
      for (const short of map.shorts) insertShort.run(shortParams(map.id, short));
    },
    hasUpload: (youtubeId) => db.prepare("SELECT 1 FROM uploads WHERE youtube_id = ?").get(youtubeId) !== undefined,
    saveUpload: (upload) => {
      const params: Params<typeof UPLOAD_COLUMNS> = {
        youtube_id: upload.youtubeId,
        map_id: upload.mapId,
        receipt: upload.receipt,
        kind: upload.kind,
        title: upload.title,
        visibility: upload.visibility,
        publish_at: upload.publishAt,
        uploaded_at: upload.uploadedAt,
        thumbnail: upload.thumbnail,
        raw_json: JSON.stringify(upload.raw),
      };
      upsertUpload.run(params);
    },
    saveMetricSnapshot: (snapshot) =>
      Number(
        insertSnapshot.run({ kind: snapshot.kind, source: snapshot.source, fetched_at: snapshot.fetchedAt, raw_json: JSON.stringify(snapshot.raw) }).changes,
      ) > 0,
    transaction: (options, work) => {
      db.exec("BEGIN IMMEDIATE");
      try {
        const result = work();
        db.exec(options.commit ? "COMMIT" : "ROLLBACK");
        return result;
      } catch (error) {
        db.exec("ROLLBACK");
        throw error;
      }
    },
    close: () => db.close(),
  };
}

