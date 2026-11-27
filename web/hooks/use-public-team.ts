'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { getLibraryTeam } from '@/lib/api/library';
import { DEFAULT_PRO_QUERY, importProTeam, listProMatches, type ProCatalogStatus, type ProTeamCount } from '@/lib/api/pro-matches';
import { PUBLIC_TEAM_DAYS, publicTeamFor, syncPublicTeam, type PublicSyncProgress } from '@/lib/analysis/public-team';
import { hasDemoCookie } from '@/lib/clipmind/demo';

export type PublicSyncState =
  | { kind: 'idle' }
  | { kind: 'syncing'; progress: PublicSyncProgress }
  | { kind: 'done'; progress: PublicSyncProgress }
  | { kind: 'failed'; name: string };

async function readPublicTeams(): Promise<{ status: ProCatalogStatus; teams: ProTeamCount[] }> {
  const page = await listProMatches({ ...DEFAULT_PRO_QUERY, days: PUBLIC_TEAM_DAYS });
  return { status: page.status, teams: page.status === 'ok' ? page.facets.teams : [] };
}

export function usePublicTeams(): readonly ProTeamCount[] | null {
  const [teams, setTeams] = useState<ProTeamCount[] | null>(null);
  useEffect(() => {
    let active = true;
    if (hasDemoCookie(document.cookie)) {
      setTeams([]);
      return;
    }
    readPublicTeams()
      .then((catalog) => {
        if (active) setTeams(catalog.teams);
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

export function usePublicTeamSync(teamId: string, onImported: () => void): { state: PublicSyncState; retry: () => void } {
  const [state, setState] = useState<PublicSyncState>({ kind: 'idle' });
  const [attempt, setAttempt] = useState(0);
  const imported = useRef(onImported);
  useEffect(() => {
    imported.current = onImported;
  }, [onImported]);
  useEffect(() => {
    let active = true;
    let name = '';
    setState({ kind: 'idle' });
    if (hasDemoCookie(document.cookie)) return;
    Promise.all([getLibraryTeam(teamId), readPublicTeams()])
      .then(async ([team, catalog]) => {
        if (!active) return;
        if (catalog.status === 'unavailable') {
          setState({ kind: 'failed', name: team.name });
          return;
        }
        const hltv = publicTeamFor(team, catalog.teams);
        if (hltv === null) return;
        name = hltv.name;
        setState({ kind: 'syncing', progress: { name, held: 0, total: 0, imported: 0, teamId: team.id } });
        const progress = await syncPublicTeam({
          hltvId: hltv.id,
          call: importProTeam,
          onBatch: (next) => {
            if (active) setState({ kind: 'syncing', progress: { ...next, name } });
            return active;
          },
        });
        if (!active) return;
        setState({ kind: 'done', progress: { ...progress, name } });
        if (progress.imported > 0) imported.current();
      })
      .catch(() => {
        if (active && name !== '') setState({ kind: 'failed', name });
      });
    return () => {
      active = false;
    };
  }, [teamId, attempt]);
  return { state, retry: useCallback(() => setAttempt((n) => n + 1), []) };
}
