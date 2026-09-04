'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import { ChevronRight, Film } from 'lucide-react';
import { streamsApi, type StreamJob } from '@/lib/api/streams';
import { streamClipCount, streamJobTag } from '@/lib/streams/list';
import { formatStreamClock, streamSourceLabel } from '@/lib/streams/plan';
import { DeleteMatchButton } from '@/components/matches/delete-match-button';
import { MediaFrame } from '@/components/studio/media-frame';
import { StatusTag } from '@/components/studio/status-tag';

/** Frame 0 of a stream clip is usually black; a media fragment seeks past it at metadata cost only. */
const POSTER_FRAGMENT = '#t=1';

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

/**
 * One stream job. Everything shown is read from the job; nothing is inferred.
 * `onDeleted` lets the page re-fetch after the two-step delete; the
 * orchestrator refuses (409) while the job is still acquiring or rendering and
 * that message shows inline, same as a Partidas row.
 */
export function StreamListRow({ job, onOpen, onDeleted }: { job: StreamJob; onOpen: () => void; onDeleted: () => void }): ReactNode {
  const tag = streamJobTag(job);
  const acquiring = job.status === 'acquiring';
  const cuts = streamClipCount(job);
  const duration = job.probe?.duration_seconds;
  // The source only loads for rows on screen: a long list must not range-fetch every MP4 at once.
  const { ref, seen } = useSeen<HTMLLIElement>();
  const meta = [
    streamSourceLabel(job.source_url) ?? 'Archivo local',
    duration !== undefined && duration > 0 ? formatStreamClock(duration) : null,
  ]
    .filter((part): part is string => part !== null)
    .join(' · ');

  return (
    <li ref={ref} className="studio-enter studio-panel flex items-stretch overflow-hidden">
      <button
        type="button"
        onClick={onOpen}
        className="flex min-w-0 flex-1 flex-wrap items-center gap-4 px-4 py-2.5 text-left transition-colors duration-(--dur-fast) ease-standard hover:bg-surface-3 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring @[44rem]/content:flex-nowrap"
      >
        <MediaFrame
          aspect="16:9"
          className="w-[84px] shrink-0 border border-border-strong"
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
              <Film aria-hidden className="size-4 text-fg-3" />
            </span>
          }
        />
        <span className="flex min-w-40 flex-1 flex-col gap-1">
          <span className="truncate font-display text-body-lg font-bold uppercase text-fg-1">
            {job.title?.trim() || 'Clip de stream'}
          </span>
          <span className="truncate font-mono text-meta uppercase tracking-wider text-fg-3">{meta}</span>
        </span>

        {acquiring ? (
          <span className="row-state text-stream-text" role="status">
            <span className="flex items-center gap-2 font-mono text-meta uppercase tracking-wider">
              <span aria-hidden className="studio-spinner" />
              {tag.label}
            </span>
            <span className="studio-bar">
              <span className="studio-indeterminate" />
            </span>
          </span>
        ) : (
          <span className="row-state flex-row items-center justify-start gap-2">
            <StatusTag>Cortes · {cuts}</StatusTag>
            <StatusTag tone={tag.tone}>
              {tag.busy ? <span aria-hidden className="studio-spinner" /> : null}
              {tag.label}
            </StatusTag>
          </span>
        )}
        <ChevronRight aria-hidden className="size-4 shrink-0 text-fg-3" />
      </button>
      <span className="flex shrink-0 items-center py-2.5 pr-4">
        <DeleteMatchButton
          label={job.title?.trim() || 'Clip de stream'}
          onConfirm={() => streamsApi.deleteJob(job.id)}
          onDeleted={onDeleted}
        />
      </span>
    </li>
  );
}
