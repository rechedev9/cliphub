import type { TacticalDocument, TacticalFrame, TacticalRound } from '../api/tactical.ts';
import { roundTimeline, timelineEvents } from '../tactical-timeline.ts';
import type { RoundTimeline, TimelineEvent } from '../tactical-timeline.ts';
import { layerEvents, layerLabels } from './layers.ts';
import type { ReplayLayers } from './layers.ts';
import { tagName } from './tag-layout.ts';

/** One round the centre draws: the focused one, plus the superposed ones in multi-round mode. */
export type CentreRound = {
  /** `entryKey` of its strip item. */
  key: string;
  matchId: string;
  doc: TacticalDocument;
  round: TacticalRound;
  /** Decoded position frames; empty when the analysis stored none for this round. */
  frames: readonly TacticalFrame[];
  timeline: RoundTimeline;
  /** On the round's own bar, already filtered by the layers. */
  events: readonly TimelineEvent[];
  /** Name tags by slot; empty with the names layer off. */
  labels: ReadonlyMap<number, string>;
  focused: boolean;
};

/** Longest name tag on the map; longer names end in an ellipsis and the team panel has them whole. */
const TAG_CHARS = 14;

export function docLabels(doc: Pick<TacticalDocument, 'players'>): Map<number, string> {
  return new Map(doc.players.map((player) => [player.slot, tagName(player.name, TAG_CHARS)]));
}

export function docNames(doc: Pick<TacticalDocument, 'players'>): Map<number, string> {
  return new Map(doc.players.map((player) => [player.slot, player.name]));
}

export function centreRound(options: {
  key: string;
  matchId: string;
  doc: TacticalDocument;
  round: TacticalRound;
  frames: readonly TacticalFrame[];
  layers: ReplayLayers;
  focused: boolean;
}): CentreRound {
  const timeline = roundTimeline(options.round, options.doc.demo.tickrate);
  return {
    key: options.key,
    matchId: options.matchId,
    doc: options.doc,
    round: options.round,
    frames: options.frames,
    timeline,
    events: layerEvents(timelineEvents(timeline, options.round.events), options.layers),
    labels: layerLabels(docLabels(options.doc), options.layers),
    focused: options.focused,
  };
}
