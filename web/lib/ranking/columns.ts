import type { GenericRole, LeaderboardRole, LeaderboardRow, SideBreakdown } from '../api/baseline.ts';
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

function roleLabel(key: string): string {
  for (const [role, label] of Object.entries(ROLE_LABEL)) if (role === key) return label;
  return key;
}

export type RoleChipData = { side: 'ct' | 't'; label: string; breakdown: string };

function chipLabel(options: { breakdown: SideBreakdown | undefined; role: LeaderboardRole | undefined }): string {
  const { breakdown, role } = options;
  if (breakdown?.map_role === ALLROUND) return 'Todoterreno';
  const place = breakdown?.places.find((share) => share.key === breakdown.map_role);
  const named = place?.label ?? breakdown?.map_role ?? '';
  if (named !== '') return named;
  const known = role ?? breakdown?.role;
  return known === undefined ? '' : ROLE_LABEL[known];
}

function breakdownShares(breakdown: SideBreakdown | undefined): string {
  if (breakdown === undefined) return '';
  const roles = breakdown.roles.filter((share) => share.pct >= 1).map((share) => `${share.label ?? roleLabel(share.key)} ${Math.round(share.pct)} %`);
  const places = breakdown.places.map((share) => `${share.label ?? share.key} ${Math.round(share.pct)} %`);
  return [...roles, ...places].join(' · ');
}

/** One chip per side asked, T first, such as {t, "Lurk de B", "Lurk de B 62 % · Canal 21 %"}; a side without a role is left out. */
export function roleChips(row: LeaderboardRow, side: '' | 'CT' | 'T'): RoleChipData[] {
  const chips: RoleChipData[] = [];
  const sides = [
    { key: 't', asked: side !== 'CT', breakdown: row.breakdown?.t, role: row.roles?.t },
    { key: 'ct', asked: side !== 'T', breakdown: row.breakdown?.ct, role: row.roles?.ct },
  ] as const;
  for (const entry of sides) {
    if (!entry.asked) continue;
    const label = chipLabel({ breakdown: entry.breakdown, role: entry.role });
    if (label !== '') chips.push({ side: entry.key, label, breakdown: breakdownShares(entry.breakdown) });
  }
  return chips;
}

/** The library role of a row, such as "AWPer", or empty without one. */
export function libraryRoleText(role: GenericRole | undefined): string {
  return role === undefined ? '' : ROLE_LABEL_LIBRARY[role];
}
