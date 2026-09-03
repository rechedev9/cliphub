'use client';

import { use, useEffect, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { AlertTriangle, Check, Download, Music, SearchX, Settings2 } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import type { Match, Video } from '@/lib/api/types';
import { HUB_LENS, hubHref, ORPHAN_MATCH_SEGMENT } from '@/lib/clips/routes';
import { OUTPUT_STATE, OUTPUT_TYPE, outputState, outputType, type OutputState } from '@/lib/clips/hub';
import { timeAgo } from '@/lib/format';
import { startPollLoop } from '@/lib/poll-loop';
import { downloadPublishMP4 } from '@/lib/publish-actions';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { ReelCover } from '@/components/brand/reel-cover';
import { CoverImage } from '@/components/studio/cover-image';
import { MediaFrame } from '@/components/studio/media-frame';
import { StatusTag, type StatusTagTone } from '@/components/studio/status-tag';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { PublishAssistantPanel } from '@/components/videos/publish-assistant-panel';
import { LibraryMusicDialog } from '@/components/videos/library-music-dialog';
import { ReviewResolutionDialog } from '@/components/videos/review-resolution-dialog';

const FAST_POLL_MS = 1500;
const IDLE_POLL_MS = 10000;

const STATE_TAG: Record<OutputState, { label: string; tone: StatusTagTone }> = {
  queue: { label: 'En cola', tone: 'neutral' },
  rec: { label: 'REC', tone: 'stream' },
  render: { label: 'Edición', tone: 'primary' },
  ready: { label: 'Listo', tone: 'success' },
  failed: { label: 'Falló', tone: 'danger' },
};

export default function PublishPage({ params }: { params: Promise<{ id: string; clipId: string }> }): ReactNode {
  const { id, clipId } = use(params);
  const router = useRouter();
  const orphan = id === ORPHAN_MATCH_SEGMENT;
  const backHref = orphan ? hubHref({ lens: HUB_LENS.clips }) : hubHref({ open: id });

  const [video, setVideo] = useState<Video | null>(null);
  const [match, setMatch] = useState<Match | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [reviewOpen, setReviewOpen] = useState(false);
  const [musicOpen, setMusicOpen] = useState(false);
  const [refreshKey, setRefreshKey] = useState(0);

  useEffect(() => {
    let active = true;
    const stop = startPollLoop({
      tick: async () => {
        const [videoResult, matchResult] = await Promise.allSettled([
          api.getVideo(clipId),
          orphan ? Promise.resolve<Match | null>(null) : api.getMatch(id),
        ]);
        if (!active) return 'idle';
        const next = videoResult.status === 'fulfilled' ? videoResult.value : null;
        setVideo(next);
        if (matchResult.status === 'fulfilled') setMatch(matchResult.value);
        setLoaded(true);
        return next !== null && outputState(next.status) !== OUTPUT_STATE.ready && outputState(next.status) !== OUTPUT_STATE.failed
          ? 'fast'
          : 'idle';
      },
      fastMs: FAST_POLL_MS,
      idleMs: IDLE_POLL_MS,
    });
    return () => {
      active = false;
      stop();
    };
  }, [clipId, id, orphan, refreshKey]);

  if (!loaded) return <LoadingState />;

  if (!video) {
    return (
      <div className="studio-enter">
        <StudioEmptyState
          icon={SearchX}
          title="Clip no encontrado"
          description="Este clip ya no está en este PC. Puede que se haya borrado con sus artefactos."
          actions={<Button onClick={() => router.push(backHref)}>Volver</Button>}
        />
      </div>
    );
  }

  const type = outputType(video);
  const state = outputState(video.status);
  const ready = state === OUTPUT_STATE.ready;
  const isShort = type === OUTPUT_TYPE.short;
  const reviewRequired = video.status === 'review_required';
  // `outputType` already reads `isLandscapeRecap`: a Short is exactly a non-recap.
  const canAddMusic = ready && isShort && !reviewRequired;
  const map = video.map || match?.map || '';
  const score = (video.score || match?.score || '').trim();
  const player = video.targetName ?? match?.player ?? '';
  const dims = isShort ? '1080×1920' : '1920×1080';
  const meta = [`${dims} · 60 fps`, map, score, timeAgo(video.createdAt)].filter((part) => part !== '').join(' · ');
  const where = `${player ? `${player} en ` : ''}${map}${score ? ` (${score})` : ''}`;
  const description = isShort
    ? `Highlights de ${where}.`
    : `Partida completa de ${where}, POV con HUD nativo y comms.`;

  function download(): void {
    if (!video?.downloadUrl) return;
    downloadPublishMP4(video.downloadUrl, video.title);
    toast('Descargando MP4', { description: `${video.title} → Descargas` });
  }

  return (
    <div className="studio-enter grid max-w-[1160px] items-start gap-6 @[56rem]/content:grid-cols-[minmax(0,1fr)_380px]">
      <section className="flex min-w-0 flex-col gap-3.5">
        <div className="flex flex-col gap-1.5">
          {ready ? (
            <p className="flex items-center gap-1.5 font-mono text-meta uppercase tracking-ultra text-success">
              <Check className="size-3.5" aria-hidden /> Listo
            </p>
          ) : (
            <StatusTag tone={STATE_TAG[state].tone} dot className="w-fit">
              {STATE_TAG[state].label}
            </StatusTag>
          )}
          <h1 className="font-display text-display-sm font-bold uppercase text-fg-1">{video.title}</h1>
        </div>

        <div className="studio-panel studio-panel-raised flex flex-wrap gap-4 p-4">
          <MediaFrame
            aspect={isShort ? '9:16' : '16:9'}
            className={isShort ? 'w-[150px] shrink-0 border border-primary' : 'w-[240px] shrink-0 border border-primary'}
            media={video.thumbnailUrl ? <CoverImage src={video.thumbnailUrl} loading="eager" /> : undefined}
            fallback={<ReelCover seed={video.id} label={map} />}
          />
          <div className="flex min-w-0 flex-1 flex-col gap-2.5">
            <p className="font-mono text-meta uppercase tracking-wider text-fg-3">{meta}</p>
            <p className="text-body-sm text-fg-2">{description}</p>
            {reviewRequired && video.warnings && video.warnings.length > 0 ? (
              <div className="border border-warning/35 bg-warning/10 px-3 py-2.5" role="status">
                <p className="flex items-center gap-1.5 font-mono text-meta uppercase tracking-wider text-warning">
                  <AlertTriangle className="size-3.5" aria-hidden /> Revisión pendiente
                </p>
                <ul className="mt-1.5 flex list-disc flex-col gap-1 pl-4 text-body-sm text-fg-2">
                  {video.warnings.map((warning) => (
                    <li key={warning}>{warning}</li>
                  ))}
                </ul>
                <Button
                  variant="warning"
                  className="mt-2.5 h-10"
                  onClick={() => setReviewOpen(true)}
                >
                  <Settings2 className="size-4" aria-hidden /> Resolver revisión QA
                </Button>
              </div>
            ) : null}
            <div className="mt-auto flex flex-wrap gap-2 pt-1">
              <Button variant="hero" size="sm" disabled={!video.downloadUrl} onClick={download} className="neon-notch focus-visible:-outline-offset-4">
                <Download className="size-4" /> Descargar MP4
              </Button>
              {canAddMusic ? (
                <Button variant="outline" size="sm" onClick={() => setMusicOpen(true)}>
                  <Music className="size-4 text-stream" aria-hidden />
                  {video.songId ? 'Cambiar música' : 'Añadir música'}
                </Button>
              ) : null}
              <Button variant="outline" size="sm" onClick={() => router.push(backHref)}>
                {orphan ? 'Volver a clips' : 'Volver a la partida'}
              </Button>
            </div>
          </div>
        </div>
      </section>

      <aside className="studio-panel p-4.5">
        <PublishAssistantPanel video={video} />
      </aside>

      {reviewRequired ? (
        <ReviewResolutionDialog
          open={reviewOpen}
          video={video}
          onOpenChange={setReviewOpen}
          onResolved={() => setRefreshKey((key) => key + 1)}
        />
      ) : null}
      {canAddMusic ? (
        <LibraryMusicDialog
          open={musicOpen}
          video={video}
          onOpenChange={setMusicOpen}
          onApplied={() => setRefreshKey((key) => key + 1)}
        />
      ) : null}
    </div>
  );
}

function LoadingState(): ReactNode {
  return (
    <div className="grid max-w-[1160px] items-start gap-6 @[56rem]/content:grid-cols-[minmax(0,1fr)_380px]" role="status" aria-label="Cargando el clip">
      <div className="flex flex-col gap-4">
        <Skeleton className="h-4 w-20" />
        <Skeleton className="h-9 w-80" />
        <Skeleton className="h-72 w-full" />
      </div>
      <Skeleton className="h-96 w-full" />
    </div>
  );
}
