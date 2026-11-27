import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import path from "node:path";
import { mapStage, type MapRecord, type MapSource, type MapStage } from "../state/model.ts";
import type { MetricSnapshot, StateStore } from "../state/store.ts";
import { decodeLegacyMap, decodeLegacyReceipt } from "./decode.ts";

export type ImportIssue = { file: string; message: string };

export type ImportTally = { read: number; created: number; updated: number };

export type LegacyImportReport = {
  dryRun: boolean;
  maps: ImportTally & { byStage: Partial<Record<MapStage["kind"], number>>; bySource: Partial<Record<MapSource, number>> };
  shorts: number;
  uploads: ImportTally;
  metricSnapshots: { read: number; created: number; existing: number };
  issues: ImportIssue[];
};

export type LegacyImportOptions = {
  store: StateStore;
  mapsDir: string | null;
  metricsDir: string | null;
  dryRun: boolean;
};

export class LegacyImportError extends Error {}

const MAP_FILE = "map.json";
const RECEIPT_SUFFIX = ".youtube.json";
const CHANNEL_SNAPSHOTS = "channel-snapshots.jsonl";
const VIDEOS_LATEST = "videos-latest.json";

function tally(): ImportTally {
  return { read: 0, created: 0, updated: 0 };
}

function isInside(child: string, parent: string): boolean {
  const relative = path.relative(path.resolve(parent), path.resolve(child));
  const outside = relative === ".." || relative.startsWith(`..${path.sep}`) || path.isAbsolute(relative);
  return !outside;
}

function readJson(file: string): { ok: true; value: unknown } | { ok: false; error: string } {
  try {
    return { ok: true, value: JSON.parse(readFileSync(file, "utf8")) };
  } catch (error) {
    return { ok: false, error: `not valid JSON: ${String(error)}` };
  }
}

function bump<K extends string>(counts: Partial<Record<K, number>>, key: K): void {
  counts[key] = (counts[key] ?? 0) + 1;
}

function importMapFolder(options: { store: StateStore; dir: string; id: string; report: LegacyImportReport }): void {
  const { store, dir, id, report } = options;
  const where = (name: string) => `${id}/${name}`;
  const raw = readJson(path.join(dir, MAP_FILE));
  let map: MapRecord | null = null;
  if (raw.ok) {
    const decoded = decodeLegacyMap(id, raw.value);
    if (decoded.ok) map = decoded.value;
    else report.issues.push({ file: where(MAP_FILE), message: decoded.error });
  } else {
    report.issues.push({ file: where(MAP_FILE), message: raw.error });
  }
  if (map !== null) {
    report.maps.read += 1;
    if (store.hasMap(id)) report.maps.updated += 1;
    else report.maps.created += 1;
    store.saveMap(map);
    report.shorts += map.shorts.length;
    bump(report.maps.byStage, mapStage(map).kind);
    bump(report.maps.bySource, map.source);
  }
  const receipts = readdirSync(dir).filter((name) => name.endsWith(RECEIPT_SUFFIX)).sort();
  for (const name of receipts) {
    const json = readJson(path.join(dir, name));
    if (!json.ok) {
      report.issues.push({ file: where(name), message: json.error });
      continue;
    }
    const receipt = decodeLegacyReceipt({ receipt: name.slice(0, -RECEIPT_SUFFIX.length), mapId: map === null ? null : id, raw: json.value });
    if (!receipt.ok) {
      report.issues.push({ file: where(name), message: receipt.error });
      continue;
    }
    report.uploads.read += 1;
    if (store.hasUpload(receipt.value.youtubeId)) report.uploads.updated += 1;
    else report.uploads.created += 1;
    store.saveUpload(receipt.value);
  }
}

type Run = { store: StateStore; report: LegacyImportReport };

function importMaps(run: Run, mapsDir: string): void {
  const { store, report } = run;
  const folders = readdirSync(mapsDir, { withFileTypes: true })
    .filter((entry) => entry.isDirectory() && existsSync(path.join(mapsDir, entry.name, MAP_FILE)))
    .map((entry) => entry.name)
    .sort();
  for (const id of folders) importMapFolder({ store, dir: path.join(mapsDir, id), id, report });
}

function saveSnapshot(run: Run, snapshot: MetricSnapshot): void {
  const { store, report } = run;
  report.metricSnapshots.read += 1;
  if (store.saveMetricSnapshot(snapshot)) report.metricSnapshots.created += 1;
  else report.metricSnapshots.existing += 1;
}

function importMetrics(run: Run, metricsDir: string): void {
  const { report } = run;
  const channelFile = path.join(metricsDir, CHANNEL_SNAPSHOTS);
  if (existsSync(channelFile)) {
    const lines = readFileSync(channelFile, "utf8").split(/\r?\n/);
    for (const [index, line] of lines.entries()) {
      if (line.trim() === "") continue;
      const where = `${CHANNEL_SNAPSHOTS}:${index + 1}`;
      let raw: unknown;
      try {
        raw = JSON.parse(line);
      } catch (error) {
        report.issues.push({ file: where, message: `not valid JSON: ${String(error)}` });
        continue;
      }
      const ts = typeof raw === "object" && raw !== null && "ts" in raw ? raw.ts : undefined;
      if (typeof ts !== "string" || ts === "") {
        report.issues.push({ file: where, message: "snapshot has no ts" });
        continue;
      }
      saveSnapshot(run, { kind: "channel", source: `legacy:${CHANNEL_SNAPSHOTS}`, fetchedAt: ts, raw });
    }
  }
  const videosFile = path.join(metricsDir, VIDEOS_LATEST);
  if (existsSync(videosFile)) {
    const raw = readJson(videosFile);
    if (!raw.ok) {
      report.issues.push({ file: VIDEOS_LATEST, message: raw.error });
      return;
    }
    const fetchedAt = statSync(videosFile).mtime.toISOString();
    saveSnapshot(run, { kind: "videos", source: `legacy:${VIDEOS_LATEST}`, fetchedAt, raw: raw.value });
  }
}

export function importLegacyState(options: LegacyImportOptions): LegacyImportReport {
  const { store, mapsDir, metricsDir, dryRun } = options;
  if (mapsDir === null && metricsDir === null) {
    throw new LegacyImportError("set legacyMapsDir or legacyMetricsDir in the config before importing legacy state");
  }
  for (const dir of [mapsDir, metricsDir]) {
    if (dir === null) continue;
    if (!existsSync(dir)) throw new LegacyImportError(`legacy folder ${dir} does not exist`);
    if (isInside(store.path, dir)) throw new LegacyImportError(`the state database ${store.path} is inside the legacy folder ${dir}; move it out first`);
  }
  const report: LegacyImportReport = {
    dryRun,
    maps: { ...tally(), byStage: {}, bySource: {} },
    shorts: 0,
    uploads: tally(),
    metricSnapshots: { read: 0, created: 0, existing: 0 },
    issues: [],
  };
  store.transaction({ commit: !dryRun }, () => {
    if (mapsDir !== null) importMaps({ store, report }, mapsDir);
    if (metricsDir !== null) importMetrics({ store, report }, metricsDir);
  });
  return report;
}
