'use client';

import { useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Download, Ellipsis, FolderOpen, History, Music, Pencil, Play, RectangleHorizontal, Smartphone, UploadCloud } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import { CHANGE_NOT_APPLIED_REASON } from '@/lib/api/failure-reason';
import { prettifyMap } from '@/lib/api/map';
import { PLAYBACK_REVIEW, demoPlaybackItem, type MediaPlaybackItem, type PlaybackFormat } from '@/lib/api/playback';
import { streamsApi } from '@/lib/api/streams';
import { CLIP_SIZE, OUTPUT_TYPE, type ClipSize, type HubModel } from '@/lib/clips/hub';
import { CREATE_AGAIN_LABEL, videoDisplayTitle, videoKind, type VideoKind } from '@/lib/clips/results';
import { ORPHAN_MATCH_SEGMENT, PRODUCE_FORMAT, editVideoHref, matchHref, produceHref, publishHref } from '@/lib/clips/routes';
import { renderVideoFile } from '@/lib/clips/video-file';
import { horizontalHref } from '@/lib/produce/long-video-kind';
import { timeAgo } from '@/lib/format';
import { PUBLISH_PREP_HINT, PUBLISH_PREP_LABEL } from '@/lib/publish-actions';
import { cn } from '@/lib/utils';
import { MapCover } from '@/components/brand/map-cover';
import { ReelCover } from '@/components/brand/reel-cover';
import { FORMAT_TONE } from '@/components/studio/format-tone';
import { MediaFrame } from '@/components/studio/media-frame';
import { StatusTag } from '@/components/studio/status-tag';
import { Button, FOCUS_RING } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { DeleteVideoDialog } from '@/components/videos/delete-video-button';
import { LibraryMusicDialog } from '@/components/videos/library-music-dialog';
import { VideoPoster } from '@/components/videos/video-poster';
import { VideoVersionsDialog } from '@/components/videos/video-versions-dialog';
import { RetryButton, downloadMP4, downloadVideoMP4 } from '@/components/clips-hub/output-item';

type HubClip = HubModel['clips'][number];

const KIND_LABEL = {
  short: 'Short',
  full: 'Demo completa',
  summary: 'Resumen',
  stream: 'Clip de stream',
} as const satisfies Record<VideoKind, string>;

// FORMAT_TONE.fill carries a hover for buttons; a static pill only needs the colour and its text.
const PILL_CLASS = {
  short: 'bg-format-short text-format-short-foreground',
  full: 'bg-format-full text-format-full-foreground',
  summary: 'bg-format-summary text-format-summary-foreground',
  stream: 'bg-format-stream text-format-stream-foreground',
} as const satisfies Record<VideoKind, string>;


// One stage height per size; both frame widths are capped so a 9:16 and a 16:9 poster fit it with the same margin.
const STAGE_CLASS = {
  S: { stage: 'h-44', tall: 'w-[5.25rem]', wide: 'w-[88%] max-w-[16rem]' },
  M: { stage: 'h-60', tall: 'w-[7.5rem]', wide: 'w-[88%] max-w-[23rem]' },
  L: { stage: 'h-80', tall: 'w-[10rem]', wide: 'w-[88%] max-w-[31rem]' },
} as const satisfies Record<ClipSize, { stage: string; tall: string; wide: string }>;

function durationLabel(seconds: number): string {
  const total = Math.max(0, Math.round(seconds));
  const minutes = Math.floor(total / 60);
  return `${minutes}:${String(total % 60).padStart(2, '0')}`;
}

type CardStageProps = {
  size: ClipSize;
  kind: VideoKind;
  format: PlaybackFormat;
  seed: string;
  /** The demo's map: its plate stands in until a cover or a frame of the MP4 loads. */
  map?: string;
  posterUrl: string | undefined;
  /** The MP4 whose frame stands in for a missing cover. */
  videoUrl?: string;
  durationSeconds?: number;
  badge?: ReactNode;
  playLabel: string;
  onPlay: ((button: HTMLElement) => void) | null;
};

