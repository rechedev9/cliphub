'use client';

import { useMemo, useState, type ReactNode } from 'react';
import { AlertTriangle } from 'lucide-react';
import { demoPlaybackItem, streamPlaybackItem, type MediaPlaybackItem } from '@/lib/api/playback';
import type { StreamJob } from '@/lib/api/streams';
import { CLIP_SIZE, OUTPUT_STATE, type ClipSize, type HubModel } from '@/lib/clips/hub';
import {
  LIBRARY_FILTER,
  LIBRARY_STAGE,
  itemStage,
  libraryChipCounts,
  matchesFormat,
  streamActivity,
  videoKind,
  type LibraryFilter,
  type LibraryItem,
  type LibraryStage,
} from '@/lib/clips/results';
import { cn } from '@/lib/utils';
import { FORMAT_TONE } from '@/components/studio/format-tone';
import { MediaPlayer } from '@/components/studio/media-player';
import { FOCUS_RING } from '@/components/ui/button';
import { ClipCard, StreamClipCard } from '@/components/clips-hub/clip-card';
import { NowSection } from '@/components/clips-hub/now-section';

type HubClip = HubModel['clips'][number];

const FILTER_LABEL = {
  all: 'Todos',
  short: 'Shorts',
  full: 'Demos completas',
  summary: 'Resúmenes',
  stream: 'Clips de stream',
} as const satisfies Record<LibraryFilter, string>;

const STAGE_LABEL = {
  all: 'Todo',
  ready: 'Listos',
  working: 'En marcha',
} as const satisfies Record<LibraryStage, string>;

const SIZE_TITLE = {
  S: 'Tarjetas pequeñas',
  M: 'Tarjetas medianas',
  L: 'Tarjetas grandes',
} as const satisfies Record<ClipSize, string>;

// A floor per size, never a track count: two cards must not leave half the row empty.
const GRID_CLASS = {
  S: 'grid-cols-[repeat(auto-fill,minmax(12rem,1fr))] gap-3',
  M: 'grid-cols-[repeat(auto-fill,minmax(17rem,1fr))] gap-4.5',
  L: 'grid-cols-[repeat(auto-fill,minmax(23rem,1fr))] gap-4.5',
} as const satisfies Record<ClipSize, string>;


const CHIP_CLASS = cn(
  'inline-flex h-10 items-center gap-2 rounded-full border px-3.5 text-body-sm transition-colors duration-(--dur-fast)',
  FOCUS_RING,
);

const EMPTY_CLASS = 'rounded-[10px] border border-dashed border-border-subtle p-8 text-center text-body-sm text-fg-2';

const READY_HEADING_ID = 'mis-videos-listos';
const SIZE_LABEL_ID = 'mis-videos-tamano';

export type ClipsLensProps = {
  clips: HubModel['clips'];
  streams: readonly StreamJob[];
  onChange: () => void;
};

function chipClass(active: boolean): string {
  return cn(CHIP_CLASS, active ? 'border-fg-1 bg-fg-1 font-bold text-background' : 'border-border text-fg-2 hover:text-fg-1');
}

