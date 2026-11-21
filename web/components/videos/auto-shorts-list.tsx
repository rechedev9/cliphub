'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { Download, Play } from 'lucide-react';
import { toast } from 'sonner';
import { autoShortsFiles, type AutoShortsFiles } from '@/lib/api/auto-shorts';
import { dataPlane } from '@/lib/api/dataplane';
import { demoPlaybackItem } from '@/lib/api/playback';
import type { Video } from '@/lib/api/types';
import { downloadPublishMP4 } from '@/lib/publish-actions';
import { Button } from '@/components/ui/button';
import { MediaPlayer } from '@/components/studio/media-player';
import { apiFetch } from '@/lib/api/edition-fetch';

type ListView = { status: 'loading' } | { status: 'failed' } | ({ status: 'ready' } & AutoShortsFiles);

/** Every Short of a finished Auto-Shorts batch; the page's own player and download cover only the first. */
export function AutoShortsList({ video }: { video: Video }): ReactNode {
  const jobId = video.jobId;
  const variant = video.variant ?? 'viral-60-clean';
  const batch = video.editConfig?.separateShorts === true && video.status === 'ready' && jobId !== undefined;
  const [view, setView] = useState<ListView>({ status: 'loading' });
  const [activeId, setActiveId] = useState<string | null>(null);
  const [returnFocus, setReturnFocus] = useState<HTMLElement | null>(null);

  useEffect(() => {
    if (!batch) return;
    const controller = new AbortController();
    apiFetch(dataPlane().renderUrl(jobId, variant), { cache: 'no-store', signal: controller.signal })
      .then(async (response) => {
        if (!response.ok) throw new Error(`render ${response.status}`);
        setView({ status: 'ready', ...autoShortsFiles(await response.json(), { jobId, variant, artifactPrefix: video.reviewArtifactPrefix }) });
      })
      .catch(() => {
        if (!controller.signal.aborted) setView({ status: 'failed' });
      });
    return () => controller.abort();
  }, [batch, jobId, variant, video.reviewArtifactPrefix]);

  if (!batch) return null;
  const files = view.status === 'ready' ? view.files : [];
  const rows = files.map((file, index) => ({
    file,
    item: demoPlaybackItem({ ...video, id: `${video.id}-${index + 1}`, title: `${video.title} · Short ${index + 1}`, downloadUrl: file.url, thumbnailUrl: undefined }),
  }));
  const items = rows.flatMap((row) => (row.item ? [row.item] : []));

  return <section className="studio-panel space-y-3 p-4" aria-label="Shorts de Auto-Shorts">
    <h2 className="font-display text-body-lg font-semibold text-fg-1">Shorts de esta tanda</h2>
    {view.status === 'loading' ? <p className="text-body-sm text-fg-2">Cargando los Shorts…</p> : null}
    {view.status === 'failed' ? <p role="alert" className="text-body-sm text-fg-2">No se pudo leer la lista de Shorts. Vuelve a abrir la página para reintentar.</p> : null}
    {view.status === 'ready' && view.replaced ? (
      <p className="text-body-sm text-fg-2">Después se creó otro vídeo con este estilo en la misma partida, así que Studio ya no guarda la lista de esta tanda. Descargar MP4 sigue dando el primer Short.</p>
    ) : null}
    {files.length > 0 ? (
      <ol className="flex flex-col gap-2">
        {rows.map(({ file, item }, index) => {
          return <li key={file.name} className="flex flex-wrap items-center gap-2">
            <span className="min-w-16 font-display text-body-sm font-semibold text-fg-1">Short {index + 1}</span>
            {item ? (
              <Button type="button" size="xs" variant="outline-primary" onClick={(event) => { setReturnFocus(event.currentTarget); setActiveId(item.id); }}>
                <Play aria-hidden />Reproducir
              </Button>
            ) : null}
            <Button type="button" size="xs" variant="outline" onClick={() => {
              downloadPublishMP4(file.url, `${video.title} ${index + 1}`);
              toast('Descargando MP4', { description: `Short ${index + 1} → Descargas` });
            }}>
              <Download aria-hidden />Descargar
            </Button>
          </li>;
        })}
      </ol>
    ) : null}
    <MediaPlayer items={items} activeId={activeId} open={activeId !== null} onActiveChange={setActiveId} onOpenChange={(open) => { if (!open) setActiveId(null); }} returnFocus={returnFocus} />
  </section>;
}
