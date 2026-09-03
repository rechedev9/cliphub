'use client';

import type { ReactNode } from 'react';
import { AlertTriangle, Download, Film } from 'lucide-react';
import { toast } from 'sonner';
import { streamsApi, type StreamEditPlan, type StreamJob, type StreamRenderState } from '@/lib/api/streams';
import { openYouTubeStudio } from '@/lib/publish-actions';
import { formatStreamClock } from '@/lib/streams/plan';
import { MediaFrame } from '@/components/studio/media-frame';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { StreamStepCard } from '@/components/streams/step-card';
import { cn } from '@/lib/utils';

const CHIP_CLASS = 'font-mono uppercase tracking-wider';

/**
 * The finished Shorts and the publish pack. Every URL is built from
 * `renderedPlan`, never from the live edits: once the plan drifts the block
 * goes stale and downloads are blocked, so nobody walks away with a file that
 * does not match what they see. Publishing is manual: YouTube Studio opens,
 * the user uploads the MP4 there.
 */
export function StreamRenderResults({
  renderState,
  job,
  renderedPlan,
  stale,
}: {
  renderState: StreamRenderState | null;
  job: StreamJob;
  /** The plan the shown render actually used; URLs must come from it. */
  renderedPlan: StreamEditPlan;
  stale: boolean;
}): ReactNode {
  if (!renderState) return null;

  const publish = (label: string): void => {
    openYouTubeStudio();
    toast('Abriendo YouTube Studio', { description: `Sube ${label} desde tu cuenta` });
  };

  return (
    <div className="flex flex-col gap-2.5">
      <div className="flex items-center justify-between gap-2">
        <span className="font-mono text-meta uppercase tracking-widest text-stream-text">
          05 · Shorts renderizados
        </span>
        <StatusTag tone={stale ? 'warning' : 'success'} dot>
          {stale ? 'Desactualizado' : 'Listo para subir'}
        </StatusTag>
      </div>

      {stale ? (
        <p className="flex items-start gap-2 border border-warning/45 bg-warning/10 px-3 py-2 text-body-sm text-warning">
          <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
          Estos Shorts se renderizaron antes de tus últimos cambios. Vuelve a crear los Shorts para
          aplicarlos; la descarga queda bloqueada hasta entonces.
        </p>
      ) : null}

      {renderState.warnings && renderState.warnings.length > 0 ? (
        <ul className="flex flex-col gap-2">
          {renderState.warnings.map((w, i) => (
            <li key={i} className="flex items-start gap-2 border border-warning/45 bg-warning/10 px-3 py-2 text-body-sm text-warning">
              <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
              {w}
            </li>
          ))}
        </ul>
      ) : null}

      {renderState.videos.length === 0 ? (
        <p className="flex items-center gap-2 border border-dashed border-border px-3.5 py-3 text-body-sm text-fg-3">
          <Film aria-hidden className="size-4 shrink-0" />
          No se generó ningún Short.
        </p>
      ) : (
        <ul className="flex flex-col gap-2.5">
          {renderState.videos.map((v, index) => {
            const url = streamsApi.videoUrl(job.id, renderedPlan.variant, v.clip_id);
            const label = v.title || `Clip ${index + 1}`;
            return (
              <li
                key={v.clip_id}
                className={cn('studio-panel flex gap-3 p-3', stale ? 'border-warning/45' : 'border-success/45')}
              >
                <MediaFrame
                  aspect="9:16"
                  className={cn('w-11 shrink-0 border', stale ? 'border-warning/45' : 'border-success/45')}
                  media={
                    <span className="block size-full bg-black">
                      {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
                      <video src={url} preload="metadata" muted className="size-full object-cover" />
                    </span>
                  }
                />
                <div className="flex min-w-0 flex-1 flex-col gap-1.5">
                  <span className="truncate font-display text-label font-semibold uppercase text-fg-1">{label}</span>
                  <span className="font-mono text-meta uppercase tracking-wider text-fg-3">
                    {v.duration_seconds !== undefined ? `${formatStreamClock(v.duration_seconds)} · ` : ''}1080×1920
                  </span>
                  <div className="mt-auto flex gap-1.5">
                    {stale ? (
                      <Button variant="outline" size="sm" disabled className={CHIP_CLASS} aria-label={`Descargar ${label} (desactualizado)`}>
                        <Download aria-hidden />
                        MP4
                      </Button>
                    ) : (
                      <Button asChild variant="outline" size="sm" className={cn(CHIP_CLASS, 'border-stream/45 text-stream-text')}>
                        <a href={url} download aria-label={`Descargar ${label}`}>
                          <Download aria-hidden />
                          MP4
                        </a>
                      </Button>
                    )}
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      disabled={stale}
                      onClick={() => publish(label)}
                      className={CHIP_CLASS}
                    >
                      Publicar
                    </Button>
                  </div>
                </div>
              </li>
            );
          })}
        </ul>
      )}

      {renderState.delivery && renderState.delivery.length > 0 ? (
        <StreamStepCard title="Paquete listo para subir">
          <div className="flex flex-wrap gap-1.5">
            {renderState.delivery.map((artifact) =>
              stale ? (
                <Button key={artifact.name} variant="outline" size="sm" disabled className={CHIP_CLASS} aria-label={`${artifact.name} (desactualizado)`}>
                  {artifact.name}
                </Button>
              ) : (
                <Button key={artifact.name} asChild variant="outline" size="sm" className={CHIP_CLASS}>
                  <a href={streamsApi.deliveryUrl(job.id, renderedPlan.variant, artifact.name)} download={artifact.name}>
                    {artifact.name}
                  </a>
                </Button>
              ),
            )}
          </div>
        </StreamStepCard>
      ) : null}
    </div>
  );
}
