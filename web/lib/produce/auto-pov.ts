// Auto-POV: the players and plays of a summary, picked by the orchestrator (GET /api/jobs/{id}/auto-pov).

import { DEFAULT_PRE_ROLL_SECONDS, type PreRollSeconds } from '../api/types.ts';

/** Mirrors internal/moments: the longest edit time one play may add, such as a victim replay. */
export const AUTO_POV_MAX_EXTRA_SECONDS = 15;

const CUE_KINDS = ['clutch', 'duel', 'bomb', 'momentum'] as const;
type AutoPovCueKind = (typeof CUE_KINDS)[number];
export type AutoPovCue = { kind: AutoPovCueKind; detail: string; count: number; value: number };
export type AutoPovPlayer = { steamId: string; name: string; plays: number };
export type AutoPovDirection = {
  segmentIds: string[];
  /** Segment id → the cues behind that pick. */
  cues: Map<string, AutoPovCue[]>;
  players: AutoPovPlayer[];
};

const SEGMENT_ID = /^[A-Za-z0-9][A-Za-z0-9._-]*$/;

const CLUTCH_RESULT = new Map([['won', ' ganado'], ['lost', ' perdido']]);
const DUEL_LABEL = new Map([['opening', 'Duelo de apertura'], ['chained', 'Duelos encadenados']]);
const BOMB_LABEL = new Map([['retake', 'Retake con la bomba plantada'], ['post_plant', 'Con la bomba plantada']]);

function record(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function finite(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0;
}

function cueKind(value: unknown): AutoPovCueKind | null {
  return CUE_KINDS.find((kind) => kind === value) ?? null;
}

function parseCues(raw: unknown): AutoPovCue[] {
  if (!Array.isArray(raw)) return [];
  return raw.flatMap((item): AutoPovCue[] => {
    if (!record(item)) return [];
    const kind = cueKind(item.kind);
    if (kind === null) return [];
    return [{ kind, detail: typeof item.detail === 'string' ? item.detail : '', count: finite(item.count), value: finite(item.value) }];
  });
}

/** The Auto-POV answer, or null when its shape is not the orchestrator's. */
export function parseAutoPov(raw: unknown): AutoPovDirection | null {
  if (!record(raw) || !Array.isArray(raw.segment_ids) || !Array.isArray(raw.plays) || !Array.isArray(raw.players)) return null;
  const segmentIds = raw.segment_ids.filter((id): id is string => typeof id === 'string' && SEGMENT_ID.test(id));
  const cues = new Map<string, AutoPovCue[]>();
  for (const play of raw.plays) {
    if (record(play) && typeof play.segment_id === 'string') cues.set(play.segment_id, parseCues(play.cues));
  }
  const players = raw.players.flatMap((p): AutoPovPlayer[] => {
    if (!record(p) || typeof p.steamid64 !== 'string' || typeof p.name !== 'string') return [];
    return [{ steamId: p.steamid64, name: p.name, plays: finite(p.plays) }];
  });
  return { segmentIds, cues, players };
}

/** A one-player summary passes its pre-roll, so plays are measured and joined as the video cuts them. */
export type AutoPovOptions = { targetSeconds: number; extraSeconds: number; preRollSeconds?: PreRollSeconds };

export function autoPovUrl(jobId: string, options: AutoPovOptions): string {
  const extra = Math.min(AUTO_POV_MAX_EXTRA_SECONDS, Math.max(0, Math.round(options.extraSeconds * 10) / 10));
  const query = new URLSearchParams({ target_seconds: String(Math.round(options.targetSeconds)), extra_seconds: String(extra) });
  if (options.preRollSeconds !== undefined && options.preRollSeconds !== DEFAULT_PRE_ROLL_SECONDS) query.set('pre_roll_seconds', String(options.preRollSeconds));
  return `/api/demos/${encodeURIComponent(jobId)}/auto-pov?${query.toString()}`;
}

export async function fetchAutoPov(jobId: string, options: AutoPovOptions & { signal?: AbortSignal }): Promise<AutoPovDirection> {
  const response = await fetch(autoPovUrl(jobId, options), { cache: 'no-store', signal: options.signal });
  if (!response.ok) throw new Error(`auto-pov ${response.status}`);
  const direction = parseAutoPov(await response.json());
  if (direction === null) throw new Error('auto-pov: unexpected answer');
  return direction;
}

/** The Spanish name of one cue, as the play card shows it; null for a detail this build does not know. */
export function cueLabel(cue: AutoPovCue): string | null {
  switch (cue.kind) {
    case 'clutch':
      return cue.count > 0 ? `Clutch 1v${cue.count}${CLUTCH_RESULT.get(cue.detail) ?? ''}` : null;
    case 'duel':
      return DUEL_LABEL.get(cue.detail) ?? null;
    case 'bomb':
      return BOMB_LABEL.get(cue.detail) ?? null;
    case 'momentum':
      if (cue.detail === 'swing') return 'Gira la ronda';
      if (cue.detail === 'comeback' && cue.count > 0) return `Corta ${cue.count} rondas perdidas`;
      if (cue.detail === 'run' && cue.count > 0) return `Racha de ${cue.count} rondas`;
      return null;
  }
}

/** Segment id → "Clutch 1v3 ganado · Racha de 4 rondas"; plays without a known cue get no note. */
export function autoPovNotes(direction: AutoPovDirection): Map<string, string> {
  const notes = new Map<string, string>();
  for (const [id, cues] of direction.cues) {
    const labels = cues.map(cueLabel).filter((label): label is string => label !== null);
    if (labels.length > 0) notes.set(id, labels.join(' · '));
  }
  return notes;
}

function namesList(names: readonly string[]): string {
  if (names.length <= 1) return names.join('');
  return `${names.slice(0, -1).join(', ')} y ${names[names.length - 1]}`;
}

/** Only the players the pick left out: the count and the split already show in the plays. */
export function autoPovSummary(direction: AutoPovDirection): string | null {
  const left = direction.players.filter((p) => p.plays === 0).map((p) => p.name);
  if (left.length === 0) return null;
  return `La elección automática no incluye ninguna jugada de ${namesList(left)}.`;
}
