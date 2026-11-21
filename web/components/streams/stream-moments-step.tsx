'use client';

import { useState, type ReactNode } from 'react';
import { Flag, Play } from 'lucide-react';
import type { StreamClipRange, StreamEditPlan, StreamJob, StreamPlay } from '@/lib/api/streams';
import { momentLengthLabel, momentsInOrder } from '@/lib/streams/moments';
import { MOMENTS_KEY, momentsShortcutLegend, removableMomentId } from '@/lib/streams/moments-keys';
import { fullVideoMoment } from '@/lib/streams/mark-moment';
import { clipOutputDuration, formatStreamClock } from '@/lib/streams/plan';
import { orderPlays, type PlayOrder } from '@/lib/streams/plays';
import { cn } from '@/lib/utils';
import { StreamClipEditor } from '@/components/streams/clip-editor';
import { StreamDetectedPlays } from '@/components/streams/detected-plays';
import { Kbd } from '@/components/streams/kbd';
import { focusPlayRow, useMomentsShortcuts } from '@/components/streams/use-moments-shortcuts';
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

function ShortcutLegend({ vod }: { vod: boolean }): ReactNode {
  return (
    <ul aria-label="Atajos de teclado" className="mt-2 flex flex-wrap gap-x-4 gap-y-1 px-1 text-meta text-fg-3">
      {momentsShortcutLegend({ vod }).map(({ keys, label }) => (
        <li key={label} className="flex items-center gap-1">
          {keys.map((key) => (
            <Kbd key={key}>{key}</Kbd>
          ))}
          <span>{label}</span>
        </li>
      ))}
    </ul>
  );
}

export function StreamMomentsStep({
  job,
  plan,
  moments,
  sourceDuration,
  playheadSeconds,
  selectedClipId,
  cardClipId,
  plays,
  activePlayId,
  busy,
  player,
  timeline,
  onWatchPlay,
  onPlayMoment,
  onPlayAll,
  onPlaySelected,
  onTogglePlay,
  onPlaysShown,
}: {
  job: StreamJob;
  plan: StreamEditPlan;
  moments: StreamMoments;
  sourceDuration: number;
  playheadSeconds: number;
  selectedClipId: string | null;
  cardClipId: string | null;
  plays: readonly StreamPlay[];
  activePlayId: string | null;
  busy: boolean;
  player: ReactNode;
  timeline: ReactNode;
  onWatchPlay: (play: StreamPlay) => void;
  onPlayMoment: (clip: StreamClipRange) => void;
  onPlayAll: () => void;
  onPlaySelected: () => void;
  onTogglePlay: () => void;
  onPlaysShown: (plays: readonly StreamPlay[]) => void;
}): ReactNode {
  const isVod = job.source_kind === 'vod';
  const [order, setOrder] = useState<PlayOrder>('best');
  const canMark = !busy && sourceDuration > 0;
  const removableId = removableMomentId({ clips: plan.clips, selectedClipId, selectedPlayId: activePlayId });
  useMomentsShortcuts({
    state: {
      plays: isVod ? orderPlays(plays, order) : [],
      clips: plan.clips,
      selectedPlayId: activePlayId,
      selectedClipId,
      canMark,
      busy,
    },
    onAction: (action) => {
      if (action.kind === 'toggle-play') onTogglePlay();
      else if (action.kind === 'watch-play') {
        onWatchPlay(action.play);
        focusPlayRow(action.play.id);
      } else if (action.kind === 'add-play') moments.addPlays([action.play]);
      else if (action.kind === 'jump') moments.selectClip(action.clip);
      else if (action.kind === 'remove') moments.removeClip(action.clip);
      else moments.markHere(playheadSeconds);
    },
  });
  return (
    <div className="grid min-w-0 grid-cols-1 gap-4 p-4 @[48rem]/content:grid-cols-[minmax(0,1fr)_minmax(20rem,27rem)]">
      <div className="min-w-0 @[48rem]/content:col-start-1 @[48rem]/content:row-start-1">{player}</div>

      <section aria-label="Línea de tiempo y momento" className="flex min-w-0 flex-col gap-3 @[48rem]/content:col-span-2 @[48rem]/content:row-start-2">
        <Button
          variant="outline"
          className="self-start"
          disabled={!canMark}
          aria-keyshortcuts="M"
          onClick={() => moments.markHere(playheadSeconds)}
        >
          <Flag aria-hidden />
          Marcar momento aquí
          <Kbd>{MOMENTS_KEY.mark}</Kbd>
        </Button>
        {timeline}
        <StreamClipEditor
          clips={plan.clips}
          plays={plays}
          sourceDuration={sourceDuration}
          selectedClipId={cardClipId}
          shortcutClipId={removableId}
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
              order={order}
              onOrderChange={setOrder}
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
          <ShortcutLegend vod={isVod} />
        </div>
      </div>
    </div>
  );
}
