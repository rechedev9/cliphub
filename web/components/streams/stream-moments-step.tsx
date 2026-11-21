'use client';

import type { ReactNode } from 'react';
import type { StreamEditPlan, StreamJob } from '@/lib/api/streams';
import { FULL_SOURCE_MOMENT_MAX_SECONDS } from '@/lib/streams/plan';
import { StreamClipEditor } from '@/components/streams/clip-editor';
import { StreamDetectedPlays } from '@/components/streams/detected-plays';
import { MomentRange } from '@/components/streams/moment-range';
import type { StreamMoments } from '@/components/streams/use-stream-moments';
import { Button } from '@/components/ui/button';

export function StreamMomentsStep({
  job,
  plan,
  moments,
  sourceDuration,
  playheadSeconds,
  selectedClipId,
  busy,
  onSeekSource,
  onPlaySelected,
}: {
  job: StreamJob;
  plan: StreamEditPlan;
  moments: StreamMoments;
  sourceDuration: number;
  playheadSeconds: number;
  selectedClipId: string | null;
  busy: boolean;
  onSeekSource: (seconds: number) => void;
  onPlaySelected: () => void;
}): ReactNode {
  const isVod = job.source_kind === 'vod';
  const { draftRange, draftIssue } = moments;
  return (
    <>
      <p className="text-body-sm text-fg-2">
        {isVod
          ? 'Añade las jugadas que quieras como Short, o marca tú el inicio y el final. Cada momento será un Short vertical.'
          : 'Reproduce el original, marca el inicio y el final y añade el momento. Cada momento será un Short.'}
      </p>
      {isVod ? (
        <StreamDetectedPlays
          jobId={job.id}
          vod
          clips={plan.clips}
          disabled={busy}
          onAdd={moments.addPlays}
          onRemove={moments.removePlay}
          onSeek={onSeekSource}
        />
      ) : null}
      {moments.creatingMoment ? (
        <section aria-label="Nuevo momento" className="flex flex-col gap-3 rounded-md border border-stream/40 p-3">
          <h3 className="font-semibold">Nuevo momento</h3>
          <MomentRange
            id="draft"
            start={draftRange.start_seconds}
            end={draftRange.end_seconds}
            duration={sourceDuration}
            playhead={playheadSeconds}
            disabled={busy}
            invalid={draftIssue !== null}
            onChange={moments.patchDraft}
          />
          {draftIssue ? (
            <p role="alert" className="text-body-sm text-destructive">
              {draftIssue}
            </p>
          ) : null}
          <Button variant="stream" disabled={busy || draftIssue !== null} onClick={() => moments.addMoment()}>
            Añadir este momento
          </Button>
          {!plan.clips.length && sourceDuration > 0 && sourceDuration <= FULL_SOURCE_MOMENT_MAX_SECONDS ? (
            <Button
              variant="outline"
              disabled={busy}
              onClick={() => moments.addMoment({ start_seconds: 0, end_seconds: sourceDuration })}
            >
              Usar vídeo completo
            </Button>
          ) : null}
          {plan.clips.length ? (
            <Button variant="ghost" onClick={moments.cancelDraft}>
              Cancelar nuevo momento
            </Button>
          ) : null}
        </section>
      ) : (
        <>
          <StreamClipEditor
            clips={plan.clips}
            sourceDuration={sourceDuration}
            selectedClipId={selectedClipId}
            onChange={moments.setClips}
            onSelect={moments.selectClip}
            onRemove={moments.removeClip}
            disabled={busy}
            playheadSeconds={playheadSeconds}
            onPlay={onPlaySelected}
          />
          <Button variant="outline" disabled={busy} onClick={() => moments.startNewMoment(playheadSeconds)}>
            Nuevo momento
          </Button>
        </>
      )}
    </>
  );
}
