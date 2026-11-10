'use client';

import { useCallback, useEffect, useState } from 'react';
import { getTeamPlaybook } from '@/lib/api/library';
import type { TeamPlaybook } from '@/lib/api/playbook';
import { scopeDateRange, type AnalysisScope } from '@/lib/analysis/scope';

export type TeamPlaybookRead = { playbook: TeamPlaybook | null; failed: boolean; retry: () => void };

/** The team's playbook on the scope's map and side; idle (null) until both are chosen. */
export function useTeamPlaybook(options: { teamId: string; scope: AnalysisScope; lineup: readonly string[] }): TeamPlaybookRead {
  const { teamId, scope, lineup } = options;
  const { map, side, opponent } = scope;
  const [now] = useState(() => Date.now());
  const { from, to } = scopeDateRange(scope.period, now);
  const [playbook, setPlaybook] = useState<TeamPlaybook | null>(null);
  const [failed, setFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    if (map === '' || side === '') return undefined;
    let active = true;
    setPlaybook(null);
    setFailed(false);
    getTeamPlaybook(teamId, { map, side, from, to, opponent_ids: opponent === '' ? [] : [opponent], lineup })
      .then((next) => {
        if (active) setPlaybook(next);
      })
      .catch(() => {
        if (active) setFailed(true);
      });
    return () => {
      active = false;
    };
  }, [teamId, map, side, opponent, from, to, lineup, attempt]);

  return { playbook, failed, retry: useCallback(() => setAttempt((n) => n + 1), []) };
}
