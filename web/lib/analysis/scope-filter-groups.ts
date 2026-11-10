import { SCOPE_FIELDS, type ScopeField } from './scope.ts';

/**
 * Where each scope field lives in the filter header: the controls in the row, or one group of the
 * "Filtros" popover, grouped like the reference's filter menu.
 */

/** Fields with a control of their own in the header row. */
export const ROW_FIELDS: ReadonlySet<ScopeField> = new Set<ScopeField>([
  'lineup', 'map', 'side', 'period', 'limit', 'buy', 'opponentBuy', 'equip', 'opponentEquip', 'reference',
]);

export type FilterGroup = {
  readonly id: string;
  readonly label: string;
  readonly fields: readonly ScopeField[];
  /** The group holds the shortcut that sets both buys at once, so it needs both of them. */
  readonly matchup?: true;
};

export const FILTER_GROUPS: readonly FilterGroup[] = [
  { id: 'weapon', label: 'Arma', fields: ['weapon', 'opponentWeapon', 'awp', 'opponentAwp'] },
  { id: 'tactic', label: 'Táctica', fields: ['tactic', 'excludeTactics'] },
  { id: 'advantage', label: 'Ventaja', fields: ['advantage'] },
  { id: 'site', label: 'Sitio atacado', fields: ['site'] },
  { id: 'round', label: 'Ronda', fields: ['rounds', 'secondRound'] },
  { id: 'overtime', label: 'Prórroga', fields: ['phase'] },
  { id: 'postplant', label: 'Postplant', fields: ['postPlant'] },
  { id: 'reason', label: 'Motivo de victoria', fields: ['endReason'] },
  { id: 'outcome', label: 'Resultado', fields: ['outcome'] },
  { id: 'tags', label: 'Etiquetas y torneo', fields: ['matchTags', 'excludeMatchTags', 'events'] },
  { id: 'rival', label: 'Rival y partidas', fields: ['opponent', 'opponentTop', 'whose'] },
  { id: 'matchup', label: 'Duelo de compras', fields: [], matchup: true },
];

/** The groups a view reads, in menu order. */
export function visibleGroups(fields: readonly ScopeField[]): FilterGroup[] {
  return FILTER_GROUPS.filter((group) =>
    group.matchup === true ? fields.includes('buy') && fields.includes('opponentBuy') : group.fields.some((field) => fields.includes(field)),
  );
}

/** The fields of a group that the view reads. */
export function groupFields(group: FilterGroup, fields: readonly ScopeField[]): ScopeField[] {
  return group.fields.filter((field) => fields.includes(field));
}

/** The fields no control of the header row or the menu reaches; none means every scope field is settable. */
export function unreachableFields(): ScopeField[] {
  const grouped = new Set(FILTER_GROUPS.flatMap((group) => group.fields));
  return SCOPE_FIELDS.filter((field) => !ROW_FIELDS.has(field) && !grouped.has(field));
}
