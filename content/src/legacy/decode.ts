import { z } from "zod";
import type { JsonObject, MapRecord, MapSource, PovRecord, RelatedLink, ShortRecord } from "../state/model.ts";
import type { UploadKind, UploadReceipt } from "../state/store.ts";

export type Decoded<T> = { ok: true; value: T } | { ok: false; error: string };

const text = z.string().nullable().optional();
const count = z.number().nullable().optional();
const whole = z.int().nullable().optional();

const legacyPov = z.object({
  video: text,
  thumbnail: text,
  title: text,
  tags: text,
  youtube_id: text,
  published_at: text,
  skipped: text,
});

const legacyShort = z.object({
  segment: text,
  segments: z.array(z.union([z.string(), z.number()])).optional(),
  round: whole,
  play: text,
  video: text,
  thumbnail: text,
  title: text,
  tags: text,
  youtube_id: text,
  published_at: text,
  scheduled_for: text,
  related_linked: z.union([z.boolean(), z.literal("no-pov")]).optional(),
  thumbnail_checked: z.boolean().optional(),
  youtube_skipped: text,
});

const legacySkipped = z.object({ at: z.string(), reason: z.string() });

const legacyMap = z.object({
  player: z.string().min(1),
  source: z.enum(["event", "faceit", "clip"]).optional(),
  hltv_match_id: whole,
  faceit_match_id: text,
  map: text,
  match_url: text,
  match_time: text,
  date: text,
  event: text,
  stage: text,
  team: text,
  opponent: text,
  score: text,
  result: text,
  kills: whole,
  deaths: whole,
  adr: count,
  rating: count,
  hs_pct: count,
  elo: whole,
  steamid: text,
  demo: text,
  job_id: text,
  pov: z.record(z.string(), z.unknown()).optional(),
  shorts: z.array(z.record(z.string(), z.unknown())).optional(),
  skipped: legacySkipped.optional(),
  cleaned_at: text,
});

const legacyReceipt = z.object({
  id: z.string().min(1),
  title: text,
  visibility: text,
  publish_at: text,
  uploaded_at: text,
  thumbnail: text,
});

function value(input: string | null | undefined): string | null {
  return input === undefined || input === null || input === "" ? null : input;
}

function rest(raw: Record<string, unknown>, shape: object): JsonObject {
  const known = new Set(Object.keys(shape));
  return Object.fromEntries(Object.entries(raw).filter(([key]) => !known.has(key)));
}

function issue(error: z.ZodError, where: string): string {
  return `${where}: ${z.prettifyError(error)}`;
}

function related(input: boolean | "no-pov" | undefined): RelatedLink {
  if (input === "no-pov") return "no-pov";
  return input === true ? "linked" : "pending";
}

function decodeShort(raw: Record<string, unknown>, position: number): Decoded<ShortRecord> {
  const parsed = legacyShort.safeParse(raw);
  if (!parsed.success) return { ok: false, error: issue(parsed.error, `shorts[${position}]`) };
  const s = parsed.data;
  const legacy = rest(raw, legacyShort.shape);
  const segment = value(s.segment);
  const segments = s.segments?.map(String) ?? (segment === null ? [] : [segment]);
  if (segment !== null && !segments.includes(segment)) legacy.segment = segment;
  return {
    ok: true,
    value: {
      position,
      round: s.round ?? null,
      segments,
      play: value(s.play),
      videoPath: value(s.video),
      thumbnailPath: value(s.thumbnail),
      title: value(s.title),
      tags: value(s.tags),
      youtubeId: value(s.youtube_id),
      scheduledFor: value(s.scheduled_for),
      publishedAt: value(s.published_at),
      related: related(s.related_linked),
      thumbnailChecked: s.thumbnail_checked === true,
      youtubeSkipped: value(s.youtube_skipped),
      legacy: Object.keys(legacy).length === 0 ? null : legacy,
    },
  };
}

