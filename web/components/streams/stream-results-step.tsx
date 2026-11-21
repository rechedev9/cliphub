'use client';

import type { ReactNode } from 'react';
import { streamsApi, type StreamEditPlan, type StreamJob, type StreamRenderState } from '@/lib/api/streams';
import type { StreamShort } from '@/lib/streams/groups';
import { formatStreamClock } from '@/lib/streams/plan';
import { StreamRenderResults } from '@/components/streams/render-results';
import { StreamRenderStage } from '@/components/streams/render-stage';

export function StreamResultsStep({
  job,
  stage,
  shorts,
  renderState,
  renderedPlan,
  hasRender,
  stale,
  variantLabel,
  selectedClipId,
  onSelect,
}: {
  job: StreamJob;
  stage: 'editing' | 'rendering' | 'rendered';
  shorts: StreamShort[];
  renderState: StreamRenderState | null;
  renderedPlan: StreamEditPlan | null;
  hasRender: boolean;
  stale: boolean;
  variantLabel: string;
  selectedClipId: string | undefined;
  onSelect: (clipId: string) => void;
}): ReactNode {
  if (stage === 'rendering') {
    return <StreamRenderStage shorts={shorts} renderState={renderState} variantLabel={variantLabel} />;
  }
  if (renderedPlan && hasRender) {
    return (
      <StreamRenderResults
        renderState={renderState}
        job={job}
        renderedPlan={renderedPlan}
        stale={stale}
        selectedClipId={selectedClipId}
        onSelect={onSelect}
      />
    );
  }
  return <p className="text-body-sm text-fg-3">Todavía no hay vídeos exportados.</p>;
}

export function StreamResultVideo({
  job,
  renderedPlan,
  video,
  title,
}: {
  job: StreamJob;
  renderedPlan: StreamEditPlan;
  video: StreamRenderState['videos'][number];
  title: string;
}): ReactNode {
  return (
    <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-3">
      <h2 className="font-semibold">{title}</h2>
      <video
        key={`${video.clip_id}:${renderedPlan.updated_at}`}
        aria-label="Vídeo final"
        controls
        playsInline
        preload="metadata"
        src={streamsApi.videoUrl(job.id, renderedPlan.variant, video.clip_id, renderedPlan.updated_at)}
        className="min-h-0 max-h-[60vh] max-w-full rounded-md bg-black"
      />
      <p className="text-label text-fg-3">{formatStreamClock(video.duration_seconds ?? 0)} · 1080 × 1920</p>
    </div>
  );
}
