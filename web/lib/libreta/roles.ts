import type { LibraryRosterEntry } from '../api/library.ts';
import { MAX_ROLE_CALLOUT, MAX_ROLE_NAME, MAX_ROLE_SLOTS, MAX_ROLE_TASK, type RoleSlot, type RoleSuggestion } from '../api/roles.ts';
import type { LibraryPlay, PlayPosition } from '../api/stratbook.ts';
import type { TacticalSide } from '../api/tactical.ts';
import { placeLabel } from '../tactical-labels.ts';

const SUGGESTED_ROLE_LABEL: ReadonlyMap<string, string> = new Map([
  ['awp', 'AWP'],
  ['entry', 'Entry'],
  ['support', 'Apoyo'],
  ['rifler', 'Rifler'],
  ['unknown', 'Rifler'],
  ['anchor', 'Ancla'],
  ['rotator', 'Rotador'],
]);

/** A suggested role in Spanish; a role a newer Studio adds reads as Rifler. */
export function suggestedRoleLabel(role: string): string {
  return SUGGESTED_ROLE_LABEL.get(role) ?? 'Rifler';
}

/** A slot's callout as players say it: a place token such as `BombsiteA` reads `Bombsite A`. */
export function roleCalloutLabel(callout: string | undefined): string {
  const text = (callout ?? '').trim();
  return text === '' ? '' : placeLabel(text);
}

/** A slot with its text trimmed and cut to the server limits, empty optional fields left out. */
function compactSlot(slot: RoleSlot): RoleSlot {
  const out: RoleSlot = { name: slot.name.trim().slice(0, MAX_ROLE_NAME) };
  const steamid64 = (slot.steamid64 ?? '').trim();
  const callout = (slot.callout ?? '').trim().slice(0, MAX_ROLE_CALLOUT);
  const task = (slot.task ?? '').trim().slice(0, MAX_ROLE_TASK);
  if (steamid64 !== '') out.steamid64 = steamid64;
  if (callout !== '') out.callout = callout;
  if (task !== '') out.task = task;
  return out;
}

function isBlank(slot: RoleSlot): boolean {
  const compact = compactSlot(slot);
  return compact.name === '' && compact.steamid64 === undefined && compact.callout === undefined && compact.task === undefined;
}

/** One side of a team's sheet from the suggestion: the role's label, the player and, on CT, the spot. */
export function sheetFromSuggestion(suggested: readonly RoleSuggestion[]): RoleSlot[] {
  return suggested
    .slice(0, MAX_ROLE_SLOTS)
    .map((entry) => compactSlot({ name: suggestedRoleLabel(entry.role), steamid64: entry.steamid64, callout: roleCalloutLabel(entry.callout) }));
}

/** A play's first roles, one per player of its positions, named after the player until the coach renames them. */
export function rolesFromPositions(play: { positions: readonly PlayPosition[] }): RoleSlot[] {
  const seen = new Set<string>();
  const out: RoleSlot[] = [];
  for (const position of play.positions) {
    const steamid64 = (position.steamid64 ?? '').trim();
    if (steamid64 !== '' && seen.has(steamid64)) continue;
    if (steamid64 !== '') seen.add(steamid64);
    const player = (position.player ?? '').trim();
    out.push(compactSlot({ name: player === '' ? `Rol ${out.length + 1}` : player, steamid64, callout: roleCalloutLabel(position.callout) }));
    if (out.length === MAX_ROLE_SLOTS) break;
  }
  return out;
}

export type CleanRoles = { ok: true; slots: RoleSlot[] } | { ok: false; error: string };

/** The rows of an editor as the server takes them: blank rows dropped, every other row named. */
export function cleanRoles(slots: readonly RoleSlot[]): CleanRoles {
  const kept = slots.filter((slot) => !isBlank(slot)).map(compactSlot);
  if (kept.some((slot) => slot.name === '')) return { ok: false, error: 'Cada rol necesita un nombre.' };
  if (kept.length > MAX_ROLE_SLOTS) return { ok: false, error: `Como mucho ${MAX_ROLE_SLOTS} roles por lado.` };
  return { ok: true, slots: kept };
}

