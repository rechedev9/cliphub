import { OUTPUT_STATE, type HubModel } from './hub.ts';
import { RESULTS_ORIGIN, type ResultsOrigin } from './routes.ts';

/** Read model of Resultados: what each origin holds and which sections a filter shows. */

export type ResultsCounts = { all: number; local: number; cloud: number };

export function resultsCounts(input: { local: number; cloud: number }): ResultsCounts {
  return { all: input.local + input.cloud, local: input.local, cloud: input.cloud };
}

export type ResultsSections = { local: boolean; cloud: boolean };

/** No filter shows both origins, but never an empty one; a filter shows its origin even when empty. */
export function resultsSections(origin: ResultsOrigin | null, counts: ResultsCounts): ResultsSections {
  if (origin === RESULTS_ORIGIN.local) return { local: true, cloud: false };
  if (origin === RESULTS_ORIGIN.cloud) return { local: false, cloud: true };
  return { local: counts.local > 0, cloud: counts.cloud > 0 };
}

export type OriginActivity = { active: boolean; newest: number };

/** Section order with no filter: work in flight first, then the newest video, so a new one lands on top. */
export function resultsOrder(input: { local: OriginActivity; cloud: OriginActivity }): ResultsOrigin[] {
  const { local, cloud } = input;
  const cloudFirst = local.active === cloud.active ? cloud.newest > local.newest : cloud.active;
  return cloudFirst ? [RESULTS_ORIGIN.cloud, RESULTS_ORIGIN.local] : [RESULTS_ORIGIN.local, RESULTS_ORIGIN.cloud];
}

export function queuedOutputCount(model: Pick<HubModel, 'clips'>): number {
  return model.clips.filter((clip) => clip.state === OUTPUT_STATE.queue).length;
}

/** Work in flight in one line: running work first, the backlog only when nothing runs. */
export function resultsActivityLabel(input: { running: number; queued: number }): string {
  const { running, queued } = input;
  if (running > 0) return `${running} ${running === 1 ? 'vídeo en marcha' : 'vídeos en marcha'}`;
  return queued > 0 ? `${queued} en cola` : 'Nada en marcha';
}
