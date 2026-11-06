import { TACTICAL_EVENT_KINDS, TACTICAL_SAMPLE_FLAGS } from '../api/tactical.ts';
import type { TacticalEventKind } from '../api/tactical.ts';

/** What the replay draws. Kills and the players' dots are always the subject; the rest can go. */
export type ReplayLayers = {
  utility: boolean;
  bomb: boolean;
  trails: boolean;
  names: boolean;
  /** Lines between enemies who can see each other, and view wedges cut at the walls. */
  sight: boolean;
  /** Short lines from attacker to victim on each hit and enemy flash. */
  hits: boolean;
  /** The opening kill, trades, plant, defuses and clutch marked on the timeline. */
  moments: boolean;
  /** How far back the trails reach. */
  trailSeconds: TrailSeconds;
  /** Slots of the players taken off the map. */
  hiddenSlots: readonly number[];
};

export type LayerToggle = 'utility' | 'bomb' | 'trails' | 'names' | 'sight' | 'hits' | 'moments';

export const TRAIL_LENGTHS = [2, 5, 10] as const;
export type TrailSeconds = (typeof TRAIL_LENGTHS)[number];

export const DEFAULT_LAYERS: ReplayLayers = {
  utility: true,
  bomb: true,
  trails: true,
  names: true,
  sight: false,
  hits: true,
  moments: true,
  trailSeconds: 2,
  hiddenSlots: [],
};

export const LAYER_LABELS: Record<LayerToggle, string> = {
  utility: 'Utilidad',
  bomb: 'Bomba',
  trails: 'Rastros',
  names: 'Nombres',
  sight: 'Quién ve a quién',
  hits: 'Daño y flashes',
  moments: 'Momentos clave',
};

const UTILITY_KINDS: ReadonlySet<TacticalEventKind> = new Set([
  TACTICAL_EVENT_KINDS.smoke,
  TACTICAL_EVENT_KINDS.flash,
  TACTICAL_EVENT_KINDS.he,
  TACTICAL_EVENT_KINDS.molotov,
  TACTICAL_EVENT_KINDS.decoy,
]);

const BOMB_KINDS: ReadonlySet<TacticalEventKind> = new Set([
  TACTICAL_EVENT_KINDS.plant,
  TACTICAL_EVENT_KINDS.defuse,
  TACTICAL_EVENT_KINDS.explode,
  TACTICAL_EVENT_KINDS.bombDrop,
]);

export function isUtilityKind(kind: TacticalEventKind): boolean {
  return UTILITY_KINDS.has(kind);
}

export function isBombKind(kind: TacticalEventKind): boolean {
  return BOMB_KINDS.has(kind);
}

export function toggleLayer(layers: ReplayLayers, key: LayerToggle): ReplayLayers {
  return { ...layers, [key]: !layers[key] };
}

export function togglePlayer(layers: ReplayLayers, slot: number): ReplayLayers {
  const hidden = layers.hiddenSlots.includes(slot)
    ? layers.hiddenSlots.filter((entry) => entry !== slot)
    : [...layers.hiddenSlots, slot].sort((a, b) => a - b);
  return { ...layers, hiddenSlots: hidden };
}

export function showAllPlayers(layers: ReplayLayers): ReplayLayers {
  return layers.hiddenSlots.length === 0 ? layers : { ...layers, hiddenSlots: [] };
}

/** Events the map and the timeline still draw; a hidden player's grenades go with them. */
export function layerEvents<E extends { event: { kind: TacticalEventKind; actor_slot?: number } }>(
  events: readonly E[],
  layers: ReplayLayers,
): E[] {
  return events.filter(({ event }) => {
    if (isUtilityKind(event.kind)) {
      if (!layers.utility) return false;
      return event.actor_slot === undefined || !layers.hiddenSlots.includes(event.actor_slot);
    }
    if (isBombKind(event.kind)) return layers.bomb;
    return true;
  });
}

/** Samples of the shown players; with the bomb layer off nobody carries it on the map. */
export function layerSamples<S extends { slot: number; flags: number }>(samples: readonly S[], layers: ReplayLayers): S[] {
  const out: S[] = [];
  for (const sample of samples) {
    if (layers.hiddenSlots.includes(sample.slot)) continue;
    if (!layers.bomb && (sample.flags & TACTICAL_SAMPLE_FLAGS.hasBomb) !== 0) {
      out.push({ ...sample, flags: sample.flags & ~TACTICAL_SAMPLE_FLAGS.hasBomb });
    } else out.push(sample);
  }
  return out;
}

/** Trails of the shown players, or none with the layer off. */
export function layerTrails<T>(trails: ReadonlyMap<number, T>, layers: ReplayLayers): ReadonlyMap<number, T> {
  if (!layers.trails) return new Map();
  if (layers.hiddenSlots.length === 0) return trails;
  const out = new Map<number, T>();
  for (const [slot, points] of trails) if (!layers.hiddenSlots.includes(slot)) out.set(slot, points);
  return out;
}

/** Name tags, or none with the layer off. */
export function layerLabels(labels: ReadonlyMap<number, string>, layers: ReplayLayers): ReadonlyMap<number, string> {
  return layers.names ? labels : new Map();
}
