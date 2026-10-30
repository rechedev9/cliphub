import type { SeriesFormat, SeriesSlot, VetoAction, VetoBasis, VetoStep, VetoTeam } from '../api/veto.ts';

/** Pure reads for series, map history and the veto simulator. */

export const SERIES_FORMAT_LABEL: Record<SeriesFormat, string> = { bo1: 'BO1', bo3: 'BO3', bo5: 'BO5' };

export const VETO_ACTION_LABEL: Record<VetoAction, string> = { ban: 'Ban', pick: 'Pick', decider: 'Decider' };

export const VETO_BASIS_LABEL: Record<VetoBasis, string> = {
  chosen: 'Elegido',
  edge: 'Por ventaja',
  history: 'Por costumbre del rival',
  leftover: 'El que queda',
};

/**
 * The step order of a format over a pool, the same rule as Go's
 * `library.VetoSequence`: two leading bans, the picks, the remaining bans and
 * the decider, teams alternating from `first`. Null when the pool does not fit.
 */
export function vetoSequence(format: SeriesFormat, pool: number, first: SeriesSlot): { team: VetoTeam; action: VetoAction }[] | null {
  const picks = { bo1: 0, bo3: 2, bo5: 4 }[format];
  if (pool < Math.max(2, picks + 1) || pool > 9) return null;
  const bans = pool - picks - 1;
  const lead = Math.min(2, bans);
  const actions: VetoAction[] = [
    ...Array.from({ length: lead }, (): VetoAction => 'ban'),
    ...Array.from({ length: picks }, (): VetoAction => 'pick'),
    ...Array.from({ length: bans - lead }, (): VetoAction => 'ban'),
  ];
  const other: SeriesSlot = first === 'a' ? 'b' : 'a';
  return [
    ...actions.map((action, i) => ({ team: i % 2 === 0 ? first : other, action })),
    { team: '' as const, action: 'decider' as const },
  ];
}

/** Fills the known steps over the format's order, keeping the maps already entered. */
export function seriesVetoTemplate(format: SeriesFormat, pool: number, first: SeriesSlot, current: readonly VetoStep[]): VetoStep[] {
  return (vetoSequence(format, pool, first) ?? []).map((step, i) => ({ ...step, map: current[i]?.map ?? '' }));
}

/** The first problem of a veto the user typed, in the words of the form, or null. */
export function vetoStepsProblem(steps: readonly VetoStep[], hasTeams: boolean): string | null {
  const seen = new Set<string>();
  for (const [i, step] of steps.entries()) {
    const n = i + 1;
    if (step.map === '') return `Falta el mapa del paso ${n}.`;
    if (seen.has(step.map)) return `El paso ${n} repite un mapa.`;
    seen.add(step.map);
    if (step.action === 'decider') {
      if (i !== steps.length - 1) return 'El decider es el último paso.';
    } else if (step.team === '') {
      return `El paso ${n} necesita un equipo.`;
    } else if (!hasTeams) {
      return 'Elige primero los dos equipos de la serie.';
    }
  }
  return null;
}

/** A share from 0 to 1 as a whole percentage. */
export function rateLabel(rate: number): string {
  return `${Math.round(rate * 100)} %`;
}

/** An edge in percentage points with its sign, as Spanish text: `+6,3 pp`. */
export function edgeLabel(edge: number): string {
  const points = Math.round(edge * 1000) / 10;
  const text = Math.abs(points).toLocaleString('es-ES', { minimumFractionDigits: 1, maximumFractionDigits: 1 });
  if (points === 0) return '0,0 pp';
  return `${points > 0 ? '+' : '-'}${text} pp`;
}

/** A side or pistol count over its rounds: `8 de 14`. */
export function countOf(won: number, total: number): string {
  return total === 0 ? 'Sin rondas' : `${won} de ${total}`;
}
