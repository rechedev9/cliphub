import type { LeaderboardRole, LeaderboardRoles } from '../api/baseline.ts';
import { formatDecimal } from '../format.ts';
import type { RankingSort } from './ranking.ts';

export const ROLE_LABEL: Readonly<Record<LeaderboardRole, string>> = {
  a_anchor: 'Ancla A',
  b_anchor: 'Ancla B',
  rotator: 'Rotador',
  a_lurk: 'Lurker de A',
  b_lurk: 'Lurker de B',
  half_lurk: 'Medio lurker',
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
  { sort: 'rating', header: 'Rating', title: 'Rating estimado con la fórmula pública de HLTV 2.0', format: decimals(2) },
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

/** "CT Ancla B · T Lurker de B", or empty when no role could be read. */
export function rolesText(roles: LeaderboardRoles | undefined, side: '' | 'CT' | 'T'): string {
  if (roles === undefined) return '';
  const parts: string[] = [];
  if (roles.ct !== undefined && side !== 'T') parts.push(`CT ${ROLE_LABEL[roles.ct]}`);
  if (roles.t !== undefined && side !== 'CT') parts.push(`T ${ROLE_LABEL[roles.t]}`);
  return parts.join(' · ');
}
