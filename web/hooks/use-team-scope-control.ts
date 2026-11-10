'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { rivalOf } from '@/lib/library/partidas';
import type { LibraryMatch } from '@/lib/api/library';
import { busiestMap, inScopePeriod, type AnalysisScope } from '@/lib/analysis/scope';
import { mapPatch, unknownTactic, type TacticOption } from '@/lib/analysis/scope-tactics';
import { useAnalysisScope, type AnalysisScopeControl } from '@/hooks/use-analysis-scope';

export type TeamScopeControl = AnalysisScopeControl & {
  /** `setScope` for the controls: a new map also drops a tactic read on another map. */
  changeScope: (patch: Partial<AnalysisScope>) => void;
};

/**
 * The scope of a team-scoped tool kept in the URL. A tactic deleted from the Libreta would empty every
 * view, so it is dropped with a notice; with `defaultMap` a missing map opens on the team's busiest one.
 */
export function useTeamScopeControl(options: {
  teamId: string;
  tactics: readonly TacticOption[] | null;
  matches: readonly LibraryMatch[];
  defaultMap: boolean;
}): TeamScopeControl {
  const { teamId, tactics, matches, defaultMap } = options;
  const control = useAnalysisScope({ kind: 'team', id: teamId });
  const { scope, setScope } = control;
  const [now] = useState(() => Date.now());
  const changeScope = useCallback(
    (patch: Partial<AnalysisScope>) =>
      setScope(patch.map === undefined ? patch : { ...patch, ...mapPatch({ map: patch.map, tactic: patch.tactic ?? scope.tactic, options: tactics ?? [] }) }),
    [scope.tactic, setScope, tactics],
  );

  useEffect(() => {
    if (tactics === null || !unknownTactic(scope.tactic, tactics)) return;
    setScope({ tactic: '' });
    toast.warning('La táctica del filtro ya no está en la Libreta y se ha quitado.');
  }, [scope.tactic, setScope, tactics]);

  const opened = useMemo(
    () =>
      busiestMap(
        matches
          .filter((match) => inScopePeriod(match.played_at, scope.period, now) && (scope.opponent === '' || rivalOf(match, teamId)?.team_id === scope.opponent))
          .map((match) => match.map),
      ),
    [matches, now, scope.opponent, scope.period, teamId],
  );
  useEffect(() => {
    if (defaultMap && scope.map === '' && opened !== '') changeScope({ map: opened });
  }, [changeScope, defaultMap, opened, scope.map]);

  return { ...control, changeScope };
}
