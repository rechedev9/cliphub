'use client';

import { useMemo, useState, type CSSProperties, type ReactNode } from 'react';
import Link from 'next/link';
import { AlertTriangle, ArrowRight, Play } from 'lucide-react';
import { PLAYBACK_REVIEW, demoPlaybackItem, streamPlaybackItem, type MediaPlaybackItem } from '@/lib/api/playback';
import type { StreamJob } from '@/lib/api/streams';
import {
  CLIP_FILTER,
  CLIP_SIZE,
  clipFilterCounts,
  isWorking,
  matchesClipFilter,
  OUTPUT_STATE,
  OUTPUT_TYPE,
  type ClipFilter,
  type ClipSize,
  type HubModel,
} from '@/lib/clips/hub';
import { ORPHAN_MATCH_SEGMENT } from '@/lib/clips/routes';
import { cn } from '@/lib/utils';
import { ReelCover } from '@/components/brand/reel-cover';
import { CoverImage } from '@/components/studio/cover-image';
import { MediaFrame } from '@/components/studio/media-frame';
import { MediaPlayer } from '@/components/studio/media-player';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { OutputActions } from '@/components/clips-hub/output-item';
import { OutputTag } from '@/components/clips-hub/output-tag';

type HubClip = HubModel['clips'][number];

const FILTER_LABEL = {
  all: 'Todos',
  short: 'Shorts',
  full: 'Vídeos largos',
  ready: 'Listos',
  working: 'En marcha',
} as const satisfies Record<ClipFilter, string>;

/** A floor per density step, never a track count: two clips must not leave half the measure empty. */
const GRID_COLUMNS = {
  S: 'repeat(auto-fill, minmax(9.5rem, 1fr))',
  M: 'repeat(auto-fill, minmax(14rem, 1fr))',
  L: 'repeat(auto-fill, minmax(19rem, 1fr))',
} as const satisfies Record<ClipSize, string>;

const CHIP_CLASS =
  'inline-flex h-10 items-center border px-3 font-mono text-meta uppercase tracking-wider transition-colors duration-(--dur-fast) hover:text-fg-1 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring';

const CLIPS_EMPTY_COPY = 'Ningún vídeo con ese filtro. Cambia el filtro o crea un clip desde una partida o un stream.';

const OPEN_MATCH_LABEL = 'Partida';
const OPEN_MATCH_ARIA = 'Abrir la partida';

export type ClipsLensProps = {
  clips: HubModel['clips'];
  streams: readonly StreamJob[];
  onOpenMatch: (matchId: string) => void;
  onChange: () => void;
};

