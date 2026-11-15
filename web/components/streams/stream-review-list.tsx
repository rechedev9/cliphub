'use client';

import { useState, type ReactNode } from 'react';
import { ArrowDown, ArrowUp } from 'lucide-react';
import type { StreamEditPlan } from '@/lib/api/streams';
import {
  SHORT_MAX_SECONDS,
  joinStreamMoments,
  moveStreamGroupMoment,
  splitStreamGroup,
  streamPlanShorts,
  type StreamShort,
} from '@/lib/streams/groups';
import { formatStreamClock } from '@/lib/streams/plan';
import { Button } from '@/components/ui/button';

/** The Revisar list: one row per exported Short, with joining, separating and reordering grouped moments. */
export function StreamReviewList({
  plan,
  selectedShortId,
  disabled,
  onSelect,
  onPlanChange,
}: {
  plan: StreamEditPlan;
  selectedShortId: string | null;
  disabled: boolean;
  onSelect: (short: StreamShort) => void;
  onPlanChange: (plan: StreamEditPlan) => void;
}): ReactNode {
  const shorts = streamPlanShorts(plan);
  const [picked, setPicked] = useState<readonly string[]>([]);
  const pickedClipIds = shorts.filter((s) => picked.includes(s.id)).flatMap((s) => s.clips.map((c) => c.id));
  const toggle = (id: string) =>
    setPicked((current) => (current.includes(id) ? current.filter((p) => p !== id) : [...current, id]));
  const join = () => {
    onPlanChange(joinStreamMoments(plan, pickedClipIds));
    setPicked([]);
  };

  return (
    <div className="flex flex-col gap-2">
      <ul className="flex flex-col gap-2">
        {shorts.map((short, index) => {
          const tooLong = short.groupId !== null && short.durationSeconds > SHORT_MAX_SECONDS;
          return (
            <li key={short.id} className="flex flex-col gap-1">
              <div className="flex items-start gap-2">
                {shorts.length > 1 ? (
                  <input
                    type="checkbox"
                    className="mt-2 size-4 shrink-0 accent-stream"
                    aria-label={`Elegir «${short.title}» para juntar`}
                    checked={picked.includes(short.id)}
                    disabled={disabled}
                    onChange={() => toggle(short.id)}
                  />
                ) : null}
                <Button
                  variant={selectedShortId === short.id ? 'secondary' : 'outline'}
                  className="h-auto min-w-0 flex-1 justify-start whitespace-normal text-left"
                  onClick={() => onSelect(short)}
                >
                  {index + 1}. {short.title} · {formatStreamClock(short.durationSeconds)}
                </Button>
              </div>
              {short.groupId !== null ? (
                <GroupMembers
                  short={short}
                  groupId={short.groupId}
                  plan={plan}
                  disabled={disabled}
                  tooLong={tooLong}
                  onPlanChange={onPlanChange}
                />
              ) : null}
            </li>
          );
        })}
      </ul>
      {pickedClipIds.length >= 2 ? (
        <Button variant="stream" disabled={disabled} onClick={join}>
          Juntar {pickedClipIds.length} momentos en un Short
        </Button>
      ) : null}
      {pickedClipIds.length < 2 && shorts.length > 1 ? (
        <p className="text-label text-fg-3">Marca dos o más para juntarlos en un mismo Short.</p>
      ) : null}
    </div>
  );
}

function GroupMembers({
  short,
  groupId,
  plan,
  disabled,
  tooLong,
  onPlanChange,
}: {
  short: StreamShort;
  groupId: string;
  plan: StreamEditPlan;
  disabled: boolean;
  tooLong: boolean;
  onPlanChange: (plan: StreamEditPlan) => void;
}): ReactNode {
  const last = short.clips.length - 1;
  const move = (clipId: string, offset: -1 | 1) => onPlanChange(moveStreamGroupMoment(plan, { groupId, clipId, offset }));
  return (
    <div className="ml-6 flex flex-col gap-1 border-l border-border-subtle pl-3">
      <ol className="flex flex-col gap-1">
        {short.clips.map((clip, i) => {
          const name = clip.title?.trim() || `Momento ${plan.clips.indexOf(clip) + 1}`;
          return (
            <li key={clip.id} className="flex items-center gap-1 text-body-sm text-fg-2">
              <span className="min-w-0 flex-1 truncate">
                {i + 1}. {name}
              </span>
              <Button
                variant="ghost"
                size="icon-xs"
                aria-label={`Subir «${name}»`}
                disabled={disabled || i === 0}
                onClick={() => move(clip.id, -1)}
              >
                <ArrowUp aria-hidden />
              </Button>
              <Button
                variant="ghost"
                size="icon-xs"
                aria-label={`Bajar «${name}»`}
                disabled={disabled || i === last}
                onClick={() => move(clip.id, 1)}
              >
                <ArrowDown aria-hidden />
              </Button>
            </li>
          );
        })}
      </ol>
      {tooLong ? (
        <p role="status" className="text-body-sm text-warning">
          Dura más de {SHORT_MAX_SECONDS / 60} min: YouTube no lo publicará como Short.
        </p>
      ) : null}
      <Button
        variant="ghost"
        size="sm"
        className="self-start"
        disabled={disabled}
        onClick={() => onPlanChange(splitStreamGroup(plan, groupId))}
      >
        Separar
      </Button>
    </div>
  );
}
