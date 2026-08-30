'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { ChevronRight, Radar, UploadCloud } from 'lucide-react';
import { api } from '@/lib/api';
import type { Match } from '@/lib/api/types';
import {
  TACTICAL_STATES,
  fetchTacticalStatus,
  isServiceUnavailableError,
} from '@/lib/api/tactical';
import type { TacticalState, TacticalStatus } from '@/lib/api/tactical';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { TacticalStateBadge } from '@/components/tactical/tactical-state-badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { matchDateLabel } from '@/lib/format';

/** One listed demo and the lifecycle of its tactical analysis. */
type DemoEntry = { match: Match; state: TacticalState; progress?: TacticalStatus['progress'] };

/**
 * The demos this PC has parsed, each with the state of its tactical analysis.
 * The status endpoint is cheap and every demo's is independent, so they are read
 * in one parallel batch rather than one after another.
 */
async function loadEntries(): Promise<DemoEntry[]> {
  const matches = await api.listMatches();
  const states = await Promise.all(
    matches.map((match) =>
      fetchTacticalStatus(match.id)
        .then((status): Pick<TacticalStatus, 'state' | 'progress'> => ({
          state: status.state,
          progress: status.progress,
        }))
        // A per-demo status failure must not blank the whole list: an unknown
        // analysis reads as "not analysed", which is what the workspace offers
        // to fix.
        .catch((): Pick<TacticalStatus, 'state' | 'progress'> => ({ state: TACTICAL_STATES.none })),
    ),
  );
  return matches.map((match, index) => ({
    match,
    state: states[index]?.state ?? TACTICAL_STATES.none,
    progress: states[index]?.progress,
  }));
}

export function TacticalDemoPicker(): ReactNode {
  const [entries, setEntries] = useState<DemoEntry[] | null>(null);
  const [offline, setOffline] = useState(false);

  const load = useCallback(async () => {
    try {
      setEntries(await loadEntries());
      setOffline(false);
    } catch (error) {
      setEntries([]);
      setOffline(isServiceUnavailableError(error));
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (!entries?.some((entry) => entry.state === TACTICAL_STATES.running || entry.state === TACTICAL_STATES.queued)) {
      return;
    }
    const timer = window.setInterval(() => {
      void load();
    }, 2500);
    return () => window.clearInterval(timer);
  }, [entries, load]);

  if (entries === null) return <DemoPickerSkeleton />;

  if (entries.length === 0) {
    return (
      <StudioEmptyState
        icon={Radar}
        title="No hay demos que analizar"
        description={
          offline
            ? 'No se pudo contactar con el servicio de análisis local. Arráncalo y recarga para ver tus demos.'
            : 'El análisis táctico trabaja sobre una demo ya parseada. Sube una para empezar.'
        }
        compact
        actions={
          <Button asChild className="font-display tracking-wide">
            <Link href="/upload">
              <UploadCloud aria-hidden />
              SUBIR UNA DEMO
            </Link>
          </Button>
        }
      />
    );
  }

  return (
    <section className="flex flex-col gap-3" aria-label="Demos analizables">
      {entries.map(({ match, state, progress }) => (
        <Link
          key={match.id}
          href={`/tactical/${match.id}`}
          className="studio-panel studio-panel-interactive flex min-h-[72px] items-center justify-between gap-4 px-4 py-4 transition-colors sm:px-5"
        >
          <div className="flex min-w-0 items-center gap-4">
            <span className="grid size-10 shrink-0 place-items-center rounded-lg border border-primary/25 bg-primary/10 text-primary">
              <Radar className="size-5" aria-hidden />
            </span>
            <div className="flex min-w-0 flex-col gap-1">
              <span className="truncate font-display text-lg font-bold uppercase leading-tight tracking-tight text-foreground">
                {match.map}
              </span>
              <span className="truncate font-mono text-meta uppercase tracking-wider text-fg-3">
                {[match.player, matchDateLabel(match)].filter(Boolean).join(' · ')}
              </span>
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-3">
            <TacticalStateBadge state={state} progress={progress} className="hidden sm:inline-flex" />
            <ChevronRight className="size-4 text-muted-foreground" aria-hidden />
          </div>
        </Link>
      ))}
    </section>
  );
}

/** Fixed-height placeholders so the list does not shift when the states land. */
function DemoPickerSkeleton(): ReactNode {
  return (
    <div className="flex flex-col gap-3" aria-hidden>
      {[0, 1, 2].map((row) => (
        <Skeleton key={row} className="h-[72px] w-full rounded-lg" />
      ))}
    </div>
  );
}
