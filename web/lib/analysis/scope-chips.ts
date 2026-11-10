import { buyLabel } from '../tactical-labels.ts';
import { prettyMapName } from '../format.ts';
import { HALF_BUY, PERIOD_PRESETS, type AnalysisScope, type ScopeBuy, type ScopeField } from './scope.ts';
import { orderedEquip, type ScopeEndReason, type ScopeEquip, type ScopeSite, type ScopeWeapon } from './scope-replay-filters.ts';

/** One active filter as the bar's chip: the field its X resets and what it reads. */
export type ScopeChip = { readonly field: ScopeField; readonly label: string };

/** Names the chips cannot read from the scope itself: rival teams and Libreta plays by id. */
export type ScopeChipNames = { readonly opponents: ReadonlyMap<string, string>; readonly tactics: ReadonlyMap<string, string> };

export const WEAPON_LABEL: Record<Exclude<ScopeWeapon, ''>, string> = {
  scout: 'Con Scout',
  no_scout: 'Sin Scout',
  hero_ak: 'AK héroe',
  hero_awp: 'AWP héroe',
};

export const SITE_LABEL: Record<Exclude<ScopeSite, ''>, string> = {
  A: 'Sitio A',
  B: 'Sitio B',
  mid: 'Medio',
  none: 'Sin sitio',
};

export const END_REASON_LABEL: Record<Exclude<ScopeEndReason, ''>, string> = {
  elimination: 'Por eliminación',
  bomb_exploded: 'Bomba explotada',
  bomb_defused: 'Bomba desactivada',
  time_expired: 'Tiempo agotado',
};

function buyText(buy: ScopeBuy): string {
  if (buy === '') return '';
  return buy === HALF_BUY ? 'Media' : buyLabel(buy);
}

/** "$1000 a $3250", "Desde $3250", "Hasta $1000" per player. */
export function equipText(input: ScopeEquip): string {
  const range = orderedEquip(input);
  if (range.from !== null && range.to !== null) return `$${range.from} a $${range.to}`;
  if (range.from !== null) return `Desde $${range.from}`;
  return range.to === null ? '' : `Hasta $${range.to}`;
}

function periodText(scope: AnalysisScope): string {
  const { period } = scope;
  if (period.kind === 'preset') return PERIOD_PRESETS.find((entry) => entry.value === period.preset)?.label ?? '';
  if (period.kind === 'all') return '';
  if (period.from !== '' && period.to !== '') return `Del ${period.from} al ${period.to}`;
  return period.from !== '' ? `Desde el ${period.from}` : `Hasta el ${period.to}`;
}

function roundsText(scope: AnalysisScope): string {
  const { from, to } = scope.rounds;
  if (from !== null && to !== null) return `Rondas ${from} a ${to}`;
  if (from !== null) return `Desde la ronda ${from}`;
  return to === null ? '' : `Hasta la ronda ${to}`;
}

function advantageText(advantage: string): string {
  const [own, rival] = advantage.split('v');
  return advantage === '' ? '' : `Vivos ${own} contra ${rival}`;
}

/** "Etiqueta: scrim", or "Etiqueta: scrim y 2 más" once the list grows. */
function listText(prefix: string, values: readonly string[]): string {
  const [first] = values;
  if (first === undefined) return '';
  return values.length === 1 ? `${prefix}: ${first}` : `${prefix}: ${first} y ${values.length - 1} más`;
}

type ChipQuery = { readonly scope: AnalysisScope; readonly field: ScopeField; readonly names: ScopeChipNames };

function chipLabel({ scope, field, names }: ChipQuery): string {
  switch (field) {
    case 'map':
      return scope.map === '' ? '' : prettyMapName(scope.map);
    case 'side':
      return scope.side === '' ? '' : `Lado ${scope.side}`;
    case 'period':
      return periodText(scope);
    case 'limit':
      return scope.limit === null ? '' : `Últimas ${scope.limit} partidas`;
    case 'buy':
      return scope.buy === '' ? '' : `Compra propia: ${buyText(scope.buy)}`;
    case 'opponentBuy':
      return scope.opponentBuy === '' ? '' : `Compra rival: ${buyText(scope.opponentBuy)}`;
    case 'opponent':
      return scope.opponent === '' ? '' : `Rival: ${names.opponents.get(scope.opponent) ?? 'otro equipo'}`;
    case 'awp':
      return scope.awp === '' ? '' : `${scope.awp === 'with' ? 'Con' : 'Sin'} AWP propia`;
    case 'opponentAwp':
      return scope.opponentAwp === '' ? '' : `${scope.opponentAwp === 'with' ? 'Con' : 'Sin'} AWP rival`;
    case 'advantage':
      return advantageText(scope.advantage);
    case 'rounds':
      return roundsText(scope);
    case 'opponentTop':
      return scope.opponentTop === null ? '' : `Rival top ${scope.opponentTop}`;
    case 'whose':
      if (scope.whose === '') return '';
      return scope.whose === 'own' ? 'Partidas de mi equipo' : 'Partidas de otros equipos';
    case 'site':
      return scope.site === '' ? '' : SITE_LABEL[scope.site];
    case 'outcome':
      if (scope.outcome === '') return '';
      return scope.outcome === 'win' ? 'Ganadas' : 'Perdidas';
    case 'postPlant':
      return scope.postPlant ? 'Post-plant' : '';
    case 'phase':
      if (scope.phase === '') return '';
      return scope.phase === 'overtime' ? 'Solo prórroga' : 'Sin prórroga';
    case 'secondRound':
      return scope.secondRound ? 'Segunda ronda' : '';
    case 'endReason':
      return scope.endReason === '' ? '' : END_REASON_LABEL[scope.endReason];
    case 'weapon':
      return scope.weapon === '' ? '' : `${WEAPON_LABEL[scope.weapon]} propia`;
    case 'opponentWeapon':
      return scope.opponentWeapon === '' ? '' : `${WEAPON_LABEL[scope.opponentWeapon]} rival`;
    case 'equip': {
      const text = equipText(scope.equip);
      return text === '' ? '' : `Equipo propio: ${text}`;
    }
    case 'opponentEquip': {
      const text = equipText(scope.opponentEquip);
      return text === '' ? '' : `Equipo rival: ${text}`;
    }
    case 'tactic':
      return scope.tactic === '' ? '' : `Táctica: ${names.tactics.get(scope.tactic) ?? 'jugada de la Libreta'}`;
    case 'excludeTactics':
      return listText('Sin táctica', scope.excludeTactics.map((id) => names.tactics.get(id) ?? 'jugada de la Libreta'));
    case 'matchTags':
      return listText('Etiqueta', scope.matchTags);
    case 'excludeMatchTags':
      return listText('Sin etiqueta', scope.excludeMatchTags);
    case 'events':
      return listText('Torneo', scope.events);
    default:
      return '';
  }
}

/** The chips of the fields that hold a filter, in the bar's order. */
export function scopeChips({ scope, fields, names }: { scope: AnalysisScope; fields: readonly ScopeField[]; names: ScopeChipNames }): ScopeChip[] {
  return fields.flatMap((field) => {
    const label = chipLabel({ scope, field, names });
    return label === '' ? [] : [{ field, label }];
  });
}
