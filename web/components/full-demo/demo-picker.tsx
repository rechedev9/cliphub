'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { ChevronRight } from 'lucide-react';
import { api } from '@/lib/api';
import type { Match } from '@/lib/api/types';
import { Skeleton } from '@/components/ui/skeleton';
import { SingleDemoParse } from '@/components/upload/single-demo-parse';
import { matchDateLabel } from '@/lib/format';
import { FULL_DEMO_HREF } from '@/lib/full-demo';

export function FullDemoPicker(): ReactNode {
  const router = useRouter();
  const [matches, setMatches] = useState<Match[] | null>(null);

  const load = useCallback(async () => {
    try {
      setMatches(await api.listPlanReadyMatches());
    } catch {
      setMatches([]);
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
      <SingleDemoParse
        onParsed={(match) => {
          router.push(`${FULL_DEMO_HREF}/${match.id}`);
        }}
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
