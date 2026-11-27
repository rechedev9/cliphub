export type JsonObject = { [key: string]: unknown };

export type MapSource = "event" | "faceit" | "clip";

export type RelatedLink = "pending" | "linked" | "no-pov";

export type PovRecord = {
  videoPath: string | null;
  thumbnailPath: string | null;
  title: string | null;
  tags: string | null;
  youtubeId: string | null;
  publishedAt: string | null;
  skipped: string | null;
};

export type ShortRecord = {
  position: number;
  round: number | null;
  segments: string[];
  play: string | null;
  videoPath: string | null;
  thumbnailPath: string | null;
  title: string | null;
  tags: string | null;
  youtubeId: string | null;
  scheduledFor: string | null;
  publishedAt: string | null;
  related: RelatedLink;
  thumbnailChecked: boolean;
  youtubeSkipped: string | null;
  legacy: JsonObject | null;
};

export type MapStats = {
  kills: number | null;
  deaths: number | null;
  adr: number | null;
  rating: number | null;
  hsPct: number | null;
  elo: number | null;
};

export type MapRecord = {
  id: string;
  source: MapSource;
  player: string;
  mapName: string | null;
  hltvMatchId: number | null;
  faceitMatchId: string | null;
  matchUrl: string | null;
  matchTime: string | null;
  matchDate: string | null;
  event: string | null;
  eventStage: string | null;
  team: string | null;
  opponent: string | null;
  score: string | null;
  result: string | null;
  stats: MapStats;
  steamid: string | null;
  demoFile: string | null;
  jobId: string | null;
  pov: PovRecord;
  shorts: ShortRecord[];
  skipped: { at: string; reason: string } | null;
  cleanedAt: string | null;
  legacy: JsonObject | null;
};

export type PovStatus =
  | { kind: "skipped"; reason: string }
  | { kind: "published"; youtubeId: string; publishedAt: string | null }
  | { kind: "ready" }
  | { kind: "pending" };

export type ShortNeed = "related_link" | "thumbnail_check";

export type ShortStatus =
  | { kind: "youtube_skipped"; reason: string }
  | { kind: "verified"; youtubeId: string; publishedAt: string | null }
  | { kind: "published"; youtubeId: string; publishedAt: string | null; needs: ShortNeed[] }
  | { kind: "ready" }
  | { kind: "pending" };

export type MapStage =
  | { kind: "cleaned"; at: string }
  | { kind: "skipped"; at: string; reason: string }
  | { kind: "done" }
  | { kind: "ready" }
  | { kind: "open" };

export type MapEvent =
  | { type: "pov_published"; youtubeId: string; publishedAt: string }
  | { type: "pov_skipped"; reason: string }
  | { type: "short_published"; position: number; youtubeId: string; publishedAt: string }
  | { type: "short_related_linked"; position: number }
  | { type: "short_thumbnail_checked"; position: number }
  | { type: "short_youtube_skipped"; position: number; reason: string }
  | { type: "map_skipped"; at: string; reason: string }
  | { type: "map_unskipped" }
  | { type: "map_cleaned"; at: string };

export type EventResult = { ok: true; map: MapRecord } | { ok: false; error: string };

type Publishable = { videoPath: string | null; thumbnailPath: string | null; title: string | null; tags: string | null };

function hasAssets(item: Publishable): boolean {
  return Boolean(item.videoPath && item.thumbnailPath && item.title && item.tags);
}

export function povStatus(pov: PovRecord): PovStatus {
  if (pov.youtubeId) return { kind: "published", youtubeId: pov.youtubeId, publishedAt: pov.publishedAt };
  if (pov.skipped) return { kind: "skipped", reason: pov.skipped };
  return hasAssets(pov) ? { kind: "ready" } : { kind: "pending" };
}

export function shortStatus(short: ShortRecord): ShortStatus {
  if (short.youtubeId) {
    const needs: ShortNeed[] = [];
    if (short.related === "pending") needs.push("related_link");
    if (!short.thumbnailChecked) needs.push("thumbnail_check");
    if (needs.length === 0) return { kind: "verified", youtubeId: short.youtubeId, publishedAt: short.publishedAt };
    return { kind: "published", youtubeId: short.youtubeId, publishedAt: short.publishedAt, needs };
  }
  if (short.youtubeSkipped) return { kind: "youtube_skipped", reason: short.youtubeSkipped };
  return hasAssets(short) ? { kind: "ready" } : { kind: "pending" };
}

