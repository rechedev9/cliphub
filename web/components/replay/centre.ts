import type { ReactNode } from 'react';
import type { Stroke } from '@/lib/api/spatial';
import type { ReplayClock } from '@/lib/replay/clock';
import type { ReplayLayers } from '@/lib/replay/layers';
import type { CentreRound } from '@/lib/replay/centre-round';
import type { TelestratorDraft } from '@/components/tactical/spatial/telestrator';

/**
 * What a centre view receives. Playback never re-renders it: read
 * `clock.seconds` and `subscribe` to the clock, then turn seconds into each
 * round's tick with `tickOf`. Samples are not filtered for you: run them through
 * `layerSamples` and trails through `layerTrails` (lib/replay/layers.ts).
 */
export type ReplayCentreProps = {
  /** `single`: seconds from the focused round's start. `multi`: seconds since each round's freeze end. */
  mode: 'single' | 'multi';
  /** Superposed rounds first, the focused round last so it draws on top. */
  rounds: readonly CentreRound[];
  clock: ReplayClock;
  tickOf: (round: CentreRound, seconds: number) => number;
  layers: ReplayLayers;
  /** Notes' drawings for the 2D plate; other views may ignore it. */
  telestrator: { draft: TelestratorDraft; shown: readonly { id: string; strokes: readonly Stroke[] }[] } | null;
};

/** A pluggable centre: the 2D radar by default, a 3D view next to it. */
export type ReplayCentreView = {
  id: string;
  label: string;
  render: (props: ReplayCentreProps) => ReactNode;
};
