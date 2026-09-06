import type { Play } from '../api/types.ts';

export function playClock(seconds: number): string {
  const tenths = Math.round(seconds * 10);
  return `${Math.floor(tenths / 600)}:${String(Math.floor(tenths / 10) % 60).padStart(2, '0')}.${tenths % 10}`;
}

export function playSourceRange(play: Play): string | null {
  if (play.startSeconds === undefined || play.endSeconds === undefined) return null;
  return `${playClock(play.startSeconds)} – ${playClock(play.endSeconds)}`;
}

export function playDescription(play: Play): string {
  const parts = [`${play.kills} ${play.kills === 1 ? 'baja' : 'bajas'}`];
  if (play.weapon) parts.push(play.weapon);
  if (play.headshots) parts.push(`${play.headshots} a la cabeza`);
  return parts.join(' · ');
}