export function mapStage(map: MapRecord): MapStage {
  if (map.cleanedAt) return { kind: "cleaned", at: map.cleanedAt };
  if (map.skipped) return { kind: "skipped", at: map.skipped.at, reason: map.skipped.reason };
  const pov = povStatus(map.pov);
  const shorts = map.shorts.map(shortStatus);
  const hasOutput = map.shorts.length > 0 || pov.kind !== "skipped";
  if (!hasOutput) return { kind: "open" };
  const povSettled = pov.kind === "published" || pov.kind === "skipped";
  const shortsSettled = shorts.every((s) => s.kind === "verified" || s.kind === "youtube_skipped");
  if (povSettled && shortsSettled) return { kind: "done" };
  const povReady = pov.kind !== "pending";
  const shortsReady = shorts.every((s) => s.kind !== "pending");
  return povReady && shortsReady ? { kind: "ready" } : { kind: "open" };
}

function reject(error: string): EventResult {
  return { ok: false, error };
}

function withShort(map: MapRecord, edit: { position: number; change: (short: ShortRecord) => EventResult | ShortRecord }): EventResult {
  const { position, change } = edit;
  const current = map.shorts.find((s) => s.position === position);
  if (current === undefined) return reject(`map ${map.id} has no Short at position ${position}`);
  const next = change(current);
  if ("ok" in next) return next;
  const shorts = map.shorts.map((s) => (s.position === position ? next : s));
  return { ok: true, map: { ...map, shorts } };
}

function shortEvent(map: MapRecord, event: Extract<MapEvent, { position: number }>): EventResult {
  return withShort(map, { position: event.position, change: (short) => {
    const status = shortStatus(short);
    const label = `Short ${event.position} of ${map.id}`;
    switch (event.type) {
      case "short_published":
        if (status.kind !== "ready") return reject(`${label} is ${status.kind}, so it cannot be published`);
        return { ...short, youtubeId: event.youtubeId, publishedAt: event.publishedAt };
      case "short_related_linked":
        if (short.related === "no-pov") return reject(`${label} has no POV to link to`);
        if (status.kind !== "published" && status.kind !== "verified") return reject(`${label} is ${status.kind}: link it after it is published`);
        return { ...short, related: "linked" };
      case "short_thumbnail_checked":
        if (status.kind !== "published" && status.kind !== "verified") return reject(`${label} is ${status.kind}: check its thumbnail after it is published`);
        return { ...short, thumbnailChecked: true };
      case "short_youtube_skipped":
        if (status.kind !== "ready" && status.kind !== "pending") return reject(`${label} is ${status.kind}, so it cannot be skipped`);
        return { ...short, youtubeSkipped: event.reason };
    }
  } });
}

export function applyMapEvent(map: MapRecord, event: MapEvent): EventResult {
  const stage = mapStage(map);
  if (stage.kind === "cleaned") return reject(`map ${map.id} was cleaned at ${stage.at}; it takes no more events`);
  if (stage.kind === "skipped" && event.type !== "map_unskipped") {
    return reject(`map ${map.id} is skipped (${stage.reason}); unskip it first`);
  }
  switch (event.type) {
    case "map_unskipped":
      if (stage.kind !== "skipped") return reject(`map ${map.id} is ${stage.kind}, not skipped`);
      return { ok: true, map: { ...map, skipped: null } };
    case "map_skipped":
      if (stage.kind === "done") return reject(`map ${map.id} is done; a done map cannot be skipped`);
      return { ok: true, map: { ...map, skipped: { at: event.at, reason: event.reason } } };
    case "map_cleaned":
      if (stage.kind !== "done") return reject(`map ${map.id} is ${stage.kind}; only a done map can be cleaned`);
      return { ok: true, map: { ...map, cleanedAt: event.at } };
    case "pov_published": {
      const pov = povStatus(map.pov);
      if (pov.kind !== "ready") return reject(`the POV of ${map.id} is ${pov.kind}, so it cannot be published`);
      return { ok: true, map: { ...map, pov: { ...map.pov, youtubeId: event.youtubeId, publishedAt: event.publishedAt } } };
    }
    case "pov_skipped": {
      const pov = povStatus(map.pov);
      if (pov.kind === "published") return reject(`the POV of ${map.id} is already published`);
      const shorts = map.shorts.map((s): ShortRecord => (s.related === "pending" ? { ...s, related: "no-pov" } : s));
      return { ok: true, map: { ...map, pov: { ...map.pov, skipped: event.reason }, shorts } };
    }
    default:
      return shortEvent(map, event);
  }
}
