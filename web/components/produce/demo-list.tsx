'use client';

import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { ArrowRight } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import { buildHubModel, HUB_ROW_STAGE, hubTransitions, sameHubProps, type HubModel } from '@/lib/clips/hub';
import { produceHref, RESULTS_HREF, type ProduceFormat } from '@/lib/clips/routes';
import { isDemoServiceUnavailable } from '@/lib/demo-parse-flow';
import { prettyMapName } from '@/lib/format';
import { startPollLoop } from '@/lib/poll-loop';
import { FLOW_PRODUCE_CTA } from '@/lib/produce/copy';
import { oneRowPerDemo } from '@/lib/produce/demo-rows';
import { HubBanner } from '@/components/clips-hub/hub-banner';
import { DemoCard, madeCount } from '@/components/produce/demo-card';
import { Button } from '@/components/ui/button';

const LIST_HINT = {
  short: 'Ya están analizadas en este PC: crea otro Short sin volver a cargarlas.',
  full: 'Ya están analizadas en este PC: prepara otro vídeo sin volver a cargarlas.',
} as const satisfies Record<ProduceFormat, string>;

const FAST_POLL_MS = 1500;
const IDLE_POLL_MS = 10000;

type DemoListProps = {
  format: ProduceFormat;
  /** How many demo rows the list shows, once its first read settles; a failed first read counts as 0. */
  onCount?: (count: number) => void;
  /** The last cell of the grid: the drop zone for one more demo. */
  addTile?: ReactNode;
};

/** The demos already on this PC as a card grid, each with the next step of this flow. */
export function DemoList({ format, onCount, addTile }: DemoListProps): ReactNode {
  const router = useRouter();
  const [state, setState] = useState<HubModel | null>(null);
  const [offline, setOffline] = useState<boolean | null>(null);
  const stateRef = useRef<HubModel | null>(null);

  const refresh = useCallback(async (): Promise<HubModel | null> => {
    try {
      const [matches, videos] = await Promise.all([api.listMatches(), api.listVideos()]);
      const next = buildHubModel(matches, videos);
      const prev = stateRef.current;
      setOffline(null);
      if (prev !== null && sameHubProps(prev, next)) return prev;
      if (prev !== null) {
        for (const row of hubTransitions(prev, next).parsed) {
          toast('Demo analizada', {
            description: row.match.player ? `${prettyMapName(row.match.map)} · Jugador: ${row.match.player}` : prettyMapName(row.match.map),
            action: {
              label: FLOW_PRODUCE_CTA[format].first,
              onClick: () => router.push(produceHref(row.match.id, format)),
            },
          });
        }
      }
      stateRef.current = next;
      setState(next);
      onCount?.(oneRowPerDemo(next.rows).length);
      return next;
    } catch (err) {
      setOffline(isDemoServiceUnavailable(err));
      if (stateRef.current === null) onCount?.(0);
      return null;
    }
  }, [format, onCount, router]);

  useEffect(() => {
    let active = true;
    const stop = startPollLoop({
      tick: async () => {
        const next = await refresh();
        if (!active || next === null) return 'idle';
        return next.rows.some((row) => row.stage === HUB_ROW_STAGE.parsing) ? 'fast' : 'idle';
      },
      fastMs: FAST_POLL_MS,
      idleMs: IDLE_POLL_MS,
    });
    return () => {
      active = false;
      stop();
    };
  }, [refresh]);

  const onChange = useCallback(() => {
    void refresh();
  }, [refresh]);

  const banner = offline === null ? null : <HubBanner offline={offline} onRetry={onChange} />;
  // The upload below is the empty state: an empty list adds nothing to it.
  if (state === null || state.rows.length === 0) return banner;

  const rows = oneRowPerDemo(state.rows);
  const anyMade = rows.some((row) => madeCount(row, format) > 0);

  return (
    <section aria-labelledby="demo-list-title" className="flex flex-col gap-3.5">
      <header className="flex flex-wrap items-end justify-between gap-x-4 gap-y-2">
        <div className="flex min-w-0 flex-col gap-1">
          <h2 id="demo-list-title" className="font-display text-section font-bold text-fg-1">
            Elige una demo <span className="text-body font-medium text-fg-3">{rows.length} en este PC</span>
          </h2>
          <p className="text-body-sm text-fg-2">{LIST_HINT[format]}</p>
        </div>
        {anyMade ? (
          <Button asChild size="sm" variant="ghost">
            <Link href={RESULTS_HREF}>
              Ver lo que ya has creado
              <ArrowRight aria-hidden />
            </Link>
          </Button>
        ) : null}
      </header>
      {banner}
      <div className="grid grid-cols-[repeat(auto-fill,minmax(17.5rem,1fr))] gap-4" aria-busy={offline !== null || undefined}>
        {rows.map((row) => (
          <DemoCard key={row.match.id} row={row} format={format} onChange={onChange} onDelete={() => deleteJobs(row.jobIds, onChange)} />
        ))}
        {addTile}
      </div>
    </section>
  );
}

/** The row stands for every job of its demo, so deleting it deletes all of them. A busy job
 * fails alone; the list still refreshes so the jobs already gone leave the row. */
async function deleteJobs(jobIds: readonly string[], onChange: () => void): Promise<void> {
  const results = await Promise.allSettled(jobIds.map((jobId) => api.deleteMatch(jobId)));
  const failed = results.find((result) => result.status === 'rejected');
  if (failed === undefined) return;
  if (results.some((result) => result.status === 'fulfilled')) onChange();
  throw failed.reason;
}
