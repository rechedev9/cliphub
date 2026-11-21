import { demoPlaybackItem, streamPlaybackItem, type MediaPlaybackItem } from '../api/playback.ts';
import type { StreamJob } from '../api/streams.ts';
import type { Video } from '../api/types.ts';
import { prettyMapName } from '../format.ts';
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
import {
  VISOR_ACTION,
  demoShortActions,
  streamShortActions,
  type LoteShort,
  type VisorAction,
  type VisorSource,
} from './visor-actions.ts';

export type VisorEntry = {
  id: string;
  title: string;
  mark: string | null;
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
  lotes?: ReadonlyMap<string, readonly LoteShort[]>;
};

export type VisorQueue = {
  entries: VisorEntry[];
  start: VisorEntry | null;
  fellBack: boolean;
  pending: boolean;
};

type Clip = HubModel['clips'][number];
type DemoEntries = { entries: VisorEntry[]; pending: boolean };

const EVERY_SHORT: VisorFilter = { format: LIBRARY_FILTER.all, stage: LIBRARY_STAGE.all };
const NO_LOTES: ReadonlyMap<string, readonly LoteShort[]> = new Map();

type EntryParts = Pick<VisorEntry, 'title' | 'mark' | 'source' | 'actions'>;

function entryOf(item: MediaPlaybackItem | null, parts: (item: MediaPlaybackItem) => EntryParts): VisorEntry[] {
  return item === null || item.format !== '9:16' ? [] : [{ id: item.id, item, ...parts(item), rendering: false }];
}

function heldWhileRendering(clip: Clip, shown: readonly VisorEntry[]): VisorEntry[] {
  if (!isWorking(clip.state)) return [];
  return shown
    .filter((entry) => entry.source.kind === 'demo' && entry.source.video.id === clip.video.id)
    .map((entry) => ({ ...entry, actions: entry.actions.filter(changesNothing), rendering: true }));
}

function changesNothing(action: VisorAction): boolean {
  return action.kind === VISOR_ACTION.download || action.kind === VISOR_ACTION.delete;
}

function isLote(video: Video): boolean {
  return video.editConfig?.separateShorts === true;
}

function mapName(video: Video): string {
  return video.map.trim() === '' || video.map === 'Unknown' ? '' : prettyMapName(video.map);
}

function playerOf(clip: Clip): string {
  return (clip.match?.player ?? clip.video.targetName ?? '').trim();
}

function loteEntries(clip: Clip, shorts: readonly LoteShort[]): VisorEntry[] {
  const map = mapName(clip.video);
  const lote = map === '' ? 'Lote' : `Lote de ${map}`;
  return shorts.flatMap((short, at) => {
    const position = at + 1;
    const title = [`Short ${position}`, map, playerOf(clip)].filter((part) => part !== '').join(' · ');
    const loteShort: LoteShort = { name: short.name, url: short.url };
    const item = demoPlaybackItem({ ...clip.video, id: `${clip.video.id}/${short.name}`, title, downloadUrl: short.url, thumbnailUrl: undefined });
    return entryOf(item, () => ({
      title,
      mark: `${lote} · ${position} de ${shorts.length}`,
      source: { kind: 'demo', video: clip.video, loteShort },
      actions: demoShortActions(clip, loteShort),
    }));
  });
}

function cardEntry(clip: Clip, actions: VisorAction[]): VisorEntry[] {
  const title = videoDisplayTitle(clip.video, clip.match?.player ?? clip.video.targetName);
  return entryOf(demoPlaybackItem(clip.video), () => ({ title, mark: null, source: { kind: 'demo', video: clip.video }, actions }));
}

function demoEntriesOf(clip: Clip, context: { shown: readonly VisorEntry[]; lotes: ReadonlyMap<string, readonly LoteShort[]> }): DemoEntries {
  if (videoKind(clip.video) !== OUTPUT_TYPE.short) return { entries: [], pending: false };
  if (clip.state !== OUTPUT_STATE.ready) return { entries: heldWhileRendering(clip, context.shown), pending: false };
  if (!isLote(clip.video)) return { entries: cardEntry(clip, demoShortActions(clip)), pending: false };
  const shorts = context.lotes.get(clip.video.id);
  return shorts === undefined ? { entries: cardEntry(clip, []), pending: true } : { entries: loteEntries(clip, shorts), pending: false };
}

export function visorEntryId(clip: Clip): string | null {
  return demoEntriesOf(clip, { shown: [], lotes: NO_LOTES }).entries[0]?.id ?? null;
}

function streamEntries(streams: readonly StreamJob[]): VisorEntry[] {
  return streams.flatMap((job) =>
    (job.rendered_outputs ?? []).flatMap((output) => {
      const item = streamPlaybackItem(job, output);
      return entryOf(item, (playable) => ({
        title: playable.title,
        mark: null,
        source: { kind: 'stream', clipId: output.clip_id },
        actions: streamShortActions(playable),
      }));
    }),
  );
}

function requestedEntry(entries: readonly VisorEntry[], requestedId: string): VisorEntry | undefined {
  return entries.find((entry) => entry.id === requestedId)
    ?? entries.find((entry) => entry.source.kind === 'demo' && entry.source.loteShort !== undefined && `demo:${entry.source.video.id}` === requestedId);
}

export function buildVisorQueue({ clips, streams, filter, requestedId, shown = [], lotes = NO_LOTES }: VisorQueueInput): VisorQueue {
  const { format, stage } = filter ?? EVERY_SHORT;
  if (stage === LIBRARY_STAGE.working) return { entries: [], start: null, fellBack: false, pending: false };
  const demo = matchesFormat(OUTPUT_TYPE.short, format) ? clips.map((clip) => demoEntriesOf(clip, { shown, lotes })) : [];
  const pending = demo.some((part) => part.pending);
  const stream = matchesFormat(LIBRARY_FILTER.stream, format) ? streamEntries(streams) : [];
  const entries = [...demo.flatMap((part) => part.entries), ...stream];
  const requested = requestedId === null ? undefined : requestedEntry(entries, requestedId);
  if (requestedId !== null && requested === undefined && pending) return { entries, start: null, fellBack: false, pending };
  const start = requested ?? entries[0] ?? null;
  return { entries, start, fellBack: requestedId !== null && requested === undefined && start !== null, pending };
}

function entryKey(entry: VisorEntry): string {
  const actions = entry.actions.map((action) => action.label);
  return [entry.id, entry.item.playbackUrl, entry.rendering, entry.title, entry.mark ?? '', ...actions].join('\n');
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
