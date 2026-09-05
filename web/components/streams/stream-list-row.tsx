'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import { ArrowRight, Film } from 'lucide-react';
import { streamsApi, type StreamJob } from '@/lib/api/streams';
import { streamClipCount, streamJobTag } from '@/lib/streams/list';
import { formatStreamClock, streamSourceLabel } from '@/lib/streams/plan';
import { DeleteMatchButton } from '@/components/matches/delete-match-button';
import { MediaFrame } from '@/components/studio/media-frame';
import { StatusTag } from '@/components/studio/status-tag';
import { Button, FOCUS_RING } from '@/components/ui/button';

/** Frame 0 of a stream clip is usually black; a media fragment seeks past it at metadata cost only. */
const POSTER_FRAGMENT = '#t=1';

const PROJECT_ACTION_LABELS = {
  acquiring: 'Ver progreso',
  uploaded: 'Continuar edición',
  ready: 'Continuar edición',
  rendering: 'Ver progreso',
  rendered: 'Ver Shorts',
  failed: 'Revisar proyecto',
} as const satisfies Record<StreamJob['status'], string>;

/** True once the element has scrolled into view; stays true so a painted frame never unmounts. */
function useSeen<T extends Element>(): { ref: React.RefObject<T | null>; seen: boolean } {
  const ref = useRef<T>(null);
  const [seen, setSeen] = useState(false);
  useEffect(() => {
    const el = ref.current;
    if (el === null || seen) return;
    if (typeof IntersectionObserver === 'undefined') {
      setSeen(true);
      return;
    }
    const observer = new IntersectionObserver((entries) => {
      if (entries.some((entry) => entry.isIntersecting)) setSeen(true);
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, [seen]);
  return { ref, seen };
}

/** Keep deletion separate from the project opener and re-fetch after confirmation. */
export function StreamListRow({ job, onOpen, onDeleted }: { job: StreamJob; onOpen: () => void; onDeleted: () => void }): ReactNode {
  const tag = streamJobTag(job);
  const acquiring = job.status === 'acquiring';
  const cuts = streamClipCount(job);
  const duration = job.probe?.duration_seconds;
  // The source only loads for rows on screen: a long list must not range-fetch every MP4 at once.
  const { ref, seen } = useSeen<HTMLLIElement>();
  const title = job.title?.trim() || 'Clip de stream';
  const meta = streamSourceLabel(job.source_url) ?? 'Archivo local';
  const actionLabel = PROJECT_ACTION_LABELS[job.status];

  return (
    <li ref={ref} className="studio-panel flex flex-col bg-none @[56rem]/content:flex-row @[56rem]/content:items-center">
      <button
        type="button"
        onClick={onOpen}
        aria-label={`Abrir ${title}`}
        className={`flex min-w-0 flex-1 items-center gap-4 rounded-lg p-4 text-left transition-colors duration-(--dur-fast) ease-standard hover:bg-surface-3 @[56rem]/content:gap-6 ${FOCUS_RING}`}
      >
        <MediaFrame
          aspect="16:9"
          className="w-28 shrink-0 rounded-md border border-border @[40rem]/content:w-40 @[64rem]/content:w-44"
          media={
            acquiring || !seen ? null : (
              <video
                src={`${streamsApi.sourceUrl(job.id)}${POSTER_FRAGMENT}`}
                preload="metadata"
                muted
                playsInline
                aria-hidden
              />
            )
          }
          fallback={
            <span className="grid size-full place-items-center">
              <Film aria-hidden className="size-6 text-fg-3" />
            </span>
          }
          footer={duration !== undefined && duration > 0 ? (
            <span className="rounded bg-surface-0 px-1.5 py-0.5 font-mono text-meta tabular-nums text-fg-1">
              {formatStreamClock(duration)}
            </span>
          ) : undefined}
        />
        <span className="flex min-w-0 flex-1 flex-col gap-2">
          <span className="truncate font-display text-title font-semibold text-fg-1" title={title}>
            {title}
          </span>
          <span className="truncate text-body-sm text-fg-2" title={meta}>{meta}</span>
        </span>
      </button>

      <div className="flex shrink-0 flex-wrap items-center gap-4 border-t border-border-subtle p-4 @[56rem]/content:border-t-0 @[56rem]/content:pl-0">
        {acquiring ? (
          <span className="flex flex-col gap-2 text-stream-text" role="status">
            <span className="flex items-center gap-2 text-body-sm">
              <span aria-hidden className="studio-spinner" />
              {tag.label}
            </span>
            <span className="studio-bar">
              <span className="studio-indeterminate" />
            </span>
          </span>
        ) : (
          <span className="flex items-center gap-4">
            <span className="text-body-sm tabular-nums text-fg-2">{cuts} {cuts === 1 ? 'corte' : 'cortes'}</span>
            <StatusTag tone={tag.tone} size="md" className="rounded-md font-sans normal-case tracking-normal">
              {tag.busy ? <span aria-hidden className="studio-spinner" /> : null}
              {tag.label}
            </StatusTag>
          </span>
        )}
        <div className="ml-auto flex flex-wrap items-center justify-end gap-3">
          <Button type="button" variant="outline" onClick={onOpen} aria-label={`${actionLabel}: ${title}`} className="bg-surface-2">
            {actionLabel}<ArrowRight aria-hidden />
          </Button>
          <DeleteMatchButton
            label={title}
            onConfirm={() => streamsApi.deleteJob(job.id)}
            onDeleted={onDeleted}
          />
        </div>
      </div>
    </li>
  );
}
