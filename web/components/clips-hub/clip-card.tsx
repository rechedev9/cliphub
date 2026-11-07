'use client';

import { useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Download, Ellipsis, FolderOpen, Music, Play, UploadCloud } from 'lucide-react';
import { prettifyMap } from '@/lib/api/map';
import { PLAYBACK_REVIEW, demoPlaybackItem, type MediaPlaybackItem, type PlaybackFormat } from '@/lib/api/playback';
import { CLIP_SIZE, OUTPUT_TYPE, type ClipSize, type HubModel } from '@/lib/clips/hub';
import { videoKind, type VideoKind } from '@/lib/clips/results';
import { ORPHAN_MATCH_SEGMENT, PRODUCE_FORMAT, matchHref, produceHref, publishHref } from '@/lib/clips/routes';
import { timeAgo } from '@/lib/format';
import { cn } from '@/lib/utils';
import { ReelCover } from '@/components/brand/reel-cover';
import { CoverImage } from '@/components/studio/cover-image';
import { FORMAT_TONE } from '@/components/studio/format-tone';
import { MediaFrame } from '@/components/studio/media-frame';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { DeleteVideoDialog } from '@/components/videos/delete-video-button';
import { LibraryMusicDialog } from '@/components/videos/library-music-dialog';
import { downloadMP4, downloadVideoMP4 } from '@/components/clips-hub/output-item';

type HubClip = HubModel['clips'][number];

const KIND_LABEL = {
  short: 'Short',
  full: 'Demo completa',
  summary: 'Resumen',
  stream: 'Clip de stream',
} as const satisfies Record<VideoKind, string>;

const CREATE_AGAIN_LABEL = {
  short: 'Crear otro Short',
  full: 'Crear otra demo completa',
  summary: 'Crear otro resumen',
  stream: 'Crear otro clip',
} as const satisfies Record<VideoKind, string>;

// FORMAT_TONE.fill carries a hover for buttons; a static pill only needs the colour and its text.
const PILL_CLASS = {
  short: 'bg-format-short text-format-short-foreground',
  full: 'bg-format-full text-format-full-foreground',
  summary: 'bg-format-summary text-format-summary-foreground',
  stream: 'bg-format-stream text-format-stream-foreground',
} as const satisfies Record<VideoKind, string>;

const FOCUS_RING = 'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring';

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
  posterUrl: string | undefined;
  durationSeconds?: number;
  badge?: ReactNode;
  playLabel: string;
  onPlay: ((button: HTMLElement) => void) | null;
};

