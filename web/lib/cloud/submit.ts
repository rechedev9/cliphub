import { buildEditRequest, type EditRequestBody } from '../api/edit-request.ts';
import { buildMusicRequest } from '../api/real.ts';
import type { EditConfig } from '../api/types.ts';
import { FULL_DEMO_CAPTURE_VARIANT, fullDemoPlanEdit, type FullDemoOptions, type FullDemoSnapshot } from '../full-demo-plan.ts';
import { usesOwnFullDemoFiles } from '../produce/full-demo-requirements.ts';
import { CLOUD_OWN_FILES_BLOCKER } from './view.ts';

const TITLE_MAX = 120;

export type CloudShortInput = {
  /** The local match (orchestrator job) the Short is cut from. */
  matchId: string;
  /** Kill plan segment ids, in plan order. */
  segmentIds: readonly string[];
  /** What the selection is called, e.g. "3 jugadas · Rondas 3, 7". */
  selectionLabel: string;
  preset: string;
  presetLabel: string;
  songId: string | null;
  /** Music gain in (0,1]; undefined keeps full volume. */
  musicVolume?: number;
  /** Game-audio gain in [0,1] when music is mixed. */
  gameVolume?: number;
  editConfig: EditConfig;
};

/** Body of `POST /api/cloud/jobs`. `preset`, `music`, `edit` and `segment_ids` are the local `/generate` body. */
export type CloudSubmitBody = {
  job_id: string;
  kind: 'short' | 'full_demo';
  title: string;
  preset: string;
  music: ReturnType<typeof buildMusicRequest>;
  edit: EditRequestBody;
  segment_ids: string[];
};

/** Builds the cloud submit for a Short with the same request parts the local capture sends. */
export function buildCloudShortSubmit(input: CloudShortInput): CloudSubmitBody {
  const hasMusic = input.songId !== null && input.songId !== '';
  const suffix = hasMusic ? `${input.presetLabel} + Music` : input.presetLabel;
  return {
    job_id: input.matchId,
    kind: 'short',
    title: `${input.selectionLabel} - ${suffix}`.slice(0, TITLE_MAX),
    preset: input.preset,
    music: buildMusicRequest({
      mode: hasMusic ? 'music' : 'clean',
      songId: input.songId ?? undefined,
      musicVolume: hasMusic ? input.musicVolume : undefined,
      gameVolume: hasMusic ? input.gameVolume : undefined,
    }),
    edit: buildEditRequest(input.editConfig),
    segment_ids: [...input.segmentIds],
  };
}

export function fullDemoCloudBlocker(options: FullDemoOptions): string | null {
  return usesOwnFullDemoFiles(options) ? CLOUD_OWN_FILES_BLOCKER : null;
}

/** Builds the cloud submit for a long video from its approved plan; Studio's cloud client turns the plan into rounds. */
export function buildCloudFullDemoSubmit(input: { matchId: string; title: string; snapshot: FullDemoSnapshot }): CloudSubmitBody {
  return {
    job_id: input.matchId,
    kind: 'full_demo',
    title: input.title.slice(0, TITLE_MAX),
    preset: FULL_DEMO_CAPTURE_VARIANT,
    music: buildMusicRequest({ mode: 'clean' }),
    edit: buildEditRequest(fullDemoPlanEdit(input.snapshot)),
    segment_ids: [],
  };
}
