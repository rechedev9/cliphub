import type { PlayerSidePositions, PositionRole, PositionZone } from '../api/team-insights.ts';
import type { TacticalRate } from '../api/tactical.ts';

/** How a player's early-round positions read: role names and the sentence of a role card. */

export const POSITION_ROLE_LABEL: Readonly<Record<PositionRole, string>> = {
  a_anchor: 'Ancla A',
  b_anchor: 'Ancla B',
  rotator: 'Rotador',
};

const ZONE_LABEL: Readonly<Record<PositionZone, string>> = { A: 'A', B: 'B', other: 'medio' };

function rounds(n: number): string {
  return n === 1 ? '1 ronda' : `${n} rondas`;
}

/** "64 % de 45 rondas"; empty without rounds. */
export function rateText(rate: TacticalRate): string {
  if (rate.total <= 0) return '';
  return `${Math.round(rate.pct)} % de ${rounds(rate.total)}`;
}

function zoneRate(side: PlayerSidePositions, zone: PositionZone): TacticalRate | undefined {
  return side.zones.find((entry) => entry.zone === zone)?.rounds;
}

/**
 * The role card's sentence: "Ancla B en Mirage, 64 % de 45 rondas". A rotator
 * reads as its split between the sites; T sides carry no role and read as the
 * zone they lean to.
 */
export function roleSentence(side: PlayerSidePositions, mapName: string): string {
  if (side.sampled <= 0) return `Sin posiciones leídas en ${mapName}`;
  if (side.role === 'a_anchor' || side.role === 'b_anchor') {
    const rate = zoneRate(side, side.role === 'a_anchor' ? 'A' : 'B');
    const share = rate ? `, ${rateText(rate)}` : '';
    return `${POSITION_ROLE_LABEL[side.role]} en ${mapName}${share}`;
  }
  const a = zoneRate(side, 'A');
  const b = zoneRate(side, 'B');
  if (side.role === 'rotator') {
    return `Rotador en ${mapName}: ${Math.round(a?.pct ?? 0)} % en A y ${Math.round(b?.pct ?? 0)} % en B de ${rounds(side.sampled)}`;
  }
  const lead = [...side.zones].sort((x, y) => y.rounds.count - x.rounds.count)[0];
  if (lead === undefined || lead.rounds.count === 0) return `Sin zona clara en ${mapName}`;
  return `Juega en ${ZONE_LABEL[lead.zone]} en ${mapName}, ${rateText(lead.rounds)}`;
}

/** "Mata con AWP en 12 de 40 rondas (30 %), 18 de sus 52 bajas"; null without AWP kills. */
export function awpText(side: PlayerSidePositions): string | null {
  if (side.awp_rounds.count === 0) return null;
  return `Mata con AWP en ${side.awp_rounds.count} de ${rounds(side.awp_rounds.total)} (${Math.round(side.awp_rounds.pct)} %), ${side.awp_kills.count} de sus ${side.awp_kills.total} bajas`;
}
