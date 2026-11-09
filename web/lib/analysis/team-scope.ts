import { SCOPE_PARAMS, scopeBuyTypes, type AnalysisScope, type ScopeField } from './scope.ts';

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
  return out;
}
