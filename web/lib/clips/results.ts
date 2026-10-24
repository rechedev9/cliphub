import { OUTPUT_STATE, type HubModel } from './hub.ts';

export function queuedOutputCount(model: Pick<HubModel, 'clips'>): number {
  return model.clips.filter((clip) => clip.state === OUTPUT_STATE.queue).length;
}

/** Work in flight in one line: running work first, the backlog only when nothing runs. */
export function resultsActivityLabel(input: { running: number; queued: number }): string {
  const { running, queued } = input;
  if (running > 0) return `${running} ${running === 1 ? 'vídeo en marcha' : 'vídeos en marcha'}`;
  return queued > 0 ? `${queued} en cola` : 'Nada en marcha';
}