/** The tinted media area: the real poster in its own frame, the format pill and the length. */
function CardStage({ size, kind, format, seed, map, posterUrl, videoUrl, durationSeconds, badge, playLabel, onPlay }: CardStageProps): ReactNode {
  const tall = format === '9:16';
  const small = size === CLIP_SIZE.s;
  const knownMap = map !== undefined && map.trim() !== '' && map !== 'Unknown';
  const fallback = knownMap ? <MapCover map={map} /> : <ReelCover seed={seed} plain />;
  return (
    <div className={cn('relative flex items-center justify-center', STAGE_CLASS[size].stage, FORMAT_TONE[kind].tint)}>
      <button
        type="button"
        disabled={onPlay === null}
        aria-label={playLabel}
        onClick={(event) => onPlay?.(event.currentTarget)}
        className={cn('block rounded-lg disabled:cursor-default', FOCUS_RING, tall ? STAGE_CLASS[size].tall : STAGE_CLASS[size].wide)}
      >
        <MediaFrame
          aspect={format}
          badge={badge}
          className="rounded-lg"
          media={<VideoPoster posterUrl={posterUrl} videoUrl={videoUrl} fallback={fallback} />}
        />
      </button>
      <span
        className={cn(
          'absolute rounded-full px-2.5 py-0.5 text-meta font-bold',
          small ? 'top-2 left-2' : 'top-3 left-3',
          PILL_CLASS[kind],
        )}
      >
        {KIND_LABEL[kind]}
      </span>
      {durationSeconds === undefined ? null : (
        <span
          className={cn(
            'absolute rounded-md bg-surface-0/80 px-1.5 py-0.5 text-meta font-bold text-fg-1 tabular-nums',
            small ? 'right-2 bottom-2' : 'right-3 bottom-3',
          )}
        >
          {durationLabel(durationSeconds)}
        </span>
      )}
    </div>
  );
}

function CardText({ size, title, meta }: { size: ClipSize; title: string; meta: string }): ReactNode {
  const small = size === CLIP_SIZE.s;
  return (
    <span className={cn('flex min-w-0 flex-col gap-1', small ? 'px-3 pt-2.5' : 'px-4 pt-3.5')}>
      <span className={cn('truncate font-display font-bold text-fg-1', small ? 'text-body-sm' : 'text-body')}>{title}</span>
      <span className="line-clamp-2 text-meta text-fg-3">{meta}</span>
    </span>
  );
}

const CARD_CLASS = 'flex flex-col overflow-hidden rounded-2xl bg-surface-2';

function actionRowClass(size: ClipSize): string {
  return cn('flex flex-wrap items-center gap-1.5', size === CLIP_SIZE.s ? 'px-3 pt-2.5 pb-3' : 'px-4 pt-3 pb-4');
}

function VisorButton({ title, onOpen }: { title: string; onOpen: (() => void) | null }): ReactNode {
  if (onOpen === null) return null;
  return (
    <Button type="button" size="icon-sm" variant="outline" aria-label={`Ver ${title} como en el móvil`} title="Ver como en el móvil" onClick={onOpen}>
      <Smartphone aria-hidden />
    </Button>
  );
}

/** Opens File Explorer on the render's MP4, found in the Almacenamiento list that the reveal route accepts. */
async function revealVideoFile(item: MediaPlaybackItem): Promise<void> {
  try {
    const files = await api.listStorageFiles();
    const file = renderVideoFile(files.videos, item);
    if (file === undefined) {
      toast.error('No se encontró el MP4 en este PC.');
      return;
    }
    await api.revealStorageFile(file.path);
  } catch {
    toast.error('No se pudo abrir la carpeta del vídeo.');
  }
}

export type ClipCardProps = {
  clip: HubClip;
  size: ClipSize;
  onChange: () => void;
  onPlay: (item: MediaPlaybackItem, button: HTMLElement) => void;
  onOpenVisor: (() => void) | null;
};

