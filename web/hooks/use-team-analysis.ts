'use client';

import { useCallback, useEffect, useState } from 'react';
import {
  getLibraryTeam,
  getTeamLineups,
  getTeamProfile,
  isLibraryNotFound,
  isLibraryOffline,
  listLibraryMatches,
  listLibraryPlays,
  listLibraryTeams,
  type LibraryMatch,
  type LibraryTeam,
} from '@/lib/api/library';
import type { TeamLineups, TeamProfile } from '@/lib/api/team-insights';
import { tacticOptions, type TacticOption } from '@/lib/analysis/scope-tactics';

/** What a team-scoped tool reads once: the team, its matches, its lineups and its profile (null when unreadable). */
export type TeamAnalysis = { team: LibraryTeam; matches: LibraryMatch[]; lineups: TeamLineups | null; profile: TeamProfile | null };
export type TeamAnalysisState = { kind: 'loading' } | { kind: 'ready'; data: TeamAnalysis } | { kind: 'missing' } | { kind: 'offline' };

/** The team record's own read, for the tools that open on a team outside its record. */
export function useTeamAnalysis(teamId: string): { state: TeamAnalysisState; reload: () => void } {
  const [state, setState] = useState<TeamAnalysisState>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let active = true;
    setState({ kind: 'loading' });
    Promise.all([
      getLibraryTeam(teamId),
      listLibraryMatches({ team_id: teamId }),
      getTeamLineups(teamId).catch(() => null),
      getTeamProfile(teamId).catch(() => null),
    ])
      .then(([team, matches, lineups, profile]) => {
        if (active) setState({ kind: 'ready', data: { team, matches, lineups, profile } });
      })
      .catch((error: unknown) => {
        if (!active) return;
        setState(isLibraryNotFound(error) && !isLibraryOffline(error) ? { kind: 'missing' } : { kind: 'offline' });
      });
    return () => {
      active = false;
    };
  }, [teamId, attempt]);
  return { state, reload: useCallback(() => setAttempt((n) => n + 1), []) };
}

/** Every team of the library by name, for the team picker; null while loading, empty when unreadable. */
export function useTeamList(): readonly LibraryTeam[] | null {
  const [teams, setTeams] = useState<LibraryTeam[] | null>(null);
  useEffect(() => {
    let active = true;
    listLibraryTeams()
      .then((list) => {
        if (active) setTeams([...list].sort((a, b) => a.name.localeCompare(b.name, 'es')));
      })
      .catch(() => {
        if (active) setTeams([]);
      });
    return () => {
      active = false;
    };
  }, []);
  return teams;
}

/** The team's Libreta plays for the tactic filter; null until the Libreta answers. */
export function useTeamTactics(teamId: string): readonly TacticOption[] | null {
  const [tactics, setTactics] = useState<readonly TacticOption[] | null>(null);
  useEffect(() => {
    let active = true;
    listLibraryPlays({ team_id: teamId, include_archived: true })
      .then((plays) => {
        if (active) setTactics(tacticOptions(plays));
      })
      .catch(() => {
        if (active) setTactics([]);
      });
    return () => {
      active = false;
    };
  }, [teamId]);
  return tactics;
}
