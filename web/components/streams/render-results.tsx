'use client';

import type { ReactNode } from 'react';
import { AlertTriangle, Download, Film } from 'lucide-react';
import { streamsApi, type StreamEditPlan, type StreamJob, type StreamRenderState } from '@/lib/api/streams';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { MediaFrame } from '@/components/studio/media-frame';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';

/**
 * The finished deliverable: the vertical Shorts and the publish pack.
 *
 * The videos are presented in the same 9:16 `MediaFrame` as the rest of the
 * product, with the bevel and the CRT pitch, because this is the last thing the
 * user sees before uploading — it used to be a bare `<video>` in a bare grid.
 *
 * Every URL is built from `renderedPlan`, never from the live edits: once the
 * plan drifts the whole block goes stale and downloads are blocked, so nobody
 * can walk away with a file that does not match what they see.
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

  return (
    <div className="studio-panel studio-panel-raised p-5 sm:p-6">
      <div className="flex flex-col gap-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <SectionEyebrow label="SHORTS RENDERIZADOS" count={renderState.videos.length} accent="magenta" />
          <StatusTag tone={stale ? 'warning' : 'success'} dot>
            {stale ? 'DESACTUALIZADO' : 'LISTO PARA SUBIR'}
          </StatusTag>
        </div>

        {stale ? (
          <p className="flex items-start gap-2 border border-warning/45 bg-warning/10 px-3 py-2 text-body-sm text-warning">
            <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
            Estos Shorts se renderizaron antes de tus últimos cambios. Pulsa Crear Shorts para
            aplicarlos — la descarga está bloqueada hasta entonces para que nunca te quedes con un
            archivo desactualizado.
          </p>
        ) : null}

        {renderState.warnings && renderState.warnings.length > 0 ? (
          <ul className="flex flex-col gap-2">
            {renderState.warnings.map((w, i) => (
              <li
                key={i}
                className="flex items-start gap-2 border border-warning/45 bg-warning/10 px-3 py-2 text-body-sm text-warning"
              >
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
          <ul className="grid gap-5 @[34rem]/editor:grid-cols-2 @[60rem]/editor:grid-cols-3">
            {renderState.videos.map((v) => {
              const url = streamsApi.videoUrl(job.id, renderedPlan.variant, v.clip_id);
              const label = v.title || v.clip_id;
              return (
                <li key={v.clip_id} className="flex flex-col gap-2.5">
                  <MediaFrame
                    aspect="9:16"
                    scanline
                    className="studio-rim border border-border-strong"
                    badge={
                      v.duration_seconds !== undefined ? (
                        <StatusTag tone="stream">{v.duration_seconds.toFixed(1)}S</StatusTag>
                      ) : null
                    }
                    media={
                      <span className="block size-full bg-black">
                        {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
                        <video src={url} controls preload="metadata" className="size-full object-contain" />
                      </span>
                    }
                  />
                  <div className="flex items-center justify-between gap-2">
                    <span className="min-w-0 truncate text-body-sm text-fg-1">{label}</span>
                    {stale ? (
                      <Button
                        variant="outline"
                        size="icon-sm"
                        disabled
                        aria-label={`Descargar ${label} (desactualizado)`}
                      >
                        <Download className="size-4" />
                      </Button>
                    ) : (
                      <Button asChild variant="outline" size="icon-sm">
                        <a href={url} download aria-label={`Descargar ${label}`}>
                          <Download className="size-4" />
                        </a>
                      </Button>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>
        )}

        {renderState.delivery && renderState.delivery.length > 0 ? (
          <section className="flex flex-col gap-2.5 border-t border-border pt-4" aria-labelledby="delivery-pack-title">
            <h3
              id="delivery-pack-title"
              className="font-display text-body-lg font-bold text-fg-1"
            >
              Paquete shortslistosparasubir
            </h3>
            <p className="text-body-sm text-fg-3">
              Incluye MP4, portada, plan, manifest, subtítulos revisados y metadata.
            </p>
            <div className="flex flex-wrap gap-2">
              {renderState.delivery.map((artifact) =>
                stale ? (
                  <Button
                    key={artifact.name}
                    variant="outline"
                    size="sm"
                    disabled
                    aria-label={`${artifact.name} (desactualizado)`}
                  >
                    <Download className="size-4" />
                    {artifact.name}
                  </Button>
                ) : (
                  <Button key={artifact.name} asChild variant="outline" size="sm">
                    <a href={streamsApi.deliveryUrl(job.id, renderedPlan.variant, artifact.name)} download={artifact.name}>
                      <Download className="size-4" />
                      {artifact.name}
                    </a>
                  </Button>
                ),
              )}
            </div>
          </section>
        ) : null}
      </div>
    </div>
  );
}
