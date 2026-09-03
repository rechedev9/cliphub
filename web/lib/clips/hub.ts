import type { Match, Video } from '../api/types.ts';
import type { StreamJob } from '../api/streams.ts';
import { captureProgressPercent } from '../capture-progress.ts';
import { matchPlanReady } from '../match-plays-empty.ts';
import { isLandscapeRecap } from '../reel-brief.ts';

/**
 * The 01 hub's read model. Everything the user produced hangs off the parsed
 * demo (the partida), so reels are regrouped by `jobId` and each becomes an
 * `output` of its match. Pure and unit-tested: pages only render this.
 */

/** Short = 9:16 highlights compile. Full = 16:9 landscape POV recap. */
export const OUTPUT_TYPE = { short: 'short', full: 'full' } as const;
export type OutputType = (typeof OUTPUT_TYPE)[keyof typeof OUTPUT_TYPE];

/** Pipeline vocabulary from the handoff: En cola → REC → Edición → Listo, or Falló. */
export const OUTPUT_STATE = {
  queue: 'queue',
  rec: 'rec',
  render: 'render',
  ready: 'ready',
  failed: 'failed',
} as const;
export type OutputState = (typeof OUTPUT_STATE)[keyof typeof OUTPUT_STATE];

export type MatchOutput = {
  id: string;
  type: OutputType;
  state: OutputState;
  title: string;
  /** 0-100 while REC/render report progress; null when the stage has none. */
  percent: number | null;
  /** Segment/round counter while capturing; null elsewhere. */
  rounds: { done: number; total: number } | null;
  /** QA review pending: still `ready` for the row, but the card says so. */
  reviewRequired: boolean;
  video: Video;
};

export type HubMatch = {
  match: Match;
  parsing: boolean;
  shorts: MatchOutput[];
  fulls: MatchOutput[];
};

export type HubModel = {
  rows: HubMatch[];
  /** Reels whose `jobId` matches no listed partida (deleted demo, mock seed). */
  orphans: MatchOutput[];
  /** Every output, for the Clips lens. */
  clips: Array<MatchOutput & { match: Match | null }>;
};

export function outputType(video: Video): OutputType {
  return video.editConfig !== undefined && isLandscapeRecap(video.editConfig)
    ? OUTPUT_TYPE.full
    : OUTPUT_TYPE.short;
}

export function outputState(status: Video['status']): OutputState {
  switch (status) {
    case 'queued':
      return OUTPUT_STATE.queue;
    case 'recording':
      return OUTPUT_STATE.rec;
    case 'composing':
      return OUTPUT_STATE.render;
    case 'failed':
      return OUTPUT_STATE.failed;
    default:
      return OUTPUT_STATE.ready;
  }
}

export function isWorking(state: OutputState): boolean {
  return state === OUTPUT_STATE.rec || state === OUTPUT_STATE.render || state === OUTPUT_STATE.queue;
}

export function toOutput(video: Video): MatchOutput {
  const state = outputState(video.status);
  const progress = video.captureProgress;
  const hasProgress =
    (state === OUTPUT_STATE.rec || state === OUTPUT_STATE.render) && progress !== undefined && progress.total > 0;
  return {
    id: video.id,
    type: outputType(video),
    state,
    title: video.title,
    percent: hasProgress ? captureProgressPercent(progress) : null,
    rounds: state === OUTPUT_STATE.rec && hasProgress ? { done: progress.done, total: progress.total } : null,
    reviewRequired: video.status === 'review_required',
    video,
  };
}

/** Newest first: the row shows the latest attempt at the top. */
function byNewest(a: MatchOutput, b: MatchOutput): number {
  return b.video.createdAt - a.video.createdAt;
}

export function buildHubModel(matches: readonly Match[], videos: readonly Video[]): HubModel {
  const byJob = new Map<string, MatchOutput[]>();
  const orphans: MatchOutput[] = [];
  const known = new Set(matches.map((match) => match.id));
  for (const video of videos) {
    const output = toOutput(video);
    if (video.jobId !== undefined && known.has(video.jobId)) {
      const list = byJob.get(video.jobId) ?? [];
      list.push(output);
      byJob.set(video.jobId, list);
    } else {
      orphans.push(output);
    }
  }
  const rows: HubMatch[] = matches.map((match) => {
    const outputs = (byJob.get(match.id) ?? []).sort(byNewest);
    return {
      match,
      parsing: !matchPlanReady(match.status),
      shorts: outputs.filter((output) => output.type === OUTPUT_TYPE.short),
      fulls: outputs.filter((output) => output.type === OUTPUT_TYPE.full),
    };
  });
  const matchById = new Map(matches.map((match) => [match.id, match]));
  const clips = videos
    .map((video) => {
      const output = toOutput(video);
      const match = video.jobId !== undefined ? (matchById.get(video.jobId) ?? null) : null;
      return { ...output, match };
    })
    .sort(byNewest);
  return { rows, orphans: orphans.sort(byNewest), clips };
}

/** Clips lens filters, with the label the chip shows. */
export const CLIP_FILTER = {
  all: 'all',
  short: 'short',
  full: 'full',
  ready: 'ready',
  working: 'working',
} as const;
export type ClipFilter = (typeof CLIP_FILTER)[keyof typeof CLIP_FILTER];

export function isClipFilter(value: string | null): value is ClipFilter {
  return Object.values<string>(CLIP_FILTER).includes(value ?? '');
}

