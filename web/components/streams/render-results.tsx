'use client';

import { useState, type ReactNode } from 'react';
import { Play } from 'lucide-react';
import { streamsApi, type StreamEditPlan, type StreamJob, type StreamRenderState } from '@/lib/api/streams';
import { PLAYBACK_REVIEW, streamPlaybackItem, type MediaPlaybackItem } from '@/lib/api/playback';
import { openYouTubeStudio } from '@/lib/publish-actions';
import { formatStreamClock } from '@/lib/streams/plan';
import { Button } from '@/components/ui/button';
import { StreamSaveButton } from '@/components/streams/stream-save-button';
import { MediaPlayer } from '@/components/studio/media-player';
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
  const [activeId, setActiveId] = useState<string | null>(null);
  const [playerOpen, setPlayerOpen] = useState(false);
  const [returnFocus, setReturnFocus] = useState<HTMLElement | null>(null);
  if (!renderState) return null;
  const videos = renderState.videos ?? [];
  const empty = videos.length === 0;
  let message = 'Tus vídeos están listos. Revísalos y guárdalos en tu equipo.';
  if (empty) message = 'No se generó ningún vídeo. Vuelve a exportar para intentarlo de nuevo.';
  else if (stale) message = 'Hay cambios sin exportar. Exporta de nuevo para aplicarlos.';
  const playbackEntries = (job.rendered_outputs ?? [])
    .filter((output) => output.variant === renderedPlan.variant)
    .map((output) => {
      const item = streamPlaybackItem(job, output);
      if (item === null) return { clipId: output.clip_id, item };
      let review = item.review;
      if (stale) review = PLAYBACK_REVIEW.stale;
      else if (renderState.warnings?.length && review === PLAYBACK_REVIEW.ready) review = PLAYBACK_REVIEW.pending;
      return {
        clipId: output.clip_id,
        item: { ...item, review, warnings: Array.from(new Set([...item.warnings, ...(renderState.warnings ?? [])])) },
      };
    })
    .filter((entry): entry is { clipId: string; item: MediaPlaybackItem } => entry.item !== null);
  const playbackItems = playbackEntries.map((entry) => entry.item);
  const playbackByClip = new Map(playbackEntries.map((entry) => [entry.clipId, entry.item]));
  return (
    <div className="flex flex-col gap-4">
      <p className={empty || stale ? 'text-warning' : 'text-success'}>{message}</p>
      {renderState.warnings?.map((warning, i) => (
        <p key={i} className="text-warning text-body-sm">
          {warning}
        </p>
      ))}
      <ul className="flex flex-col gap-3">
        {videos.map((v, index) => {
          const label = v.title || `Short ${index + 1}`;
          const playback = playbackByClip.get(v.clip_id);
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
              {playback ? (
                <Button
                  type="button"
                  variant="outline-primary"
                  size="sm"
                  className="mt-2"
                  onClick={(event) => {
                    setActiveId(playback.id);
                    setReturnFocus(event.currentTarget);
                    setPlayerOpen(true);
                  }}
                >
                  <Play aria-hidden /> Reproducir
                </Button>
              ) : null}
            </li>
          );
        })}
      </ul>
      {!empty ? (
        <div className="border-t border-border-subtle pt-3">
          <Button variant="outline" disabled={stale} onClick={openYouTubeStudio}>
            Abrir YouTube Studio
          </Button>
          <p className="mt-2 text-label text-fg-3">Guarda el vídeo y súbelo desde tu cuenta de YouTube.</p>
        </div>
      ) : null}
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
      <MediaPlayer
        items={playbackItems}
        activeId={activeId}
        open={playerOpen}
        onActiveChange={setActiveId}
        onOpenChange={setPlayerOpen}
        returnFocus={returnFocus}
      />
    </div>
  );
}
