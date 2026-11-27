'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { getLibraryTeam, listLibraryMatches } from '@/lib/api/library';
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

const PUBLIC_TEAMS_RETRY_MS = 3000;
const PUBLIC_TEAMS_RETRIES = 5;
const CATALOG_DOWN: { status: ProCatalogStatus; teams: ProTeamCount[] } = { status: 'unavailable', teams: [] };

export function usePublicTeams(): readonly ProTeamCount[] | null {
  const [teams, setTeams] = useState<ProTeamCount[] | null>(null);
  useEffect(() => {
    let active = true;
    let timer: number | undefined;
    if (hasDemoCookie(document.cookie)) {
      setTeams([]);
      return;
    }
    const read = (left: number): void => {
      readPublicTeams()
        .catch(() => CATALOG_DOWN)
        .then((catalog) => {
          if (!active) return;
          setTeams(catalog.teams);
          const pending = catalog.status === 'unavailable' || catalog.status === 'missing';
          if (pending && left > 0) timer = window.setTimeout(() => read(left - 1), PUBLIC_TEAMS_RETRY_MS);
        });
    };
    read(PUBLIC_TEAMS_RETRIES);
    return () => {
      active = false;
      window.clearTimeout(timer);
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
    Promise.all([getLibraryTeam(teamId), listLibraryMatches({ team_id: teamId }), readPublicTeams()])
      .then(async ([team, matches, catalog]) => {
        if (!active || !matches.some((match) => match.source === 'pro')) return;
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
