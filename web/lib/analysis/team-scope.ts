import { SCOPE_PARAMS, scopeBuyTypes, type AnalysisScope, type ScopeField } from './scope.ts';
import { DEFAULT_REPLAY_FILTERS, orderedEquip, replayFilterEntries, type ReplayFilters } from './scope-replay-filters.ts';

export type TeamScopeEntry = readonly [string, string];

/** The query entries of the scope fields a team view reads; `limit` travels as `max_matches`. */
export function teamScopeEntries(scope: AnalysisScope, fields: readonly ScopeField[]): TeamScopeEntry[] {
  const reads = new Set(fields);
  const out: TeamScopeEntry[] = [];
  const add = (field: ScopeField, name: string, value: string | number | null): void => {
    if (reads.has(field) && value !== null && value !== '') out.push([name, String(value)]);
  };
  add('limit', 'max_matches', scope.limit);
  if (reads.has('buy')) for (const buy of scopeBuyTypes(scope.buy)) out.push([SCOPE_PARAMS.buy, buy]);
  if (reads.has('opponentBuy')) for (const buy of scopeBuyTypes(scope.opponentBuy)) out.push([SCOPE_PARAMS.opponentBuy, buy]);
  add('awp', SCOPE_PARAMS.awp, scope.awp);
  add('opponentAwp', SCOPE_PARAMS.opponentAwp, scope.opponentAwp);
  add('advantage', SCOPE_PARAMS.advantage, scope.advantage);
  add('rounds', SCOPE_PARAMS.roundFrom, scope.rounds.from);
  add('rounds', SCOPE_PARAMS.roundTo, scope.rounds.to);
  add('opponentTop', SCOPE_PARAMS.opponentTop, scope.opponentTop);
  add('whose', SCOPE_PARAMS.whose, scope.whose);
  out.push(...replayFilterEntries(readFilters(scope, reads)));
  return out;
}

/** The scope's Replay filters, those the view does not read back at their defaults. */
function readFilters(scope: AnalysisScope, reads: ReadonlySet<ScopeField>): ReplayFilters {
  const pick = <K extends keyof ReplayFilters>(field: K): ReplayFilters[K] => (reads.has(field) ? scope[field] : DEFAULT_REPLAY_FILTERS[field]);
  return {
    site: pick('site'),
    outcome: pick('outcome'),
    postPlant: pick('postPlant'),
    phase: pick('phase'),
    secondRound: pick('secondRound'),
    endReason: pick('endReason'),
    weapon: pick('weapon'),
    opponentWeapon: pick('opponentWeapon'),
    equip: orderedEquip(pick('equip')),
    opponentEquip: orderedEquip(pick('opponentEquip')),
    tactic: pick('tactic'),
    excludeTactics: pick('excludeTactics'),
    matchTags: pick('matchTags'),
    excludeMatchTags: pick('excludeMatchTags'),
    events: pick('events'),
  };
}
