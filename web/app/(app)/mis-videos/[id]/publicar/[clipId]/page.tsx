'use client';

import { use, useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { AlertTriangle, Music, Play, SearchX } from 'lucide-react';
import { api } from '@/lib/api';
import type { Match, Video } from '@/lib/api/types';
import { loteShortVideo } from '@/lib/api/auto-shorts';
import { demoPlaybackItem } from '@/lib/api/playback';
import { ORPHAN_MATCH_SEGMENT, PUBLISH_SHORT_QUERY, RESULTS_HREF, matchHref } from '@/lib/clips/routes';
import { OUTPUT_STATE, OUTPUT_TYPE, outputState, outputType, type OutputState } from '@/lib/clips/hub';
import { videoDisplayTitle } from '@/lib/clips/results';
import { prettyMapName, timeAgo } from '@/lib/format';
import { startPollLoop } from '@/lib/poll-loop';
import { publishSummary } from '@/lib/publish-summary';
import { cn } from '@/lib/utils';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { ReelCover } from '@/components/brand/reel-cover';
import { MediaFrame } from '@/components/studio/media-frame';
import { MediaPlayer } from '@/components/studio/media-player';
import { StatusTag, type StatusTagTone } from '@/components/studio/status-tag';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { PublishSteps } from '@/components/videos/publish-steps';
import { LibraryMusicDialog } from '@/components/videos/library-music-dialog';
import { FullDemoEvidence } from '@/components/videos/full-demo-evidence';
import { AutoShortsList, useAutoShortsFiles } from '@/components/videos/auto-shorts-list';
import { VideoPoster } from '@/components/videos/video-poster';

const FAST_POLL_MS = 1500;
const IDLE_POLL_MS = 10000;

const STATE_TAG: Record<OutputState, { label: string; tone: StatusTagTone }> = {
  queue: { label: 'En cola', tone: 'neutral' },
  rec: { label: 'REC', tone: 'stream' },
  render: { label: 'Edición', tone: 'primary' },
  ready: { label: 'Listo', tone: 'success' },
  failed: { label: 'Falló', tone: 'danger' },
};

const LAYOUT_CLASS = 'grid items-start gap-6 @[80rem]/content:grid-cols-[minmax(0,1fr)_360px]';
const ASIDE_CLASS = 'flex min-w-0 flex-col gap-4 @[80rem]/content:col-start-2 @[80rem]/content:row-start-1';
const MAIN_CLASS = 'flex min-w-0 flex-col gap-4 @[80rem]/content:col-start-1 @[80rem]/content:row-start-1';

type PublishPageProps = {
  params: Promise<{ id: string; clipId: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
};

export default function PublishPage({ params, searchParams }: PublishPageProps): ReactNode {
  const { id, clipId } = use(params);
  const requestedShort = use(searchParams)[PUBLISH_SHORT_QUERY];
  const short = typeof requestedShort === 'string' ? requestedShort : null;
  const router = useRouter();
  const orphan = id === ORPHAN_MATCH_SEGMENT;

  const [video, setVideo] = useState<Video | null>(null);
  const [match, setMatch] = useState<Match | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [loadFailed, setLoadFailed] = useState(false);
  const [musicOpen, setMusicOpen] = useState(false);
  const [playerOpen, setPlayerOpen] = useState(false);
  const [returnFocus, setReturnFocus] = useState<HTMLElement | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);
  const loteView = useAutoShortsFiles(video);

  useEffect(() => {
    let active = true;
    const stop = startPollLoop({
      tick: async () => {
        const [videoResult, matchResult] = await Promise.allSettled([
          api.getVideo(clipId),
          orphan ? Promise.resolve<Match | null>(null) : api.getMatch(id),
        ]);
        if (!active) return 'idle';
        if (videoResult.status === 'fulfilled') setVideo(videoResult.value);
        setLoadFailed(videoResult.status === 'rejected');
        if (matchResult.status === 'fulfilled') setMatch(matchResult.value);
        setLoaded(true);
        if (videoResult.status === 'rejected') return 'fast';
        const next = videoResult.value;
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
      <StudioEmptyState
        icon={loadFailed ? AlertTriangle : SearchX}
        title={loadFailed ? 'No se pudo cargar el vídeo' : 'Vídeo no encontrado'}
        description={
          loadFailed
            ? 'El servicio local no respondió. Reintenta o vuelve a Mis vídeos.'
            : 'Este vídeo ya no está en este PC. Puede que se haya borrado.'
        }
        actions={
          <>
            {loadFailed ? (
              <Button variant="outline-primary" onClick={() => setRefreshKey((k) => k + 1)}>
                Reintentar
              </Button>
            ) : null}
            <Button onClick={() => router.push(RESULTS_HREF)}>Volver a Mis vídeos</Button>
          </>
        }
      />
    );
  }

  const type = outputType(video);
  const state = outputState(video.status);
  const ready = state === OUTPUT_STATE.ready;
  const isShort = type === OUTPUT_TYPE.short;
  const canAddMusic = ready && isShort && !video.recovered;
  const map = video.map || match?.map || '';
  const score = (video.score || match?.score || '').trim();
  const player = video.targetName ?? match?.player ?? '';
  const title = videoDisplayTitle({ title: video.title, map }, player);
  const dims = isShort ? '1080×1920' : '1920×1080';
  const mapName = map === '' ? '' : prettyMapName(map);
  const meta = [`${dims} · 60 fps`, mapName, score, timeAgo(video.createdAt)].filter((part) => part !== '').join(' · ');
  const shown = loteShortVideo(video, { short, files: loteView.status === 'ready' ? loteView.files : null });
  const playback = demoPlaybackItem(shown);
  const posterWidth = isShort ? 'mx-auto w-44 @[30rem]:mx-0 @[30rem]:w-28' : 'w-full @[30rem]:w-56';
  const frame = (
    <MediaFrame
      aspect={isShort ? '9:16' : '16:9'}
      media={<VideoPoster posterUrl={shown.thumbnailUrl} videoUrl={playback?.playbackUrl} loading="eager" fallback={<ReelCover seed={video.id} label={mapName} />} />}
    />
  );

  return (
    <div className="measure-work flex flex-col gap-6">
      <header className="flex flex-col gap-2">
        {ready ? null : (
          <StatusTag tone={STATE_TAG[state].tone} dot className="w-fit">
            {STATE_TAG[state].label}
          </StatusTag>
        )}
        <h1 className="font-display text-display-sm font-bold text-fg-1">{title}</h1>
        <p className="max-w-read text-body-sm text-fg-2">{publishSummary(video, { map, player, score })}</p>
      </header>

      <div className={LAYOUT_CLASS}>
        <aside className={ASIDE_CLASS} aria-label="Vídeo">
          <section className="studio-panel @container p-3">
            <div className="flex flex-col gap-3 @[30rem]:flex-row @[30rem]:items-start">
              {playback ? (
                <button
                  type="button"
                  aria-label={`Reproducir ${title}`}
                  onClick={(event) => {
                    setReturnFocus(event.currentTarget);
                    setPlayerOpen(true);
                  }}
                  className={cn('group/poster relative block shrink-0 overflow-hidden rounded-md border border-border', FOCUS_RING, posterWidth)}
                >
                  {frame}
                  <span aria-hidden className="absolute inset-0 flex items-center justify-center">
                    <span
                      className={cn(
                        'flex items-center gap-1.5 rounded-md border border-border-strong bg-surface-1/90 px-3 py-1.5',
                        'text-body-sm font-semibold uppercase tracking-wider text-fg-1',
                        'transition-[background-color,border-color] duration-(--dur-fast) ease-standard',
                        'group-hover/poster:border-fg-3 group-hover/poster:bg-surface-3',
                      )}
                    >
                      <Play className="size-4" /> Reproducir
                    </span>
                  </span>
                </button>
              ) : (
                <div className={cn('shrink-0 overflow-hidden rounded-md border border-border', posterWidth)}>{frame}</div>
              )}
              <div className="flex min-w-0 flex-col items-start gap-2.5">
                <p className="text-meta text-fg-3">{meta}</p>
                {canAddMusic ? (
                  <Button variant="outline" size="sm" onClick={() => setMusicOpen(true)}>
                    <Music className="size-4 text-stream" aria-hidden />
                    {video.songId ? 'Cambiar música' : 'Añadir música'}
                  </Button>
                ) : null}
                {orphan || match === null ? null : (
                  <Link href={matchHref(id)} className={cn('rounded-sm text-body-sm text-fg-1 underline decoration-fg-4 underline-offset-4 hover:decoration-fg-1', FOCUS_RING)}>
                    Ver la partida
                  </Link>
                )}
              </div>
            </div>
          </section>
          <AutoShortsList video={video} view={loteView} />
        </aside>

        <div className={MAIN_CLASS}>
          <PublishSteps video={video} file={shown} />
          <FullDemoEvidence video={video} />
        </div>
      </div>

      {canAddMusic ? (
        <LibraryMusicDialog
          open={musicOpen}
          video={video}
          onOpenChange={setMusicOpen}
          onApplied={() => setRefreshKey((key) => key + 1)}
        />
      ) : null}
      {playback ? (
        <MediaPlayer
          items={[playback]}
          activeId={playback.id}
          open={playerOpen}
          onOpenChange={setPlayerOpen}
          returnFocus={returnFocus}
        />
      ) : null}
    </div>
  );
}

function LoadingState(): ReactNode {
  return (
    <div className="measure-work flex flex-col gap-6" role="status" aria-label="Cargando el vídeo">
      <div className="flex flex-col gap-2">
        <Skeleton className="h-9 w-80 max-w-full" />
        <Skeleton className="h-4 w-96 max-w-full" />
      </div>
      <div className={LAYOUT_CLASS}>
        <Skeleton className={cn('h-64 w-full', ASIDE_CLASS)} />
        <Skeleton className={cn('h-[36rem] w-full', MAIN_CLASS)} />
      </div>
    </div>
  );
}
