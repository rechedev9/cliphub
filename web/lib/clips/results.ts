import type { StreamJob } from '../api/streams.ts';
import type { Video } from '../api/types.ts';
import { streamRenderProgressView } from '../streams/render-progress.ts';
import { streamTitle } from '../streams/title.ts';
import { isHighlightsVideo, OUTPUT_STATE, OUTPUT_TYPE, type HubModel, type OutputState } from './hub.ts';

/** The format chips of Mis vídeos: one per format colour, plus Todos. */
export const LIBRARY_FILTER = {
  all: 'all',
  short: 'short',
  full: 'full',
  summary: 'summary',
  stream: 'stream',
} as const;
export type LibraryFilter = (typeof LIBRARY_FILTER)[keyof typeof LIBRARY_FILTER];
export type VideoKind = Exclude<LibraryFilter, 'all'>;

/** The state chips: every video, only the ready grid or only the work in progress. */
export const LIBRARY_STAGE = { all: 'all', ready: 'ready', working: 'working' } as const;
export type LibraryStage = (typeof LIBRARY_STAGE)[keyof typeof LIBRARY_STAGE];

/** Where a video sits on the page. A failed one is done, not "En marcha". */
export type ItemStage = 'ready' | 'working' | 'failed';

export type LibraryItem = { kind: VideoKind; stage: ItemStage };

/** A demo video's format: a Short, a whole match or a summary of chosen plays. */
export function videoKind(video: Video): Exclude<VideoKind, 'stream'> {
  if (video.editConfig?.format !== 'landscape-16x9') return OUTPUT_TYPE.short;
  return isHighlightsVideo(video) ? LIBRARY_FILTER.summary : OUTPUT_TYPE.full;
}

export function itemStage(state: OutputState): ItemStage {
  if (state === OUTPUT_STATE.ready) return 'ready';
  return state === OUTPUT_STATE.failed ? 'failed' : 'working';
}

export function matchesFormat(kind: VideoKind, filter: LibraryFilter): boolean {
  return filter === LIBRARY_FILTER.all || filter === kind;
}

export function matchesStage(stage: ItemStage, filter: LibraryStage): boolean {
  return filter === LIBRARY_STAGE.all || filter === stage;
}

export function libraryFilterCounts(kinds: readonly VideoKind[]): Record<LibraryFilter, number> {
  const counts: Record<LibraryFilter, number> = { all: kinds.length, short: 0, full: 0, summary: 0, stream: 0 };
  for (const kind of kinds) counts[kind] += 1;
  return counts;
}

/** Each chip group counts what the other group lets through, so a chip's number is what it would show. */
export function libraryChipCounts(
  items: readonly LibraryItem[],
  selected: { format: LibraryFilter; stage: LibraryStage },
): { format: Record<LibraryFilter, number>; stage: Record<LibraryStage, number> } {
  const format = libraryFilterCounts(items.filter((item) => matchesStage(item.stage, selected.stage)).map((item) => item.kind));
  const inFormat = items.filter((item) => matchesFormat(item.kind, selected.format));
  const stage: Record<LibraryStage, number> = {
    all: inFormat.length,
    ready: inFormat.filter((item) => item.stage === 'ready').length,
    working: inFormat.filter((item) => item.stage === 'working').length,
  };
  return { format, stage };
}

export function outputStateCount(model: Pick<HubModel, 'clips'>, state: OutputState): number {
  return model.clips.filter((clip) => clip.state === state).length;
}

/** A stream project that is downloading its source or exporting its clips right now. */
export type StreamActivity = {
  jobId: string;
  title: string;
  stage: 'download' | 'export';
  /** 0-100, or null while the stage reports no progress. */
  percent: number | null;
  /** "Short 2 de 4" while exporting several clips. */
  clipLabel: string | null;
};

export function streamActivity(streams: readonly StreamJob[]): StreamActivity[] {
  return streams.flatMap((job): StreamActivity[] => {
    if (job.status === 'acquiring') {
      const percent = job.download_progress?.percent;
      return [{
        jobId: job.id,
        title: streamTitle(job),
        stage: 'download',
        percent: percent === undefined || !Number.isFinite(percent) ? null : Math.min(100, Math.max(0, Math.round(percent))),
        clipLabel: null,
      }];
    }
    if (job.status !== 'rendering') return [];
    const view = streamRenderProgressView(job.render_progress);
    return [{
      jobId: job.id,
      title: streamTitle(job),
      stage: 'export',
      percent: view?.percent ?? null,
      clipLabel: view !== null && view.clips > 1 ? view.clipLabel : null,
    }];
  });
}

export type LibrarySummaryCounts = {
  ready: number;
  recording: number;
  /** Renders: the running work in the Now section that is not a capture. */
  working: number;
  queued: number;
  failed: number;
  /** Demos still being analysed: their videos can only start once it ends. */
  analysing: number;
  streamsDownloading: number;
  streamsExporting: number;
};

function plural(count: number, one: string, many: string): string {
  return `${count} ${count === 1 ? one : many}`;
}

/** The one line under the Mis vídeos title, e.g. "6 listos, 1 grabándose ahora, 1 demo analizándose." */
export function librarySummary(counts: LibrarySummaryCounts): string {
  const { ready, recording, working, queued, failed, analysing, streamsDownloading, streamsExporting } = counts;
  const parts: string[] = [];
  if (ready > 0) parts.push(plural(ready, 'listo', 'listos'));
  if (recording > 0) parts.push(`${recording} grabándose ahora`);
  if (working > 0) parts.push(`${working} en marcha`);
  if (analysing > 0) parts.push(`${plural(analysing, 'demo', 'demos')} analizándose`);
  if (streamsDownloading > 0) parts.push(`${plural(streamsDownloading, 'proyecto de stream', 'proyectos de stream')} descargándose`);
  if (streamsExporting > 0) parts.push(`${plural(streamsExporting, 'proyecto de stream', 'proyectos de stream')} exportándose`);
  if (queued > 0) parts.push(`${queued} en cola`);
  if (failed > 0) parts.push(`${failed} ${failed === 1 ? 'falló' : 'fallaron'}`);
  return parts.length === 0 ? 'Nada en marcha.' : `${parts.join(', ')}.`;
}
