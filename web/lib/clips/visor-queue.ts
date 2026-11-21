import { demoPlaybackItem, streamPlaybackItem, type MediaPlaybackItem } from '../api/playback.ts';
import type { StreamJob } from '../api/streams.ts';
import { OUTPUT_STATE, OUTPUT_TYPE, isWorking, type HubModel } from './hub.ts';
import {
  LIBRARY_FILTER,
  LIBRARY_STAGE,
  matchesFormat,
  videoDisplayTitle,
  videoKind,
  type LibraryFilter,
  type LibraryStage,
} from './results.ts';
import { VISOR_ACTION, demoShortActions, streamShortActions, type VisorAction, type VisorSource } from './visor-actions.ts';

export type VisorEntry = {
  id: string;
  title: string;
  item: MediaPlaybackItem;
  source: VisorSource;
  actions: VisorAction[];
  rendering: boolean;
};

export type VisorFilter = { format: LibraryFilter; stage: LibraryStage };

export type VisorQueueInput = {
  clips: HubModel['clips'];
  streams: readonly StreamJob[];
  filter: VisorFilter | null;
  requestedId: string | null;
  shown?: readonly VisorEntry[];
};

export type VisorQueue = {
  entries: VisorEntry[];
  start: VisorEntry | null;
};

const EVERY_SHORT: VisorFilter = { format: LIBRARY_FILTER.all, stage: LIBRARY_STAGE.all };

type EntryParts = Pick<VisorEntry, 'title' | 'source' | 'actions'>;

function entryOf(item: MediaPlaybackItem | null, parts: (item: MediaPlaybackItem) => EntryParts): VisorEntry[] {
  return item === null || item.format !== '9:16' ? [] : [{ id: item.id, item, ...parts(item), rendering: false }];
}

function heldWhileRendering(clip: HubModel['clips'][number], shown: readonly VisorEntry[]): VisorEntry[] {
  const previous = shown.find((entry) => entry.id === `demo:${clip.video.id}`);
  if (previous === undefined || !isWorking(clip.state)) return [];
  return [{ ...previous, actions: previous.actions.filter(changesNothing), rendering: true }];
}

function changesNothing(action: VisorAction): boolean {
  return action.kind === VISOR_ACTION.download || action.kind === VISOR_ACTION.delete;
}

export function buildVisorQueue({ clips, streams, filter, requestedId, shown = [] }: VisorQueueInput): VisorQueue {
  const { format, stage } = filter ?? EVERY_SHORT;
  if (stage === LIBRARY_STAGE.working) return { entries: [], start: null };
  const demoEntries = matchesFormat(OUTPUT_TYPE.short, format)
    ? clips.flatMap((clip): VisorEntry[] => {
      if (videoKind(clip.video) !== OUTPUT_TYPE.short) return [];
      if (clip.state !== OUTPUT_STATE.ready) return heldWhileRendering(clip, shown);
      const title = videoDisplayTitle(clip.video, clip.match?.player ?? clip.video.targetName);
      return entryOf(demoPlaybackItem(clip.video), () => ({ title, source: { kind: 'demo', video: clip.video }, actions: demoShortActions(clip) }));
    })
    : [];
  const streamEntries = matchesFormat(LIBRARY_FILTER.stream, format)
    ? streams.flatMap((job) =>
      (job.rendered_outputs ?? []).flatMap((output) => {
        const item = streamPlaybackItem(job, output);
        return entryOf(item, (playable) => ({
          title: playable.title,
          source: { kind: 'stream', clipId: output.clip_id },
          actions: streamShortActions(playable),
        }));
      }),
    )
    : [];
  const entries = [...demoEntries, ...streamEntries];
  const requested = entries.find((entry) => entry.id === requestedId);
  return { entries, start: requested ?? entries[0] ?? null };
}

function entryKey(entry: VisorEntry): string {
  return [entry.id, entry.item.playbackUrl, entry.rendering, entry.title, ...entry.actions.map((action) => action.label)].join('\n');
}

export function sameVisorEntries(a: readonly VisorEntry[], b: readonly VisorEntry[]): boolean {
  return a.length === b.length && a.every((entry, at) => {
    const other = b[at];
    return other !== undefined && entryKey(entry) === entryKey(other);
  });
}

export function visorAfterDelete(entries: readonly VisorEntry[], deletedId: string): string | null {
  const at = entries.findIndex((entry) => entry.id === deletedId);
  const next = entries[at + 1] ?? entries[at - 1];
  return at === -1 || next === undefined ? null : next.id;
}
