'use client';

import type { ReactNode } from 'react';
import { Film } from 'lucide-react';
import type { StreamRenderState } from '@/lib/api/streams';
import type { StreamShort } from '@/lib/streams/groups';
import { formatStreamClock } from '@/lib/streams/plan';
import { shortsWord, streamRenderQueued } from '@/lib/streams/editor';
import { streamRenderProgressView } from '@/lib/streams/render-progress';
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
 * The bar fills with the encode progress the worker reads from FFmpeg, and
 * stays indeterminate while queued; the frames are the Shorts the plan
 * exports, a joined Short as one, labelled with their real output durations.
 */
export function StreamRenderStage({
  shorts,
  renderState,
  variantLabel,
}: {
  shorts: StreamShort[];
  renderState: StreamRenderState | null;
  variantLabel: string;
}): ReactNode {
  const elapsed = useElapsedSeconds(true);
  const queued = streamRenderQueued(renderState);
  const stage = queued ? 'En cola' : 'Creando vídeos';
  const progress = queued ? null : streamRenderProgressView(renderState?.render_progress);
  const scope = `${variantLabel} · 1080×1920`;

  return (
    <section className="studio-panel flex flex-col gap-4 p-3.5">
      <h2 className="text-body-sm font-semibold text-fg-1">Exportación</h2>

      <LongOperation
        stage={stage}
        detail={progress ? `${progress.clipLabel} · ${scope}` : `${shortsWord(shorts.length)} · ${scope}`}
        percent={progress?.percent}
        elapsedSec={elapsed}
        tone="stream"
      />

      <ul className="grid grid-cols-2 gap-3">
        {shorts.map((short, index) => (
          <li key={short.id} className="flex flex-col gap-2">
            {/* The progress bar above already names the stage; pending tiles stay neutral. */}
            <MediaFrame
              aspect="9:16"
              scanline
              className="border border-border"
              badge={<StatusTag>{String(index + 1).padStart(2, '0')}</StatusTag>}
              fallback={
                <span className="grid size-full place-items-center">
                  <Film aria-hidden className="size-6 text-fg-3" />
                </span>
              }
            />
            <p className="truncate text-body-sm text-fg-2">{short.title}</p>
            <p className="text-meta tabular-nums text-fg-3">{formatStreamClock(short.durationSeconds)}</p>
          </li>
        ))}
      </ul>
    </section>
  );
}