/** The tinted media area: the real poster in its own frame, the format pill and the length. */
function CardStage({ size, kind, format, seed, posterUrl, durationSeconds, badge, playLabel, onPlay }: CardStageProps): ReactNode {
  const tall = format === '9:16';
  const small = size === CLIP_SIZE.s;
  return (
    <div className={cn('relative flex items-center justify-center', STAGE_CLASS[size].stage, FORMAT_TONE[kind].tint)}>
      <button
        type="button"
        disabled={onPlay === null}
        aria-label={playLabel}
        onClick={(event) => onPlay?.(event.currentTarget)}
        className={cn('block rounded-[10px] disabled:cursor-default', FOCUS_RING, tall ? STAGE_CLASS[size].tall : STAGE_CLASS[size].wide)}
      >
        <MediaFrame
          aspect={format}
          badge={badge}
          className="rounded-[10px]"
          fallback={<ReelCover seed={seed} plain />}
          media={posterUrl === undefined ? undefined : <CoverImage src={posterUrl} />}
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

const CARD_CLASS = 'flex flex-col overflow-hidden rounded-[18px] bg-surface-2';

function actionRowClass(size: ClipSize): string {
  return cn('flex flex-wrap items-center gap-1.5', size === CLIP_SIZE.s ? 'px-3 pt-2.5 pb-3' : 'px-4 pt-3 pb-4');
}

export type ClipCardProps = {
  clip: HubClip;
  size: ClipSize;
  onChange: () => void;
  onPlay: (item: MediaPlaybackItem, button: HTMLElement) => void;
};

/** A ready demo video: Publicar and MP4 up front, the rest behind the "Más acciones" menu. */
export function ClipCard({ clip, size, onChange, onPlay }: ClipCardProps): ReactNode {
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

  return (
    <article className={CARD_CLASS}>
      <CardStage
        size={size}
        kind={kind}
        format={isShort ? '9:16' : '16:9'}
        seed={video.id}
        posterUrl={video.thumbnailUrl}
        playLabel={playback === null ? clip.title : `Reproducir ${clip.title}`}
        onPlay={playback === null ? null : (button) => onPlay(playback, button)}
      />
      <CardText
        size={size}
        title={clip.title}
        meta={[prettifyMap(video.map), clip.match?.player ?? video.targetName, timeAgo(video.createdAt)].filter(Boolean).join(' · ')}
      />
      <span className={actionRowClass(size)}>
        {video.publishable === false ? null : (
          <Button asChild size={iconOnly ? 'icon-sm' : 'sm'}>
            <Link
              href={publishHref(matchId ?? ORPHAN_MATCH_SEGMENT, video.id)}
              aria-label={iconOnly ? 'Publicar' : undefined}
              title={iconOnly ? 'Publicar' : undefined}
            >
              <UploadCloud aria-hidden />
              {iconOnly ? null : 'Publicar'}
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
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button type="button" size="icon-sm" variant="ghost" className="ml-auto" aria-label={`Más acciones de ${clip.title}`}>
              <Ellipsis aria-hidden />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            {canChangeMusic ? (
              <DropdownMenuItem onSelect={() => setMusicOpen(true)}>
                <Music aria-hidden />
                {video.songId ? 'Cambiar música' : 'Añadir música'}
              </DropdownMenuItem>
            ) : null}
            {matchId === null ? null : (
              <>
                <DropdownMenuItem asChild>
                  <Link href={matchHref(matchId)}>Ver estadísticas de la partida</Link>
                </DropdownMenuItem>
                <DropdownMenuItem asChild>
                  <Link
                    href={produceHref(matchId, isShort ? PRODUCE_FORMAT.short : PRODUCE_FORMAT.full)}
                  >
                    {CREATE_AGAIN_LABEL[kind]}
                  </Link>
                </DropdownMenuItem>
              </>
            )}
            {canChangeMusic || matchId !== null ? <DropdownMenuSeparator /> : null}
            <DropdownMenuItem variant="destructive" onSelect={() => setDeleteOpen(true)}>
              Borrar
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </span>
      <DeleteVideoDialog video={video} open={deleteOpen} onOpenChange={setDeleteOpen} onDeleted={onChange} />
      {canChangeMusic ? (
        <LibraryMusicDialog open={musicOpen} video={video} onOpenChange={setMusicOpen} onApplied={onChange} />
      ) : null}
    </article>
  );
}

export type StreamClipCardProps = {
  item: MediaPlaybackItem;
  size: ClipSize;
  onPlay: (button: HTMLElement) => void;
};

export function StreamClipCard({ item, size, onPlay }: StreamClipCardProps): ReactNode {
  const warning = item.review === PLAYBACK_REVIEW.stale;
  const isShort = item.format === '9:16';
  const iconOnly = size === CLIP_SIZE.s;
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
      <CardText size={size} title={item.title} meta={`Clip de stream · ${isShort ? 'Short' : 'Horizontal'}`} />
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
        <Button asChild size={iconOnly ? 'icon-sm' : 'sm'} variant="outline" className="ml-auto">
          <Link
            href={`/streams/${item.jobId}`}
            aria-label={iconOnly ? 'Abrir proyecto' : undefined}
            title={iconOnly ? 'Abrir proyecto' : undefined}
          >
            {iconOnly ? <FolderOpen aria-hidden /> : 'Proyecto'}
          </Link>
        </Button>
      </span>
    </article>
  );
}
