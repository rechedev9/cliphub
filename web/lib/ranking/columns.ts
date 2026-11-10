import type { GenericRole, LeaderboardRole, LeaderboardRoles, LeaderboardRow, SideBreakdown } from '../api/baseline.ts';
import { RATING_FORMULA_TEXT } from '../analysis/rating.ts';
import { formatDecimal } from '../format.ts';
import { ROLE_LABEL as ROLE_LABEL_LIBRARY } from '../library/partidas.ts';
import { ALLROUND, type RankingSort } from './ranking.ts';

export const ROLE_LABEL: Readonly<Record<LeaderboardRole, string>> = {
  a_anchor: 'Ancla A',
  b_anchor: 'Ancla B',
  rotator: 'Rotador',
  a_lurk: 'Lurk de A',
  b_lurk: 'Lurk de B',
  half_lurk: 'Medio lurk',
  pack: 'Pack',
};

export type RankingColumn = {
  readonly sort: RankingSort;
  readonly header: string;
  /** Spoken in full where the header is short. */
  readonly title: string;
  readonly format: (value: number) => string;
};

const decimals = (digits: number) => (value: number): string => formatDecimal(value, digits);
const percent = (value: number): string => `${formatDecimal(value, 0)} %`;

/** Table columns after the player, in reading order. Rating leads, as in every player ranking. */
export const RANKING_COLUMNS: readonly RankingColumn[] = [
  { sort: 'rating', header: 'Rating', title: RATING_FORMULA_TEXT, format: decimals(2) },
  { sort: 'adr', header: 'ADR', title: 'Daño medio por ronda', format: decimals(1) },
  { sort: 'kast', header: 'KAST', title: 'Rondas con baja, asistencia, supervivencia o muerte vengada', format: percent },
  { sort: 'kd', header: 'K/D', title: 'Bajas entre muertes', format: decimals(2) },
  { sort: 'kpr', header: 'KPR', title: 'Bajas por ronda', format: decimals(2) },
  { sort: 'dpr', header: 'DPR', title: 'Muertes por ronda, mejor cuanto menos', format: decimals(2) },
  { sort: 'opening_win_pct', header: 'Aperturas', title: 'Duelos de apertura ganados', format: percent },
  { sort: 'utility_damage_per_round', header: 'UDR', title: 'Daño de utilidad por ronda', format: decimals(1) },
  { sort: 'flash_assists_per_round', header: 'FAR', title: 'Asistencias con flash por ronda', format: decimals(2) },
  { sort: 'rounds', header: 'Rondas', title: 'Rondas jugadas dentro del filtro', format: decimals(0) },
];

function roleName(key: string): string {
  for (const [role, label] of Object.entries(ROLE_LABEL)) if (role === key) return label.toLowerCase();
  return key;
}

function sideText(side: 'CT' | 'T', breakdown: SideBreakdown | undefined): string {
  if (breakdown === undefined) return '';
  const place = breakdown.places.find((share) => share.key === breakdown.map_role);
  const named = place?.label ?? breakdown.map_role ?? '';
  const main = breakdown.map_role === ALLROUND ? 'Todoterreno' : named;
  const roles = breakdown.roles.filter((share) => share.pct >= 1).map((share) => `${Math.round(share.pct)} % ${roleName(share.key)}`);
  const places = breakdown.places.map((share) => `${Math.round(share.pct)} % ${share.label ?? share.key}`);
  const details = [roles.join(', '), places.join(', ')].filter((part) => part !== '').join('; ');
  return [`${side} ${main}`.trim(), details === '' ? '' : `(${details})`].filter((part) => part !== '').join(' ');
}

/** The shares of a player's rounds by role and by position per side, such as "CT Canal (70 % ancla B; 60 % Canal)", or empty without them. */
export function breakdownText(row: LeaderboardRow, side: '' | 'CT' | 'T'): string {
  const parts: string[] = [];
  if (side !== 'T') parts.push(sideText('CT', row.breakdown?.ct));
  if (side !== 'CT') parts.push(sideText('T', row.breakdown?.t));
  return parts.filter((part) => part !== '').join(' · ');
}

/** The library role of a row, such as "AWPer", or empty without one. */
export function libraryRoleText(role: GenericRole | undefined): string {
  return role === undefined ? '' : ROLE_LABEL_LIBRARY[role];
}

/** "CT Ancla B · T Lurk de B", or empty when no role could be read. */
export function rolesText(roles: LeaderboardRoles | undefined, side: '' | 'CT' | 'T'): string {
  if (roles === undefined) return '';
  const parts: string[] = [];
  if (roles.ct !== undefined && side !== 'T') parts.push(`CT ${ROLE_LABEL[roles.ct]}`);
  if (roles.t !== undefined && side !== 'CT') parts.push(`T ${ROLE_LABEL[roles.t]}`);
  return parts.join(' · ');
}
