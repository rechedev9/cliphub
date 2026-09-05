'use client';
import type { ReactNode } from 'react';
import { streamsApi, type StreamEditPlan, type StreamJob, type StreamRenderState } from '@/lib/api/streams';
import { openYouTubeStudio } from '@/lib/publish-actions';
import { formatStreamClock } from '@/lib/streams/plan';
import { Button } from '@/components/ui/button';
import { StreamSaveButton } from '@/components/streams/stream-save-button';
export function StreamRenderResults({
  renderState,
  job,
  renderedPlan,
  stale,
  selectedClipId,
  onSelect,
}: {
  renderState: StreamRenderState | null;
  job: StreamJob;
  renderedPlan: StreamEditPlan;
  stale: boolean;
  selectedClipId?: string;
  onSelect: (clipId: string) => void;
}): ReactNode {
  if (!renderState) return null;
  return (
    <div className="flex flex-col gap-4">
      <p className={stale ? 'text-warning' : 'text-success'}>
        {stale
          ? 'Hay cambios sin exportar. Exporta de nuevo para aplicarlos.'
          : 'Tus vídeos están listos. Revísalos y guárdalos en tu equipo.'}
      </p>
      {renderState.warnings?.map((warning, i) => (
        <p key={i} className="text-warning text-body-sm">
          {warning}
        </p>
      ))}
      <ul className="flex flex-col gap-3">
        {renderState.videos.map((v, index) => {
          const label = v.title || `Short ${index + 1}`;
          return (
            <li key={v.clip_id} className="rounded-md border border-border-subtle p-3">
              <Button
                variant={selectedClipId === v.clip_id ? 'secondary' : 'ghost'}
                className="mb-2 h-auto w-full justify-start whitespace-normal text-left"
                onClick={() => onSelect(v.clip_id)}
              >
                {label} · {formatStreamClock(v.duration_seconds ?? 0)}
              </Button>
              <StreamSaveButton
                jobId={job.id}
                variant={renderedPlan.variant}
                revision={renderedPlan.updated_at}
                clipId={v.clip_id}
                title={label}
                disabled={stale}
              />
            </li>
          );
        })}
      </ul>
      <div className="border-t border-border-subtle pt-3">
        <Button variant="outline" disabled={stale} onClick={openYouTubeStudio}>
          Abrir YouTube Studio
        </Button>
        <p className="mt-2 text-label text-fg-3">Guarda el vídeo y súbelo desde tu cuenta de YouTube.</p>
      </div>
      {renderState.delivery?.length ? (
        <details className="rounded-md border border-border-subtle p-3">
          <summary className="cursor-pointer text-body-sm text-fg-3">Archivos adicionales</summary>
          <div className="mt-3 flex flex-wrap gap-2">
            {renderState.delivery
              .filter((a) => !a.name.endsWith('.mp4'))
              .map((a) => (
                <Button key={a.name} asChild={!stale} variant="outline" size="sm" disabled={stale}>
                  {stale ? (
                    a.name
                  ) : (
                    <a href={streamsApi.deliveryUrl(job.id, renderedPlan.variant, a.name)} download={a.name}>
                      {a.name}
                    </a>
                  )}
                </Button>
              ))}
          </div>
        </details>
      ) : null}
    </div>
  );
}
