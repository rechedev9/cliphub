import type { FullDemoDocument, FullDemoOptions } from '../full-demo-plan.ts';
import { fullDemoWarningText } from './full-demo-notices.ts';

export type CameraMoment = { id: string; round: number; enemy: string; time: string };

function clock(seconds: number): string {
  const whole = Math.max(0, Math.round(seconds));
  return `${Math.floor(whole / 60)}:${String(whole % 60).padStart(2, '0')}`;
}

export function fullDemoCameraMoments(document: FullDemoDocument): CameraMoment[] {
  return (document.second_camera?.windows ?? []).map((window) => {
    const round = document.rounds.find((entry) => entry.round_id === window.round_id);
    const seconds = round ? (window.kill_tick - round.live_start_tick) / document.clock.tick_rate : 0;
    return { id: window.id, round: window.round, enemy: window.enemy_name || 'Rival', time: clock(seconds) };
  });
}

function roundsKey(rounds: readonly number[] | undefined): string {
  return [...(rounds ?? [])].sort((a, b) => a - b).join(',');
}

export function fullDemoCameraListed(document: FullDemoDocument | null, options: FullDemoOptions): FullDemoDocument | null {
  const planned = document?.options.second_camera;
  if (!document || planned?.enabled !== true || planned.max_windows !== options.second_camera?.max_windows) return null;
  return roundsKey(document.options.rounds) === roundsKey(options.rounds) ? document : null;
}

export function cameraIdRound(id: string): number | null {
  const found = /^round-(\d+)-camera-\d+$/.exec(id)?.[1];
  return found === undefined ? null : Number(found);
}

export function cameraOmittedText(id: string): string {
  const round = cameraIdRound(id);
  return `${round === null ? 'La segunda cámara' : `La segunda cámara de la ronda ${round}`} no se pudo grabar y el vídeo sale sin ella.`;
}

export function cameraEmptyReason(document: FullDemoDocument): string {
  const notice = (document.warnings ?? []).find((entry) => entry.code.startsWith('second_camera_'));
  return (notice ? fullDemoWarningText(notice) : '') || 'Todos los momentos elegidos se han quitado.';
}
