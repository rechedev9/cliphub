'use client';

import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { ArrowRight, Plus } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import { jobFailureTitle, parseFailureReason } from '@/lib/api/failure-reason';
import { MATCH_ROW_UNPICKED_CTA, MATCH_ROW_UNPICKED_HINT, MATCH_ROW_UNPICKED_TITLE } from '@/lib/clips/copy';
import {
  buildHubModel,
  fullChipLabel,
  HUB_ROW_STAGE,
  hubTransitions,
  matchMetaParts,
  OUTPUT_TONE,
  sameHubProps,
  shortsChipTone,
  type HubMatch,
  type HubModel,
} from '@/lib/clips/hub';
import { flowHref, PRODUCE_FORMAT, produceHref, RESULTS_HREF, type ProduceFormat } from '@/lib/clips/routes';
import { isDemoServiceUnavailable } from '@/lib/demo-parse-flow';
import { matchDateLabel, prettyMapName } from '@/lib/format';
import { startPollLoop } from '@/lib/poll-loop';
import { FLOW_PRODUCE_CTA } from '@/lib/produce/copy';
import { demoDeletePrompt, oneRowPerDemo, type DemoRowGroup } from '@/lib/produce/demo-rows';
import { cn } from '@/lib/utils';
import { MapCover } from '@/components/brand/map-cover';
import { HubBanner } from '@/components/clips-hub/hub-banner';
import { DeleteMatchButton } from '@/components/matches/delete-match-button';
import { parseScore } from '@/components/matches/match-score';
import { CoverImage } from '@/components/studio/cover-image';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';

const FAST_POLL_MS = 1500;
const IDLE_POLL_MS = 10000;

const LIST_TITLE = 'Demos cargadas';
const LIST_HINT = {
  short: 'Ya están analizadas en este PC: crea otro Short sin volver a cargarlas.',
  full: 'Ya están analizadas en este PC: prepara otro vídeo sin volver a cargarlas.',
} as const satisfies Record<ProduceFormat, string>;

