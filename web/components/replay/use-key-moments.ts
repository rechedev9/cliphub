import { useMemo } from 'react';
import { docNames } from '@/lib/replay/centre-round';
import type { CentreRound } from '@/lib/replay/centre-round';
import { keyMoments } from '@/lib/replay/key-moments';
import { roundClockLabelFor, timelineSeconds } from '@/lib/tactical-timeline';
import type { RoundTimeline } from '@/lib/tactical-timeline';
import type { ShownMoment } from './key-moments-panel';

/** A jump lands this long before the moment, so the play is seen happening. */
const JUMP_LEAD_SECONDS = 1;

/** The focused round's key moments on the transport's scale, each with its jump. */
export function useKeyMoments(options: {
  focused: CentreRound | null;
  enabled: boolean;
  transport: RoundTimeline;
  clockSecondsOf: (timeline: RoundTimeline, tick: number) => number;
  onJump: (seconds: number) => void;
}): ShownMoment[] {
  const { focused, enabled, transport, clockSecondsOf, onJump } = options;
  return useMemo(() => {
    if (focused === null || !enabled) return [];
    const { timeline } = focused;
    const lead = Math.round(JUMP_LEAD_SECONDS * timeline.tickrate);
    const found = keyMoments({ round: focused.round, frames: focused.frames, tickrate: timeline.tickrate, names: docNames(focused.doc), endTick: timeline.endTick });
    return found.map((moment, index) => ({
      key: `${moment.kind}-${moment.tick}-${index}`,
      kind: moment.kind,
      label: moment.label,
      side: moment.side,
      clock: roundClockLabelFor(timeline, timelineSeconds(timeline, moment.tick)),
      at: transport.durationSeconds > 0 ? Math.min(1, Math.max(0, clockSecondsOf(timeline, moment.tick) / transport.durationSeconds)) : 0,
      onJump: () => onJump(clockSecondsOf(timeline, moment.tick - lead)),
    }));
  }, [clockSecondsOf, enabled, focused, onJump, transport]);
}
