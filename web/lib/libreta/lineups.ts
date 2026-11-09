import type { TeamGrenade } from '../api/grenades.ts';
import type { LibraryLineup, LineupClick, LineupMovement, LineupThrow } from '../api/lineups.ts';
import { PRACTICE_MAX_SPOTS, type PracticeGrenade, type PracticeSpot } from '../api/practice.ts';

export const MOVEMENT_LABEL: Record<LineupMovement, string> = {
  standing: 'Parado',
  walking: 'Andando',
  running: 'Corriendo',
  crouching: 'Agachado',
};

export const CLICK_LABEL: Record<LineupClick, string> = {
  left: 'Clic izquierdo',
  right: 'Clic derecho',
  both: 'Los dos clics',
};

/** The server's caps (`library.MaxLineupTitleRunes`, `maxWorldCoordinate`). */
export const MAX_LINEUP_TITLE = 120;
const MAX_WORLD_COORDINATE = 65536;

export type SetposAim = { origin: [number, number, number]; pitch: number; yaw: number };

const NUMBER = String.raw`[-+]?(?:\d+(?:\.\d*)?|\.\d+)(?:e[-+]?\d+)?`;
const SETPOS = new RegExp(String.raw`\bsetpos(?:_exact)?\s+(${NUMBER})\s+(${NUMBER})\s+(${NUMBER})`, 'i');
const SETANG = new RegExp(String.raw`\bsetang(?:_exact)?\s+(${NUMBER})\s+(${NUMBER})`, 'i');

/** Mirrors `tacticalplan.SetangPitch`: a demo pitch of 270 to 360 reads as -90 to 0. */
function setangPitch(deg: number): number {
  let out = deg % 360;
  if (out > 180) out -= 360;
  else if (out < -180) out += 360;
  return Math.min(Math.max(out, -90), 90);
}

function setangYaw(deg: number): number {
  let out = deg % 360;
  if (out > 180) out -= 360;
  else if (out <= -180) out += 360;
  return out;
}

/**
 * Reads what CS2's `getpos` prints, `setpos x y z;setang pitch yaw roll`, in
 * either order, with `_exact` or not; null when either half is missing.
 */
export function parseSetpos(text: string): SetposAim | null {
  const pos = SETPOS.exec(text);
  const ang = SETANG.exec(text);
  if (pos === null || ang === null) return null;
  const origin: [number, number, number] = [Number(pos[1]), Number(pos[2]), Number(pos[3])];
  const pitch = Number(ang[1]);
  const yaw = Number(ang[2]);
  if (!origin.every((v) => Number.isFinite(v) && Math.abs(v) <= MAX_WORLD_COORDINATE)) return null;
  if (!Number.isFinite(pitch) || !Number.isFinite(yaw)) return null;
  return { origin, pitch: setangPitch(pitch), yaw: setangYaw(yaw) };
}

function fixed(value: number): string {
  const text = value.toFixed(2);
  return text === '-0.00' ? '0.00' : text;
}

/** The console line that puts a player on the lineup: `setpos x y z;setang pitch yaw 0`; z is the feet origin. */
export function setposCommand(aim: SetposAim): string {
  const [x, y, z] = aim.origin;
  return `setpos ${fixed(x)} ${fixed(y)} ${fixed(z)};setang ${fixed(aim.pitch)} ${fixed(aim.yaw)} 0`;
}

/** The console line that puts you where a replay player stands and looks, from one position sample. */
export function sampleSetpos(sample: { x: number; y: number; z: number; yaw: number; pitch?: number }): string {
  return setposCommand({ origin: [sample.x, sample.y, sample.z], pitch: setangPitch(sample.pitch ?? 0), yaw: setangYaw(sample.yaw) });
}

/** CT throws an incendiary, T a molotov; CS2 hands them out as different weapons. */
function practiceGrenade(lineup: Pick<LibraryLineup, 'grenade' | 'side'>): PracticeGrenade {
  return lineup.grenade === 'molotov' && lineup.side === 'CT' ? 'incendiary' : lineup.grenade;
}

/** The practice spot that drops the player on the lineup with its grenade in hand. */
export function lineupSpot(lineup: Pick<LibraryLineup, 'title' | 'origin' | 'pitch' | 'yaw' | 'grenade' | 'side'>): PracticeSpot {
  return {
    label: lineup.title.slice(0, 200),
    pos: [lineup.origin[0], lineup.origin[1], lineup.origin[2]],
    ang: [lineup.pitch, lineup.yaw],
    grenade: practiceGrenade(lineup),
  };
}

/** Why these lineups cannot go into one practice, or null when they can. */
export function practiceBlockReason(lineups: readonly Pick<LibraryLineup, 'map'>[]): string | null {
  if (lineups.length === 0) return 'Elige al menos un lineup.';
  if (new Set(lineups.map((lineup) => lineup.map)).size > 1) return 'Una práctica carga un solo mapa: elige lineups del mismo mapa.';
  if (lineups.length > PRACTICE_MAX_SPOTS) return `Caben ${PRACTICE_MAX_SPOTS} lineups por práctica.`;
  return null;
}

/** "Corriendo, salto, clic izquierdo". */
export function techniqueLabel(lineup: Pick<LibraryLineup, 'movement' | 'jump' | 'click'>): string {
  const parts = [MOVEMENT_LABEL[lineup.movement]];
  if (lineup.jump) parts.push('salto');
  parts.push(CLICK_LABEL[lineup.click].toLocaleLowerCase('es'));
  return parts.join(', ');
}

/** The throw the server reads a lineup from, or null when the demo did not record where and how it was thrown. */
export function lineupThrow(grenade: TeamGrenade): LineupThrow | null {
  if (grenade.origin === null || grenade.view === null || grenade.throw_tick === null) return null;
  return { match_id: grenade.match_id, round: grenade.round, throw_tick: grenade.throw_tick };
}

/** What to tell the user when a throw cannot be saved, by the server's error code. */
export function lineupSaveError(code: string | undefined): string {
  switch (code) {
    case 'throw_missing':
      return 'Esta partida se analizó antes de que Studio guardara los lanzamientos. Vuelve a analizar la demo y prueba otra vez.';
    case 'throw_not_found':
      return 'No se encontró ese lanzamiento en la ronda. Vuelve a analizar la demo y prueba otra vez.';
    case 'unknown_match':
      return 'La partida ya no está en la biblioteca.';
    case 'tactical_not_ready':
      return 'El análisis de la partida todavía no está listo. Prueba en unos minutos.';
    default:
      return 'No se pudo guardar el lineup.';
  }
}
