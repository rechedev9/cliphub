import type { RefObject } from 'react';
import type { Stroke } from '@/lib/api/spatial';
import type { ReplayClock } from '@/lib/replay/clock';
import type { ReplayLayers } from '@/lib/replay/layers';
import type { MapSettings } from '@/lib/replay/map-settings';
import type { PlayerFocus } from '@/lib/replay/player-focus';
import type { CentreRound } from '@/lib/replay/centre-round';
import type { TelestratorDraft } from '@/components/tactical/spatial/telestrator';
import type { MapCapture } from './gif-export';

/**
 * What the map receives. Playback never re-renders it: read
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
  /** Notes' drawings over the map. */
  telestrator: { draft: TelestratorDraft; shown: readonly { id: string; strokes: readonly Stroke[] }[] } | null;
  /** The zones a finding's link marks, as CS2 place tokens. */
  zones?: readonly string[];
  /** The player the map keeps centred, set with F. */
  followSlot: number | null;
  onFollowSlot: (slot: number | null) => void;
  /** The players in the foreground; the rest of the focused match are drawn translucent. */
  focus: PlayerFocus;
  /** A click on a player; `additive` is Shift held. */
  onPickSlot: (slot: number, additive: boolean) => void;
  /** The player under the cursor, for the setpos shortcut. */
  onHoverSlot: (slot: number | null) => void;
  /** A right-click on a player: copy their setpos, as S does. */
  onCopySlot: (slot: number) => void;
  settings: MapSettings;
  /** Where the map puts its zoom buttons: the bar over it. */
  barSlot: HTMLElement | null;
  /** Filled with a way to draw any moment off screen, for the GIF; null while the map cannot draw. */
  captureRef?: RefObject<MapCapture | null>;
};