/** Every video recorded on this PC: what is in progress on top, the ready ones as a filtered grid. */
export function ClipsLens({ clips, streams, onChange }: ClipsLensProps): ReactNode {
  const [format, setFormat] = useState<LibraryFilter>(LIBRARY_FILTER.all);
  const [stage, setStage] = useState<LibraryStage>(LIBRARY_STAGE.all);
  const [size, setSize] = useState<ClipSize>(CLIP_SIZE.m);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [playerOpen, setPlayerOpen] = useState(false);
  const [returnFocus, setReturnFocus] = useState<HTMLElement | null>(null);
  const streamItems = useMemo(
    () => streams.flatMap((job) => (job.rendered_outputs ?? []).map((output) => streamPlaybackItem(job, output)).filter((item): item is MediaPlaybackItem => item !== null)),
    [streams],
  );
  const activity = useMemo(() => streamActivity(streams), [streams]);
  const unavailableStreamOutputs = streams.filter((job) => job.rendered_outputs_unavailable === true).length;

  const items: LibraryItem[] = [
    ...clips.map((clip): LibraryItem => ({ kind: videoKind(clip.video), stage: itemStage(clip.state) })),
    ...streamItems.map((): LibraryItem => ({ kind: 'stream', stage: 'ready' })),
    ...activity.map((): LibraryItem => ({ kind: 'stream', stage: 'working' })),
  ];
  const counts = libraryChipCounts(items, { format, stage });
  const readyTotal = items.filter((item) => item.stage === 'ready').length;

  const inFormat = clips.filter((clip) => matchesFormat(videoKind(clip.video), format));
  const byState = (state: HubClip['state']): HubClip[] => inFormat.filter((clip) => clip.state === state);
  const running = inFormat.filter((clip) => clip.state === OUTPUT_STATE.rec || clip.state === OUTPUT_STATE.render);
  // Capture first: the hero is the clip that holds CS2 right now.
  running.sort((a, b) => Number(b.state === OUTPUT_STATE.rec) - Number(a.state === OUTPUT_STATE.rec));
  const queued = byState(OUTPUT_STATE.queue);
  const streamsAtWork = matchesFormat('stream', format) ? activity : [];
  const showNow = stage !== LIBRARY_STAGE.ready;
  const showReady = stage !== LIBRARY_STAGE.working;
  // A failed video is not "En marcha": that chip keeps only the work that still moves.
  const failed = stage === LIBRARY_STAGE.all ? byState(OUTPUT_STATE.failed) : [];
  const nothingNow = running.length + queued.length + streamsAtWork.length + failed.length === 0;

  const visible = showReady ? byState(OUTPUT_STATE.ready) : [];
  const visibleStreams = showReady && matchesFormat('stream', format) ? streamItems : [];
  const playable = [
    ...visible.map((clip) => demoPlaybackItem(clip.video)).filter((item): item is MediaPlaybackItem => item !== null),
    ...visibleStreams,
  ];
  const play = (id: string, button: HTMLElement): void => {
    setActiveId(id);
    setReturnFocus(button);
    setPlayerOpen(true);
  };

  return (
    <div className="flex flex-col gap-8">
      {unavailableStreamOutputs > 0 ? (
        <div className="flex items-start gap-2 border border-warning/45 bg-warning/10 px-3 py-2 text-body-sm text-warning" role="status">
          <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
          <span>
            No se pudo cargar el historial de vídeos de {unavailableStreamOutputs === 1 ? 'un stream' : `${unavailableStreamOutputs} streams`}.
          </span>
        </div>
      ) : null}

      <div className="flex flex-wrap items-center gap-2.5">
        <div role="group" aria-label="Tipo de vídeo" className="flex flex-wrap gap-1.5">
          {Object.values(LIBRARY_FILTER).map((key) => (
            <button key={key} type="button" aria-pressed={format === key} onClick={() => setFormat(key)} className={chipClass(format === key)}>
              <span aria-hidden className={cn('size-2.5 rounded-[3px]', key === LIBRARY_FILTER.all ? 'bg-fg-3' : FORMAT_TONE[key].bg)} />
              {FILTER_LABEL[key]}
              <span className="opacity-70">{counts.format[key]}</span>
            </button>
          ))}
        </div>
        <span aria-hidden className="mx-1 h-6 w-px bg-border" />
        <div role="group" aria-label="Estado" className="flex flex-wrap gap-1.5">
          {Object.values(LIBRARY_STAGE).map((key) => (
            <button key={key} type="button" aria-pressed={stage === key} onClick={() => setStage(key)} className={chipClass(stage === key)}>
              {STAGE_LABEL[key]}
              <span className="opacity-70">{counts.stage[key]}</span>
            </button>
          ))}
        </div>
        <div className="ml-auto flex items-center gap-2">
          <span id={SIZE_LABEL_ID} className="text-meta text-fg-3">
            Tamaño
          </span>
          <div role="group" aria-labelledby={SIZE_LABEL_ID} className="inline-flex h-10 rounded-full border border-border p-1">
            {Object.values(CLIP_SIZE).map((key) => (
              <button
                key={key}
                type="button"
                aria-pressed={size === key}
                title={SIZE_TITLE[key]}
                onClick={() => setSize(key)}
                className={cn(
                  'min-w-8 rounded-full px-2.5 text-body-sm transition-colors duration-(--dur-fast)',
                  FOCUS_RING,
                  size === key ? 'bg-fg-1 font-bold text-background' : 'text-fg-2 hover:text-fg-1',
                )}
              >
                {key}
              </button>
            ))}
          </div>
        </div>
      </div>

      {showNow ? (
        <NowSection running={running} queued={queued} failed={failed} streams={streamsAtWork} onChange={onChange} />
      ) : null}
      {showNow && !showReady && nothingNow ? (
        <p className={EMPTY_CLASS}>{format === LIBRARY_FILTER.all ? 'Nada en marcha ahora mismo.' : 'Nada en marcha con ese filtro.'}</p>
      ) : null}

      {showReady ? (
        <section aria-labelledby={READY_HEADING_ID} className="flex flex-col gap-4">
          <h2 id={READY_HEADING_ID} className="font-display text-title font-bold text-fg-1">
            Listos
          </h2>
          {visible.length === 0 && visibleStreams.length === 0 ? (
            <p className={EMPTY_CLASS}>
              {readyTotal === 0 ? 'Aún no hay vídeos listos. Aparecerán aquí en cuanto terminen.' : 'Ningún vídeo con ese filtro.'}
            </p>
          ) : (
            <div className={cn('grid', GRID_CLASS[size])}>
              {visible.map((clip) => (
                <ClipCard key={clip.id} clip={clip} size={size} onChange={onChange} onPlay={(item, button) => play(item.id, button)} />
              ))}
              {visibleStreams.map((item) => (
                <StreamClipCard key={item.id} item={item} size={size} onPlay={(button) => play(item.id, button)} />
              ))}
            </div>
          )}
        </section>
      ) : null}
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
