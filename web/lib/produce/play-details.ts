import type { Play } from '../api/types.ts';

export function playClock(seconds: number): string {
  const tenths = Math.round(seconds * 10);
  return `${Math.floor(tenths / 600)}:${String(Math.floor(tenths / 10) % 60).padStart(2, '0')}.${tenths % 10}`;
}

export function playSourceRange(play: Play): string | null {
  if (play.startSeconds === undefined || play.endSeconds === undefined) return null;
  return `${playClock(play.startSeconds)} a ${playClock(play.endSeconds)}`;
}

export function playDescription(play: Play): string {
  if (play.death) {
    const { killer, weapon, headshot } = play.death;
    return [killer ? `Muere ante ${killer}` : 'Muere', weapon, headshot ? 'HS' : undefined].filter(Boolean).join(' · ');
  }
  const parts = [`${play.kills} ${play.kills === 1 ? 'kill' : 'kills'}`];
  if (play.weapon) parts.push(play.weapon);
  if (play.headshots) parts.push(`${play.headshots} HS`);
  return [...parts, ...playHighlights(play)].join(' · ');
}

/** Server reason codes worth naming on the card, in this order. */
const HIGHLIGHT_LABELS: ReadonlyArray<readonly [string, string]> = [
  ['fast_ace', 'Ace en 45 s'],
  ['fast_quad', 'Cuádruple en 15 s'],
  ['fast_triple', 'Triple en 10 s'],
  ['fast_double', 'Doble rápida'],
  ['noscope', 'Noscope'],
  ['through_smoke', 'A través de humo'],
  ['attacker_blind', 'Cegado'],
];

/** Clutch with its result, named kill types and the win chance swing, for plays the server scored. */
export function playHighlights(play: Play): string[] {
  const out: string[] = [];
  if (play.clutchVs && play.clutchVs > 0) {
    let result = '';
    if (play.clutchWon !== undefined) result = play.clutchWon ? ' ganado' : ' perdido';
    out.push(`Clutch 1v${play.clutchVs}${result}`);
  }
  for (const [reason, label] of HIGHLIGHT_LABELS) {
    if (play.reasons?.includes(reason)) out.push(label);
  }
  const swing = Math.round((play.winProbSwing ?? 0) * 100);
  if (swing !== 0) out.push(`${swing > 0 ? '+' : ''}${swing} % prob. de ronda`);
  return out;
}
