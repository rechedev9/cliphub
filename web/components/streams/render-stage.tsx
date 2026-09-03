'use client';

import type { ReactNode } from 'react';
import { Film } from 'lucide-react';
import type { StreamClipRange, StreamRenderState } from '@/lib/api/streams';
import { clipOutputDuration } from '@/lib/streams/plan';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { LongOperation } from '@/components/studio/long-operation';
import { MediaFrame } from '@/components/studio/media-frame';
import { StatusTag } from '@/components/studio/status-tag';
import { useElapsedSeconds } from '@/components/streams/use-elapsed-seconds';

/**
 * What a multi-minute FFmpeg render looks like while it runs.
 *
 * Before this existed the render stage relabelled the CTA and spun a 16px
 * loader — no real feedback for a multi-minute job. It now uses the same
 * `LongOperation` surface as the demo pipeline and the same
 * 9:16 `MediaFrame` the Library and the results grid use, so both pipelines
 * speak one visual language.
 *
 * The bar is indeterminate because the render reports a status, not a
 * percentage; the frames are the real clips from the plan, labelled with their
 * real output durations. Nothing here is invented to fill the grid.
 */
export function StreamRenderStage({
  clips,
  renderState,
  variantLabel,
}: {
  clips: StreamClipRange[];
  renderState: StreamRenderState | null;
  variantLabel: string;
}): ReactNode {
  const elapsed = useElapsedSeconds(true);
  const queued = renderState === null || renderState.status === 'queued';
  const stage = queued ? 'EN COLA' : 'RENDERIZANDO';

  return (
    <section className="studio-panel flex flex-col gap-4 p-3.5">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <SectionEyebrow label="RENDER" accent="magenta" />
        <StatusTag tone="stream" dot>
          {stage}
        </StatusTag>
      </div>

      <LongOperation
        stage={stage}
        detail={`${clips.length} ${clips.length === 1 ? 'CLIP' : 'CLIPS'} · ${variantLabel} · 1080×1920`}
        elapsedSec={elapsed}
        tone="stream"
      />

      <ul className="grid grid-cols-2 gap-3">
        {clips.map((clip, index) => (
          <li key={clip.id} className="flex flex-col gap-2">
            <MediaFrame
              aspect="9:16"
              scanline
              className="studio-rim border border-stream/30"
              badge={<StatusTag tone="stream">{String(index + 1).padStart(2, '0')}</StatusTag>}
              fallback={
                <span className="grid size-full place-items-center bg-[linear-gradient(180deg,color-mix(in_oklch,var(--stream)_12%,transparent),transparent_72%)]">
                  <Film aria-hidden className="size-6 text-stream-text" />
                </span>
              }
            />
            <p className="truncate text-body-sm text-fg-2">{clip.title?.trim() || `Clip ${index + 1}`}</p>
            <p className="font-mono text-meta tabular-nums text-fg-3">
              {clipOutputDuration(clip).toFixed(2)} S
            </p>
          </li>
        ))}
      </ul>
    </section>
  );
}
