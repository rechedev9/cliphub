'use client';

import { useState, type CSSProperties, type ReactNode } from 'react';
import { ArrowRight } from 'lucide-react';
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
import { OutputActions } from '@/components/clips-hub/output-item';
import { OutputTag } from '@/components/clips-hub/output-tag';

type HubClip = HubModel['clips'][number];

const FILTER_LABEL = {
  all: 'Todos',
  short: 'Shorts',
  full: 'Full POV',
  ready: 'Listos',
  working: 'En marcha',
} as const satisfies Record<ClipFilter, string>;

const GRID_COLUMNS = {
  S: 'repeat(6, minmax(0, 1fr))',
  M: 'repeat(4, minmax(0, 1fr))',
  L: 'repeat(3, minmax(0, 1fr))',
} as const satisfies Record<ClipSize, string>;

const CHIP_CLASS =
  'inline-flex h-8 items-center border px-2.5 font-mono text-meta uppercase tracking-wider transition-colors duration-(--dur-fast) hover:text-fg-1 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring';

const CLIPS_EMPTY_COPY = 'Ningún clip con ese filtro. Los clips nacen desde una partida: abre una y clipea.';

export type ClipsLensProps = {
  clips: HubModel['clips'];
  onOpenMatch: (matchId: string) => void;
  onChange: () => void;
};

/** The Clips lens: every output as a card, filtered and sized locally. */
export function ClipsLens({ clips, onOpenMatch, onChange }: ClipsLensProps): ReactNode {
  const [filter, setFilter] = useState<ClipFilter>(CLIP_FILTER.all);
  const [size, setSize] = useState<ClipSize>(CLIP_SIZE.m);
  const counts = clipFilterCounts(clips);
  const visible = clips.filter((clip) => matchesClipFilter(clip, filter));
  const small = size === CLIP_SIZE.s;
  const gridStyle: CSSProperties = { gridTemplateColumns: GRID_COLUMNS[size] };

  return (
    <div className="studio-enter flex flex-col gap-3">
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
        <span className="ml-auto flex items-center gap-3 font-mono text-meta uppercase tracking-wider text-fg-3">
          <span className="hidden @[40rem]/content:inline">Guardados en este PC</span>
          <span className="flex items-center gap-2">
            Tamaño
            <span className="flex">
              {Object.values(CLIP_SIZE).map((key) => (
                <button
                  key={key}
                  type="button"
                  aria-pressed={size === key}
                  onClick={() => setSize(key)}
                  className={cn(
                    '-ml-px grid h-8 w-8 place-items-center border font-mono text-meta transition-colors duration-(--dur-fast) first:ml-0 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring',
                    size === key ? 'border-primary bg-primary text-primary-foreground' : 'border-border-strong text-fg-3 hover:text-fg-1',
                  )}
                >
                  {key}
                </button>
              ))}
            </span>
          </span>
        </span>
      </div>

      {visible.length === 0 ? (
        <p className="rounded-[10px] border border-dashed border-border-subtle p-8 text-center text-body-sm text-fg-2">
          {CLIPS_EMPTY_COPY}
        </p>
      ) : (
        <section aria-label="Clips" className={cn('grid', small ? 'gap-2' : 'gap-3')} style={gridStyle}>
          {visible.map((clip) => (
            <ClipCard key={clip.id} clip={clip} small={small} onOpenMatch={onOpenMatch} onChange={onChange} />
          ))}
        </section>
      )}
    </div>
  );
}

function ClipCard({
  clip,
  small,
  onOpenMatch,
  onChange,
}: {
  clip: HubClip;
  small: boolean;
  onOpenMatch: (matchId: string) => void;
  onChange: () => void;
}): ReactNode {
  const isShort = clip.type === OUTPUT_TYPE.short;
  const { video } = clip;
  const player = clip.match?.player ?? video.targetName ?? '—';
  const sub = isShort ? 'Short' : 'Full POV';
  const matchId = clip.match?.id ?? null;

  return (
    <article className={cn('studio-panel studio-enter flex flex-col rounded-[10px]', small ? 'gap-1.5 p-2' : 'gap-2.5 p-3')}>
      <span className={cn('relative', isShort && 'w-[54%] self-center')}>
        <MediaFrame
          aspect={isShort ? '9:16' : '16:9'}
          className="border border-border"
          fallback={<ReelCover seed={video.id} plain />}
          media={video.thumbnailUrl === undefined ? undefined : <CoverImage src={video.thumbnailUrl} />}
        />
        <span className="absolute top-2 left-2">
          <OutputTag output={clip} />
        </span>
        {isWorking(clip.state) ? (
          <span className={cn('studio-bar absolute inset-x-0 bottom-0', clip.state === OUTPUT_STATE.rec ? 'text-stream-text' : 'text-primary')}>
            <span
              className={clip.percent === null ? 'studio-indeterminate' : undefined}
              style={clip.percent === null ? undefined : { width: `${clip.percent}%` }}
            />
          </span>
        ) : null}
      </span>

      <span className="flex min-w-0 flex-col gap-0.5">
        <span className="truncate font-display text-body-sm font-bold uppercase text-fg-1">{clip.title}</span>
        <span className="truncate font-mono text-meta uppercase tracking-wider text-fg-3">
          {video.map} · {player} · {sub}
        </span>
      </span>

      {small ? null : (
        <span className="flex flex-wrap items-center gap-1.5">
          <OutputActions output={clip} matchId={matchId ?? ORPHAN_MATCH_SEGMENT} onChange={onChange} />
          {matchId !== null ? (
            <button
              type="button"
              onClick={() => onOpenMatch(matchId)}
              className="ml-auto inline-flex h-8 items-center gap-1 px-1 font-mono text-meta uppercase tracking-wider text-fg-3 transition-colors duration-(--dur-fast) hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
            >
              Partida
              <ArrowRight aria-hidden className="size-3" />
            </button>
          ) : null}
        </span>
      )}
    </article>
  );
}