function decodePov(raw: Record<string, unknown> | undefined): Decoded<{ pov: PovRecord; legacy: JsonObject }> {
  const parsed = legacyPov.safeParse(raw ?? {});
  if (!parsed.success) return { ok: false, error: issue(parsed.error, "pov") };
  const p = parsed.data;
  return {
    ok: true,
    value: {
      pov: {
        videoPath: value(p.video),
        thumbnailPath: value(p.thumbnail),
        title: value(p.title),
        tags: value(p.tags),
        youtubeId: value(p.youtube_id),
        publishedAt: value(p.published_at),
        skipped: value(p.skipped),
      },
      legacy: rest(raw ?? {}, legacyPov.shape),
    },
  };
}

function inferSource(id: string, map: z.infer<typeof legacyMap>): MapSource | null {
  if (map.source) return map.source;
  if (value(map.faceit_match_id) !== null) return "faceit";
  if (typeof map.hltv_match_id === "number") return "event";
  return id.startsWith("clip-") ? "clip" : null;
}

export function decodeLegacyMap(id: string, raw: unknown): Decoded<MapRecord> {
  if (typeof raw !== "object" || raw === null || Array.isArray(raw)) return { ok: false, error: "map.json is not a JSON object" };
  const record: Record<string, unknown> = { ...raw };
  const parsed = legacyMap.safeParse(record);
  if (!parsed.success) return { ok: false, error: issue(parsed.error, "map.json") };
  const m = parsed.data;
  const source = inferSource(id, m);
  if (source === null) return { ok: false, error: "map.json has no source, faceit_match_id or hltv_match_id" };
  const pov = decodePov(m.pov);
  if (!pov.ok) return pov;
  const shorts: ShortRecord[] = [];
  for (const [index, short] of (m.shorts ?? []).entries()) {
    const decoded = decodeShort(short, index + 1);
    if (!decoded.ok) return decoded;
    shorts.push(decoded.value);
  }
  const legacy = rest(record, legacyMap.shape);
  if (Object.keys(pov.value.legacy).length > 0) legacy.pov = pov.value.legacy;
  const rawSkipped = record.skipped;
  if (typeof rawSkipped === "object" && rawSkipped !== null && !Array.isArray(rawSkipped)) {
    const extra = rest({ ...rawSkipped }, legacySkipped.shape);
    if (Object.keys(extra).length > 0) legacy.skipped = extra;
  }
  return {
    ok: true,
    value: {
      id,
      source,
      player: m.player,
      mapName: value(m.map),
      hltvMatchId: m.hltv_match_id ?? null,
      faceitMatchId: value(m.faceit_match_id),
      matchUrl: value(m.match_url),
      matchTime: value(m.match_time),
      matchDate: value(m.date),
      event: value(m.event),
      eventStage: value(m.stage),
      team: value(m.team),
      opponent: value(m.opponent),
      score: value(m.score),
      result: value(m.result),
      stats: {
        kills: m.kills ?? null,
        deaths: m.deaths ?? null,
        adr: m.adr ?? null,
        rating: m.rating ?? null,
        hsPct: m.hs_pct ?? null,
        elo: m.elo ?? null,
      },
      steamid: value(m.steamid),
      demoFile: value(m.demo),
      jobId: value(m.job_id),
      pov: pov.value.pov,
      shorts,
      skipped: m.skipped ?? null,
      cleanedAt: value(m.cleaned_at),
      legacy: Object.keys(legacy).length === 0 ? null : legacy,
    },
  };
}

function receiptKind(receipt: string): UploadKind {
  return receipt.startsWith("pov") ? "pov" : "short";
}

export function decodeLegacyReceipt(options: { receipt: string; mapId: string | null; raw: unknown }): Decoded<UploadReceipt> {
  const { raw } = options;
  if (typeof raw !== "object" || raw === null || Array.isArray(raw)) return { ok: false, error: "receipt is not a JSON object" };
  const parsed = legacyReceipt.safeParse(raw);
  if (!parsed.success) return { ok: false, error: issue(parsed.error, "receipt") };
  const r = parsed.data;
  return {
    ok: true,
    value: {
      youtubeId: r.id,
      mapId: options.mapId,
      receipt: options.receipt,
      kind: receiptKind(options.receipt),
      title: value(r.title),
      visibility: value(r.visibility),
      publishAt: value(r.publish_at),
      uploadedAt: value(r.uploaded_at),
      thumbnail: value(r.thumbnail),
      raw: { ...raw },
    },
  };
}