/** A ready demo video: Publicar and MP4 up front, the rest behind the "Más acciones" menu. */
export function ClipCard({ clip, size, onChange, onPlay, onOpenVisor }: ClipCardProps): ReactNode {
  const { video } = clip;
  const kind = videoKind(video);
  const isShort = clip.type === OUTPUT_TYPE.short;
  const matchId = clip.match?.id ?? null;
  const playback = demoPlaybackItem(video);
  const canChangeMusic = isShort && video.recovered !== true;
  // An S card is too narrow for two labelled buttons and the menu on one row.
  const iconOnly = size === CLIP_SIZE.s;
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [musicOpen, setMusicOpen] = useState(false);
  const [versionsOpen, setVersionsOpen] = useState(false);
  const produceFormat = isShort ? PRODUCE_FORMAT.short : PRODUCE_FORMAT.full;
  // Editar reopens the producer with this video's options; a recovered render has no intent to edit.
  const editTarget = matchId !== null && video.variant !== undefined && video.recovered !== true ? { jobId: matchId, variant: video.variant } : null;
  // Crear horizontal: a 16:9 summary of a Short's kills; a death reel or Auto-Shorts has none.
  const horizontalFrom = video.editConfig?.format === 'short-9x16' && video.editConfig.deathReel !== true && video.editConfig.separateShorts !== true;
  const player = clip.match?.player ?? video.targetName;
  const title = videoDisplayTitle(video, player);
  const canReveal = playback !== null && video.jobId !== undefined;

  return (
    <article className={CARD_CLASS}>
      <CardStage
        size={size}
        kind={kind}
        format={isShort ? '9:16' : '16:9'}
        seed={video.id}
        map={video.map}
        posterUrl={video.thumbnailUrl}
        videoUrl={playback?.playbackUrl}
        playLabel={playback === null ? title : `Reproducir ${title}`}
        onPlay={playback === null ? null : (button) => onPlay(playback, button)}
      />
      <CardText
        size={size}
        title={title}
        meta={[prettifyMap(video.map), player, timeAgo(video.createdAt)].filter(Boolean).join(' · ')}
      />
      {video.changePending ? (
        <span className={cn('flex flex-col items-start gap-2', size === CLIP_SIZE.s ? 'px-3 pt-2.5' : 'px-4 pt-3')}>
          <span className="text-meta text-warning">{CHANGE_NOT_APPLIED_REASON}</span>
          <RetryButton video={video} onChange={onChange} />
        </span>
      ) : null}
      <span className={actionRowClass(size)}>
        {video.publishable === false ? null : (
          <Button asChild size={iconOnly ? 'icon-sm' : 'sm'}>
            <Link
              href={publishHref(matchId ?? ORPHAN_MATCH_SEGMENT, video.id)}
              aria-label={iconOnly ? PUBLISH_PREP_LABEL : undefined}
              title={PUBLISH_PREP_HINT}
            >
              <UploadCloud aria-hidden />
              {iconOnly ? null : PUBLISH_PREP_LABEL}
            </Link>
          </Button>
        )}
        <Button
          type="button"
          size={iconOnly ? 'icon-sm' : 'sm'}
          variant="outline"
          disabled={video.downloadUrl === undefined}
          aria-label={iconOnly ? 'Descargar MP4' : undefined}
          title={iconOnly ? 'Descargar MP4' : undefined}
          onClick={() => downloadVideoMP4(video)}
        >
          <Download aria-hidden />
          {iconOnly ? null : 'MP4'}
        </Button>
        <VisorButton title={title} onOpen={onOpenVisor} />
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button type="button" size="icon-sm" variant="ghost" className="ml-auto" aria-label={`Más acciones de ${title}`}>
              <Ellipsis aria-hidden />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            {playback === null || !canReveal ? null : (
              <DropdownMenuItem onSelect={() => void revealVideoFile(playback)}>
                <FolderOpen aria-hidden />
                Abrir carpeta
              </DropdownMenuItem>
            )}
            {canChangeMusic ? (
              <DropdownMenuItem onSelect={() => setMusicOpen(true)}>
                <Music aria-hidden />
                {video.songId ? 'Cambiar música' : 'Añadir música'}
              </DropdownMenuItem>
            ) : null}
            {editTarget ? (
              <>
                <DropdownMenuItem asChild>
                  <Link href={editVideoHref({ matchId: editTarget.jobId, format: produceFormat, variant: editTarget.variant })}>
                    <Pencil aria-hidden />
                    Editar
                  </Link>
                </DropdownMenuItem>
                <DropdownMenuItem onSelect={() => setVersionsOpen(true)}>
                  <History aria-hidden />
                  Versiones
                </DropdownMenuItem>
                {horizontalFrom ? (
                  <DropdownMenuItem asChild>
                    <Link href={horizontalHref({ jobId: editTarget.jobId, variant: editTarget.variant })}>
                      <RectangleHorizontal aria-hidden />
                      Crear horizontal
                    </Link>
                  </DropdownMenuItem>
                ) : null}
              </>
            ) : null}
            {matchId === null ? null : (
              <>
                <DropdownMenuItem asChild>
                  <Link href={matchHref(matchId)}>Ver estadísticas de la partida</Link>
                </DropdownMenuItem>
                <DropdownMenuItem asChild>
                  <Link
                    href={produceHref(matchId, produceFormat)}
                  >
                    {CREATE_AGAIN_LABEL[kind]}
                  </Link>
                </DropdownMenuItem>
              </>
            )}
            {canReveal || canChangeMusic || matchId !== null ? <DropdownMenuSeparator /> : null}
            <DropdownMenuItem variant="destructive" onSelect={() => setDeleteOpen(true)}>
              Borrar
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </span>
      <DeleteVideoDialog
        title={title}
        onConfirm={() => api.deleteVideo(video.id)}
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        onDeleted={onChange}
      />
      {canChangeMusic ? (
        <LibraryMusicDialog open={musicOpen} video={video} onOpenChange={setMusicOpen} onApplied={onChange} />
      ) : null}
      {editTarget ? (
        <VideoVersionsDialog open={versionsOpen} onOpenChange={setVersionsOpen} title={title} jobId={editTarget.jobId} variant={editTarget.variant} format={produceFormat} />
      ) : null}
    </article>
  );
}

