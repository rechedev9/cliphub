import { PATTERN_BLOCK_KINDS, PATTERN_MAX_BLOCKS, PATTERN_MAX_PLAYERS } from '../api/patterns.ts';
import type { PatternArea, PatternBlock, PatternBlockKind, PatternDefinition, PatternResult, PatternTrend } from '../api/patterns.ts';
import { TEAM_GRENADE_KINDS } from '../api/grenades.ts';
import type { TeamGrenadeKind } from '../api/grenades.ts';
import type { OverlayTrack } from '../api/spatial.ts';
import type { RadarLevel } from '../api/tactical.ts';
import { ROUND_CLOCK_SECONDS, WHOLE_ROUND, isWholeRound, roundClock, tramoEnd } from '../replay/tramo.ts';
import type { Tramo } from '../replay/tramo.ts';
import type { ReplayRef } from './replay-link.ts';
import type { WorldBox } from './grenade-patterns.ts';

/** Query key of the pattern the finder shows, as JSON; a link or `studio_path` restores it. */
export const PATTERN_QUERY = 'patron';

/** The CSS tokens a player's paths and swatch take, by the player's place in the list. */
export const PATTERN_PLAYER_TOKENS = ['--chart-1', '--chart-2', '--chart-3', '--chart-6', '--chart-7'] as const;

export function patternPlayerToken(index: number): string {
  return PATTERN_PLAYER_TOKENS[index % PATTERN_PLAYER_TOKENS.length] ?? '--fg-1';
}

export const BLOCK_LABEL: Record<PatternBlockKind, string> = { player: 'Jugador', utility: 'Granada', plant: 'Plant' };

/** What inverting a block keeps, said as the rounds it leaves. */
export const INVERT_LABEL: Record<PatternBlockKind, string> = {
  player: 'Rondas en que no pasa',
  utility: 'Rondas en que no la tiran',
  plant: 'Rondas sin plant en la zona',
};

export const RIVAL_LABEL = 'Buscar en el equipo rival';

/** A new block: anyone of the side, anywhere, the whole round. */
export function newBlock(kind: PatternBlockKind): PatternBlock {
  return { kind, from: 0 };
}

export function defaultPattern(): PatternDefinition {
  return { blocks: [newBlock('player')] };
}

/** The block's clock window as a tramo; an open end is the end of the round. */
export function blockTramo(block: PatternBlock): Tramo {
  const to = block.to ?? 0;
  if (block.from <= 0 && (to <= 0 || to >= ROUND_CLOCK_SECONDS)) return WHOLE_ROUND;
  return { from: block.from, to: to <= 0 ? ROUND_CLOCK_SECONDS : to };
}

/** The block with its window set from a tramo; a tramo from the plant reads as the whole round. */
export function withTramo(block: PatternBlock, tramo: Tramo): PatternBlock {
  const next: PatternBlock = { ...block, from: 0 };
  delete next.to;
  if (tramo.fromPlant === true || isWholeRound(tramo)) return next;
  next.from = tramo.from;
  const end = tramoEnd(tramo);
  if (end < ROUND_CLOCK_SECONDS) next.to = end;
  return next;
}

export type DrawnBox = { box: WorldBox; level: RadarLevel; twoFloors: boolean };

/** A dragged box as a block's area; on a two-floor map it keeps the floor it was drawn on. */
export function areaOfBox(drawn: DrawnBox): PatternArea {
  const { box } = drawn;
  const area: PatternArea = {
    min_x: Math.round(box.minX),
    min_y: Math.round(box.minY),
    max_x: Math.round(box.maxX),
    max_y: Math.round(box.maxY),
  };
  return drawn.twoFloors ? { ...area, level: drawn.level } : area;
}

export function boxOfArea(area: PatternArea): WorldBox {
  return { minX: area.min_x, minY: area.min_y, maxX: area.max_x, maxY: area.max_y };
}

