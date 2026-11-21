'use client';

import type { ReactNode } from 'react';
import type { MomentEdge } from '@/lib/streams/moment-edges';
import { momentLengthLabel } from '@/lib/streams/moments';
import { formatStreamClock } from '@/lib/streams/plan';
import { Button } from '@/components/ui/button';

const EDGES: readonly { edge: MomentEdge; label: string; verb: string }[] = [
  { edge: 'start', label: 'Empieza', verb: 'Empezar' },
  { edge: 'end', label: 'Termina', verb: 'Terminar' },
];

export function MomentEdgesEditor({
  start,
  end,
  disabled,
  onNudge,
}: {
  start: number;
  end: number;
  disabled: boolean;
  onNudge: (change: { edge: MomentEdge; delta: number }) => void;
}): ReactNode {
  return (
    <div className="flex flex-wrap items-end gap-x-5 gap-y-2">
      {EDGES.map(({ edge, label, verb }) => (
        <div key={edge} className="flex flex-col gap-1">
          <span className="text-meta text-fg-3">{label}</span>
          <div className="flex items-center gap-1.5">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={disabled}
              aria-label={`${verb} un segundo antes`}
              onClick={() => onNudge({ edge, delta: -1 })}
            >
              -1 s
            </Button>
            <span className="min-w-12 text-center font-semibold tabular-nums text-fg-1">
              {formatStreamClock(edge === 'start' ? start : end)}
            </span>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={disabled}
              aria-label={`${verb} un segundo después`}
              onClick={() => onNudge({ edge, delta: 1 })}
            >
              +1 s
            </Button>
          </div>
        </div>
      ))}
      <div className="flex flex-col gap-1">
        <span className="text-meta text-fg-3">Dura</span>
        <span className="flex h-8 items-center font-semibold tabular-nums text-fg-1">{momentLengthLabel(end - start)}</span>
      </div>
    </div>
  );
}