export type StreamClipCardProps = {
  item: MediaPlaybackItem;
  size: ClipSize;
  onChange: () => void;
  onPlay: (button: HTMLElement) => void;
  onOpenVisor: (() => void) | null;
};

/** A rendered stream moment: Reproducir and MP4 up front, the project and Borrar behind "Más acciones". */
export function StreamClipCard({ item, size, onChange, onPlay, onOpenVisor }: StreamClipCardProps): ReactNode {
  const warning = item.review === PLAYBACK_REVIEW.stale;
  const isShort = item.format === '9:16';
  const iconOnly = size === CLIP_SIZE.s;
  const [deleteOpen, setDeleteOpen] = useState(false);
  // The server names a stream clip's MP4 after its clip id.
  const clipId = item.artifactName.replace(/\.mp4$/, '');
  return (
    <article className={cn(CARD_CLASS, warning && 'ring-1 ring-warning/45')}>
      <CardStage
        size={size}
        kind="stream"
        format={item.format}
        seed={item.id}
        posterUrl={item.posterUrl}
        durationSeconds={item.durationSeconds}
        badge={warning ? <StatusTag tone="warning">Desactualizado</StatusTag> : undefined}
        playLabel={`Reproducir ${item.title}`}
        onPlay={onPlay}
      />
      <CardText size={size} title={item.title} meta={isShort ? 'Short vertical' : 'Vídeo horizontal'} />
      <span className={actionRowClass(size)}>
        <Button
          type="button"
          size={iconOnly ? 'icon-sm' : 'sm'}
          variant="outline-primary"
          aria-label={iconOnly ? `Reproducir ${item.title}` : undefined}
          title={iconOnly ? 'Reproducir' : undefined}
          onClick={(event) => onPlay(event.currentTarget)}
        >
          <Play aria-hidden />
          {iconOnly ? null : 'Reproducir'}
        </Button>
        <Button
          type="button"
          size={iconOnly ? 'icon-sm' : 'sm'}
          variant="outline"
          aria-label={iconOnly ? 'Descargar MP4' : undefined}
          title={iconOnly ? 'Descargar MP4' : undefined}
          onClick={() => downloadMP4(item.playbackUrl, item.title)}
        >
          <Download aria-hidden />
          {iconOnly ? null : 'MP4'}
        </Button>
        <VisorButton title={item.title} onOpen={onOpenVisor} />
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button type="button" size="icon-sm" variant="ghost" className="ml-auto" aria-label={`Más acciones de ${item.title}`}>
              <Ellipsis aria-hidden />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem asChild>
              <Link href={`/streams/${item.jobId}`}>
                <FolderOpen aria-hidden />
                Abrir proyecto
              </Link>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onSelect={() => setDeleteOpen(true)}>
              Borrar
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </span>
      <DeleteVideoDialog
        title={item.title}
        onConfirm={() => streamsApi.deleteVideo(item.jobId, item.variant, clipId)}
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        onDeleted={onChange}
      />
    </article>
  );
}