export function matchesClipFilter(output: MatchOutput, filter: ClipFilter): boolean {
  switch (filter) {
    case CLIP_FILTER.all:
      return true;
    case CLIP_FILTER.short:
      return output.type === OUTPUT_TYPE.short;
    case CLIP_FILTER.full:
      return output.type === OUTPUT_TYPE.full;
    case CLIP_FILTER.ready:
      return output.state === OUTPUT_STATE.ready;
    case CLIP_FILTER.working:
      return output.state !== OUTPUT_STATE.ready;
  }
}

export function clipFilterCounts(outputs: readonly MatchOutput[]): Record<ClipFilter, number> {
  const counts: Record<ClipFilter, number> = { all: 0, short: 0, full: 0, ready: 0, working: 0 };
  for (const output of outputs) {
    for (const filter of Object.values(CLIP_FILTER)) {
      if (matchesClipFilter(output, filter)) counts[filter] += 1;
    }
  }
  return counts;
}

/** Card grid density. S hides the action row. */
export const CLIP_SIZE = { s: 'S', m: 'M', l: 'L' } as const;
export type ClipSize = (typeof CLIP_SIZE)[keyof typeof CLIP_SIZE];

/** True while any output is on CS2 (REC). The next Full POV must queue behind it. */
export function recBusy(model: Pick<HubModel, 'rows' | 'orphans'>): boolean {
  const all = [...model.rows.flatMap((row) => [...row.shorts, ...row.fulls]), ...model.orphans];
  return all.some((output) => output.state === OUTPUT_STATE.rec);
}

/** Active work count for the hub summary line: parsing partidas plus non-terminal outputs. */
export function activeJobCount(model: HubModel, streams: readonly StreamJob[] = []): number {
  const outputs = [...model.rows.flatMap((row) => [...row.shorts, ...row.fulls]), ...model.orphans];
  const parsing = model.rows.filter((row) => row.parsing).length;
  const working = outputs.filter((output) => isWorking(output.state)).length;
  const streaming = streams.filter((job) => job.status === 'acquiring' || job.status === 'rendering').length;
  return parsing + working + streaming;
}

/** Full POV chip label for a collapsed row. */
export function fullChipLabel(fulls: readonly MatchOutput[]): string {
  const latest = fulls[0];
  if (latest === undefined) return 'Full POV · —';
  switch (latest.state) {
    case OUTPUT_STATE.ready:
      return 'Full POV · listo';
    case OUTPUT_STATE.rec:
      return 'Full POV · REC';
    case OUTPUT_STATE.render:
      return 'Full POV · render';
    case OUTPUT_STATE.queue:
      return 'Full POV · en cola';
    case OUTPUT_STATE.failed:
      return 'Full POV · falló';
  }
}

export function pluralClips(count: number): string {
  return `${count} ${count === 1 ? 'clip' : 'clips'}`;
}

/** Tone vocabulary for an output's tag; mirrors `StatusTagTone` by name. */
export const OUTPUT_TONE = {
  ready: 'success',
  rec: 'stream',
  render: 'primary',
  queue: 'neutral',
  failed: 'danger',
} as const satisfies Record<OutputState, string>;
export type OutputTone = (typeof OUTPUT_TONE)[OutputState];

/** A ready output whose QA left warnings: the render exists, a human must sign it off. */
export const REVIEW_TAG_LABEL = 'REVISIÓN QA';

/** The uppercase HUD tag an output item shows: LISTO, RENDER 41%, REC R3/20, EN COLA, FALLÓ. */
export function outputTagLabel(
  output: Pick<MatchOutput, 'state' | 'percent' | 'rounds'> & Partial<Pick<MatchOutput, 'reviewRequired'>>,
): string {
  switch (output.state) {
    case OUTPUT_STATE.ready:
      return output.reviewRequired === true ? REVIEW_TAG_LABEL : 'LISTO';
    case OUTPUT_STATE.render:
      return output.percent === null ? 'RENDER' : `RENDER ${output.percent}%`;
    case OUTPUT_STATE.rec:
      return output.rounds === null ? 'REC' : `REC R${output.rounds.done}/${output.rounds.total}`;
    case OUTPUT_STATE.queue:
      return 'EN COLA';
    case OUTPUT_STATE.failed:
      return 'FALLÓ';
  }
}

/** Shorts chip tone for a collapsed row: working beats existing beats none. */
export function shortsChipTone(shorts: readonly MatchOutput[]): OutputTone {
  if (shorts.some((output) => isWorking(output.state))) return OUTPUT_TONE.render;
  return shorts.length > 0 ? OUTPUT_TONE.ready : OUTPUT_TONE.queue;
}

/** Round count from an `x-y` score; null when either side is not a number. */
export function roundsFromScore(score: string): number | null {
  const [left, right] = score.split('-', 2);
  const ours = Number.parseInt(left ?? '', 10);
  const theirs = Number.parseInt(right ?? '', 10);
  return Number.isNaN(ours) || Number.isNaN(theirs) ? null : ours + theirs;
}

export type HubTransitions = {
  /** Rows that were parsing on the previous tick and are ready now. */
  parsed: HubMatch[];
  /** Outputs that were not ready on the previous tick and are ready now. */
  ready: HubModel['clips'];
};

/** What changed between two polls; the page turns these into toasts. */
export function hubTransitions(prev: HubModel, next: HubModel): HubTransitions {
  const wasParsing = new Set(prev.rows.filter((row) => row.parsing).map((row) => row.match.id));
  const prevState = new Map(prev.clips.map((clip) => [clip.id, clip.state]));
  return {
    parsed: next.rows.filter((row) => !row.parsing && wasParsing.has(row.match.id)),
    ready: next.clips.filter((clip) => {
      const before = prevState.get(clip.id);
      return clip.state === OUTPUT_STATE.ready && before !== undefined && before !== OUTPUT_STATE.ready;
    }),
  };
}
