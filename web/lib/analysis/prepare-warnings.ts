import type { DataWarning } from '../api/first-death.ts';

/** One server warning as the line on top of Preparar says it. */
export function dataWarningText(warning: DataWarning, team: string): string {
  switch (warning.kind) {
    case 'stale':
      return `La última partida de ${team} fue hace ${warning.days} días: puede que hayan cambiado de playbook`;
    case 'lineup':
      return `Alineación distinta en ${warning.changed} de ${warning.matches} partidas`;
    case 'small_sample':
      return `Muestra pequeña: ${warning.rounds} rondas`;
  }
}

/** What a coach must know before trusting a preparation, one line per server warning. */
export function prepareWarnings(warnings: readonly DataWarning[] | null, team: string): string[] {
  return (warnings ?? []).map((warning) => dataWarningText(warning, team));
}
