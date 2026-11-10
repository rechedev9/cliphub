'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { getTeamRounds } from '@/lib/api/library';
import type { TeamRounds } from '@/lib/api/team-insights';
import type { AnalysisScope } from '@/lib/analysis/scope';
import { teamRoundRefs, teamRoundsQuery, type TeamRoundRefs } from '@/lib/replay/team-rounds';

export type TeamRoundsRead = {
  /** The last answer: it stays on screen while the next one loads, so the replay is not rebuilt per filter. */
  refs: TeamRoundRefs | null;
  skipped: TeamRounds['skipped'];
  awpEstimated: number | undefined;
  loading: boolean;
  failed: boolean;
};

/** The rounds the team scope keeps, read from the same endpoint as the team record's Replay tab. Idle without a map. */
export function useTeamRounds(options: { teamId: string; scope: AnalysisScope; lineup: readonly string[] }): TeamRoundsRead {
  const { teamId, scope, lineup } = options;
  const [now] = useState(() => Date.now());
  const query = teamRoundsQuery({ scope, lineup, now });
  const key = JSON.stringify(query);
  const latest = useRef(query);
  useEffect(() => {
    latest.current = query;
  });
  const [answer, setAnswer] = useState<TeamRounds | null>(null);
  const [loading, setLoading] = useState(false);
  const [failed, setFailed] = useState(false);
  const idle = scope.map === '';

  useEffect(() => {
    if (idle) return;
    let active = true;
    setLoading(true);
    setFailed(false);
    getTeamRounds(teamId, latest.current)
      .then((data) => {
        if (!active) return;
        setAnswer(data);
        setLoading(false);
      })
      .catch(() => {
        if (!active) return;
        setFailed(true);
        setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [teamId, key, idle]);

  const refs = useMemo(() => (answer === null ? null : teamRoundRefs(answer)), [answer]);

  return {
    refs,
    skipped: answer?.skipped ?? [],
    awpEstimated: answer?.awp_estimated,
    loading,
    failed,
  };
}
