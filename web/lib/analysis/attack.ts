import type { AttackBand, AttackSite, AttackTempo, AttackWarning, TeamAttack } from '../api/attack.ts';
import type { PrepWindow } from '../api/playbook.ts';
import type { ScoutEvidence } from '../api/scout.ts';
import { ROUND_CLOCK_SECONDS } from '../replay/tramo.ts';
import type { Tramo } from '../replay/tramo.ts';
import { findingHref } from './replay-link.ts';

export const BAND_LABEL: Record<AttackBand, string> = { pistol: 'Pistola', eco: 'Eco', half: 'Media compra', full: 'Compra completa' };

export const SITE_LABEL: Record<AttackSite, string> = { A: 'Sitio A', B: 'Sitio B', both: 'Los dos sitios', none: 'Ningún sitio' };

export const TEMPO_LABEL: Record<AttackTempo, string> = {
  rush: 'Rush',
  set_strat: 'Estrategia preparada',
  mid_round: 'Media ronda',
  no_site: 'Sin pisar sitio',
};

function known<K extends string>(labels: Record<K, string>, key: string): key is K {
  return Object.hasOwn(labels, key);
}

export function siteLabel(key: string): string {
  return known(SITE_LABEL, key) ? SITE_LABEL[key] : key;
}

export function tempoLabel(key: string): string {
  return known(TEMPO_LABEL, key) ? TEMPO_LABEL[key] : key;
}

/** The bombsite zones a site key marks on the map. */
export function siteZones(site: string): string[] {
  if (site === 'A') return ['BombsiteA'];
  if (site === 'B') return ['BombsiteB'];
  return site === 'both' ? ['BombsiteA', 'BombsiteB'] : [];
}

/** A window of the prep as the Replay's tramo: an open end runs to the end of the round clock. */
export function windowTramo(window: PrepWindow): Tramo {
  return { from: window.from_seconds, to: window.to_seconds === 0 ? ROUND_CLOCK_SECONDS : window.to_seconds };
}

/** The opening of the round a default is read in, the same tramo as the first prep window. */
export const DEFAULT_TRAMO: Tramo = { from: 0, to: 55 };

/** The age of the sample, always shown in the Ataque header. */
export function lastMatchText(days: number): string {
  if (days <= 0) return 'Última partida hoy';
  return `Última partida hace ${days} ${days === 1 ? 'día' : 'días'}`;
}

export function warningText(warning: AttackWarning): string {
  switch (warning.kind) {
    case 'stale':
      return `La última partida es de hace ${warning.days ?? 0} días: puede que ya no jueguen así.`;
    case 'few_rounds':
      return `Solo hay ${warning.rounds ?? 0} rondas de T en este mapa: tómalo como pista, no como norma.`;
    case 'capped':
      return `Se leyeron las ${warning.matches ?? 0} partidas más recientes de ${warning.available ?? 0}: acota las fechas o los rivales para leer el resto.`;
    case 'roster_changed': {
      const parts: string[] = [];
      if ((warning.joined ?? []).length > 0) parts.push(`entraron ${(warning.joined ?? []).join(', ')}`);
      if ((warning.left ?? []).length > 0) parts.push(`salieron ${(warning.left ?? []).join(', ')}`);
      return `La plantilla cambió en estas partidas (${parts.join('; ')}).`;
    }
  }
}

/** The Replay link of one finding of the attack prep; undefined when it has no rounds. */
export function attackHref(
  attack: Pick<TeamAttack, 'team_name' | 'map'>,
  finding: { refs: readonly ScoutEvidence[]; what: string; zones?: readonly string[]; focus?: readonly string[]; tramo?: Tramo },
): string | undefined {
  if (finding.refs.length === 0) return undefined;
  const refs = finding.refs.map((ref) => ({ matchId: ref.match_id, round: ref.round }));
  return findingHref({ refs, title: `${attack.team_name} · ${finding.what}`, zones: finding.zones, focus: finding.focus, tramo: finding.tramo });
}