/** The demos already on this PC, each with the next step of this flow. */
export function DemoList({ format }: { format: ProduceFormat }): ReactNode {
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
            description: `${prettyMapName(row.match.map)} · Jugador: ${row.match.player ?? '—'}`,
            action: {
              label: FLOW_PRODUCE_CTA[format].first,
              onClick: () => router.push(produceHref(row.match.id, format)),
            },
          });
        }
      }
      stateRef.current = next;
      setState(next);
      return next;
    } catch (err) {
      setOffline(isDemoServiceUnavailable(err));
      return null;
    }
  }, [format, router]);

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
  // The upload above is the empty state: an empty list adds nothing to it.
  if (state === null || state.rows.length === 0) return banner;

  const rows = oneRowPerDemo(state.rows);
  const anyMade = rows.some((row) => madeCount(row, format) > 0);

  return (
    <section aria-labelledby="demo-list-title" className="flex flex-col gap-3">
      <header className="flex flex-wrap items-end justify-between gap-x-4 gap-y-2">
        <div className="flex min-w-0 flex-col gap-1">
          <h2 id="demo-list-title" className="font-display text-title font-bold text-fg-1">
            {LIST_TITLE} <span className="text-body-sm font-normal text-fg-3">· {rows.length}</span>
          </h2>
          <p className="text-body-sm text-fg-2">{LIST_HINT[format]}</p>
        </div>
        {anyMade ? (
          <Button asChild size="sm" variant="ghost">
            <Link href={RESULTS_HREF}>
              Ver en Mis vídeos
              <ArrowRight aria-hidden />
            </Link>
          </Button>
        ) : null}
      </header>
      {banner}
      <div className="flex flex-col gap-3" aria-busy={offline !== null || undefined}>
        {rows.map((row) => (
          <DemoRow key={row.match.id} row={row} format={format} onChange={onChange} />
        ))}
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

function madeCount(row: HubMatch, format: ProduceFormat): number {
  return format === PRODUCE_FORMAT.short ? row.shorts.length : row.fulls.length;
}

type DemoRowProps = {
  row: DemoRowGroup;
  format: ProduceFormat;
  onChange: () => void;
};

function DemoRow({ row, format, onChange }: DemoRowProps): ReactNode {
  const { match, stage } = row;
  const { ours, theirs } = parseScore(match.score);
  const hasScore = ours !== null && theirs !== null;
  const win = hasScore && ours > theirs;
  const loss = hasScore && ours < theirs;
  const ready = stage === HUB_ROW_STAGE.ready;
  const made = madeCount(row, format);

  let bar = 'bg-border-strong';
  if (ready && win) bar = 'bg-success';
  else if (ready && loss) bar = 'bg-destructive';

  let stateBlock: ReactNode;
  if (stage === HUB_ROW_STAGE.parsing) stateBlock = <ParsingBlock player={match.player} />;
  else if (stage === HUB_ROW_STAGE.unpicked) stateBlock = <UnpickedBlock />;
  else if (stage === HUB_ROW_STAGE.failed) stateBlock = <FailedBlock reason={match.failureReason} />;
  else {
    stateBlock = (
      <span className="row-state flex-row flex-wrap items-center justify-start gap-3 @[44rem]/content:basis-52">
        {hasScore ? (
          <span role="img" aria-label={`Marcador ${ours} a ${theirs}`} className="text-title font-bold leading-none tabular-nums">
            <span className={cn(win && 'text-success', loss && 'text-destructive', !win && !loss && 'text-fg-1')}>{ours}</span>
            <span className="text-fg-3"> : </span>
            <span className="text-fg-1">{theirs}</span>
          </span>
        ) : null}
        <MadeChip row={row} format={format} />
      </span>
    );
  }

  return (
    <article className="studio-panel flex w-full flex-wrap items-stretch overflow-hidden @[44rem]/content:flex-nowrap">
      {/* Same gate as .row-state / .row-actions in globals.css: below it the actions wrap under the header. */}
      <div className="flex min-w-0 flex-1 flex-wrap items-stretch gap-4 py-3 pr-3 pl-[18px] @[44rem]/content:flex-nowrap">
        <span aria-hidden className={cn('w-1 shrink-0 self-stretch', bar)} />
        <span aria-hidden className="relative h-[47px] w-[84px] shrink-0 self-center overflow-hidden border border-border-strong">
          <MapCover map={match.map} />
          <span className="absolute inset-0">
            <CoverImage src={match.thumbnailUrl} />
          </span>
        </span>
        <span className="flex min-w-[200px] flex-1 flex-col justify-center gap-1">
          <span className="truncate font-display text-body-lg font-bold text-fg-1">{prettyMapName(match.map)}</span>
          {/* Each part moves as a whole so a date never splits; the separator stays glued to the part before it. */}
          <span className="text-body-sm text-fg-2">
            {matchMetaParts(match, matchDateLabel(match)).map((part, index, parts) => (
              <span key={index}>
                {index > 0 ? ' ' : null}
                <span className="inline-block max-w-full wrap-anywhere">
                  {part}
                  {index < parts.length - 1 ? ' ·' : null}
                </span>
              </span>
            ))}
          </span>
        </span>
        {stateBlock}
      </div>

      <span className="row-actions w-full px-[18px] pb-3 @[44rem]/content:w-auto @[44rem]/content:py-3 @[44rem]/content:pl-0">
        {stage === HUB_ROW_STAGE.unpicked ? (
          <Button asChild size="sm" variant="outline-primary">
            <Link href={flowHref(format, { job: match.id })}>{MATCH_ROW_UNPICKED_CTA}</Link>
          </Button>
        ) : null}
        {ready ? (
          <Button asChild size="sm" variant="outline-primary">
            <Link href={produceHref(match.id, format)}>
              <Plus aria-hidden />
              {made === 0 ? FLOW_PRODUCE_CTA[format].first : FLOW_PRODUCE_CTA[format].again}
            </Link>
          </Button>
        ) : null}
        <DeleteMatchButton label={prettyMapName(match.map)} confirmText={demoDeletePrompt(row)} onConfirm={() => deleteJobs(row.jobIds, onChange)} onDeleted={onChange} />
      </span>
    </article>
  );
}

/** How many videos of this format the demo already has; nothing when there are none. */
function MadeChip({ row, format }: { row: HubMatch; format: ProduceFormat }): ReactNode {
  if (format === PRODUCE_FORMAT.full) {
    const [latest] = row.fulls;
    return latest === undefined ? null : <StatusTag tone={OUTPUT_TONE[latest.state]}>{fullChipLabel(row.fulls)}</StatusTag>;
  }
  return row.shorts.length > 0 ? <StatusTag tone={shortsChipTone(row.shorts)}>Shorts · {row.shorts.length}</StatusTag> : null;
}

/** Terminal failure: the reason stays on the row so the user knows what to do before deleting it. */
function FailedBlock({ reason }: { reason?: string }): ReactNode {
  const failure = parseFailureReason(reason, { job: true });
  return (
    <span role="status" className="row-state row-state-block">
      <span className="text-meta text-destructive">{jobFailureTitle(reason)}</span>
      <span className="text-body-sm text-fg-2">{failure.message}</span>
    </span>
  );
}

function ParsingBlock({ player }: { player?: string }): ReactNode {
  return (
    <span role="status" className="row-state row-state-block">
      <span className="flex items-center gap-2 text-meta text-fg-1">
        <span aria-hidden className="studio-spinner" />
        {player === undefined ? 'Analizando la demo' : `Analizando las jugadas de ${player}`}
      </span>
      <span className="studio-bar text-primary">
        <span className="studio-indeterminate" />
      </span>
      <span className="text-meta text-fg-3">Podrás crear al terminar</span>
    </span>
  );
}

function UnpickedBlock(): ReactNode {
  return (
    <span className="row-state row-state-block">
      <span className="text-meta text-warning">{MATCH_ROW_UNPICKED_TITLE}</span>
      <span className="text-meta text-fg-3">{MATCH_ROW_UNPICKED_HINT}</span>
    </span>
  );
}
