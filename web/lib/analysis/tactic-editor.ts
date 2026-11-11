import type { PlaybookCombo, PlaybookTactic, TacticKind, TeamPlaybook } from '../api/playbook.ts';

export type EditorPanelId = 'ronda' | 'compra' | 'utilidad' | 'jugadores';

/** A panel of the Editor de tácticas: the playbook's tactic kinds it lets the user add or remove as conditions. */
export type EditorPanel = { id: EditorPanelId; title: string; kinds: readonly TacticKind[] };

/** Round, buys, utility and player conditions, in the order the left column stacks them. */
export const EDITOR_PANELS: readonly EditorPanel[] = [
  { id: 'ronda', title: 'Ronda', kinds: ['category', 'site'] },
  { id: 'compra', title: 'Compra', kinds: ['buy', 'opponent_buy'] },
  { id: 'utilidad', title: 'Utilidad', kinds: ['utility'] },
  { id: 'jugadores', title: 'Jugadores', kinds: ['opening', 'opening_result', 'awp', 'zone'] },
];

export type KindRows = { kind: TacticKind; tactics: PlaybookTactic[] };

/**
 * The rows of a panel, by kind: the tactics the kept rounds show, then those of the combination that no
 * kept round shows, at zero rounds so the user can still take them off.
 */
export function panelRows(input: {
  playbook: Pick<TeamPlaybook, 'tactics'>;
  view: Pick<TeamPlaybook, 'tactics' | 'combo'>;
  kinds: readonly TacticKind[];
}): KindRows[] {
  const { playbook, view, kinds } = input;
  const all = new Map(playbook.tactics.map((tactic) => [tactic.key, tactic]));
  const shown = new Set(view.tactics.map((tactic) => tactic.key));
  const zero = [...view.combo.with, ...view.combo.without].flatMap((key) => {
    const base = all.get(key);
    if (base === undefined || shown.has(key)) return [];
    return [{ ...base, rounds: 0, refs: [], win_rate: { ...base.win_rate, count: 0, total: 0, pct: 0 } }];
  });
  const rows = [...view.tactics, ...zero];
  return kinds.flatMap((kind) => {
    const tactics = rows.filter((tactic) => tactic.kind === kind);
    return tactics.length === 0 ? [] : [{ kind, tactics }];
  });
}

/** How many conditions of the combination sit in the panel's kinds. */
export function panelConditions(input: { combo: PlaybookCombo; playbook: Pick<TeamPlaybook, 'tactics'>; kinds: readonly TacticKind[] }): number {
  const kindOf = new Map(input.playbook.tactics.map((tactic) => [tactic.key, tactic.kind]));
  return [...input.combo.with, ...input.combo.without].filter((key) => {
    const kind = kindOf.get(key);
    return kind !== undefined && input.kinds.includes(kind);
  }).length;
}

/** "42 de 120 rondas": the rounds the conditions keep, of those the scope reads. */
export function keptText(input: { kept: number; total: number }): string {
  return `${input.kept} de ${input.total} ${input.total === 1 ? 'ronda' : 'rondas'}`;
}

/** The zone keys of a combination, as the map paints them. */
export function chosenZones(combo: PlaybookCombo): string[] {
  return combo.with.flatMap((key) => (key.startsWith('zone:') ? [key.slice('zone:'.length)] : []));
}
