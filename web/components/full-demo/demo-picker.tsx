'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { ChevronRight, MonitorPlay, Rocket } from 'lucide-react';
import { api } from '@/lib/api';
import type { Match } from '@/lib/api/types';
import { SERVICE_UNAVAILABLE_CODE } from '@/lib/api/types';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { matchDateLabel } from '@/lib/format';
import { FULL_DEMO_HREF } from '@/lib/full-demo';

export function FullDemoPicker(): ReactNode {
  const [matches, setMatches] = useState<Match[] | null>(null);
  const [offline, setOffline] = useState(false);

  const load = useCallback(async () => {
    try {
      setMatches(await api.listPlanReadyMatches());
      setOffline(false);
    } catch (error) {
      setMatches([]);
      setOffline((error as { code?: string } | null)?.code === SERVICE_UNAVAILABLE_CODE);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  if (matches === null) {
    return (
      <div className="flex flex-col gap-3" aria-hidden>
        {[0, 1, 2].map((row) => (
          <Skeleton key={row} className="h-[72px] w-full" />
        ))}
      </div>
    );
  }

  if (matches.length === 0) {
    return (
      <StudioEmptyState
        icon={MonitorPlay}
        title="No hay demos para forjar"
        description={
          offline
            ? 'El servicio local no responde. Arráncalo y recarga.'
            : 'Hace falta una demo parseada. Empieza por Inicio: pega un código o sube el archivo.'
        }
        compact
        actions={
          <Button asChild>
            <Link href="/onboarding">
              <Rocket aria-hidden />
              IR A INICIO
            </Link>
          </Button>
        }
      />
    );
  }

  return (
    <section className="flex flex-col gap-3" aria-label="Demos para full demo to video">
      {matches.map((match) => (
        <Link
          key={match.id}
          href={`${FULL_DEMO_HREF}/${match.id}`}
          className="studio-panel studio-panel-interactive flex min-h-[72px] items-center justify-between gap-4 px-4 py-4 sm:px-5"
        >
          <div className="flex min-w-0 flex-col gap-1">
            <span className="truncate font-display text-lg font-bold uppercase tracking-tight text-fg-1">
              {match.map}
            </span>
            <span className="truncate font-mono text-meta uppercase tracking-wider text-fg-3">
              {[match.player, matchDateLabel(match)].filter(Boolean).join(' · ')}
            </span>
          </div>
          <ChevronRight className="size-4 shrink-0 text-fg-3" aria-hidden />
        </Link>
      ))}
    </section>
  );
}
