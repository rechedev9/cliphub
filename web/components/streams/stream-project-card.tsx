'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { ArrowRight, Clapperboard, Film } from 'lucide-react';
import { streamsApi, type StreamJob } from '@/lib/api/streams';
import { streamSummaryHref } from '@/lib/clips/routes';
import { streamClipCount, streamCutMarks, streamJobShortCount, streamJobTag, streamProjectActions, type StreamJobTagTone } from '@/lib/streams/list';
import { streamMomentsLabel } from '@/lib/streams/groups';
import { formatStreamClock, streamSourceLabel } from '@/lib/streams/plan';
import { streamRenderProgressView } from '@/lib/streams/render-progress';
import { streamTitle } from '@/lib/streams/title';
import { useSeen } from '@/components/streams/use-seen';
import { DeleteMatchButton } from '@/components/matches/delete-match-button';
import { FORMAT_TONE } from '@/components/studio/format-tone';
import { MediaFrame } from '@/components/studio/media-frame';
import { Button, FOCUS_RING, buttonVariants } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { cn } from '@/lib/utils';

/** Frame 0 of a stream clip is usually black; a media fragment seeks past it at metadata cost only. */
const POSTER_FRAGMENT = '#t=1';

const STATUS_PILL_CLASS = {
  neutral: 'bg-surface-5 text-fg-1',
  stream: `${FORMAT_TONE.stream.bg} text-format-stream-foreground`,
  success: 'bg-success text-success-foreground',
  danger: 'bg-destructive-solid text-destructive-foreground',
} as const satisfies Record<StreamJobTagTone, string>;

/** A project poster card. Deletion stays apart from the opener and re-fetches after confirmation. */
export function StreamProjectCard({ job, onOpen, onDeleted }: { job: StreamJob; onOpen: () => void; onDeleted: () => void }): ReactNode {
  const tag = streamJobTag(job);
  const acquiring = job.status === 'acquiring';
  const marks = streamCutMarks(job);
  const duration = job.probe?.duration_seconds;
  // The source only loads for cards on screen: a long list must not range-fetch every MP4 at once.
  const { ref, seen } = useSeen<HTMLLIElement>();
  const title = streamTitle(job);
  const kind = job.source_kind === 'vod' ? 'VOD completo' : 'Clip';
  const meta = `${kind} · ${streamSourceLabel(job.source_url) ?? 'Archivo local'} · ${streamMomentsLabel(streamClipCount(job), streamJobShortCount(job))}`;
  const actions = streamProjectActions(job);
  const actionLabel = actions.shortsLabel;
  const renderPercent = job.status === 'rendering' ? streamRenderProgressView(job.render_progress)?.percent : undefined;

  return (
    <li
      ref={ref}
      className="flex min-w-0 flex-col overflow-hidden rounded-xl border border-border-subtle bg-surface-2 transition-colors duration-(--dur-fast) ease-standard hover:border-border-strong"
    >
      <button type="button" onClick={onOpen} aria-label={`Abrir ${title}`} className={cn('flex flex-col text-left', FOCUS_RING)}>
        <span className="relative block">
          <MediaFrame
            aspect="16:9"
            media={
              acquiring || !seen ? null : (
                <video src={`${streamsApi.sourceUrl(job.id)}${POSTER_FRAGMENT}`} preload="metadata" muted playsInline aria-hidden />
              )
            }
            fallback={
              <span className="grid size-full place-items-center bg-linear-170 from-format-stream/25 via-surface-3 to-surface-0">
                <Film aria-hidden className="size-7 text-fg-3" />
              </span>
            }
          />
          <span
            role={tag.busy ? 'status' : undefined}
            className={cn(
              'absolute top-2.5 left-2.5 inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-meta font-bold',
              STATUS_PILL_CLASS[tag.tone],
            )}
          >
            {tag.busy ? <span aria-hidden className="studio-spinner" /> : null}
            {tag.label}
          </span>
          {duration !== undefined && duration > 0 ? (
            <span className="absolute top-2.5 right-2.5 rounded-md bg-surface-0/80 px-1.5 py-0.5 text-meta font-bold tabular-nums text-fg-1">
              {formatStreamClock(duration)}
            </span>
          ) : null}
          {marks !== null ? (
            <span aria-hidden className="absolute inset-x-2.5 bottom-2.5 h-1.5 rounded-full bg-surface-0/70">
              {marks.map((mark) => (
                <span
                  key={mark.id}
                  className={cn('absolute inset-y-0 min-w-1 rounded-sm', FORMAT_TONE.stream.bg)}
                  style={{ left: `${mark.leftPercent}%`, width: `${mark.widthPercent}%` }}
                />
              ))}
            </span>
          ) : null}
        </span>
        <span className="flex min-w-0 flex-col gap-1.5 px-4 pt-3.5">
          <span className="truncate font-display text-body font-bold text-fg-1" title={title}>
            {title}
          </span>
          <span className="truncate text-meta text-fg-3" title={meta}>{meta}</span>
        </span>
      </button>

      <div className="mt-auto flex flex-col gap-3 px-4 pt-3 pb-4">
        {acquiring || job.status === 'rendering' ? (
          <Progress
            aria-label="Progreso"
            tone="stream"
            size="xs"
            value={renderPercent ?? null}
            indeterminate={renderPercent === undefined}
          />
        ) : null}
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="flex flex-wrap items-center gap-2">
            <Button type="button" variant="outline" size="sm" onClick={onOpen} aria-label={`${actionLabel}: ${title}`} className="bg-surface-2">
              {actionLabel}<ArrowRight aria-hidden />
            </Button>
            {actions.summary ? (
              <Link
                href={streamSummaryHref(job.id)}
                aria-label={`Hacer resumen: ${title}`}
                className={cn(buttonVariants({ variant: 'outline', size: 'sm' }), 'bg-surface-2')}
              >
                <Clapperboard aria-hidden />
                Hacer resumen
              </Link>
            ) : null}
          </div>
          <DeleteMatchButton label={title} onConfirm={() => streamsApi.deleteJob(job.id)} onDeleted={onDeleted} messageBelow />
        </div>
      </div>
    </li>
  );
}