/** Whether a world position falls in a block's area, on its floor when it names one. */
export function inArea(area: PatternArea, x: number, y: number, level: RadarLevel): boolean {
  if (x < area.min_x || x > area.max_x || y < area.min_y || y > area.max_y) return false;
  return area.level === undefined || area.level === level;
}

const WHAT_IN_AREA: Record<PatternBlockKind, string> = {
  player: 'Pasa por la caja dibujada',
  utility: 'La granada cae en la caja dibujada',
  plant: 'Planta en la caja dibujada',
};

const ANYWHERE: Record<PatternBlockKind, string> = { player: 'Juega la ronda', utility: 'Tira la granada', plant: 'Planta la bomba' };

/** The block's condition in a sentence, with its window on the round clock. */
export function blockSentence(block: PatternBlock): string {
  let what = WHAT_IN_AREA[block.kind];
  if (block.kind === 'utility' && block.from_throw === true) what = 'La tira desde la caja dibujada';
  if (block.area === undefined) what = ANYWHERE[block.kind];
  const tramo = blockTramo(block);
  const when = isWholeRound(tramo) ? 'en toda la ronda' : `entre ${roundClock(tramo.from)} y ${roundClock(tramoEnd(tramo))}`;
  return `${what} ${when}`;
}

/** A moment on the round clock; past the plant, or the clock's end without one, the time since then. */
export function patternClock(seconds: number, plant?: number): string {
  const from = plant !== undefined && plant < ROUND_CLOCK_SECONDS ? plant : ROUND_CLOCK_SECONDS;
  if (seconds >= from && (plant !== undefined || seconds > from)) return `+${Math.round(seconds - from)}s tras el plant`;
  return roundClock(seconds);
}

export type PatternEntry = { kind: PatternBlockKind; seconds: number; plant?: number };

/** When the round's first timed block held, as its row says it. */
export function entryLabel(entry: PatternEntry): string {
  const clock = patternClock(entry.seconds, entry.plant);
  if (entry.kind === 'utility') return `La tira a ${clock}`;
  if (entry.kind === 'plant') return `Planta a ${clock}`;
  return `Entra en la zona a ${clock}`;
}

/** The kind of the block whose time a row shows: the first one not inverted, and with a zone for a player. */
export function timedKind(pattern: PatternDefinition): PatternBlockKind {
  return pattern.blocks.find((block) => block.invert !== true && (block.kind !== 'player' || block.area !== undefined))?.kind ?? 'player';
}

/** A row without a moment: the pattern holds the whole round. */
export const WHOLE_ROUND_ROW = 'Juega la ronda';

export const TREND_LABEL: Record<PatternTrend, string> = { up: 'Sube', down: 'Baja', flat: 'Estable', '': 'Sin datos' };

export function percentLabel(rate: number): string {
  return `${Math.round(rate * 100)} %`;
}

export function roundCountLabel(n: number): string {
  return `${n} ${n === 1 ? 'ronda' : 'rondas'}`;
}

/** "3 de 12 rondas": matched rounds over the rounds read. */
export function ofRoundsLabel(matched: number, rounds: number): string {
  return `${matched} de ${roundCountLabel(rounds)}`;
}

export function matchCountLabel(n: number): string {
  return `${n} ${n === 1 ? 'partida' : 'partidas'}`;
}

const MONTHS = ['ene', 'feb', 'mar', 'abr', 'may', 'jun', 'jul', 'ago', 'sep', 'oct', 'nov', 'dic'];

/** `2026-11` as "nov 2026". */
export function monthLabel(month: string): string {
  const [year = '', number = ''] = month.split('-');
  return `${MONTHS[Number(number) - 1] ?? number} ${year}`;
}

/** Every matched round with the moment the pattern first held, to open as a playlist. */
export function patternReplayRefs(result: PatternResult): ReplayRef[] {
  return result.matches.flatMap((match) =>
    match.matched.map((row) => (row.tick === undefined ? { matchId: match.match_id, round: row.round } : { matchId: match.match_id, round: row.round, tick: row.tick })),
  );
}

