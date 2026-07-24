'use client';

import type { ReactNode } from 'react';
import { ExternalLink, Film } from 'lucide-react';
import type { StreamJob } from '@/lib/api/streams';
import { formatStreamTimestamp, streamSourceLabel } from '@/lib/streams/plan';
import { IconTile } from '@/components/studio/icon-tile';
import { StatusTag } from '@/components/studio/status-tag';

/**
 * Identity strip for the job under edit: what it is, what was actually probed
 * from the file, and where it came from.
 *
 * Every fact here is read from `job.probe`; nothing is inferred. A source whose
 * probe is missing simply shows fewer facts instead of a plausible-looking
 * placeholder.
 */
export function StreamJobHeader({ job }: { job: StreamJob }): ReactNode {
  const sourceLabel = streamSourceLabel(job.source_url);
  const probe = job.probe;
  const hasFrameSize = probe !== undefined && probe.width > 0 && probe.height > 0;
  const hasDuration = probe !== undefined && Number.isFinite(probe.duration_seconds) && probe.duration_seconds > 0;

  return (
    <section className="studio-panel flex flex-wrap items-center justify-between gap-x-5 gap-y-3 p-4">
      <div className="flex min-w-0 flex-1 items-center gap-3.5">
        <IconTile icon={Film} tone="stream" depth="inset" />
        <div className="min-w-0">
          <h2 className="truncate font-display text-body-lg font-bold text-fg-1">
            {job.title?.trim() || 'Clip de stream'}
          </h2>
          <p className="mt-1 flex flex-wrap items-center gap-x-2.5 gap-y-1 font-mono text-meta uppercase tracking-wider tabular-nums text-fg-3">
            {hasFrameSize ? <span>{probe.width}×{probe.height}</span> : null}
            {hasFrameSize && hasDuration ? <span aria-hidden>·</span> : null}
            {hasDuration ? <span>{formatStreamTimestamp(probe.duration_seconds)}</span> : null}
            {probe?.audio_codec ? (
              <>
                <span aria-hidden>·</span>
                <span>{probe.audio_codec}</span>
              </>
            ) : null}
          </p>
        </div>
      </div>

      <div className="flex shrink-0 flex-wrap items-center gap-2.5">
        <StatusTag tone="stream" dot>
          EN EDICIÓN
        </StatusTag>
        {job.source_url && sourceLabel ? (
          <a
            href={job.source_url}
            target="_blank"
            rel="noreferrer"
            className="inline-flex min-h-10 items-center gap-1.5 px-1 font-mono text-meta uppercase tracking-wider text-stream-text underline-offset-4 transition-colors duration-(--dur-fast) ease-standard hover:text-stream hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
          >
            {sourceLabel}
            <ExternalLink aria-hidden className="size-3.5" />
          </a>
        ) : null}
      </div>

      <p className="w-full text-body-sm text-fg-3">
        El título se ha copiado al primer rango y puedes editarlo allí.
      </p>
    </section>
  );
}
