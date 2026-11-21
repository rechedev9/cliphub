import { demoPlaybackItem, streamPlaybackItem, type MediaPlaybackItem } from '../api/playback.ts';
import type { StreamJob } from '../api/streams.ts';
import { OUTPUT_STATE, OUTPUT_TYPE, type HubModel } from './hub.ts';
import {
  LIBRARY_FILTER,
  LIBRARY_STAGE,
  matchesFormat,
  videoDisplayTitle,
  videoKind,
  type LibraryFilter,
  type LibraryStage,
} from './results.ts';

export type VisorEntry = {
  id: string;
  title: string;
  item: MediaPlaybackItem;
};

export type VisorFilter = { format: LibraryFilter; stage: LibraryStage };

export type VisorQueueInput = {
  clips: HubModel['clips'];
  streams: readonly StreamJob[];
  filter: VisorFilter | null;
  requestedId: string | null;
};

export type VisorQueue = {
  entries: VisorEntry[];
  start: VisorEntry | null;
  fellBack: boolean;
};

const EVERY_SHORT: VisorFilter = { format: LIBRARY_FILTER.all, stage: LIBRARY_STAGE.all };

function entryOf(item: MediaPlaybackItem | null, title: string): VisorEntry[] {
  return item === null || item.format !== '9:16' ? [] : [{ id: item.id, title, item }];
}

function demoEntryOf(clip: HubModel['clips'][number]): VisorEntry[] {
  if (clip.state !== OUTPUT_STATE.ready || videoKind(clip.video) !== OUTPUT_TYPE.short) return [];
  const title = videoDisplayTitle(clip.video, clip.match?.player ?? clip.video.targetName);
  return entryOf(demoPlaybackItem(clip.video), title);
}

export function visorEntryId(clip: HubModel['clips'][number]): string | null {
  return demoEntryOf(clip)[0]?.id ?? null;
}

export function buildVisorQueue({ clips, streams, filter, requestedId }: VisorQueueInput): VisorQueue {
  const { format, stage } = filter ?? EVERY_SHORT;
  if (stage === LIBRARY_STAGE.working) return { entries: [], start: null, fellBack: false };
  const demoEntries = matchesFormat(OUTPUT_TYPE.short, format)
    ? clips.flatMap(demoEntryOf)
    : [];
  const streamEntries = matchesFormat(LIBRARY_FILTER.stream, format)
    ? streams.flatMap((job) =>
      (job.rendered_outputs ?? []).flatMap((output) => {
        const item = streamPlaybackItem(job, output);
        return entryOf(item, item?.title ?? '');
      }),
    )
    : [];
  const entries = [...demoEntries, ...streamEntries];
  const requested = entries.find((entry) => entry.id === requestedId);
  const start = requested ?? entries[0] ?? null;
  return { entries, start, fellBack: requestedId !== null && requested === undefined && start !== null };
}