export type TrackWindow = { track: Pick<OverlayTrack, 'start' | 'points'>; step: number; tramo: Tramo };

/** A track's points inside a clock window; `step` is the seconds between points. */
export function trackInWindow(w: TrackWindow): OverlayTrack['points'] {
  const { track, step, tramo } = w;
  if (step <= 0) return [];
  const from = isWholeRound(tramo) ? 0 : tramo.from;
  const to = isWholeRound(tramo) ? Number.POSITIVE_INFINITY : tramoEnd(tramo);
  return track.points.filter((_, i) => {
    const seconds = (track.start + i) * step;
    return seconds >= from && seconds <= to;
  });
}

/* Reading a pattern back from the URL ------------------------------------- */

function field(value: unknown, key: string): unknown {
  return value !== null && typeof value === 'object' ? Reflect.get(value, key) : undefined;
}

function finite(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null;
}

function parseArea(raw: unknown): PatternArea | undefined | null {
  if (raw === undefined) return undefined;
  const minX = finite(field(raw, 'min_x'));
  const minY = finite(field(raw, 'min_y'));
  const maxX = finite(field(raw, 'max_x'));
  const maxY = finite(field(raw, 'max_y'));
  if (minX === null || minY === null || maxX === null || maxY === null || minX >= maxX || minY >= maxY) return null;
  const level = field(raw, 'level');
  const area: PatternArea = { min_x: minX, min_y: minY, max_x: maxX, max_y: maxY };
  if (level === 'default' || level === 'lower') return { ...area, level };
  return level === undefined ? area : null;
}

function isGrenade(value: unknown): value is TeamGrenadeKind {
  return TEAM_GRENADE_KINDS.some((kind) => kind === value);
}

function parseBlock(raw: unknown): PatternBlock | null {
  const kind = PATTERN_BLOCK_KINDS.find((entry) => entry === field(raw, 'kind'));
  const from = finite(field(raw, 'from')) ?? 0;
  const to = finite(field(raw, 'to'));
  const area = parseArea(field(raw, 'area'));
  if (kind === undefined || area === null || from < 0 || (to !== null && to !== 0 && to < from)) return null;
  const block: PatternBlock = { kind, from };
  if (to !== null && to > 0) block.to = to;
  if (area !== undefined) block.area = area;
  const players = field(raw, 'players');
  if (Array.isArray(players)) {
    const ids = players.filter((id): id is string => typeof id === 'string' && /^\d{17}$/.test(id)).slice(0, PATTERN_MAX_PLAYERS);
    if (ids.length > 0) block.players = ids;
  }
  if (field(raw, 'rival') === true) block.rival = true;
  if (field(raw, 'invert') === true) block.invert = true;
  const grenade = field(raw, 'grenade');
  if (kind === 'utility' && isGrenade(grenade)) block.grenade = grenade;
  if (kind === 'utility' && field(raw, 'from_throw') === true) block.from_throw = true;
  return block;
}

/** The pattern in the URL, or null when there is none or it does not read as one. */
export function patternFromQuery(query: Pick<URLSearchParams, 'get'>): PatternDefinition | null {
  const raw = query.get(PATTERN_QUERY);
  if (raw === null || raw === '') return null;
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return null;
  }
  const blocks = field(parsed, 'blocks');
  if (!Array.isArray(blocks) || blocks.length === 0 || blocks.length > PATTERN_MAX_BLOCKS) return null;
  const read = blocks.map(parseBlock);
  const ok = read.filter((block): block is PatternBlock => block !== null);
  if (ok.length !== read.length) return null;
  const side = field(parsed, 'side');
  return side === 'CT' || side === 'T' ? { side, blocks: ok } : { blocks: ok };
}

/** The query with the pattern written over it, every other key kept; null removes it. */
export function writePatternQuery(current: string, pattern: PatternDefinition | null): string {
  const params = new URLSearchParams(current);
  if (pattern === null) params.delete(PATTERN_QUERY);
  else params.set(PATTERN_QUERY, JSON.stringify(pattern));
  return params.toString();
}