/** True when both lists hold the same roles once blank rows and spaces are dropped. */
export function sameRoles(a: readonly RoleSlot[], b: readonly RoleSlot[]): boolean {
  const left = a.filter((slot) => !isBlank(slot)).map(compactSlot);
  const right = b.filter((slot) => !isBlank(slot)).map(compactSlot);
  return JSON.stringify(left) === JSON.stringify(right);
}

export type RoleSides = { t: RoleSlot[]; ct: RoleSlot[] };

/** Fills each side that has no role yet from the suggestion; `filled` names the sides it changed. */
export function fillFromSuggestion(sides: RoleSides, suggested: { t: readonly RoleSuggestion[]; ct: readonly RoleSuggestion[] }): RoleSides & { filled: TacticalSide[] } {
  const filled: TacticalSide[] = [];
  const pick = (current: RoleSlot[], from: readonly RoleSuggestion[], side: TacticalSide): RoleSlot[] => {
    if (from.length === 0 || current.some((slot) => !isBlank(slot))) return current;
    filled.push(side);
    return sheetFromSuggestion(from);
  };
  const t = pick(sides.t, suggested.t, 'T');
  const ct = pick(sides.ct, suggested.ct, 'CT');
  return { t, ct, filled };
}

export type RolePlayer = { steamid64: string; nick: string };

/** The players a role can name: the roster by nick, then players of a play or a suggestion who are not on it. */
export function rolePlayers(roster: readonly LibraryRosterEntry[], others: readonly { steamid64?: string; player?: string }[] = []): RolePlayer[] {
  const out = [...roster]
    .map((entry) => ({ steamid64: entry.steamid64, nick: entry.nick }))
    .sort((a, b) => a.nick.localeCompare(b.nick, 'es'));
  const seen = new Set(out.map((player) => player.steamid64));
  for (const other of others) {
    const steamid64 = (other.steamid64 ?? '').trim();
    if (steamid64 === '' || seen.has(steamid64)) continue;
    seen.add(steamid64);
    out.push({ steamid64, nick: (other.player ?? '').trim() || steamid64 });
  }
  return out;
}

export type PlayRoleRow = { key: string; play: LibraryPlay; slot: RoleSlot; player: string };

export type PlayRoleScope = {
  map: string;
  /** A team id, or null for the plays without a team. */
  team: string | null;
  /** Nicks by SteamID64, such as the team's roster; the play's own positions fill the gaps. */
  names?: ReadonlyMap<string, string>;
};

/** Every role of the plays on one map and team, one row each, in play order. */
export function playRoleRows(plays: readonly LibraryPlay[], scope: PlayRoleScope): PlayRoleRow[] {
  const rows: PlayRoleRow[] = [];
  for (const play of plays) {
    if (play.map !== scope.map || play.team_id !== scope.team) continue;
    play.roles.forEach((slot, index) => {
      const steamid64 = slot.steamid64 ?? '';
      const fromPlay = play.positions.find((position) => position.steamid64 === steamid64)?.player;
      const player = steamid64 === '' ? '' : (scope.names?.get(steamid64) ?? fromPlay ?? '');
      rows.push({ key: `${play.id}:${index}`, play, slot, player });
    });
  }
  return rows;
}

/** The team with most live plays, so the Roles tab opens where the work is; null when no play has a team. */
export function busiestTeam(plays: readonly LibraryPlay[]): string | null {
  const counts = new Map<string, number>();
  for (const play of plays) {
    if (play.team_id !== null && play.status !== 'archived') counts.set(play.team_id, (counts.get(play.team_id) ?? 0) + 1);
  }
  let best: string | null = null;
  let most = 0;
  for (const [team, count] of counts) {
    if (count > most) {
      best = team;
      most = count;
    }
  }
  return best;
}
