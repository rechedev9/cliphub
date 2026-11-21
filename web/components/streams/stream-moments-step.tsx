'use client';

import type { ReactNode } from 'react';
import { Flag, Play } from 'lucide-react';
import type { StreamClipRange, StreamEditPlan, StreamJob, StreamPlay } from '@/lib/api/streams';
import { momentLengthLabel, momentsInOrder } from '@/lib/streams/moments';
import { fullVideoMoment } from '@/lib/streams/mark-moment';
import { clipOutputDuration, formatStreamClock } from '@/lib/streams/plan';
import { cn } from '@/lib/utils';
import { StreamClipEditor } from '@/components/streams/clip-editor';
import { StreamDetectedPlays } from '@/components/streams/detected-plays';
import type { StreamMoments } from '@/components/streams/use-stream-moments';
import { FORMAT_TONE } from '@/components/studio/format-tone';
import { Button } from '@/components/ui/button';

function MomentsStrip({
  clips,
  selectedClipId,
  disabled,
  vod,
  onPlay,
  onPlayAll,
}: {
  clips: readonly StreamClipRange[];
  selectedClipId: string | null;
  disabled: boolean;
  vod: boolean;
  onPlay: (clip: StreamClipRange) => void;
  onPlayAll: () => void;
}): ReactNode {
  const moments = momentsInOrder(clips);
  return (
    <section aria-label="Tus momentos" className="flex min-w-0 flex-col gap-3 rounded-md border border-border-subtle bg-surface-1 p-3">
      <div className="flex flex-wrap items-center gap-2">
        <h3 className="font-semibold">
          Tus momentos<span className="ml-1.5 text-fg-3 tabular-nums">{moments.length}</span>
        </h3>
        {moments.length > 1 ? (
          <Button variant="outline" size="sm" className="ml-auto" disabled={disabled} onClick={onPlayAll}>
            <Play aria-hidden />
            Ver todos seguidos
          </Button>
        ) : null}
      </div>
      {moments.length ? (
        <ul className="grid min-w-0 grid-cols-[repeat(auto-fill,minmax(11rem,1fr))] gap-2">
          {moments.map(({ clip, number }) => {
            const selected = clip.id === selectedClipId;
            return (
              <li key={clip.id} className="min-w-0">
                <button
                  type="button"
                  disabled={disabled}
                  aria-pressed={selected}
                  onClick={() => onPlay(clip)}
                  className={cn(
                    'flex w-full min-w-0 items-center gap-2 rounded border px-2.5 py-2 text-left text-label',
                    selected ? 'border-stream bg-stream/10' : 'border-border-subtle hover:border-border-strong',
                  )}
                >
                  <span aria-hidden className={cn('grid h-6 min-w-6 shrink-0 place-items-center rounded-md px-1 text-meta font-extrabold tabular-nums text-format-stream-foreground', FORMAT_TONE.stream.bg)}>
                    {number}
                  </span>
                  <span className="flex min-w-0 flex-col">
                    <span className="truncate font-semibold text-fg-1">
                      <span className="sr-only">{number}. </span>
                      {clip.title?.trim() || `Momento ${number}`}
                    </span>
                    <span className="text-meta tabular-nums text-fg-3">
                      {formatStreamClock(clip.start_seconds)} · {momentLengthLabel(clipOutputDuration(clip))}
                    </span>
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      ) : (
        <p className="text-body-sm text-fg-3">
          Aquí irá lo que te quedes.{' '}
          {vod ? 'Pulsa una jugada para verla y, si te gusta, añádela.' : 'Marca un momento para empezar.'}
        </p>
      )}
    </section>
  );
}

function ShortSourcePanel({
  plan,
  sourceDuration,
  busy,
  onUseFull,
}: {
  plan: StreamEditPlan;
  sourceDuration: number;
  busy: boolean;
  onUseFull: () => void;
}): ReactNode {
  const canUseFull = fullVideoMoment({ clips: plan.clips, sourceDuration }) !== null;
  return (
    <section aria-label="Vídeo corto" className="flex flex-col gap-3 rounded-md border border-border-subtle bg-surface-1 p-3">
      <h3 className="font-semibold">Vídeo corto</h3>
      {sourceDuration > 0 ? <p className="text-body-sm text-fg-2">Dura {formatStreamClock(sourceDuration)}.</p> : null}
      <p className="text-body-sm text-fg-2">En un vídeo corto no buscamos jugadas. Lo normal es usarlo entero.</p>
      {canUseFull ? (
        <Button variant="stream" disabled={busy} onClick={onUseFull}>
          Usar vídeo completo
        </Button>
      ) : null}
      <p className="text-body-sm text-fg-3">O quédate solo con un trozo: muévete por la línea de tiempo y pulsa «Marcar momento aquí».</p>
    </section>
  );
}

export function StreamMomentsStep({
  job,
  plan,
  moments,
  sourceDuration,
  playheadSeconds,
  selectedClipId,
  plays,
  activePlayId,
  busy,
  player,
  timeline,
  onWatchPlay,
  onPlayMoment,
  onPlayAll,
  onPlaySelected,
  onPlaysShown,
}: {
  job: StreamJob;
  plan: StreamEditPlan;
  moments: StreamMoments;
  sourceDuration: number;
  playheadSeconds: number;
  selectedClipId: string | null;
  plays: readonly StreamPlay[];
  activePlayId: string | null;
  busy: boolean;
  player: ReactNode;
  timeline: ReactNode;
  onWatchPlay: (play: StreamPlay) => void;
  onPlayMoment: (clip: StreamClipRange) => void;
  onPlayAll: () => void;
  onPlaySelected: () => void;
  onPlaysShown: (plays: readonly StreamPlay[]) => void;
}): ReactNode {
  const isVod = job.source_kind === 'vod';
  return (
    <div className="grid min-w-0 grid-cols-1 gap-4 p-4 @[48rem]/content:grid-cols-[minmax(0,1fr)_minmax(20rem,27rem)]">
      <div className="min-w-0 @[48rem]/content:col-start-1 @[48rem]/content:row-start-1">{player}</div>

      <section aria-label="Línea de tiempo y momento" className="flex min-w-0 flex-col gap-3 @[48rem]/content:col-span-2 @[48rem]/content:row-start-2">
        <Button
          variant="outline"
          className="self-start"
          disabled={busy || sourceDuration <= 0}
          onClick={() => moments.markHere(playheadSeconds)}
        >
          <Flag aria-hidden />
          Marcar momento aquí
        </Button>
        {timeline}
        <StreamClipEditor
          clips={plan.clips}
          plays={plays}
          sourceDuration={sourceDuration}
          selectedClipId={selectedClipId}
          onChange={moments.setClips}
          onSelect={moments.selectClip}
          onRemove={moments.removeClip}
          onNudgeEdge={moments.nudgeEdge}
          onSuggest={(clip) => moments.restoreSuggestedCut(clip, plays)}
          disabled={busy}
          onPlay={onPlaySelected}
        />
      </section>

      <div className="min-w-0 @[48rem]/content:col-span-2 @[48rem]/content:row-start-3">
        <MomentsStrip
          clips={plan.clips}
          selectedClipId={selectedClipId}
          disabled={busy}
          vod={isVod}
          onPlay={onPlayMoment}
          onPlayAll={onPlayAll}
        />
      </div>

      <div className="min-w-0 @[48rem]/content:relative @[48rem]/content:col-start-2 @[48rem]/content:row-start-1 @[48rem]/content:min-h-[24rem]">
        <div className="flex min-w-0 flex-col @[48rem]/content:absolute @[48rem]/content:inset-0 @[48rem]/content:overflow-y-auto">
          {isVod ? (
            <StreamDetectedPlays
              jobId={job.id}
              vod
              clips={plan.clips}
              disabled={busy}
              activePlayId={activePlayId}
              onAdd={moments.addPlays}
              onWatch={onWatchPlay}
              onJump={moments.selectClip}
              onPlaysShown={onPlaysShown}
            />
          ) : (
            <ShortSourcePanel
              plan={plan}
              sourceDuration={sourceDuration}
              busy={busy}
              onUseFull={moments.addFullVideo}
            />
          )}
        </div>
      </div>
    </div>
  );
}