/** The Clips lens: every output as a card, filtered and sized locally. */
export function ClipsLens({ clips, streams, onOpenMatch, onChange }: ClipsLensProps): ReactNode {
  const [filter, setFilter] = useState<ClipFilter>(CLIP_FILTER.all);
  const [size, setSize] = useState<ClipSize>(CLIP_SIZE.m);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [playerOpen, setPlayerOpen] = useState(false);
  const [returnFocus, setReturnFocus] = useState<HTMLElement | null>(null);
  const streamItems = useMemo(
    () => streams.flatMap((job) => (job.rendered_outputs ?? []).map((output) => streamPlaybackItem(job, output)).filter((item): item is MediaPlaybackItem => item !== null)),
    [streams],
  );
  const unavailableStreamOutputs = streams.filter((job) => job.rendered_outputs_unavailable === true).length;
  const counts = clipFilterCounts(clips);
  for (const streamItem of streamItems) {
    counts.all += 1;
    if (streamItem.format === '9:16') counts.short += 1;
    if (streamItem.format === '16:9') counts.full += 1;
    if (streamItem.review === PLAYBACK_REVIEW.ready) counts.ready += 1;
  }
  const visible = clips.filter((clip) => matchesClipFilter(clip, filter));
  const visibleStreams = streamItems.filter((streamItem) => {
    if (filter === CLIP_FILTER.all) return true;
    if (filter === CLIP_FILTER.short) return streamItem.format === '9:16';
    if (filter === CLIP_FILTER.full) return streamItem.format === '16:9';
    if (filter === CLIP_FILTER.ready) return streamItem.review === PLAYBACK_REVIEW.ready;
    return false;
  });
  const playable = [
    ...visible.map((clip) => demoPlaybackItem(clip.video)).filter((item): item is MediaPlaybackItem => item !== null),
    ...visibleStreams,
  ];
  const small = size === CLIP_SIZE.s;
  const gridStyle: CSSProperties = { gridTemplateColumns: GRID_COLUMNS[size] };

  return (
    <div className="flex flex-col gap-3">
      {unavailableStreamOutputs > 0 ? (
        <div className="flex items-start gap-2 border border-warning/45 bg-warning/10 px-3 py-2 text-body-sm text-warning" role="status">
          <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
          <span>
            No se pudo cargar el historial de vídeos de {unavailableStreamOutputs === 1 ? 'un stream' : `${unavailableStreamOutputs} streams`}.
          </span>
        </div>
      ) : null}
      <div className="flex flex-wrap items-center gap-2">
        {Object.values(CLIP_FILTER).map((key) => (
          <button
            key={key}
            type="button"
            aria-pressed={filter === key}
            onClick={() => setFilter(key)}
            className={cn(CHIP_CLASS, filter === key ? 'border-primary text-primary' : 'border-border-strong text-fg-3')}
          >
            {FILTER_LABEL[key]} · {counts[key]}
          </button>
        ))}
        <span className="ml-auto flex items-center gap-2 font-mono text-meta uppercase tracking-wider text-fg-3">
          Tamaño
          <span className="flex">
            {Object.values(CLIP_SIZE).map((key) => (
              <button
                key={key}
                type="button"
                aria-pressed={size === key}
                onClick={() => setSize(key)}
                className={cn(
                  '-ml-px grid size-10 place-items-center border font-mono text-meta transition-colors duration-(--dur-fast) first:ml-0 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring',
                  size === key ? 'border-primary bg-primary text-primary-foreground' : 'border-border-strong text-fg-3 hover:text-fg-1',
                )}
              >
                {key}
              </button>
            ))}
          </span>
        </span>
      </div>

      {visible.length === 0 && visibleStreams.length === 0 ? (
        <p className="rounded-[10px] border border-dashed border-border-subtle p-8 text-center text-body-sm text-fg-2">
          {CLIPS_EMPTY_COPY}
        </p>
      ) : (
        <section
          aria-label="Clips"
          /* items-start, not the default stretch: a 9:16 card is ~200px taller
             than its 16:9 neighbour, and stretching the short one just prints
             an empty box under its controls. Cards end where their content
             ends. */
          className={cn('grid items-start', small ? 'gap-2' : 'gap-3')}
          style={gridStyle}
        >
          {visible.map((clip) => (
            <ClipCard
              key={clip.id}
              clip={clip}
              small={small}
              onOpenMatch={onOpenMatch}
              onChange={onChange}
              onPlay={(item, button) => {
                setActiveId(item.id);
                setReturnFocus(button);
                setPlayerOpen(true);
              }}
            />
          ))}
          {visibleStreams.map((item) => (
            <StreamClipCard
              key={item.id}
              item={item}
              small={small}
              onPlay={(button) => {
                setActiveId(item.id);
                setReturnFocus(button);
                setPlayerOpen(true);
              }}
            />
          ))}
        </section>
      )}
      <MediaPlayer
        items={playable}
        activeId={activeId}
        open={playerOpen}
        onActiveChange={setActiveId}
        onOpenChange={setPlayerOpen}
        returnFocus={returnFocus}
      />
    </div>
  );
}

function ClipCard({
  clip,
  small,
  onOpenMatch,
  onChange,
  onPlay,
}: {
  clip: HubClip;
  small: boolean;
  onOpenMatch: (matchId: string) => void;
  onChange: () => void;
  onPlay: (item: MediaPlaybackItem, button: HTMLElement) => void;
}): ReactNode {
  const isShort = clip.type === OUTPUT_TYPE.short;
  const { video } = clip;
  const player = clip.match?.player ?? video.targetName ?? '—';
  const sub = isShort ? 'Short · 9:16' : 'Vídeo largo · 16:9';
  const matchId = clip.match?.id ?? null;
  const playback = demoPlaybackItem(video);

  return (
    <article className={cn('studio-panel studio-enter flex flex-col rounded-[10px]', small ? 'gap-1.5 p-2' : 'gap-2.5 p-3')}>
      {/* A true 9:16 frame, narrowed rather than capped. `capHeight` pins the
          height of a full-width box, so the frame stops being 9:16 and
          object-cover eats a third of the reel — a Shorts tool must not show a
          reel it did not make (media-frame.tsx). Constraining the width keeps
          the shape honest and still lands the card near its landscape row. */}
      <button
        type="button"
        disabled={playback === null}
        aria-label={playback === null ? clip.title : `Reproducir ${clip.title}`}
        onClick={(event) => {
          if (playback) onPlay(playback, event.currentTarget);
        }}
        className={cn('relative text-left focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:cursor-default', isShort && 'w-[54%] self-center')}
      >
        <MediaFrame
          aspect={isShort ? '9:16' : '16:9'}
          badge={<OutputTag output={clip} />}
          className="border border-border"
          fallback={<ReelCover seed={video.id} plain />}
          media={video.thumbnailUrl === undefined ? undefined : <CoverImage src={video.thumbnailUrl} />}
        />
        {isWorking(clip.state) ? (
          <span className={cn('studio-bar absolute inset-x-0 bottom-0', clip.state === OUTPUT_STATE.rec ? 'text-stream-text' : 'text-primary')}>
            <span
              className={clip.percent === null ? 'studio-indeterminate' : undefined}
              style={clip.percent === null ? undefined : { width: `${clip.percent}%` }}
            />
          </span>
        ) : null}
      </button>

      <span className="flex min-w-0 flex-col gap-0.5">
        <span className="truncate font-display text-body-sm font-bold uppercase text-fg-1">{clip.title}</span>
        <span className="truncate font-mono text-meta uppercase tracking-wider text-fg-3">
          {video.map} · {player} · {sub}
        </span>
      </span>

      {/* S is the contact sheet: a 152px track cannot hold three controls
          without wrapping them into more chrome than thumbnail, which is the
          opposite of what the density step was asked for. */}
      {small ? null : (
        <span className="flex flex-wrap items-center gap-1.5">
          <OutputActions
            output={clip}
            matchId={matchId ?? ORPHAN_MATCH_SEGMENT}
            onChange={onChange}
            onPlay={playback === null ? undefined : (button) => onPlay(playback, button)}
          />
          {matchId !== null ? (
            <button
              type="button"
              aria-label={OPEN_MATCH_ARIA}
              onClick={() => onOpenMatch(matchId)}
              className="ml-auto inline-flex h-10 items-center gap-1 px-2 font-mono text-meta uppercase tracking-wider text-fg-3 transition-colors duration-(--dur-fast) hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
            >
              {OPEN_MATCH_LABEL}
              <ArrowRight aria-hidden className="size-3" />
            </button>
          ) : null}
        </span>
      )}
    </article>
  );
}

function StreamClipCard({ item, small, onPlay }: { item: MediaPlaybackItem; small: boolean; onPlay: (button: HTMLElement) => void }): ReactNode {
  const warning = item.review !== PLAYBACK_REVIEW.ready;
  const isShort = item.format === '9:16';
  let reviewLabel = 'Stream';
  if (item.review === PLAYBACK_REVIEW.stale) reviewLabel = 'Desactualizado';
  else if (item.review === PLAYBACK_REVIEW.pending) reviewLabel = 'Revisión QA';
  return (
    <article className={cn('studio-panel studio-enter flex flex-col rounded-[10px]', small ? 'gap-1.5 p-2' : 'gap-2.5 p-3', warning && 'border-warning/45')}>
      <button
        type="button"
        aria-label={`Reproducir ${item.title}`}
        onClick={(event) => onPlay(event.currentTarget)}
        className={cn('relative text-left focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring', isShort && 'w-[54%] self-center')}
      >
        <MediaFrame
          aspect={item.format}
          badge={<StatusTag tone={warning ? 'warning' : 'success'}>{reviewLabel}</StatusTag>}
          className="border border-border"
          fallback={<ReelCover seed={item.id} plain />}
          media={item.posterUrl ? <CoverImage src={item.posterUrl} /> : undefined}
        />
      </button>
      <span className="flex min-w-0 flex-col gap-0.5">
        <span className="truncate font-display text-body-sm font-bold uppercase text-fg-1">{item.title}</span>
        <span className="truncate font-mono text-meta uppercase tracking-wider text-fg-3">Stream · {isShort ? 'Short' : 'Full POV'}</span>
      </span>
      {small ? null : (
        <span className="flex flex-wrap items-center gap-1.5">
          <Button type="button" size="xs" variant="outline-primary" onClick={(event) => onPlay(event.currentTarget)}>
            <Play aria-hidden /> Reproducir
          </Button>
          <Button asChild size="xs" variant="outline" className="ml-auto">
            <Link href={`/streams/${item.jobId}`}>Proyecto</Link>
          </Button>
        </span>
      )}
    </article>
  );
}
